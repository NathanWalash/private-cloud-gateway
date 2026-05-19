// Package api implements the REST API endpoints for the dashboard.
package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"log/slog"

	"github.com/NathanWalash/private-cloud-gateway/apps/core/internal/blueprint"
	"github.com/NathanWalash/private-cloud-gateway/apps/core/internal/caddy"
	"github.com/NathanWalash/private-cloud-gateway/apps/core/internal/docker"
)

// Handler provides general API endpoints.
type Handler struct {
	db           *sql.DB
	startTime    time.Time
	version      string
	docker       *docker.Manager
	caddy        *caddy.Manager
	blueprintDir string
	cookieDomain string
}

func NewHandler(db *sql.DB, version string, dm *docker.Manager, cm *caddy.Manager, blueprintDir, cookieDomain string) *Handler {
	return &Handler{
		db:           db,
		startTime:    time.Now(),
		version:      version,
		docker:       dm,
		caddy:        cm,
		blueprintDir: blueprintDir,
		cookieDomain: cookieDomain,
	}
}

// ── Status ────────────────────────────────────────────────────────────────────

// Status returns server uptime and version.
// GET /api/status
func (h *Handler) Status(w http.ResponseWriter, _ *http.Request) {
	uptime := time.Since(h.startTime).Round(time.Second).String()
	w.Header().Set("Content-Type", "application/json")
	_, _ = fmt.Fprintf(w, `{"uptime":%q,"version":%q}`, uptime, h.version)
}

// ── Apps ──────────────────────────────────────────────────────────────────────

// AppRecord is the API representation of an installed app.
type AppRecord struct {
	ID            int64  `json:"id"`
	BlueprintID   string `json:"blueprint_id"`
	Name          string `json:"name"`
	Icon          string `json:"icon"`
	Subdomain     string `json:"subdomain"`
	URL           string `json:"url"`
	Status        string `json:"status"`
	InternalPort  int    `json:"internal_port"`
	ContainerName string `json:"container_name"`
}

