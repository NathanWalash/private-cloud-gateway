package backup_test

import (
	"database/sql"
	"io"
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

	if err := backup.Create(dbPath, bpDir, dest, "", nil, nil, nil, nil); err != nil {
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
	if err := backup.Create(dbPath, bpDir, dest, "", nil, nil, nil, nil); err != nil {
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

	backup.Create(dbPath, bpDir, plain, "", nil, nil, nil, nil)
	backup.Create(dbPath, bpDir, enc, "my-passphrase", nil, nil, nil, nil)

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

	backup.Create(dbPath, bpDir, filepath.Join(backupDir, "first.pcg-backup"), "", nil, nil, nil, nil)
	backup.Create(dbPath, bpDir, filepath.Join(backupDir, "second.pcg-backup"), "", nil, nil, nil, nil)

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

func TestServiceDumpRoundTrip(t *testing.T) {
	dbPath, bpDir, backupDir := setupTestData(t)
	dest := filepath.Join(backupDir, "dumps.pcg-backup")

	dumps := []backup.ServiceDump{
		{AppID: "umami", Service: "db", ContainerName: "pcg-umami-db", Command: []string{"pg_dump"}},
	}
	// Fake dumper writes deterministic content instead of running docker exec.
	dumper := func(container string, cmd []string, out io.Writer) error {
		_, err := out.Write([]byte("-- dump for " + container + "\nSELECT 1;\n"))
		return err
	}

	if err := backup.Create(dbPath, bpDir, dest, "", nil, nil, dumps, dumper); err != nil {
		t.Fatalf("Create: %v", err)
	}

	var got string
	err := backup.ForEachServiceDump(dest, "", func(appID, service string, data []byte) error {
		if appID == "umami" && service == "db" {
			got = string(data)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("ForEachServiceDump: %v", err)
	}
	if !strings.Contains(got, "dump for pcg-umami-db") {
		t.Errorf("dump content not archived/round-tripped: %q", got)
	}
}

func TestPrune(t *testing.T) {
	dir := t.TempDir()
	// Names sort chronologically (FileName format), so these are oldest..newest.
	names := []string{
		"pcg-backup-20260101-000000.pcg-backup",
		"pcg-backup-20260102-000000.pcg-backup",
		"pcg-backup-20260103-000000.pcg-backup",
		"pcg-backup-20260104-000000.pcg-backup",
	}
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// A non-backup file must be left alone.
	os.WriteFile(filepath.Join(dir, "keep.txt"), []byte("x"), 0o600)

	if err := backup.Prune(dir, 2); err != nil {
		t.Fatal(err)
	}
	left := map[string]bool{}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		left[e.Name()] = true
	}
	if !left["pcg-backup-20260104-000000.pcg-backup"] || !left["pcg-backup-20260103-000000.pcg-backup"] {
		t.Error("Prune should keep the 2 newest archives")
	}
	if left["pcg-backup-20260101-000000.pcg-backup"] || left["pcg-backup-20260102-000000.pcg-backup"] {
		t.Error("Prune should delete the 2 oldest archives")
	}
	if !left["keep.txt"] {
		t.Error("Prune must not touch non-backup files")
	}

	// keep=0 disables pruning.
	os.WriteFile(filepath.Join(dir, "pcg-backup-20260105-000000.pcg-backup"), []byte("x"), 0o600)
	before, _ := backup.ListBackups(dir)
	_ = backup.Prune(dir, 0)
	after, _ := backup.ListBackups(dir)
	if len(before) != len(after) {
		t.Error("Prune(0) should keep all")
	}
}
