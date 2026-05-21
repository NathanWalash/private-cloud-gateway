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

// Install pulls the image and creates the container (does not start it).
func (m *Manager) Install(ctx context.Context, bp *blueprint.Blueprint) error {
	// Pull image — stream is discarded; we just wait for completion.
	slog.Info("pulling image", "image", bp.Container.Image)
	pullURL := fmt.Sprintf("/images/create?fromImage=%s", bp.Container.Image)
	resp, err := m.do(ctx, "POST", pullURL, nil)
	if err != nil {
		return fmt.Errorf("pull image: %w", err)
	}
	defer resp.Body.Close()
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		return fmt.Errorf("drain pull: %w", err)
	}

	// Build container config.
	type hostConfig struct {
		Binds         []string          `json:"Binds,omitempty"`
		RestartPolicy map[string]any    `json:"RestartPolicy"`
		Memory        int64             `json:"Memory,omitempty"`
		NetworkMode   string            `json:"NetworkMode"`
	}
	type endpointSettings struct{}
	type networkingConfig struct {
		EndpointsConfig map[string]*endpointSettings `json:"EndpointsConfig"`
	}
	type createBody struct {
		Image        string              `json:"Image"`
		Env          []string            `json:"Env,omitempty"`
		Labels       map[string]string   `json:"Labels,omitempty"`
		HostConfig   hostConfig          `json:"HostConfig"`
		NetworkingConfig networkingConfig `json:"NetworkingConfig"`
	}

	body := createBody{
		Image: bp.Container.Image,
		Env:   bp.Container.Environment,
		Labels: map[string]string{
			"pcg.app":     bp.ID,
			"pcg.managed": "true",
		},
		HostConfig: hostConfig{
			Binds:       bp.Container.Volumes,
			RestartPolicy: map[string]any{"Name": "unless-stopped"},
			Memory:      parseMemoryLimit(bp.Resources.MemoryLimit),
			NetworkMode: privateNetwork,
		},
		NetworkingConfig: networkingConfig{
			EndpointsConfig: map[string]*endpointSettings{
				privateNetwork: {},
			},
		},
	}

	createURL := fmt.Sprintf("/containers/create?name=%s", bp.ContainerName())
	resp2, err := m.do(ctx, "POST", createURL, body)
	if err != nil {
		return fmt.Errorf("create container: %w", err)
	}
	defer resp2.Body.Close()
	if err := checkStatus(resp2, http.StatusCreated); err != nil {
		return err
	}

	slog.Info("container created", "name", bp.ContainerName())
	return nil
}

// Start starts a stopped container.
func (m *Manager) Start(ctx context.Context, containerName string) error {
	resp, err := m.do(ctx, "POST", "/containers/"+containerName+"/start", nil)
	if err != nil {
		return fmt.Errorf("start %s: %w", containerName, err)
	}
	defer resp.Body.Close()
	// 204 = started, 304 = already running — both acceptable.
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotModified {
		return fmt.Errorf("start %s: unexpected status %d", containerName, resp.StatusCode)
	}
	return nil
}

// Stop gracefully stops a running container (10 second timeout).
func (m *Manager) Stop(ctx context.Context, containerName string) error {
	resp, err := m.do(ctx, "POST", "/containers/"+containerName+"/stop?t=10", nil)
	if err != nil {
		return fmt.Errorf("stop %s: %w", containerName, err)
	}
	defer resp.Body.Close()
	return nil
}

// Restart restarts a container.
func (m *Manager) Restart(ctx context.Context, containerName string) error {
	resp, err := m.do(ctx, "POST", "/containers/"+containerName+"/restart?t=10", nil)
	if err != nil {
		return fmt.Errorf("restart %s: %w", containerName, err)
	}
	defer resp.Body.Close()
	return nil
}

// Remove stops (if needed) and removes the container.
func (m *Manager) Remove(ctx context.Context, containerName string) error {
	resp, err := m.do(ctx, "DELETE", "/containers/"+containerName+"?force=true", nil)
	if err != nil {
		return fmt.Errorf("remove %s: %w", containerName, err)
	}
	defer resp.Body.Close()
	return nil
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
		Names  []string `json:"Names"`
		State  string   `json:"State"`
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
