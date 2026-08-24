package backup_test

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	"github.com/NathanWalash/private-cloud-gateway/apps/core/internal/backup"
)

// TestRestore_MissingDBIsError guards against a corrupt/partial archive being
// reported as a successful restore when it contains no database.
func TestRestore_MissingDBIsError(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "nodb.pcg-backup")

	// Build an unencrypted zip with a blueprint but NO cloud-core.db.
	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, _ := zw.Create("blueprints/app.yaml")
	_, _ = w.Write([]byte("id: app\n"))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()

	dbDest := filepath.Join(dir, "restored.db")
	err = backup.Restore(archivePath, "", dbDest, dir)
	if err == nil {
		t.Fatal("Restore of an archive with no database must return an error")
	}
	if _, statErr := os.Stat(dbDest); !os.IsNotExist(statErr) {
		t.Error("no database file should be written when the archive has none")
	}
}
