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
	https        bool   // production HTTPS mode
	adminEmail   string // Let's Encrypt email (required for HTTPS mode)
	client       *http.Client
}

// New creates a Manager in local-dev mode (HTTP only, auto_https off).
func New(adminAddr, cookieDomain, _ string) *Manager {
	return &Manager{
		adminAddr:    adminAddr,
		cookieDomain: cookieDomain,
		https:        false,
		client:       &http.Client{Timeout: 10 * time.Second},
	}
}

// NewProduction creates a Manager in production mode (HTTPS via Let's Encrypt).
func NewProduction(adminAddr, cookieDomain, adminEmail string) *Manager {
	return &Manager{
		adminAddr:    adminAddr,
		cookieDomain: cookieDomain,
		https:        true,
		adminEmail:   adminEmail,
		client:       &http.Client{Timeout: 10 * time.Second},
	}
}

// BuildCaddyfileForTest exposes buildCaddyfile for unit tests.
func (m *Manager) BuildCaddyfileForTest(apps []AppRoute) string {
	return m.buildCaddyfile(apps)
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

// buildCaddyfile generates the complete Caddyfile.
// In dev mode: HTTP only, includes whoami test route.
// In production mode: HTTPS via Let's Encrypt, no test routes.
func (m *Manager) buildCaddyfile(apps []AppRoute) string {
	var sb strings.Builder
	proto := "http"

	if m.https {
		// Production: HTTPS with Let's Encrypt, admin API off public internet
		sb.WriteString(fmt.Sprintf(`{
	admin :2019
	email %s
	log {
		level INFO
	}
}

`, m.adminEmail))
		proto = "https"
	} else {
		// Local dev: HTTP only, no cert requests
		sb.WriteString(`{
	auto_https off
	admin :2019
	log {
		level INFO
	}
}

`)
	}

	// Dashboard and auth — always present.
	sb.WriteString(fmt.Sprintf("%s://home.%s {\n\treverse_proxy core:8080\n}\n\n", proto, m.cookieDomain))

	if !m.https {
		// Test app — local dev only. Only add if no installed app uses the 'files' subdomain.
		filesInUse := false
		for _, app := range apps {
			if app.Subdomain == "files" {
				filesInUse = true
				break
			}
		}
		if !filesInUse {
			sb.WriteString(fmt.Sprintf(
				"http://files.%s {\n\tforward_auth core:8080 {\n\t\turi /api/auth/verify\n\t\tcopy_headers X-Auth-User-ID\n\t}\n\treverse_proxy whoami:80\n}\n\n",
				m.cookieDomain,
			))
		}
	}

	// One block per installed app.
	for _, app := range apps {
		sb.WriteString(fmt.Sprintf(
			"%s://%s.%s {\n\tforward_auth core:8080 {\n\t\turi /api/auth/verify\n\t\tcopy_headers X-Auth-User-ID\n\t}\n\treverse_proxy %s:%d\n}\n\n",
			proto, app.Subdomain, m.cookieDomain,
			app.ContainerName, app.InternalPort,
		))
	}

	// Catch-all: any unrecognised subdomain redirects to the home dashboard.
	// This handles: apps not yet installed, typos, and Caddy redirecting /login
	// to home.* for the React setup wizard to handle.
	sb.WriteString(fmt.Sprintf(
		"%s://*.%s {\n\tredir %s://home.%s{uri} temporary\n}\n\n",
		proto, m.cookieDomain,
		proto, m.cookieDomain,
	))

	return sb.String()
}
