package db_test

import (
	"database/sql"
	"testing"

	"github.com/NathanWalash/private-cloud-gateway/apps/core/internal/db"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	if err := db.Migrate(database); err != nil {
		t.Fatalf("db.Migrate: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func TestOpen(t *testing.T) {
	database := testDB(t)
	if err := database.Ping(); err != nil {
		t.Fatalf("Ping after Open: %v", err)
	}
}

func TestMigrate_Idempotent(t *testing.T) {
	database := testDB(t)
	if err := db.Migrate(database); err != nil {
		t.Fatalf("second Migrate should be a no-op, got: %v", err)
	}
}

func TestMigrate_Tables(t *testing.T) {
	database := testDB(t)
	for _, tbl := range []string{"users", "sessions", "audit_log"} {
		var name string
		err := database.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?", tbl,
		).Scan(&name)
		if err != nil {
			t.Errorf("table %q not found after migration", tbl)
		}
	}
}

func TestBootstrap_CreatesFirstUser(t *testing.T) {
	database := testDB(t)
	if err := db.Bootstrap(database, "admin@example.com", "password123"); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	var count int
	database.QueryRow("SELECT COUNT(*) FROM users WHERE email = 'admin@example.com'").Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 user after bootstrap, got %d", count)
	}
}

func TestBootstrap_NoopWhenUsersExist(t *testing.T) {
	database := testDB(t)
	db.Bootstrap(database, "first@example.com", "password")
	if err := db.Bootstrap(database, "second@example.com", "password"); err == nil {
		t.Error("Bootstrap should return an error when users already exist")
	}
	var count int
	database.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 user total, got %d", count)
	}
}
