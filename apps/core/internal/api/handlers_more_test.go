package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/NathanWalash/private-cloud-gateway/apps/core/internal/db"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := db.Migrate(database); err != nil {
		t.Fatal(err)
	}
	return database
}

func insertApp(t *testing.T, database *sql.DB, container string) int64 {
	t.Helper()
	res, err := database.Exec(
		`INSERT INTO apps (blueprint_id, name, icon, subdomain, internal_port, image, container_name, status)
		 VALUES ('memos','Memos','','memos',5230,'neosmemo/memos:0.30.0',?,'running')`, container)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	return id
}

// TestAppsListsInstalledAppsWithSchemeURL covers the Apps() list endpoint and the
// scheme-aware app URL (https in production).
func TestAppsListsInstalledAppsWithSchemeURL(t *testing.T) {
	database := newTestDB(t)
	insertApp(t, database, "pcg-memos")
	h := &Handler{
		db: database, startTime: time.Now(),
		docker: &fakeDocker{}, caddy: fakeCaddy{},
		cookieDomain: "example.com", scheme: "https",
	}

	rec := httptest.NewRecorder()
	h.Apps(rec, httptest.NewRequest("GET", "/api/apps", nil))
	if rec.Code != 200 {
		t.Fatalf("Apps status = %d", rec.Code)
	}

	var apps []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &apps); err != nil {
		t.Fatal(err)
	}
	if len(apps) != 1 {
		t.Fatalf("want 1 app, got %d", len(apps))
	}
	if apps[0]["url"] != "https://memos.example.com" {
		t.Errorf("url = %v, want https://memos.example.com", apps[0]["url"])
	}
}

// TestUninstallRemovesAppAndContainer covers Uninstall(): the container is removed
// via Docker and the DB row is deleted.
func TestUninstallRemovesAppAndContainer(t *testing.T) {
	database := newTestDB(t)
	id := insertApp(t, database, "pcg-memos")
	fd := &fakeDocker{}
	h := &Handler{
		db: database, startTime: time.Now(),
		docker: fd, caddy: fakeCaddy{},
		cookieDomain: "example.com", scheme: "https",
	}

	rec := httptest.NewRecorder()
	h.Uninstall(rec, httptest.NewRequest("DELETE", fmt.Sprintf("/api/apps/%d", id), nil))
	if rec.Code != 204 {
		t.Fatalf("Uninstall status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if fd.removed != "pcg-memos" {
		t.Errorf("docker.Remove called with %q, want pcg-memos", fd.removed)
	}
	var n int
	_ = database.QueryRow("SELECT COUNT(*) FROM apps").Scan(&n)
	if n != 0 {
		t.Errorf("app row not deleted (count=%d)", n)
	}
}
