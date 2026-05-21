package api

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/NathanWalash/private-cloud-gateway/apps/core/internal/backup"
	"github.com/NathanWalash/private-cloud-gateway/apps/core/internal/blueprint"
)

func (h *Handler) backupPassphrase() string { return os.Getenv("CLOUD_CORE_BACKUP_PASSPHRASE") }
func (h *Handler) backupDir() string {
	if d := os.Getenv("CLOUD_CORE_BACKUP_DIR"); d != "" {
		return d
	}
	return "/backups"
}
func (h *Handler) dbPath() string {
	if d := os.Getenv("CLOUD_CORE_DATABASE_PATH"); d != "" {
		return d
	}
	return "./data/cloud-core.db"
}

// collectVolumes builds the list of AppVolume entries for all installed apps.
func (h *Handler) collectVolumes(ctx context.Context) []backup.AppVolume {
	if h.docker == nil {
		return nil
	}
	rows, err := h.db.QueryContext(ctx,
		"SELECT blueprint_id, container_name FROM apps WHERE status = 'running'")
	if err != nil {
		return nil
	}
	defer rows.Close()

	var volumes []backup.AppVolume
	for rows.Next() {
		var bpID, containerName string
		if rows.Scan(&bpID, &containerName) != nil {
			continue
		}
		bpPath := filepath.Join(h.blueprintDir, bpID+".yaml")
		data, err := os.ReadFile(bpPath)
		if err != nil {
			continue
		}
		bp, err := blueprint.Parse(data)
		if err != nil || !bp.Backup.Enabled {
			continue
		}
		for _, p := range bp.Backup.ContainerPaths {
			volumes = append(volumes, backup.AppVolume{
				AppID:         bpID,
				ContainerName: containerName,
				ContainerPath: p,
			})
		}
	}
	return volumes
}

// volumeReader wraps docker.Manager.CopyFromContainer as a backup.VolumeReader.
func (h *Handler) volumeReader(containerName, containerPath string) (io.ReadCloser, error) {
	return h.docker.CopyFromContainer(context.Background(), containerName, containerPath)
}

// BackupCreate triggers a backup (DB + blueprints + app volumes).
// POST /api/backup/create
func (h *Handler) BackupCreate(w http.ResponseWriter, r *http.Request) {
	if err := os.MkdirAll(h.backupDir(), 0o700); err != nil {
		jsonErr(w, "cannot create backup dir", http.StatusInternalServerError)
		return
	}

	name := backup.FileName(time.Now())
	destPath := filepath.Join(h.backupDir(), name)
	volumes := h.collectVolumes(r.Context())

	var vr backup.VolumeReader
	if h.docker != nil {
		vr = h.volumeReader
	}

	if err := backup.Create(h.dbPath(), h.blueprintDir, destPath, h.backupPassphrase(), volumes, vr); err != nil {
		slog.Error("backup create failed", "err", err)
		jsonErr(w, "backup failed", http.StatusInternalServerError)
		return
	}

	info, _ := os.Stat(destPath)
	var size int64
	if info != nil {
		size = info.Size()
	}

	slog.Info("backup created", "file", name, "size", size, "volumes", len(volumes))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_, _ = fmt.Fprintf(w, `{"name":%q,"size":%d,"volumes":%d}`, name, size, len(volumes))
}

// BackupList returns available backups.
// GET /api/backup/list
func (h *Handler) BackupList(w http.ResponseWriter, _ *http.Request) {
	backups, err := backup.ListBackups(h.backupDir())
	if err != nil {
		jsonOK(w, []backup.BackupInfo{})
		return
	}
	if backups == nil {
		backups = []backup.BackupInfo{}
	}
	jsonOK(w, backups)
}

// SafeEscape creates a backup and streams it directly to the browser.
// GET /api/backup/safe-escape
func (h *Handler) SafeEscape(w http.ResponseWriter, r *http.Request) {
	name := backup.FileName(time.Now())
	tmpPath := filepath.Join(os.TempDir(), name)
	defer os.Remove(tmpPath)

	volumes := h.collectVolumes(r.Context())
	var vr backup.VolumeReader
	if h.docker != nil {
		vr = h.volumeReader
	}

	if err := backup.Create(h.dbPath(), h.blueprintDir, tmpPath, h.backupPassphrase(), volumes, vr); err != nil {
		slog.Error("safe escape failed", "err", err)
		http.Error(w, "backup failed", http.StatusInternalServerError)
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
	if h.backupPassphrase() != "" {
		contentType = "application/octet-stream"
	}

	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.Header().Set("Content-Type", contentType)
	if info != nil {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))
	}
	http.ServeContent(w, r, name, time.Now(), f)

	slog.Info("safe escape downloaded", "file", name, "volumes", len(volumes))
}

// BackupRestore restores a backup archive uploaded via multipart form.
// POST /api/backup/restore
// Form fields: file (required), passphrase (optional, overrides env var)
func (h *Handler) BackupRestore(w http.ResponseWriter, r *http.Request) {
	// 64MB max upload
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		jsonErr(w, "invalid multipart form", http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		jsonErr(w, "file field required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	passphrase := r.FormValue("passphrase")
	if passphrase == "" {
		passphrase = h.backupPassphrase()
	}

	// Write upload to temp file
	tmp, err := os.CreateTemp("", "pcg-restore-*.pcg-backup")
	if err != nil {
		jsonErr(w, "cannot create temp file", http.StatusInternalServerError)
		return
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	if _, err := io.Copy(tmp, file); err != nil {
		jsonErr(w, "cannot write upload", http.StatusInternalServerError)
		return
	}
	tmp.Close()

	// Restore DB and blueprints
	if err := backup.Restore(tmp.Name(), passphrase, h.dbPath(), h.blueprintDir); err != nil {
		slog.Error("restore failed", "err", err)
		jsonErr(w, "restore failed", http.StatusInternalServerError)
		return
	}

	slog.Info("backup restored — restart required")
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"restored","message":"Restart the service to apply the restored database."}`))
}
