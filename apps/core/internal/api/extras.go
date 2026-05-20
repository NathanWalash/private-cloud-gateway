package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"log/slog"

	"github.com/NathanWalash/private-cloud-gateway/apps/core/internal/blueprint"
)

// ── App logs ─────────────────────────────────────────────────────────────────

// GET /api/apps/:id/logs?tail=100
func (h *Handler) Logs(w http.ResponseWriter, r *http.Request) {
	if h.docker == nil {
		jsonErr(w, "Docker unavailable", http.StatusServiceUnavailable)
		return
	}
	id, err := pathID(r)
	if err != nil {
		jsonErr(w, "invalid id", http.StatusBadRequest)
		return
	}
	var containerName string
	if err := h.db.QueryRowContext(r.Context(),
		"SELECT container_name FROM apps WHERE id = ?", id).Scan(&containerName); err != nil {
		jsonErr(w, "app not found", http.StatusNotFound)
		return
	}
	tail := 150
	if t := r.URL.Query().Get("tail"); t != "" {
		if n, err := strconv.Atoi(t); err == nil && n > 0 && n <= 1000 {
			tail = n
		}
	}
	logs, err := h.docker.Logs(r.Context(), containerName, tail)
	if err != nil {
		jsonErr(w, "logs unavailable: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	b, _ := json.Marshal(map[string]string{"lines": logs})
	_, _ = w.Write(b)
}

// ── App update ────────────────────────────────────────────────────────────────

// POST /api/apps/:id/update — pull latest image and recreate the container.
func (h *Handler) UpdateApp(w http.ResponseWriter, r *http.Request) {
	if h.docker == nil {
		jsonErr(w, "Docker unavailable", http.StatusServiceUnavailable)
		return
	}
	id, err := pathID(r)
	if err != nil {
		jsonErr(w, "invalid id", http.StatusBadRequest)
		return
	}
	var bpID, containerName, image string
	if err := h.db.QueryRowContext(r.Context(),
		"SELECT blueprint_id, container_name, image FROM apps WHERE id = ?", id,
	).Scan(&bpID, &containerName, &image); err != nil {
		jsonErr(w, "app not found", http.StatusNotFound)
		return
	}

	slog.Info("updating app", "id", id, "image", image)
	if err := h.docker.UpdateImage(r.Context(), image); err != nil {
		jsonErr(w, "pull failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	_ = h.docker.Stop(r.Context(), containerName)
	_ = h.docker.Remove(r.Context(), containerName)

	bpPath := filepath.Join(h.blueprintDir, bpID+".yaml")
	data, err := os.ReadFile(bpPath)
	if err != nil {
		jsonErr(w, "blueprint not found", http.StatusNotFound)
		return
	}
	bp, err := blueprint.Parse(data)
	if err != nil {
		jsonErr(w, "blueprint parse error", http.StatusInternalServerError)
		return
	}
	if err := h.docker.Install(r.Context(), bp); err != nil {
		jsonErr(w, "recreate failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if err := h.docker.Start(r.Context(), containerName); err != nil {
		jsonErr(w, "start failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	_, _ = h.db.ExecContext(r.Context(),
		"UPDATE apps SET status='running', image=?, updated_at=CURRENT_TIMESTAMP WHERE id=?",
		bp.Container.Image, id)

	slog.Info("app updated", "id", id)
	h.reloadCaddy(r.Context())
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"updated"}`))
}

// ── Settings ──────────────────────────────────────────────────────────────────

type Setting struct {
	Key       string `json:"key"`
	Value     string `json:"value"`
	UpdatedAt string `json:"updated_at"`
}

// GET /api/settings
func (h *Handler) GetSettings(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(),
		"SELECT key, value, updated_at FROM settings ORDER BY key")
	if err != nil {
		jsonOK(w, []Setting{})
		return
	}
	defer rows.Close()
	settings := []Setting{}
	for rows.Next() {
		var s Setting
		if rows.Scan(&s.Key, &s.Value, &s.UpdatedAt) == nil {
			settings = append(settings, s)
		}
	}
	jsonOK(w, settings)
}

// PUT /api/settings/:key
func (h *Handler) PutSetting(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	key := parts[len(parts)-1]
	if key == "" || key == "settings" {
		jsonErr(w, "key required", http.StatusBadRequest)
		return
	}
	var body struct {
		Value string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonErr(w, "invalid body", http.StatusBadRequest)
		return
	}
	_, err := h.db.ExecContext(r.Context(),
		`INSERT INTO settings(key,value,updated_at) VALUES(?,?,CURRENT_TIMESTAMP)
		 ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=CURRENT_TIMESTAMP`,
		key, body.Value,
	)
	if err != nil {
		jsonErr(w, "db error", http.StatusInternalServerError)
		return
	}
	slog.Info("setting updated", "key", key)
	w.Header().Set("Content-Type", "application/json")
	_, _ = fmt.Fprintf(w, `{"key":%q,"value":%q}`, key, body.Value)
}

// ── Audit log ─────────────────────────────────────────────────────────────────

type AuditEntry struct {
	ID        int64  `json:"id"`
	Action    string `json:"action"`
	Actor     string `json:"actor"`
	Detail    string `json:"detail"`
	CreatedAt string `json:"created_at"`
}

// GET /api/audit?limit=50
func (h *Handler) AuditLog(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT id, action, COALESCE(actor,''), COALESCE(detail,''), created_at
		 FROM audit_log ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		jsonOK(w, []AuditEntry{})
		return
	}
	defer rows.Close()
	entries := []AuditEntry{}
	for rows.Next() {
		var e AuditEntry
		if rows.Scan(&e.ID, &e.Action, &e.Actor, &e.Detail, &e.CreatedAt) == nil {
			entries = append(entries, e)
		}
	}
	jsonOK(w, entries)
}

// ── API monitors ──────────────────────────────────────────────────────────────

type Monitor struct {
	ID          int64    `json:"id"`
	Name        string   `json:"name"`
	URL         string   `json:"url"`
	Status      string   `json:"status"`
	StatusCode  *int     `json:"status_code"`
	LatencyMs   *int     `json:"latency_ms"`
	LastChecked *string  `json:"last_checked"`
}

// GET /api/monitors
func (h *Handler) MonitorList(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(),
		"SELECT id, name, url, status, status_code, latency_ms, last_checked FROM monitors ORDER BY name")
	if err != nil {
		jsonOK(w, []Monitor{})
		return
	}
	defer rows.Close()
	monitors := []Monitor{}
	for rows.Next() {
		var m Monitor
		if rows.Scan(&m.ID, &m.Name, &m.URL, &m.Status, &m.StatusCode, &m.LatencyMs, &m.LastChecked) == nil {
			monitors = append(monitors, m)
		}
	}
	jsonOK(w, monitors)
}

// POST /api/monitors
func (h *Handler) MonitorCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" || body.URL == "" {
		jsonErr(w, "name and url are required", http.StatusBadRequest)
		return
	}
	if _, err := url.ParseRequestURI(body.URL); err != nil {
		jsonErr(w, "invalid url", http.StatusBadRequest)
		return
	}
	res, err := h.db.ExecContext(r.Context(),
		"INSERT INTO monitors(name, url) VALUES(?, ?)", body.Name, body.URL)
	if err != nil {
		jsonErr(w, "url may already exist", http.StatusConflict)
		return
	}
	id, _ := res.LastInsertId()
	go RunMonitorCheck(h.db, id, body.URL)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_, _ = fmt.Fprintf(w, `{"id":%d}`, id)
}

// DELETE /api/monitors/:id
func (h *Handler) MonitorDelete(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		jsonErr(w, "invalid id", http.StatusBadRequest)
		return
	}
	_, _ = h.db.ExecContext(r.Context(), "DELETE FROM monitors WHERE id=?", id)
	w.WriteHeader(http.StatusNoContent)
}

// RunMonitorCheck pings a URL and records status in the DB. Safe to call in a goroutine.
func RunMonitorCheck(db *sql.DB, id int64, targetURL string) {
	client := &http.Client{Timeout: 10 * time.Second}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		db.Exec("UPDATE monitors SET status='down', last_checked=CURRENT_TIMESTAMP WHERE id=?", id)
		return
	}

	start := time.Now()
	resp, err := client.Do(req)
	latency := int(time.Since(start).Milliseconds())

	var status string
	var code *int
	if err != nil {
		status = "down"
	} else {
		resp.Body.Close()
		c := resp.StatusCode
		code = &c
		if c < 400 {
			status = "up"
		} else {
			status = "down"
		}
	}
	db.Exec(
		"UPDATE monitors SET status=?, status_code=?, latency_ms=?, last_checked=CURRENT_TIMESTAMP WHERE id=?",
		status, code, latency, id,
	)
}

// PollAllMonitors checks every monitor. Called on a timer from main.go.
func PollAllMonitors(db *sql.DB) {
	rows, err := db.QueryContext(context.Background(), "SELECT id, url FROM monitors")
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var u string
		if rows.Scan(&id, &u) == nil {
			go RunMonitorCheck(db, id, u)
		}
	}
}
