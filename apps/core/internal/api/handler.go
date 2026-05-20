// Package api implements the REST API endpoints for the dashboard.
package api

import (
	"context"
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

// POST /api/apps/install
func (h *Handler) Install(w http.ResponseWriter, r *http.Request) {
	if h.docker == nil {
		jsonErr(w, "Docker is not available. Check that /var/run/docker.sock is mounted.", http.StatusServiceUnavailable)
		return
	}

	var req InstallRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.BlueprintID == "" {
		jsonErr(w, "blueprint_id is required", http.StatusBadRequest)
		return
	}

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

	// Check not already installed in DB.
	var existing int
	_ = h.db.QueryRowContext(r.Context(),
		"SELECT COUNT(*) FROM apps WHERE blueprint_id = ?", bp.ID).Scan(&existing)
	if existing > 0 {
		jsonErr(w, "app already installed", http.StatusConflict)
		return
	}

	// Remove any stale container with the same name before creating.
	// This handles the case where a previous install partially succeeded.
	_ = h.docker.Remove(r.Context(), bp.ContainerName())

	if err := h.docker.Install(r.Context(), bp); err != nil {
		slog.Error("docker install failed", "app", bp.ID, "err", err)
		jsonErr(w, "docker install failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := h.docker.Start(r.Context(), bp.ContainerName()); err != nil {
		slog.Error("docker start failed", "app", bp.ID, "err", err)
		jsonErr(w, "start failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Wait up to 5 seconds for the container to stay up — catches immediate crashes
	// caused by permission errors, missing config, etc.
	initialStatus := h.docker.StatusAfterStart(r.Context(), bp.ContainerName(), 5)

	icon := bp.Icon
	if icon == "" {
		icon = "📦"
	}
	res, err := h.db.ExecContext(r.Context(),
		`INSERT INTO apps (blueprint_id, name, icon, subdomain, internal_port, image, container_name, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		bp.ID, bp.Name, icon, bp.Route.Subdomain, bp.Route.InternalPort,
		bp.Container.Image, bp.ContainerName(), initialStatus,
	)
	if err != nil {
		slog.Error("db insert app failed", "err", err)
		jsonErr(w, "db error", http.StatusInternalServerError)
		return
	}

	id, _ := res.LastInsertId()
	slog.Info("app installed", "app", bp.ID, "id", id)

	// Reload Caddy with all app routes now including the new one.
	h.reloadCaddy(r.Context())

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_, _ = fmt.Fprintf(w, `{"id":%d,"status":"running"}`, id)
}

// DELETE /api/apps/:id
func (h *Handler) Uninstall(w http.ResponseWriter, r *http.Request) {
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

	if h.docker != nil {
		if err := h.docker.Remove(r.Context(), containerName); err != nil {
			slog.Warn("docker remove failed", "container", containerName, "err", err)
		}
	}

	if _, err := h.db.ExecContext(r.Context(), "DELETE FROM apps WHERE id = ?", id); err != nil {
		jsonErr(w, "db error", http.StatusInternalServerError)
		return
	}

	slog.Info("app uninstalled", "id", id, "container", containerName)

	// Reload Caddy without the removed app's route.
	h.reloadCaddy(r.Context())

	w.WriteHeader(http.StatusNoContent)
}

// POST /api/apps/:id/start
func (h *Handler) StartApp(w http.ResponseWriter, r *http.Request) {
	h.lifecycleAction(w, r, "start", func(cn string) error { return h.docker.Start(r.Context(), cn) })
}

// POST /api/apps/:id/stop
func (h *Handler) StopApp(w http.ResponseWriter, r *http.Request) {
	h.lifecycleAction(w, r, "stop", func(cn string) error { return h.docker.Stop(r.Context(), cn) })
}

// POST /api/apps/:id/restart
func (h *Handler) RestartApp(w http.ResponseWriter, r *http.Request) {
	h.lifecycleAction(w, r, "restart", func(cn string) error { return h.docker.Restart(r.Context(), cn) })
}

func (h *Handler) lifecycleAction(w http.ResponseWriter, r *http.Request, action string, fn func(string) error) {
	if h.docker == nil {
		jsonErr(w, "Docker is not available", http.StatusServiceUnavailable)
		return
	}

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

	status := map[string]string{"start": "running", "stop": "stopped", "restart": "running"}[action]
	_, _ = h.db.ExecContext(r.Context(),
		"UPDATE apps SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", status, id)

	slog.Info("app lifecycle", "action", action, "id", id)
	w.Header().Set("Content-Type", "application/json")
	_, _ = fmt.Fprintf(w, `{"status":%q}`, status)
}

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

// reloadCaddy queries all installed apps and reloads Caddy with the complete config.
func (h *Handler) reloadCaddy(ctx context.Context) {
	if h.caddy == nil {
		return
	}
	routes, err := h.queryAppRoutes(ctx)
	if err != nil {
		slog.Warn("caddy reload: failed to query routes", "err", err)
		return
	}
	if err := h.caddy.ReloadAll(ctx, routes); err != nil {
		slog.Warn("caddy reload failed", "err", err)
	} else {
		slog.Info("caddy reloaded", "routes", len(routes))
	}
}

// queryAppRoutes returns caddy.AppRoute for all apps in the DB.
func (h *Handler) queryAppRoutes(ctx context.Context) ([]caddy.AppRoute, error) {
	rows, err := h.db.QueryContext(ctx,
		"SELECT subdomain, container_name, internal_port FROM apps ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var routes []caddy.AppRoute
	for rows.Next() {
		var r caddy.AppRoute
		if err := rows.Scan(&r.Subdomain, &r.ContainerName, &r.InternalPort); err != nil {
			continue
		}
		routes = append(routes, r)
	}
	return routes, nil
}

// QueryAppRoutes is the exported version used by main.go for startup route sync.
func (h *Handler) QueryAppRoutes(ctx context.Context) ([]caddy.AppRoute, error) {
	return h.queryAppRoutes(ctx)
}

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

func pathID(r *http.Request) (int64, error) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	for i := len(parts) - 1; i >= 0; i-- {
		if id, err := strconv.ParseInt(parts[i], 10, 64); err == nil {
			return id, nil
		}
	}
	return 0, fmt.Errorf("no integer id in path %s", r.URL.Path)
}
