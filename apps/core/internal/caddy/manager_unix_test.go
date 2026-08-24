package caddy_test

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/NathanWalash/private-cloud-gateway/apps/core/internal/caddy"
)

// TestManager_UnixSocketAdmin proves that when the admin address is an absolute
// path the manager talks to Caddy over a Unix-domain socket (the production
// hardening that keeps the admin API off the app-reachable network).
func TestManager_UnixSocketAdmin(t *testing.T) {
	// Keep the socket path short (Unix socket paths are length-limited).
	sock := filepath.Join("/tmp", "pcg-caddy-admin-test.sock")
	_ = os.Remove(sock)
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}
	defer ln.Close()
	defer os.Remove(sock)

	gotPath := make(chan string, 1)
	gotBody := make(chan string, 1)
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotPath <- r.URL.Path
		gotBody <- string(b)
		w.WriteHeader(http.StatusOK)
	})}
	go func() { _ = srv.Serve(ln) }()
	defer srv.Close()

	// An absolute path selects the Unix-socket transport.
	m := caddy.New(sock, "example.com", "")
	if err := m.ReloadAll(context.Background(), []caddy.AppRoute{
		{Subdomain: "notes", ContainerName: "pcg-notes", InternalPort: 8080},
	}); err != nil {
		t.Fatalf("ReloadAll over unix socket: %v", err)
	}

	if p := <-gotPath; p != "/load" {
		t.Errorf("admin path = %q, want /load", p)
	}
	if body := <-gotBody; body == "" {
		t.Error("admin server received an empty Caddyfile body")
	}
}
