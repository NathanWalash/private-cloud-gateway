package backup_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NathanWalash/private-cloud-gateway/apps/core/internal/backup"
	_ "modernc.org/sqlite"
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

	if err := backup.Create(dbPath, bpDir, dest, "", nil, nil); err != nil {
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

// TestCreate_CheckpointsWAL ensures committed data still sitting in the -wal
// file is captured by the backup. Without the pre-copy checkpoint, the raw .db
// copy would miss the row and the restored DB wouldn't have the table.
func TestCreate_CheckpointsWAL(t *testing.T) {
	_, bpDir, backupDir := setupTestData(t)

	dbPath := filepath.Join(t.TempDir(), "wal.db")
	live, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := live.Exec("CREATE TABLE t(v TEXT)"); err != nil {
		t.Fatal(err)
	}
	if _, err := live.Exec("INSERT INTO t(v) VALUES('hello-wal')"); err != nil {
		t.Fatal(err)
	}
	// Keep `live` open (simulating the running server) so the write stays in WAL.
	defer live.Close()

	dest := filepath.Join(backupDir, "wal.pcg-backup")
	if err := backup.Create(dbPath, bpDir, dest, "", nil, nil); err != nil {
		t.Fatalf("Create: %v", err)
	}

	restoreDir := t.TempDir()
	restored := filepath.Join(restoreDir, "restored.db")
	if err := backup.Restore(dest, "", restored, restoreDir); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	rdb, err := sql.Open("sqlite", restored)
	if err != nil {
		t.Fatal(err)
	}
	defer rdb.Close()
	var v string
	if err := rdb.QueryRow("SELECT v FROM t").Scan(&v); err != nil {
		t.Fatalf("row missing from backup — WAL not checkpointed: %v", err)
	}
	if v != "hello-wal" {
		t.Errorf("got %q, want hello-wal", v)
	}
}

func TestCreate_Encrypted_LargerThanPlain(t *testing.T) {
	dbPath, bpDir, backupDir := setupTestData(t)

	plain := filepath.Join(backupDir, "plain.pcg-backup")
	enc := filepath.Join(backupDir, "enc.pcg-backup")

	backup.Create(dbPath, bpDir, plain, "", nil, nil)
	backup.Create(dbPath, bpDir, enc, "my-passphrase", nil, nil)

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

	backup.Create(dbPath, bpDir, filepath.Join(backupDir, "first.pcg-backup"), "", nil, nil)
	backup.Create(dbPath, bpDir, filepath.Join(backupDir, "second.pcg-backup"), "", nil, nil)

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
