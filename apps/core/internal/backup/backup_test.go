package backup_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NathanWalash/private-cloud-gateway/apps/core/internal/backup"
)

func setupTestData(t *testing.T) (dbPath, bpDir, backupDir string) {
	t.Helper()
	dir := t.TempDir()

	dbPath = filepath.Join(dir, "test.db")
	os.WriteFile(dbPath, []byte("fake sqlite data"), 0o600)

	bpDir = filepath.Join(dir, "blueprints")
	os.MkdirAll(bpDir, 0o755)
	os.WriteFile(filepath.Join(bpDir, "test-app.yaml"), []byte("id: test-app\n"), 0o644)

	backupDir = filepath.Join(dir, "backups")
	os.MkdirAll(backupDir, 0o755)
	return
}

func TestCreate_Unencrypted(t *testing.T) {
	dbPath, bpDir, backupDir := setupTestData(t)
	dest := filepath.Join(backupDir, "backup.pcg-backup")

	if err := backup.Create(dbPath, bpDir, dest, ""); err != nil {
		t.Fatalf("Create: %v", err)
	}
	info, err := os.Stat(dest)
	if err != nil {
		t.Fatalf("backup file not created: %v", err)
	}
	if info.Size() == 0 {
		t.Error("backup file is empty")
	}
}

func TestCreate_Encrypted_LargerThanPlain(t *testing.T) {
	dbPath, bpDir, backupDir := setupTestData(t)

	plain := filepath.Join(backupDir, "plain.pcg-backup")
	enc := filepath.Join(backupDir, "enc.pcg-backup")

	backup.Create(dbPath, bpDir, plain, "")
	backup.Create(dbPath, bpDir, enc, "my-passphrase")

	plainInfo, _ := os.Stat(plain)
	encInfo, _ := os.Stat(enc)

	// Encrypted file has extra salt + nonce + auth tag overhead
	if encInfo.Size() <= plainInfo.Size() {
		t.Errorf("encrypted (%d bytes) should be larger than plain (%d bytes)", encInfo.Size(), plainInfo.Size())
	}
}

func TestListBackups(t *testing.T) {
	_, _, backupDir := setupTestData(t)
	dbPath, bpDir, _ := setupTestData(t)

	backup.Create(dbPath, bpDir, filepath.Join(backupDir, "first.pcg-backup"), "")
	backup.Create(dbPath, bpDir, filepath.Join(backupDir, "second.pcg-backup"), "")

	list, err := backup.ListBackups(backupDir)
	if err != nil {
		t.Fatalf("ListBackups: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 backups, got %d", len(list))
	}
	for _, b := range list {
		if b.Size == 0 {
			t.Errorf("backup %q has zero size", b.Name)
		}
	}
}

func TestFileName(t *testing.T) {
	name := backup.FileName(time.Date(2026, 5, 19, 12, 30, 0, 0, time.UTC))
	if !strings.HasPrefix(name, "pcg-backup-20260519-") {
		t.Errorf("unexpected filename: %q", name)
	}
	if !strings.HasSuffix(name, ".pcg-backup") {
		t.Errorf("unexpected extension in: %q", name)
	}
}
