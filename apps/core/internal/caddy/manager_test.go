package caddy_test

import (
	"strings"
	"testing"

	"github.com/NathanWalash/private-cloud-gateway/apps/core/internal/caddy"
)

func TestBuildCaddyfile_DevMode(t *testing.T) {
	m := caddy.New("caddy:2019", "localtest.me", "http://home.localtest.me/login")

	routes := []caddy.AppRoute{
		{Subdomain: "pdf", ContainerName: "pcg-stirling-pdf", InternalPort: 8080},
		{Subdomain: "notes", ContainerName: "pcg-silverbullet", InternalPort: 3000},
	}

	config := m.BuildCaddyfileForTest(routes)

	// Global block must disable HTTPS in dev mode
	if !strings.Contains(config, "auto_https off") {
		t.Error("dev Caddyfile missing 'auto_https off'")
	}
	// Admin must be accessible on all interfaces
	if !strings.Contains(config, "admin :2019") {
		t.Error("dev Caddyfile missing 'admin :2019'")
	}
	// Home route always present
	if !strings.Contains(config, "http://home.localtest.me") {
		t.Error("dev Caddyfile missing home route")
	}
	// Test app (whoami) always present in dev
	if !strings.Contains(config, "files.localtest.me") {
		t.Error("dev Caddyfile missing files.localtest.me test route")
	}
	// Installed app routes
	if !strings.Contains(config, "http://pdf.localtest.me") {
		t.Error("dev Caddyfile missing pdf app route")
	}
	if !strings.Contains(config, "pcg-stirling-pdf:8080") {
		t.Error("dev Caddyfile missing stirling-pdf upstream")
	}
	if !strings.Contains(config, "http://notes.localtest.me") {
		t.Error("dev Caddyfile missing notes app route")
	}
	// forward_auth present for each app
	if strings.Count(config, "forward_auth") < 3 { // files + 2 apps
		t.Errorf("expected at least 3 forward_auth blocks, got %d", strings.Count(config, "forward_auth"))
	}
	// Catch-all must redirect unknown subdomains to home
	if !strings.Contains(config, "*.localtest.me") {
		t.Error("dev Caddyfile missing catch-all wildcard block")
	}
	if !strings.Contains(config, "redir") {
		t.Error("dev Caddyfile catch-all missing redir directive")
	}
}

func TestBuildCaddyfile_ProductionMode(t *testing.T) {
	m := caddy.NewProduction("caddy:2019", "nathan.me", "admin@nathan.me")
	routes := []caddy.AppRoute{
		{Subdomain: "files", ContainerName: "pcg-filebrowser", InternalPort: 8080},
	}
	config := m.BuildCaddyfileForTest(routes)

	// Production must NOT have auto_https off
	if strings.Contains(config, "auto_https off") {
		t.Error("production Caddyfile must not have 'auto_https off'")
	}
	// Must have the email for Let's Encrypt
	if !strings.Contains(config, "admin@nathan.me") {
		t.Error("production Caddyfile missing admin email")
	}
	// Must use https:// protocol
	if !strings.Contains(config, "https://home.nathan.me") {
		t.Error("production Caddyfile missing https home route")
	}
	if !strings.Contains(config, "https://files.nathan.me") {
		t.Error("production Caddyfile missing https app route")
	}
	// No whoami test route in production
	if strings.Contains(config, "whoami") {
		t.Error("production Caddyfile must not contain whoami test route")
	}
	// Catch-all uses https and redirects to home
	if !strings.Contains(config, "*.nathan.me") {
		t.Error("production Caddyfile missing catch-all")
	}
	if !strings.Contains(config, "https://home.nathan.me") || !strings.Contains(config, "redir") {
		t.Error("production Caddyfile catch-all must redir to https home")
	}
}

func TestBuildCaddyfile_EmptyApps(t *testing.T) {
	m := caddy.New("caddy:2019", "localtest.me", "")
	config := m.BuildCaddyfileForTest(nil)
	if !strings.Contains(config, "http://home.localtest.me") {
		t.Error("empty apps: home route missing")
	}
}
