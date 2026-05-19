package api

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/NathanWalash/private-cloud-gateway/apps/core/internal/backup"
)

// BackupCreate triggers a backup and returns the filename.
// POST /api/backup/create
func (h *Handler) BackupCreate(w http.ResponseWriter, r *http.Request) {
	passphrase := os.Getenv("CLOUD_CORE_BACKUP_PASSPHRASE")
	destDir := os.Getenv("CLOUD_CORE_BACKUP_DIR")
	if destDir == "" {
		destDir = "/backups"
	}

	if err := os.MkdirAll(destDir, 0o700); err != nil {
		jsonErr(w, "cannot create backup dir", http.StatusInternalServerError)
		return
	}

	name := backup.FileName(time.Now())
	destPath := filepath.Join(destDir, name)

	dbPath := os.Getenv("CLOUD_CORE_DATABASE_PATH")
	if dbPath == "" {
		dbPath = "./data/cloud-core.db"
	}
	bpDir := os.Getenv("CLOUD_CORE_BLUEPRINT_DIR")
	if bpDir == "" {
		bpDir = "/blueprints"
	}

	if err := backup.Create(dbPath, bpDir, destPath, passphrase); err != nil {
		slog.Error("backup create failed", "err", err)
		jsonErr(w, "backup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	info, _ := os.Stat(destPath)
	var size int64
	if info != nil {
		size = info.Size()
	}

	slog.Info("backup created", "file", name, "size", size)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_, _ = fmt.Fprintf(w, `{"name":%q,"size":%d}`, name, size)
}

// BackupList returns available backups.
// GET /api/backup/list
func (h *Handler) BackupList(w http.ResponseWriter, _ *http.Request) {
	destDir := os.Getenv("CLOUD_CORE_BACKUP_DIR")
	if destDir == "" {
		destDir = "/backups"
	}

	backups, err := backup.ListBackups(destDir)
	if err != nil {
		jsonOK(w, []backup.BackupInfo{})
		return
	}
	if backups == nil {
		backups = []backup.BackupInfo{}
	}
	jsonOK(w, backups)
}

// SafeEscape creates a backup on the fly and streams it directly to the browser.
// GET /api/backup/safe-escape
func (h *Handler) SafeEscape(w http.ResponseWriter, _ *http.Request) {
	passphrase := os.Getenv("CLOUD_CORE_BACKUP_PASSPHRASE")
	dbPath := os.Getenv("CLOUD_CORE_DATABASE_PATH")
	if dbPath == "" {
		dbPath = "./data/cloud-core.db"
	}
	bpDir := os.Getenv("CLOUD_CORE_BLUEPRINT_DIR")
	if bpDir == "" {
		bpDir = "/blueprints"
	}

	name := backup.FileName(time.Now())
	tmpPath := filepath.Join(os.TempDir(), name)
	defer os.Remove(tmpPath)

	if err := backup.Create(dbPath, bpDir, tmpPath, passphrase); err != nil {
		slog.Error("safe escape failed", "err", err)
		http.Error(w, "backup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	f, err := os.Open(tmpPath)
	if err != nil {
		http.Error(w, "cannot open backup", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	info, _ := f.Stat()
	contentType := "application/zip"
	if passphrase != "" {
		contentType = "application/octet-stream"
	}

	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.Header().Set("Content-Type", contentType)
	if info != nil {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))
	}
	http.ServeContent(w, &http.Request{Method: "GET"}, name, time.Now(), f)

	slog.Info("safe escape downloaded", "file", name)
}