// Apps returns the list of installed apps.
// GET /api/apps
func (h *Handler) Apps(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT id, blueprint_id, name, icon, subdomain, internal_port, container_name, status
		 FROM apps ORDER BY name`)
	if err != nil {
		jsonErr(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	apps := make([]AppRecord, 0)
	for rows.Next() {
		var a AppRecord
		if err := rows.Scan(&a.ID, &a.BlueprintID, &a.Name, &a.Icon, &a.Subdomain,
			&a.InternalPort, &a.ContainerName, &a.Status); err != nil {
			continue
		}
		a.URL = fmt.Sprintf("http://%s.%s", a.Subdomain, h.cookieDomain)
		apps = append(apps, a)
	}

	jsonOK(w, apps)
}

// InstallRequest is the body for POST /api/apps/install.
type InstallRequest struct {
	BlueprintID string `json:"blueprint_id"`
}

// Install reads a blueprint from disk and installs the app.
// POST /api/apps/install
func (h *Handler) Install(w http.ResponseWriter, r *http.Request) {
	var req InstallRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.BlueprintID == "" {
		jsonErr(w, "blueprint_id is required", http.StatusBadRequest)
		return
	}

	// Load blueprint from disk.
	bpPath := filepath.Join(h.blueprintDir, req.BlueprintID+".yaml")
	data, err := os.ReadFile(bpPath)
	if err != nil {
		jsonErr(w, "blueprint not found", http.StatusNotFound)
		return
	}

	bp, err := blueprint.Parse(data)
	if err != nil {
		jsonErr(w, "invalid blueprint: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Check not already installed.
	var existing int
	_ = h.db.QueryRowContext(r.Context(),
		"SELECT COUNT(*) FROM apps WHERE blueprint_id = ?", bp.ID).Scan(&existing)
	if existing > 0 {
		jsonErr(w, "app already installed", http.StatusConflict)
		return
	}

	// Install Docker container.
	if err := h.docker.Install(r.Context(), bp); err != nil {
		slog.Error("docker install failed", "app", bp.ID, "err", err)
		jsonErr(w, "docker install failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Start the container.
	if err := h.docker.Start(r.Context(), bp.ContainerName()); err != nil {
		slog.Error("docker start failed", "app", bp.ID, "err", err)
		jsonErr(w, "start failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Register Caddy route.
	if h.caddy != nil {
		if err := h.caddy.RegisterApp(bp.Route.Subdomain, bp.ContainerName(), bp.Route.InternalPort); err != nil {
			slog.Warn("caddy route registration failed", "app", bp.ID, "err", err)
			// Non-fatal — app is running, route can be added manually.
		}
	}

	// Record in DB.
	icon := bp.Icon
	if icon == "" {
		icon = "📦"
	}
	res, err := h.db.ExecContext(r.Context(),
		`INSERT INTO apps (blueprint_id, name, icon, subdomain, internal_port, image, container_name, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, 'running')`,
		bp.ID, bp.Name, icon, bp.Route.Subdomain, bp.Route.InternalPort,
		bp.Container.Image, bp.ContainerName(),
	)
	if err != nil {
		slog.Error("db insert app failed", "err", err)
		jsonErr(w, "db error", http.StatusInternalServerError)
		return
	}

	id, _ := res.LastInsertId()
	slog.Info("app installed", "app", bp.ID, "id", id)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_, _ = fmt.Fprintf(w, `{"id":%d,"status":"running"}`, id)
}

// Uninstall stops and removes an app.
// DELETE /api/apps/:id
func (h *Handler) Uninstall(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		jsonErr(w, "invalid id", http.StatusBadRequest)
		return
	}

	var containerName, subdomain string
	if err := h.db.QueryRowContext(r.Context(),
		"SELECT container_name, subdomain FROM apps WHERE id = ?", id,
	).Scan(&containerName, &subdomain); err != nil {
		jsonErr(w, "app not found", http.StatusNotFound)
		return
	}

	if h.caddy != nil {
		_ = h.caddy.DeregisterApp(subdomain)
	}

	if err := h.docker.Remove(r.Context(), containerName); err != nil {
		slog.Warn("docker remove failed", "container", containerName, "err", err)
	}

	if _, err := h.db.ExecContext(r.Context(), "DELETE FROM apps WHERE id = ?", id); err != nil {
		jsonErr(w, "db error", http.StatusInternalServerError)
		return
	}

	slog.Info("app uninstalled", "id", id, "container", containerName)
	w.WriteHeader(http.StatusNoContent)
}

// Start starts a stopped app container.
// POST /api/apps/:id/start
func (h *Handler) StartApp(w http.ResponseWriter, r *http.Request) {
	h.lifecycleAction(w, r, "start", func(containerName string) error {
		return h.docker.Start(r.Context(), containerName)
	})
}

// Stop stops a running app container.
// POST /api/apps/:id/stop
func (h *Handler) StopApp(w http.ResponseWriter, r *http.Request) {
	h.lifecycleAction(w, r, "stop", func(containerName string) error {
		return h.docker.Stop(r.Context(), containerName)
	})
}

// Restart restarts an app container.
// POST /api/apps/:id/restart
func (h *Handler) RestartApp(w http.ResponseWriter, r *http.Request) {
	h.lifecycleAction(w, r, "restart", func(containerName string) error {
		return h.docker.Restart(r.Context(), containerName)
	})
}

func (h *Handler) lifecycleAction(w http.ResponseWriter, r *http.Request, action string, fn func(string) error) {
	id, err := pathID(r)
	if err != nil {
		jsonErr(w, "invalid id", http.StatusBadRequest)
		return
	}

	var containerName string
	if err := h.db.QueryRowContext(r.Context(),
		"SELECT container_name FROM apps WHERE id = ?", id,
	).Scan(&containerName); err != nil {
		jsonErr(w, "app not found", http.StatusNotFound)
		return
	}

	if err := fn(containerName); err != nil {
		slog.Error("lifecycle action failed", "action", action, "container", containerName, "err", err)
		jsonErr(w, action+" failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Determine new status.
	status := map[string]string{
		"start":   "running",
		"stop":    "stopped",
		"restart": "running",
	}[action]

	_, _ = h.db.ExecContext(r.Context(),
		"UPDATE apps SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", status, id)

	slog.Info("app lifecycle", "action", action, "id", id)
	w.Header().Set("Content-Type", "application/json")
	_, _ = fmt.Fprintf(w, `{"status":%q}`, status)
}

// Blueprints lists available blueprints from the blueprints directory.
// GET /api/blueprints
func (h *Handler) Blueprints(w http.ResponseWriter, r *http.Request) {
	entries, err := os.ReadDir(h.blueprintDir)
	if err != nil {
		jsonOK(w, []any{})
		return
	}

	type bpSummary struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Icon        string `json:"icon"`
		Category    string `json:"category"`
	}

	var summaries []bpSummary
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(h.blueprintDir, e.Name()))
		if err != nil {
			continue
		}
		bp, err := blueprint.Parse(data)
		if err != nil {
			continue
		}
		summaries = append(summaries, bpSummary{
			ID:          bp.ID,
			Name:        bp.Name,
			Description: bp.Description,
			Icon:        bp.Icon,
			Category:    bp.Category,
		})
	}

	if summaries == nil {
		summaries = []bpSummary{}
	}
	jsonOK(w, summaries)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("json encode", "err", err)
	}
}

func jsonErr(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = fmt.Fprintf(w, `{"error":%q}`, msg)
}

// pathID extracts the last path segment as an integer ID.
// Works with patterns like /api/apps/42/start → 42.
func pathID(r *http.Request) (int64, error) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	// Walk backwards to find the first integer segment.
	for i := len(parts) - 1; i >= 0; i-- {
		if id, err := strconv.ParseInt(parts[i], 10, 64); err == nil {
			return id, nil
		}
	}
	return 0, fmt.Errorf("no integer id in path %s", r.URL.Path)
}
