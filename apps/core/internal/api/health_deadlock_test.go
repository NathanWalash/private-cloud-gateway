package api_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/NathanWalash/private-cloud-gateway/apps/core/internal/api"
	"github.com/NathanWalash/private-cloud-gateway/apps/core/internal/db"
)

// TestRunAppHealthChecks_NoDeadlock guards against the connection-starvation
// deadlock where a DB write was issued inside an open rows loop. The prod DB
// pool is capped at one connection (db.Open sets SetMaxOpenConns(1)), so if the
// UPDATE ran while the SELECT's rows were still open it would block forever and
// hang every DB request. The test fails (via the timeout) if that regresses.
func TestRunAppHealthChecks_NoDeadlock(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := db.Migrate(database); err != nil {
		t.Fatal(err)
	}

	bpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(bpDir, "testapp.yaml"), []byte(
		"id: testapp\nname: Test\ncontainer:\n  image: nginx\nroute:\n  subdomain: test\n  internal_port: 80\nhealth:\n  path: /\n  expected_status: 200\n",
	), 0o644); err != nil {
		t.Fatal(err)
	}

	// A single running app is enough: the old code issued the UPDATE while the
	// SELECT's rows were still open, so the write blocked on the loop's own
	// connection and never returned.
	if _, err := database.Exec(
		`INSERT INTO apps (blueprint_id, name, icon, subdomain, internal_port, image, container_name, status)
		 VALUES ('testapp', 'Test', 'x', 'test', 80, 'nginx', 'pcg-testapp', 'running')`); err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})
	go func() {
		// Health endpoints are unreachable (no such containers) → "unreachable",
		// which still triggers the UPDATE that used to deadlock.
		api.RunAppHealthChecks(database, bpDir, nil)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("RunAppHealthChecks deadlocked — a DB write inside an open rows loop?")
	}

	// The UPDATE must have run (proves the write path completed, not deadlocked).
	var health string
	if err := database.QueryRow(
		"SELECT health_status FROM apps WHERE container_name='pcg-testapp'").Scan(&health); err != nil {
		t.Fatal(err)
	}
	if health == "" || health == "unknown" {
		t.Errorf("health_status not updated (got %q) — write path did not run", health)
	}
}
