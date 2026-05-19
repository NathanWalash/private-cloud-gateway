// Package caddy manages dynamic route registration via Caddy's Admin API.
package caddy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Manager registers and removes routes in a running Caddy instance.
type Manager struct {
	adminAddr  string // e.g. "localhost:2019"
	cookieDomain string
	loginURL   string
	client     *http.Client
}

// New creates a Manager targeting the given Caddy admin address.
func New(adminAddr, cookieDomain, loginURL string) *Manager {
	return &Manager{
		adminAddr:    adminAddr,
		cookieDomain: cookieDomain,
		loginURL:     loginURL,
		client:       &http.Client{Timeout: 5 * time.Second},
	}
}

// caddyRoute is the JSON shape for a single Caddy route.
type caddyRoute struct {
	Match  []map[string]any `json:"match"`
	Handle []map[string]any `json:"handle"`
}

// RegisterApp adds a forward-auth protected reverse-proxy route for the app.
// The route is appended to Caddy's current route list via the Admin API.
func (m *Manager) RegisterApp(ctx context.Context, subdomain, containerName string, internalPort int) error {
	host := fmt.Sprintf("%s.%s", subdomain, m.cookieDomain)
	upstream := fmt.Sprintf("%s:%d", containerName, internalPort)

	route := caddyRoute{
		Match: []map[string]any{
			{"host": []string{host}},
		},
		Handle: []map[string]any{
			{
				"handler": "subroute",
				"routes": []map[string]any{
					{
						"handle": []map[string]any{
							{
								"handler": "forward_auth",
								"upstreams": []map[string]any{
									{"dial": "core:8080"},
								},
								"uri":          "/api/auth/verify",
								"copy_headers": []string{"X-Auth-User-ID"},
							},
						},
					},
					{
						"handle": []map[string]any{
							{
								"handler": "reverse_proxy",
								"upstreams": []map[string]any{
									{"dial": upstream},
								},
							},
						},
					},
				},
			},
		},
	}

	body, err := json.Marshal(route)
	if err != nil {
		return fmt.Errorf("marshal caddy route: %w", err)
	}

	url := fmt.Sprintf("http://%s/config/apps/http/servers/srv0/routes/0", m.adminAddr)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build caddy request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.client.Do(req)
	if err != nil {
		return fmt.Errorf("caddy admin post: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("caddy admin returned %d", resp.StatusCode)
	}
	return nil
}

// DeregisterApp removes the route for the given subdomain.
func (m *Manager) DeregisterApp(ctx context.Context, subdomain string) error {
	host := fmt.Sprintf("%s.%s", subdomain, m.cookieDomain)

	routes, err := m.getRoutes(ctx)
	if err != nil {
		return err
	}

	for i, route := range routes {
		if routeMatchesHost(route, host) {
			url := fmt.Sprintf("http://%s/config/apps/http/servers/srv0/routes/%d", m.adminAddr, i)
			req, _ := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
			resp, err := m.client.Do(req)
			if err != nil {
				return fmt.Errorf("caddy admin delete: %w", err)
			}
			resp.Body.Close()
			return nil
		}
	}
	return nil // route not found — already gone
}

func (m *Manager) getRoutes(ctx context.Context) ([]map[string]any, error) {
	url := fmt.Sprintf("http://%s/config/apps/http/servers/srv0/routes", m.adminAddr)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	resp, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("caddy get routes: %w", err)
	}
	defer resp.Body.Close()

	var routes []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&routes); err != nil {
		return nil, fmt.Errorf("decode caddy routes: %w", err)
	}
	return routes, nil
}

func routeMatchesHost(route map[string]any, host string) bool {
	matches, ok := route["match"].([]any)
	if !ok {
		return false
	}
	for _, m := range matches {
		mm, ok := m.(map[string]any)
		if !ok {
			continue
		}
		hosts, ok := mm["host"].([]any)
		if !ok {
			continue
		}
		for _, h := range hosts {
			if h == host {
				return true
			}
		}
	}
	return false
}
