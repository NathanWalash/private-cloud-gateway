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
	"time"

	"github.com/NathanWalash/private-cloud-gateway/apps/core/internal/blueprint"
)

const (
	socketPath     = "/var/run/docker.sock"
	apiVersion     = "v1.41"
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
