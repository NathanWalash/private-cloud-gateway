// Package docker manages app containers via Docker's REST API over the Unix socket.
// Using the REST API directly avoids the Docker SDK module split in v28+.
package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/NathanWalash/private-cloud-gateway/apps/core/internal/blueprint"
)

const (
	socketPath     = "/var/run/docker.sock"
	apiVersion     = "v1.44" // Docker Desktop requires 1.44+; 1.41 is too old
	privateNetwork = "cloud_core_private"
)

// Manager manages Docker containers via the REST API.
type Manager struct {
	client *http.Client
}

// New creates a Manager that dials the host Docker socket.
func New() (*Manager, error) {
	c := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
			},
		},
		Timeout: 60 * time.Second,
	}
	m := &Manager{client: c}
	// Verify connectivity.
	if err := m.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("docker socket unavailable: %w", err)
	}
	return m, nil
}

// Close is a no-op (HTTP client has no persistent connections to close).
func (m *Manager) Close() error { return nil }

// Ping checks Docker daemon connectivity.
func (m *Manager) Ping(ctx context.Context) error {
	resp, err := m.do(ctx, "GET", "/_ping", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return checkStatus(resp, http.StatusOK)
}

// hostConfig mirrors the subset of Docker's HostConfig we set.
type hostConfig struct {
	Binds          []string       `json:"Binds,omitempty"`
	RestartPolicy  map[string]any `json:"RestartPolicy"`
	Memory         int64          `json:"Memory,omitempty"`
	NetworkMode    string         `json:"NetworkMode"`
	SecurityOpt    []string       `json:"SecurityOpt,omitempty"`
	CapDrop        []string       `json:"CapDrop,omitempty"`
	CapAdd         []string       `json:"CapAdd,omitempty"`
	ReadonlyRootfs bool           `json:"ReadonlyRootfs,omitempty"`
}

type endpointSettings struct {
	Aliases []string `json:"Aliases,omitempty"`
}

type networkingConfig struct {
	EndpointsConfig map[string]*endpointSettings `json:"EndpointsConfig"`
}

type createBody struct {
	Image            string            `json:"Image"`
	Env              []string          `json:"Env,omitempty"`
	Labels           map[string]string `json:"Labels,omitempty"`
	HostConfig       hostConfig        `json:"HostConfig"`
	NetworkingConfig networkingConfig  `json:"NetworkingConfig"`
}

// appNetwork is the per-app bridge network name for a multi-container app.
func appNetwork(appID string) string { return "pcg-net-" + appID }

// serviceContainerName is the container name for a sidecar service.
func serviceContainerName(appID, service string) string {
	return "pcg-" + appID + "-" + service
}

// deriveAppID recovers the app id from a main container name ("pcg-<id>").
func deriveAppID(containerName string) string {
	return strings.TrimPrefix(containerName, "pcg-")
}

// netAttach is a network to join, with optional DNS aliases. The first attach
// is the container's primary network (NetworkMode).
type netAttach struct {
	name    string
	aliases []string
}

type containerSpec struct {
	image   string
	env     []string
	volumes []string
	memory  string
	sec     blueprint.Security
	labels  map[string]string
	nets    []netAttach
}

func makeCreateBody(spec containerSpec) createBody {
	var securityOpt []string
	if !spec.sec.AllowPrivilegeEscalation {
		securityOpt = append(securityOpt, "no-new-privileges:true")
	}
	endpoints := make(map[string]*endpointSettings, len(spec.nets))
	primary := ""
	for i, n := range spec.nets {
		endpoints[n.name] = &endpointSettings{Aliases: n.aliases}
		if i == 0 {
			primary = n.name
		}
	}
	return createBody{
		Image:  spec.image,
		Env:    spec.env,
		Labels: spec.labels,
		HostConfig: hostConfig{
			Binds:          spec.volumes,
			RestartPolicy:  map[string]any{"Name": "unless-stopped"},
			Memory:         parseMemoryLimit(spec.memory),
			NetworkMode:    primary,
			SecurityOpt:    securityOpt,
			CapDrop:        spec.sec.CapDrop,
			CapAdd:         spec.sec.CapAdd,
			ReadonlyRootfs: spec.sec.ReadOnlyRootfs,
		},
		NetworkingConfig: networkingConfig{EndpointsConfig: endpoints},
	}
}

// buildCreateBody builds the main app container's create body. With no sidecar
// services it joins only the shared private network (unchanged single-container
// behaviour). With services it joins the per-app network first (so it resolves
// sidecars by service name) plus the shared network (so Caddy can reach it).
func buildCreateBody(bp *blueprint.Blueprint) createBody {
	labels := map[string]string{"pcg.app": bp.ID, "pcg.managed": "true", "pcg.role": "app"}
	nets := []netAttach{{name: privateNetwork}}
	if len(bp.Services) > 0 {
		nets = []netAttach{
			{name: appNetwork(bp.ID), aliases: []string{bp.ID}},
			{name: privateNetwork},
		}
	}
	return makeCreateBody(containerSpec{
		image:   bp.Container.Image,
		env:     bp.Container.Environment,
		volumes: bp.Container.Volumes,
		memory:  bp.Resources.MemoryLimit,
		sec:     bp.Container.Security,
		labels:  labels,
		nets:    nets,
	})
}

// buildServiceCreateBody builds a sidecar service container's create body. It
// joins only the per-app network (alias = service name), so it is reachable by
// the app but not by Caddy or other apps.
func buildServiceCreateBody(appID string, s blueprint.Service) createBody {
	return makeCreateBody(containerSpec{
		image:   s.Image,
		env:     s.Environment,
		volumes: s.Volumes,
		memory:  s.MemoryLimit,
		sec:     s.Security,
		labels: map[string]string{
			"pcg.app": appID, "pcg.managed": "true", "pcg.role": "service", "pcg.service": s.Name,
		},
		nets: []netAttach{{name: appNetwork(appID), aliases: []string{s.Name}}},
	})
}

// Install pulls images and creates the container(s) — the app plus any sidecar
// services on a per-app network. It does not start them (the caller starts the
// group via Start).
func (m *Manager) Install(ctx context.Context, bp *blueprint.Blueprint) error {
	// Multi-container: create the per-app network and sidecars first.
	if len(bp.Services) > 0 {
		if err := m.ensureNetwork(ctx, appNetwork(bp.ID), bp.ID); err != nil {
			return fmt.Errorf("create app network: %w", err)
		}
		for _, s := range bp.Services {
			if err := m.pull(ctx, s.Image); err != nil {
				return fmt.Errorf("pull service %s: %w", s.Name, err)
			}
			name := serviceContainerName(bp.ID, s.Name)
			_ = m.removeContainer(ctx, name) // clear any stale sidecar
			if err := m.createContainer(ctx, name, buildServiceCreateBody(bp.ID, s)); err != nil {
				return fmt.Errorf("create service %s: %w", s.Name, err)
			}
			slog.Info("service container created", "name", name)
		}
	}

	if err := m.pull(ctx, bp.Container.Image); err != nil {
		return fmt.Errorf("pull image: %w", err)
	}
	if err := m.createContainer(ctx, bp.ContainerName(), buildCreateBody(bp)); err != nil {
		return fmt.Errorf("create container: %w", err)
	}
	slog.Info("container created", "name", bp.ContainerName())
	return nil
}

// ── internal single-container ops ──────────────────────────────────────────────

func (m *Manager) pull(ctx context.Context, image string) error {
	slog.Info("pulling image", "image", image)
	resp, err := m.do(ctx, "POST", "/images/create?fromImage="+image, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, err = io.Copy(io.Discard, resp.Body)
	return err
}

func (m *Manager) createContainer(ctx context.Context, name string, body createBody) error {
	resp, err := m.do(ctx, "POST", "/containers/create?name="+name, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return checkStatus(resp, http.StatusCreated)
}

func (m *Manager) startContainer(ctx context.Context, name string) error {
	resp, err := m.do(ctx, "POST", "/containers/"+name+"/start", nil)
	if err != nil {
		return fmt.Errorf("start %s: %w", name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotModified {
		return fmt.Errorf("start %s: unexpected status %d", name, resp.StatusCode)
	}
	return nil
}

func (m *Manager) stopContainer(ctx context.Context, name string) error {
	resp, err := m.do(ctx, "POST", "/containers/"+name+"/stop?t=10", nil)
	if err != nil {
		return fmt.Errorf("stop %s: %w", name, err)
	}
	defer resp.Body.Close()
	return nil
}

func (m *Manager) restartContainer(ctx context.Context, name string) error {
	resp, err := m.do(ctx, "POST", "/containers/"+name+"/restart?t=10", nil)
	if err != nil {
		return fmt.Errorf("restart %s: %w", name, err)
	}
	defer resp.Body.Close()
	return nil
}

func (m *Manager) removeContainer(ctx context.Context, name string) error {
	resp, err := m.do(ctx, "DELETE", "/containers/"+name+"?force=true", nil)
	if err != nil {
		return fmt.Errorf("remove %s: %w", name, err)
	}
	defer resp.Body.Close()
	return nil
}

// ── group lifecycle (app + its sidecars) ───────────────────────────────────────

// Start starts the app and all its sidecars (services first, then the app).
func (m *Manager) Start(ctx context.Context, containerName string) error {
	svc, app := m.groupSplit(ctx, deriveAppID(containerName), containerName)
	for _, s := range svc {
		if err := m.startContainer(ctx, s); err != nil {
			return err
		}
	}
	return m.startContainer(ctx, app)
}

// Stop stops the app first, then its sidecars.
func (m *Manager) Stop(ctx context.Context, containerName string) error {
	svc, app := m.groupSplit(ctx, deriveAppID(containerName), containerName)
	_ = m.stopContainer(ctx, app)
	for _, s := range svc {
		_ = m.stopContainer(ctx, s)
	}
	return nil
}

// Restart restarts the app and its sidecars.
func (m *Manager) Restart(ctx context.Context, containerName string) error {
	svc, app := m.groupSplit(ctx, deriveAppID(containerName), containerName)
	for _, s := range svc {
		_ = m.restartContainer(ctx, s)
	}
	return m.restartContainer(ctx, app)
}

// Remove removes the app, its sidecars, their per-app network, and the sidecar
// data volumes (so a reinstall starts clean). The app's own named volumes are
// left in place, matching single-container behaviour.
func (m *Manager) Remove(ctx context.Context, containerName string) error {
	appID := deriveAppID(containerName)
	svc, app := m.groupSplit(ctx, appID, containerName)

	// Collect sidecar volumes before removing their containers.
	var svcVolumes []string
	for _, s := range svc {
		svcVolumes = append(svcVolumes, m.containerVolumes(ctx, s)...)
	}

	_ = m.removeContainer(ctx, app)
	for _, s := range svc {
		_ = m.removeContainer(ctx, s)
	}
	for _, v := range svcVolumes {
		m.removeVolume(ctx, v)
	}
	if len(svc) > 0 {
		m.removeNetwork(ctx, appNetwork(appID))
	}
	return nil
}

// groupSplit returns the sidecar container names and the app container name for
// an app, by querying the pcg.app label. Falls back to the single container when
// nothing is labelled (e.g. a single-container app, or Docker unavailable).
func (m *Manager) groupSplit(ctx context.Context, appID, fallbackApp string) (services []string, app string) {
	filter := fmt.Sprintf(`{"label":["pcg.app=%s"]}`, appID)
	resp, err := m.do(ctx, "GET", "/containers/json?all=1&filters="+url.QueryEscape(filter), nil)
	if err != nil {
		return nil, fallbackApp
	}
	defer resp.Body.Close()
	var cs []struct {
		Names  []string          `json:"Names"`
		Labels map[string]string `json:"Labels"`
	}
	if json.NewDecoder(resp.Body).Decode(&cs) != nil {
		return nil, fallbackApp
	}
	app = fallbackApp
	for _, c := range cs {
		if len(c.Names) == 0 {
			continue
		}
		name := strings.TrimPrefix(c.Names[0], "/")
		if c.Labels["pcg.role"] == "service" {
			services = append(services, name)
		} else {
			app = name
		}
	}
	return services, app
}

// containerVolumes returns the named volumes mounted by a container.
func (m *Manager) containerVolumes(ctx context.Context, name string) []string {
	resp, err := m.do(ctx, "GET", "/containers/"+name+"/json", nil)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	var info struct {
		Mounts []struct {
			Type string `json:"Type"`
			Name string `json:"Name"`
		} `json:"Mounts"`
	}
	if json.NewDecoder(resp.Body).Decode(&info) != nil {
		return nil
	}
	var vols []string
	for _, mt := range info.Mounts {
		if mt.Type == "volume" && mt.Name != "" {
			vols = append(vols, mt.Name)
		}
	}
	return vols
}

func (m *Manager) removeVolume(ctx context.Context, name string) {
	if resp, err := m.do(ctx, "DELETE", "/volumes/"+name, nil); err == nil {
		resp.Body.Close()
	}
}

// ensureNetwork creates the per-app bridge network if it doesn't already exist.
func (m *Manager) ensureNetwork(ctx context.Context, name, appID string) error {
	body := map[string]any{
		"Name":   name,
		"Driver": "bridge",
		"Labels": map[string]string{"pcg.app": appID, "pcg.managed": "true"},
	}
	resp, err := m.do(ctx, "POST", "/networks/create", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	// 201 created, 409 already exists — both fine.
	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusConflict {
		return nil
	}
	return fmt.Errorf("network create %s: status %d", name, resp.StatusCode)
}

func (m *Manager) removeNetwork(ctx context.Context, name string) {
	if resp, err := m.do(ctx, "DELETE", "/networks/"+name, nil); err == nil {
		resp.Body.Close()
	}
}

// Status returns "running", "stopped", or "missing".
func (m *Manager) Status(ctx context.Context, containerName string) string {
	resp, err := m.do(ctx, "GET", "/containers/"+containerName+"/json", nil)
	if err != nil || resp.StatusCode == http.StatusNotFound {
		if resp != nil {
			resp.Body.Close()
		}
		return "missing"
	}
	defer resp.Body.Close()

	var info struct {
		State struct {
			Status string `json:"Status"`
		} `json:"State"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return "missing"
	}
	switch info.State.Status {
	case "running":
		return "running"
	case "exited", "dead", "":
		return "stopped"
	default:
		return info.State.Status
	}
}

// LogsFollow returns a streaming reader of live container logs (follow=true).
// The caller owns the ReadCloser and must close it when done.
func (m *Manager) LogsFollow(ctx context.Context, containerName string) (io.ReadCloser, error) {
	path := fmt.Sprintf("/containers/%s/logs?stdout=1&stderr=1&follow=1&tail=0", containerName)
	resp, err := m.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("logs follow %s: %w", containerName, err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("logs follow %s: status %d", containerName, resp.StatusCode)
	}
	return resp.Body, nil
}

// StatusAfterStart polls the container status for up to maxSeconds seconds.
// Returns "running" if stable, "error" if it keeps restarting or exits immediately.
func (m *Manager) StatusAfterStart(ctx context.Context, containerName string, maxSeconds int) string {
	deadline := time.Now().Add(time.Duration(maxSeconds) * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(500 * time.Millisecond)
		s := m.Status(ctx, containerName)
		if s == "running" {
			return "running"
		}
		if s == "stopped" || s == "missing" {
			return "error"
		}
	}
	// Still not running after timeout
	s := m.Status(ctx, containerName)
	if s == "running" {
		return "running"
	}
	return "error"
}

// CopyFromContainer returns a tar stream of the given path inside the container.
// The caller is responsible for closing the returned ReadCloser.
func (m *Manager) CopyFromContainer(ctx context.Context, containerName, srcPath string) (io.ReadCloser, error) {
	resp, err := m.do(ctx, "GET", "/containers/"+containerName+"/archive?path="+srcPath, nil)
	if err != nil {
		return nil, fmt.Errorf("archive %s:%s: %w", containerName, srcPath, err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("archive %s:%s returned %d", containerName, srcPath, resp.StatusCode)
	}
	return resp.Body, nil
}

// Logs returns the last n lines of stdout+stderr from the container.
func (m *Manager) Logs(ctx context.Context, containerName string, tail int) (string, error) {
	path := fmt.Sprintf("/containers/%s/logs?stdout=1&stderr=1&tail=%d&timestamps=1", containerName, tail)
	resp, err := m.do(ctx, "GET", path, nil)
	if err != nil {
		return "", fmt.Errorf("logs %s: %w", containerName, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("logs %s: status %d", containerName, resp.StatusCode)
	}
	// Docker multiplexes stdout/stderr with an 8-byte header per chunk.
	// We strip the headers and return plain text.
	var sb strings.Builder
	header := make([]byte, 8)
	buf := make([]byte, 4096)
	for {
		if _, err := io.ReadFull(resp.Body, header); err != nil {
			break
		}
		size := int(header[4])<<24 | int(header[5])<<16 | int(header[6])<<8 | int(header[7])
		for size > 0 {
			n := size
			if n > len(buf) {
				n = len(buf)
			}
			nr, err := resp.Body.Read(buf[:n])
			if nr > 0 {
				sb.Write(buf[:nr])
				size -= nr
			}
			if err != nil {
				goto done
			}
		}
	}
done:
	return sb.String(), nil
}

// UpdateImage pulls the latest version of a container's image.
// The container must be stopped and removed before calling Install again.
func (m *Manager) UpdateImage(ctx context.Context, image string) error {
	resp, err := m.do(ctx, "POST", "/images/create?fromImage="+image, nil)
	if err != nil {
		return fmt.Errorf("pull %s: %w", image, err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

// StatusAll returns a map of containerName → status for quick bulk polling.
func (m *Manager) StatusAll(ctx context.Context) map[string]string {
	resp, err := m.do(ctx, "GET", "/containers/json?all=1&filters=%7B%22label%22%3A%5B%22pcg.managed%3Dtrue%22%5D%7D", nil)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	var containers []struct {
		Names []string `json:"Names"`
		State string   `json:"State"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&containers); err != nil {
		return nil
	}

	result := make(map[string]string, len(containers))
	for _, c := range containers {
		for _, name := range c.Names {
			key := strings.TrimPrefix(name, "/")
			switch c.State {
			case "running":
				result[key] = "running"
			case "exited", "dead":
				result[key] = "stopped"
			default:
				result[key] = c.State
			}
		}
	}
	return result
}

// ── helpers ───────────────────────────────────────────────────────────────────

func (m *Manager) do(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(b)
	}

	url := "http://docker/" + apiVersion + path
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return m.client.Do(req)
}

func checkStatus(resp *http.Response, expected int) error {
	if resp.StatusCode == expected {
		return nil
	}
	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("docker API %d: %s", resp.StatusCode, string(body))
}

func parseMemoryLimit(s string) int64 {
	if len(s) < 2 {
		return 0
	}
	var n int64
	_, _ = fmt.Sscanf(s[:len(s)-1], "%d", &n)
	switch s[len(s)-1] {
	case 'k', 'K':
		return n * 1024
	case 'm', 'M':
		return n * 1024 * 1024
	case 'g', 'G':
		return n * 1024 * 1024 * 1024
	}
	return n
}
