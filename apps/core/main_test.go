package main

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// makeSQLite creates a valid single-file SQLite database containing a marker row.
func makeSQLite(t *testing.T, path, marker string) {
	t.Helper()
	d, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if _, err := d.Exec("CREATE TABLE t(v TEXT)"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Exec("INSERT INTO t(v) VALUES(?)", marker); err != nil {
		t.Fatal(err)
	}
}

func readMarker(t *testing.T, path string) string {
	t.Helper()
	d, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	var v string
	if err := d.QueryRow("SELECT v FROM t").Scan(&v); err != nil {
		t.Fatalf("read marker from %s: %v", path, err)
	}
	return v
}

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

	// A live DB with stale WAL/SHM sidecars, plus a valid staged restore.
	makeSQLite(t, dbPath, "OLD")
	_ = os.WriteFile(dbPath+"-wal", []byte("stale-wal"), 0o600)
	_ = os.WriteFile(dbPath+"-shm", []byte("stale-shm"), 0o600)
	makeSQLite(t, dbPath+".restored", "NEW")

	if err := adoptRestoredDB(dbPath); err != nil {
		t.Fatalf("adopt: %v", err)
	}

	if got := readMarker(t, dbPath); got != "NEW" {
		t.Errorf("db marker = %q, want NEW", got)
	}
	if _, err := os.Stat(dbPath + ".restored"); !os.IsNotExist(err) {
		t.Error("staged file should be gone after adoption")
	}
	if _, err := os.Stat(dbPath + "-wal"); !os.IsNotExist(err) {
		t.Error("stale -wal should be removed")
	}
	if _, err := os.Stat(dbPath + "-shm"); !os.IsNotExist(err) {
		t.Error("stale -shm should be removed")
	}
}

// TestAdoptRestoredDB_DiscardsCorrupt: a corrupt staged DB must be discarded and
// the live database left untouched, so a bad restore can't destroy existing data.
func TestAdoptRestoredDB_DiscardsCorrupt(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "cloud-core.db")

	makeSQLite(t, dbPath, "ORIGINAL")
	if err := os.WriteFile(dbPath+".restored", []byte("not a sqlite file at all"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := adoptRestoredDB(dbPath); err != nil {
		t.Fatalf("adopt should not error on a corrupt staged DB: %v", err)
	}
	// Corrupt staged discarded...
	if _, err := os.Stat(dbPath + ".restored"); !os.IsNotExist(err) {
		t.Error("corrupt staged file should be discarded")
	}
	// ...and the live DB preserved.
	if got := readMarker(t, dbPath); got != "ORIGINAL" {
		t.Errorf("live DB marker = %q, want ORIGINAL (must be untouched)", got)
	}
}
