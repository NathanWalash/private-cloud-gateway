package backup_test

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/NathanWalash/private-cloud-gateway/apps/core/internal/backup"
)

// TestEncryptedRoundTrip proves an encrypted archive can be restored with the
// right passphrase, and refuses the wrong one — the whole point of the feature.
func TestEncryptedRoundTrip(t *testing.T) {
	dbPath, bpDir, backupDir := setupTestData(t)
	dest := filepath.Join(backupDir, "enc.pcg-backup")
	const pass = "correct horse battery staple"

	if err := backup.Create(dbPath, bpDir, dest, pass, nil, nil, nil, nil); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// The file must not be a plain zip — the first bytes are the random salt,
	// never the "PK" zip magic.
	head, _ := os.ReadFile(dest)
	if len(head) >= 2 && head[0] == 'P' && head[1] == 'K' {
		t.Fatal("encrypted backup must not start with the PK zip magic")
	}

	// Correct passphrase restores the original DB bytes and the blueprint.
	restoreDir := t.TempDir()
	restoredDB := filepath.Join(restoreDir, "restored.db")
	if err := backup.Restore(dest, pass, restoredDB, restoreDir); err != nil {
		t.Fatalf("Restore with correct passphrase: %v", err)
	}
	got, _ := os.ReadFile(restoredDB)
	if string(got) != "fake sqlite data" {
		t.Errorf("restored db = %q, want original contents", got)
	}
	if _, err := os.Stat(filepath.Join(restoreDir, "test-app.yaml")); err != nil {
		t.Errorf("blueprint not restored: %v", err)
	}

	// Wrong passphrase must fail (GCM auth), not silently produce garbage.
	if err := backup.Restore(dest, "wrong-passphrase", filepath.Join(t.TempDir(), "x.db"), t.TempDir()); err == nil {
		t.Error("Restore with the wrong passphrase must fail")
	}

	// Treating an encrypted archive as unencrypted must also fail cleanly.
	if err := backup.Restore(dest, "", filepath.Join(t.TempDir(), "y.db"), t.TempDir()); err == nil {
		t.Error("Restore of an encrypted archive with no passphrase must fail")
	}
}

// TestExtractVolumeTar round-trips a volume archive through an encrypted backup:
// a fake VolumeReader supplies a tar, and ExtractVolumeTar reads it back.
func TestExtractVolumeTar(t *testing.T) {
	dbPath, bpDir, backupDir := setupTestData(t)
	dest := filepath.Join(backupDir, "vol.pcg-backup")
	const pass = "vol-pass-vol-pass"

	// A tiny tar to stand in for a volume export.
	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)
	_ = tw.WriteHeader(&tar.Header{Name: "hello.txt", Mode: 0o600, Size: 5})
	_, _ = tw.Write([]byte("world"))
	_ = tw.Close()
	tarBytes := tarBuf.Bytes()

	vols := []backup.AppVolume{{AppID: "memos", ContainerName: "pcg-memos", ContainerPath: "/data"}}
	readVolume := func(container, path string) (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(tarBytes)), nil
	}

	if err := backup.Create(dbPath, bpDir, dest, pass, vols, readVolume, nil, nil); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// pathBase is the base name of the container path (/data -> "data").
	tr, closeFn, err := backup.ExtractVolumeTar(dest, pass, "memos", "data")
	if err != nil {
		t.Fatalf("ExtractVolumeTar: %v", err)
	}
	defer closeFn()

	hdr, err := tr.Next()
	if err != nil {
		t.Fatalf("reading extracted tar: %v", err)
	}
	if hdr.Name != "hello.txt" {
		t.Errorf("tar entry = %q, want hello.txt", hdr.Name)
	}
	body, _ := io.ReadAll(tr)
	if string(body) != "world" {
		t.Errorf("tar body = %q, want world", body)
	}
}

func TestExtractVolumeTar_NotFound(t *testing.T) {
	dbPath, bpDir, backupDir := setupTestData(t)
	dest := filepath.Join(backupDir, "novol.pcg-backup")
	if err := backup.Create(dbPath, bpDir, dest, "", nil, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	if _, _, err := backup.ExtractVolumeTar(dest, "", "nope", "data"); err == nil {
		t.Error("ExtractVolumeTar must error when the volume is absent")
	}
}
