package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAdoptRestoredDB(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "cloud-core.db")

	// No staged file → no-op, no error, no db created.
	if err := adoptRestoredDB(dbPath); err != nil {
		t.Fatalf("no-op adopt: %v", err)
	}
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		t.Error("adopt with nothing staged should not create the db")
	}

	// Simulate a live DB with stale WAL/SHM sidecars and a staged restore.
	if err := os.WriteFile(dbPath, []byte("OLD"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dbPath+"-wal", []byte("stale-wal"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dbPath+"-shm", []byte("stale-shm"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dbPath+".restored", []byte("NEW"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := adoptRestoredDB(dbPath); err != nil {
		t.Fatalf("adopt: %v", err)
	}

	// The staged DB replaced the live one...
	got, _ := os.ReadFile(dbPath)
	if string(got) != "NEW" {
		t.Errorf("db content = %q, want NEW", got)
	}
	// ...the staged file is consumed...
	if _, err := os.Stat(dbPath + ".restored"); !os.IsNotExist(err) {
		t.Error("staged file should be gone after adoption")
	}
	// ...and the stale WAL/SHM of the old DB are removed.
	if _, err := os.Stat(dbPath + "-wal"); !os.IsNotExist(err) {
		t.Error("stale -wal should be removed")
	}
	if _, err := os.Stat(dbPath + "-shm"); !os.IsNotExist(err) {
		t.Error("stale -shm should be removed")
	}
}
