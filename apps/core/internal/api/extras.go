package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"log/slog"

	"github.com/NathanWalash/private-cloud-gateway/apps/core/internal/blueprint"
)

// ── App logs ─────────────────────────────────────────────────────────────────

// GET /api/apps/:id/logs?tail=100.
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
		jsonErr(w, "logs unavailable", http.StatusInternalServerError)
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
		jsonErr(w, "image pull failed", http.StatusInternalServerError)
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
	if err := h.docker.Install(r.Context(), bp.Render(h.cookieDomain, h.scheme)); err != nil {
		jsonErr(w, "container recreate failed", http.StatusInternalServerError)
		return
	}
	if err := h.docker.Start(r.Context(), containerName); err != nil {
		jsonErr(w, "failed to start container", http.StatusInternalServerError)
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

// GET /api/settings.
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

// PUT /api/settings/:key.
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

// GET /api/audit?limit=50&offset=0.
func (h *Handler) AuditLog(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}
	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		if n, err := strconv.Atoi(o); err == nil && n >= 0 {
			offset = n
		}
	}
	// Return total count for pagination UI
	var total int
	h.db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM audit_log").Scan(&total) //nolint:errcheck

	rows, err := h.db.QueryContext(r.Context(),
		`SELECT id, action, COALESCE(actor,''), COALESCE(detail,''), created_at
		 FROM audit_log ORDER BY id DESC LIMIT ? OFFSET ?`, limit, offset)
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
	w.Header().Set("Content-Type", "application/json")
	b, _ := json.Marshal(map[string]any{"entries": entries, "total": total, "limit": limit, "offset": offset})
	_, _ = w.Write(b)
}

// ── API monitors ──────────────────────────────────────────────────────────────

type Monitor struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	URL         string  `json:"url"`
	Status      string  `json:"status"`
	StatusCode  *int    `json:"status_code"`
	LatencyMs   *int    `json:"latency_ms"`
	LastChecked *string `json:"last_checked"`
}

// GET /api/monitors.
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

// POST /api/monitors.
func (h *Handler) MonitorCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" || body.URL == "" {
		jsonErr(w, "name and url are required", http.StatusBadRequest)
		return
	}
	if err := validateMonitorURL(body.URL); err != nil {
		jsonErr(w, err.Error(), http.StatusBadRequest)
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

// DELETE /api/monitors/:id.
func (h *Handler) MonitorDelete(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		jsonErr(w, "invalid id", http.StatusBadRequest)
		return
	}
	_, _ = h.db.ExecContext(r.Context(), "DELETE FROM monitors WHERE id=?", id)
	w.WriteHeader(http.StatusNoContent)
}

// blockedIP reports whether an IP is one that monitors must not reach: loopback,
// private (RFC1918 / IPv6 ULA), link-local (incl. the cloud metadata address
// 169.254.169.254), or the unspecified address. Blocking these prevents an
// authenticated user from turning monitors into a server-side request forgery
// (SSRF) tool against the host, other containers, or cloud metadata.
func blockedIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified()
}

// validateMonitorURL rejects URLs that are malformed, use a non-HTTP scheme, or
// (when the host is a literal IP) point at a blocked address. Hostnames are
// checked again at dial time by monitorClient, which defeats DNS rebinding.
func validateMonitorURL(raw string) error {
	u, err := url.ParseRequestURI(raw)
	if err != nil {
		return errors.New("invalid url")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("url must use http or https")
	}
	host := u.Hostname()
	if host == "" {
		return errors.New("invalid url")
	}
	if ip := net.ParseIP(host); ip != nil && blockedIP(ip) {
		return errors.New("url points to a private or reserved address")
	}
	return nil
}

// monitorClient returns an HTTP client whose dialer rejects connections to
// blocked IPs at connect time — after DNS resolution and on every redirect hop —
// so a hostname that resolves (or rebinds) to a private address is still refused.
func monitorClient() *http.Client {
	dialer := &net.Dialer{
		Timeout: 10 * time.Second,
		Control: func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			ip := net.ParseIP(host)
			if ip == nil {
				return fmt.Errorf("unresolvable address %q", host)
			}
			if blockedIP(ip) {
				return fmt.Errorf("blocked address %s", ip)
			}
			return nil
		},
	}
	return &http.Client{
		Timeout:   10 * time.Second,
		Transport: &http.Transport{DialContext: dialer.DialContext},
	}
}

// RunMonitorCheck pings a URL and records status in the DB. Safe to call in a goroutine.
func RunMonitorCheck(db *sql.DB, id int64, targetURL string) {
	client := monitorClient()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		_, _ = db.Exec("UPDATE monitors SET status='down', last_checked=CURRENT_TIMESTAMP WHERE id=?", id)
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
	_, _ = db.Exec(
		"UPDATE monitors SET status=?, status_code=?, latency_ms=?, last_checked=CURRENT_TIMESTAMP WHERE id=?",
		status, code, latency, id,
	)
}

// Notifier is a minimal interface so extras.go doesn't import the notify package directly.
type Notifier interface {
	Notify(ctx context.Context, event, title, detail string)
}

// PollAllMonitors checks every monitor. Called on a timer from main.go.
func PollAllMonitors(db *sql.DB) {
	PollAllMonitorsWithNotify(db, nil, nil)
}

// PollAllMonitorsWithNotify checks every monitor and sends Telegram notifications on state changes.
func PollAllMonitorsWithNotify(db *sql.DB, notifier Notifier, prevStatus map[int64]string) {
	// Collect all monitors and close the rows before doing any checks: the DB
	// pool is one connection, and the per-monitor goroutines below write to it.
	// Holding the rows open would deadlock once the semaphore fills (the loop
	// would block spawning goroutines that are themselves blocked on the DB).
	type monitorRow struct {
		id            int64
		name, u, prev string
	}
	rows, err := db.QueryContext(context.Background(), "SELECT id, name, url, status FROM monitors")
	if err != nil {
		return
	}
	var monitors []monitorRow
	for rows.Next() {
		var m monitorRow
		if rows.Scan(&m.id, &m.name, &m.u, &m.prev) == nil {
			monitors = append(monitors, m)
		}
	}
	rows.Close()

	sem := make(chan struct{}, 10)
	var wg sync.WaitGroup
	// prevStatus is shared across the worker goroutines below; Go maps are not
	// safe for concurrent access (even distinct keys), so every read/write is
	// guarded. Without this the process crashes ("concurrent map writes") once
	// two monitors change state in the same poll.
	var statusMu sync.Mutex
	for _, m := range monitors {
		monID, monURL, monName := m.id, m.u, m.name
		oldStatus := m.prev
		if prevStatus != nil {
			statusMu.Lock()
			if s, ok := prevStatus[m.id]; ok {
				oldStatus = s
			}
			statusMu.Unlock()
		}
		sem <- struct{}{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			RunMonitorCheck(db, monID, monURL)
			// Check if status changed and notify
			if notifier != nil {
				var newStatus string
				db.QueryRowContext(context.Background(), "SELECT status FROM monitors WHERE id=?", monID).Scan(&newStatus) //nolint:errcheck
				if prevStatus != nil {
					statusMu.Lock()
					prevStatus[monID] = newStatus
					statusMu.Unlock()
				}
				if oldStatus != newStatus && oldStatus != "" {
					switch newStatus {
					case "down":
						notifier.Notify(context.Background(), "monitor.down",
							"Monitor DOWN: "+monName, monURL)
					case "up":
						notifier.Notify(context.Background(), "monitor.up",
							"Monitor UP: "+monName, monURL)
					}
				}
			}
		}()
	}
	wg.Wait()
}
