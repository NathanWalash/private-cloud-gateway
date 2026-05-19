package api

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"
)

// Handler provides general API endpoints (status, apps, etc.)
type Handler struct {
	db        *sql.DB
	startTime time.Time
	version   string
}

func NewHandler(db *sql.DB, version string) *Handler {
	return &Handler{db: db, startTime: time.Now(), version: version}
}

// Status returns server uptime and version info.
// GET /api/status
func (h *Handler) Status(w http.ResponseWriter, _ *http.Request) {
	uptime := time.Since(h.startTime).Round(time.Second).String()
	w.Header().Set("Content-Type", "application/json")
	_, _ = fmt.Fprintf(w, `{"uptime":%q,"version":%q}`, uptime, h.version)
}

// Apps returns the list of installed apps.
// GET /api/apps  — placeholder until Milestone 3 implements blueprints.
func (h *Handler) Apps(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte("[]"))
}

