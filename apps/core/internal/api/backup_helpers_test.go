package api

import (
	"archive/tar"
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSingleFileTar_RoundTrip(t *testing.T) {
	r, err := singleFileTar("dump.sql", []byte("SELECT 1;"))
	if err != nil {
		t.Fatalf("singleFileTar: %v", err)
	}
	tr := tar.NewReader(r)
	hdr, err := tr.Next()
	if err != nil {
		t.Fatalf("reading tar header: %v", err)
	}
	if hdr.Name != "dump.sql" {
		t.Errorf("name = %q, want dump.sql", hdr.Name)
	}
	if hdr.Size != int64(len("SELECT 1;")) {
		t.Errorf("size = %d, want %d", hdr.Size, len("SELECT 1;"))
	}
	body, _ := io.ReadAll(tr)
	if string(body) != "SELECT 1;" {
		t.Errorf("body = %q, want SELECT 1;", body)
	}
	if _, err := tr.Next(); err != io.EOF {
		t.Error("archive should contain exactly one file")
	}
}

func TestMustReadFile_MissingReturnsNil(t *testing.T) {
	if b := mustReadFile("/no/such/file/really.yaml"); b != nil {
		t.Errorf("mustReadFile on a missing path = %v, want nil", b)
	}
}

func TestBackupList_EmptyDirReturnsEmptyArray(t *testing.T) {
	t.Setenv("CLOUD_CORE_BACKUP_DIR", t.TempDir())
	h := &Handler{startTime: time.Now()}

	rec := httptest.NewRecorder()
	h.BackupList(rec, httptest.NewRequest("GET", "/api/backup/list", nil))
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	// Must serialise as [] (never null) so the UI can iterate it safely.
	if got := strings.TrimSpace(rec.Body.String()); got != "[]" {
		t.Errorf("body = %q, want []", got)
	}
	var list []any
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("body is not valid JSON array: %v", err)
	}
}

// loadServiceDump takes appID and service names straight from an uploaded
// archive's entry paths, so a malicious archive must not be able to escape into
// an arbitrary blueprint path or container name. These guards run before any
// filesystem or Docker access.
func TestLoadServiceDump_RejectsPathTraversal(t *testing.T) {
	h := &Handler{docker: &fakeDocker{}, blueprintDir: t.TempDir()}
	ctx := context.Background()

	if err := h.loadServiceDump(ctx, "../../etc/passwd", "db", []byte("x")); err == nil {
		t.Error("a path-traversal app id must be rejected")
	}
	if err := h.loadServiceDump(ctx, "umami", "../evil", []byte("x")); err == nil {
		t.Error("an invalid service name must be rejected")
	}
}

func TestLoadServiceDump_NoDockerIsNoOp(t *testing.T) {
	h := &Handler{blueprintDir: t.TempDir()} // docker == nil
	if err := h.loadServiceDump(context.Background(), "umami", "db", []byte("x")); err != nil {
		t.Errorf("with no docker manager loadServiceDump should be a no-op, got %v", err)
	}
}
