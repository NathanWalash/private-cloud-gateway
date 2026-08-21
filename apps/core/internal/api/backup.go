package api

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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

// collectServiceDumps builds the list of database dumps to include, one per
// installed app service that declares a backup.dump command.
func (h *Handler) collectServiceDumps(ctx context.Context) []backup.ServiceDump {
	if h.docker == nil {
		return nil
	}
	rows, err := h.db.QueryContext(ctx, "SELECT blueprint_id FROM apps WHERE status = 'running'")
	if err != nil {
		return nil
	}
	var appIDs []string
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			appIDs = append(appIDs, id)
		}
	}
	rows.Close()

	var dumps []backup.ServiceDump
	for _, bpID := range appIDs {
		data, err := os.ReadFile(filepath.Join(h.blueprintDir, bpID+".yaml"))
		if err != nil {
			continue
		}
		bp, err := blueprint.Parse(data)
		if err != nil {
			continue
		}
		for _, s := range bp.Services {
			if len(s.Backup.Dump) == 0 {
				continue
			}
			dumps = append(dumps, backup.ServiceDump{
				AppID:         bpID,
				Service:       s.Name,
				ContainerName: "pcg-" + bpID + "-" + s.Name,
				Command:       s.Backup.Dump,
			})
		}
	}
	return dumps
}

// serviceDumper wraps docker.Manager.ExecCapture as a backup.ServiceDumper.
func (h *Handler) serviceDumper(containerName string, cmd []string, out io.Writer) error {
	return h.docker.ExecCapture(context.Background(), containerName, cmd, out)
}

// BackupCreate triggers a backup (DB + blueprints + app volumes).
// POST /api/backup/create.
func (h *Handler) BackupCreate(w http.ResponseWriter, r *http.Request) {
	if err := os.MkdirAll(h.backupDir(), 0o700); err != nil {
		jsonErr(w, "cannot create backup dir", http.StatusInternalServerError)
		return
	}

	name := backup.FileName(time.Now())
	destPath := filepath.Join(h.backupDir(), name)
	volumes := h.collectVolumes(r.Context())

	var vr backup.VolumeReader
	var df backup.ServiceDumper
	if h.docker != nil {
		vr = h.volumeReader
		df = h.serviceDumper
	}
	dumps := h.collectServiceDumps(r.Context())

	if err := backup.Create(h.dbPath(), h.blueprintDir, destPath, h.backupPassphrase(), volumes, vr, dumps, df); err != nil {
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
	// Store last-run timestamp for dashboard display
	_, _ = h.db.ExecContext(r.Context(),
		`INSERT INTO settings(key,value,updated_at) VALUES('LAST_BACKUP_TIME',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)
		 ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=CURRENT_TIMESTAMP`)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_, _ = fmt.Fprintf(w, `{"name":%q,"size":%d,"volumes":%d}`, name, size, len(volumes))
}

// BackupList returns available backups.
// GET /api/backup/list.
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
// GET /api/backup/safe-escape.
func (h *Handler) SafeEscape(w http.ResponseWriter, r *http.Request) {
	name := backup.FileName(time.Now())
	tmpPath := filepath.Join(os.TempDir(), name)
	defer os.Remove(tmpPath)

	volumes := h.collectVolumes(r.Context())
	var vr backup.VolumeReader
	var df backup.ServiceDumper
	if h.docker != nil {
		vr = h.volumeReader
		df = h.serviceDumper
	}
	dumps := h.collectServiceDumps(r.Context())

	if err := backup.Create(h.dbPath(), h.blueprintDir, tmpPath, h.backupPassphrase(), volumes, vr, dumps, df); err != nil {
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
// Form fields: file (required), passphrase (optional, overrides env var).
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

	// Best-effort: load database dumps into any service containers that are
	// currently running. (After a fresh restore the apps may not be recreated
	// yet; reinstalling an app then restoring will load its data.)
	_ = backup.ForEachServiceDump(tmp.Name(), passphrase, func(appID, service string, data []byte) error {
		if err := h.loadServiceDump(r.Context(), appID, service, data); err != nil {
			slog.Warn("restore: load service dump skipped", "app", appID, "service", service, "err", err)
		}
		return nil
	})

	slog.Info("backup restored — restart required")
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"restored","message":"Restart the service to apply the restored database."}`))
}

// loadServiceDump loads a database dump into a running service container: it
// copies the dump in and runs the blueprint's service restore command against
// it. Best-effort — skipped if the blueprint/service/restore command is missing.
func (h *Handler) loadServiceDump(ctx context.Context, appID, service string, data []byte) error {
	if h.docker == nil {
		return nil
	}
	// appID/service come from the uploaded archive's entry paths — validate them
	// before using them in a file path or a container name (path-traversal guard).
	if err := blueprint.ValidateBlueprintID(appID); err != nil {
		return fmt.Errorf("invalid app id %q in archive: %w", appID, err)
	}
	if !blueprint.ValidServiceName(service) {
		return fmt.Errorf("invalid service name %q in archive", service)
	}
	bp, err := blueprint.Parse(mustReadFile(filepath.Join(h.blueprintDir, appID+".yaml")))
	if err != nil {
		return err
	}
	var restore []string
	for _, s := range bp.Services {
		if s.Name == service {
			restore = s.Backup.Restore
		}
	}
	if len(restore) == 0 {
		return nil // nothing to do
	}
	container := "pcg-" + appID + "-" + service

	// Copy the dump into the container as a tar, then run: <restore> < dump.
	tar, err := singleFileTar("pcg-restore.sql", data)
	if err != nil {
		return err
	}
	if err := h.docker.CopyToContainer(ctx, container, "/tmp", tar); err != nil {
		return err
	}
	cmd := []string{"sh", "-c", strings.Join(restore, " ") + " < /tmp/pcg-restore.sql"}
	return h.docker.ExecCapture(ctx, container, cmd, io.Discard)
}

// mustReadFile returns file bytes or nil (Parse will then error cleanly).
func mustReadFile(path string) []byte {
	b, _ := os.ReadFile(path)
	return b
}

// singleFileTar builds an in-memory tar archive containing one file.
func singleFileTar(name string, data []byte) (io.Reader, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: int64(len(data))}); err != nil {
		return nil, err
	}
	if _, err := tw.Write(data); err != nil {
		return nil, err
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	return &buf, nil
}

// BackupLastRun returns the timestamp of the most recent backup.
// GET /api/backup/last-run.
func (h *Handler) BackupLastRun(w http.ResponseWriter, r *http.Request) {
	var t string
	h.db.QueryRowContext(r.Context(), "SELECT value FROM settings WHERE key='LAST_BACKUP_TIME'").Scan(&t) //nolint:errcheck
	w.Header().Set("Content-Type", "application/json")
	if t == "" {
		_, _ = w.Write([]byte(`{"last_run":null}`))
	} else {
		_, _ = fmt.Fprintf(w, `{"last_run":%q}`, t)
	}
}
