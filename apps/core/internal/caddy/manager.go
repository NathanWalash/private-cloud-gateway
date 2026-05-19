// Package caddy manages Caddy routes by regenerating and reloading the complete
// Caddyfile via the Admin API. This is more reliable than patching individual
// JSON routes because the Caddyfile format is well-understood and tested.
package caddy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// AppRoute represents a single installed app route.
type AppRoute struct {
	Subdomain     string
	ContainerName string
	InternalPort  int
}

// Manager reloads Caddy config dynamically.
type Manager struct {
	adminAddr    string
	cookieDomain string
	client       *http.Client
}

// New creates a Manager targeting the given Caddy admin address (e.g. "caddy:2019").
func New(adminAddr, cookieDomain, _ string) *Manager {
	return &Manager{
		adminAddr:    adminAddr,
		cookieDomain: cookieDomain,
		client:       &http.Client{Timeout: 10 * time.Second},
	}
}

// ReloadAll generates a complete Caddyfile from the given app routes and tells
// Caddy to reload. Always call this after any install or uninstall.
func (m *Manager) ReloadAll(ctx context.Context, apps []AppRoute) error {
	config := m.buildCaddyfile(apps)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"http://"+m.adminAddr+"/load",
		strings.NewReader(config),
	)
	if err != nil {
		return fmt.Errorf("build caddy load request: %w", err)
	}
	req.Header.Set("Content-Type", "text/caddyfile; charset=utf-8")

	resp, err := m.client.Do(req)
	if err != nil {
		return fmt.Errorf("caddy /load: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("caddy /load returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

// buildCaddyfile generates the complete Caddyfile for all installed apps.
func (m *Manager) buildCaddyfile(apps []AppRoute) string {
	var sb strings.Builder

	sb.WriteString(`{
	auto_https off
	admin :2019
	log {
		level INFO
	}
}

`)

	// Dashboard and auth — always present.
	sb.WriteString(fmt.Sprintf("http://home.%s {\n\treverse_proxy core:8080\n}\n\n", m.cookieDomain))

	// One block per installed app.
	for _, app := range apps {
		sb.WriteString(fmt.Sprintf(
			"http://%s.%s {\n\tforward_auth core:8080 {\n\t\turi /api/auth/verify\n\t\tcopy_headers X-Auth-User-ID\n\t}\n\treverse_proxy %s:%d\n}\n\n",
			app.Subdomain, m.cookieDomain,
			app.ContainerName, app.InternalPort,
		))
	}

	return sb.String()
}
