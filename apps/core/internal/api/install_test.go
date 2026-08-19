package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NathanWalash/private-cloud-gateway/apps/core/internal/blueprint"
	"github.com/NathanWalash/private-cloud-gateway/apps/core/internal/caddy"
	"github.com/NathanWalash/private-cloud-gateway/apps/core/internal/db"
)

// fakeDocker records the blueprint passed to Install so we can assert what would
// actually be sent to Docker.
type fakeDocker struct {
	installed *blueprint.Blueprint
	removed   string
}

func (f *fakeDocker) Install(_ context.Context, bp *blueprint.Blueprint) error {
	f.installed = bp
	return nil
}
func (f *fakeDocker) Start(context.Context, string) error { return nil }
func (f *fakeDocker) Stop(context.Context, string) error  { return nil }
func (f *fakeDocker) Restart(context.Context, string) error {
	return nil
}
func (f *fakeDocker) Remove(_ context.Context, name string) error {
	f.removed = name
	return nil
}
func (f *fakeDocker) StatusAfterStart(context.Context, string, int) string { return "running" }
func (f *fakeDocker) Logs(context.Context, string, int) (string, error)    { return "", nil }
func (f *fakeDocker) LogsFollow(context.Context, string) (io.ReadCloser, error) {
	return nil, nil
}
func (f *fakeDocker) UpdateImage(context.Context, string) error { return nil }
func (f *fakeDocker) CopyFromContainer(context.Context, string, string) (io.ReadCloser, error) {
	return nil, nil
}

type fakeCaddy struct{}

func (fakeCaddy) ReloadAll(context.Context, []caddy.AppRoute) error { return nil }

// TestInstallRendersDomainAndScheme is the integration test for the render→install
// seam: it proves ${DOMAIN}/${SCHEME} placeholders are substituted with the
// deployment's real values before the blueprint reaches Docker.
func TestInstallRendersDomainAndScheme(t *testing.T) {
	tmp := t.TempDir()
	yaml := "id: n8n\n" +
		"name: n8n\n" +
		"container:\n" +
		"  image: n8nio/n8n:2.35.3\n" +
		"  environment:\n" +
		"    - N8N_HOST=n8n.${DOMAIN}\n" +
		"    - WEBHOOK_URL=${SCHEME}://n8n.${DOMAIN}/\n" +
		"route:\n" +
		"  subdomain: n8n\n" +
		"  internal_port: 5678\n"
	if err := os.WriteFile(filepath.Join(tmp, "n8n.yaml"), []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}

	database, err := db.Open(filepath.Join(tmp, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.Migrate(database); err != nil {
		t.Fatal(err)
	}

	fd := &fakeDocker{}
	h := &Handler{
		db:           database,
		startTime:    time.Now(),
		version:      "test",
		docker:       fd,
		caddy:        fakeCaddy{},
		blueprintDir: tmp,
		cookieDomain: "example.com",
		scheme:       "https",
	}

	body, _ := json.Marshal(map[string]string{"blueprint_id": "n8n"})
	rec := httptest.NewRecorder()
	h.Install(rec, httptest.NewRequest("POST", "/api/apps/install", bytes.NewReader(body)))

	if rec.Code >= 300 {
		t.Fatalf("install status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if fd.installed == nil {
		t.Fatal("docker.Install was never called")
	}
	env := strings.Join(fd.installed.Container.Environment, "\n")
	if !strings.Contains(env, "N8N_HOST=n8n.example.com") {
		t.Errorf("${DOMAIN} not substituted: %q", env)
	}
	if !strings.Contains(env, "WEBHOOK_URL=https://n8n.example.com/") {
		t.Errorf("${SCHEME}/${DOMAIN} not substituted: %q", env)
	}
}
