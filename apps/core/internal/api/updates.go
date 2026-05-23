package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// UpdateInfo describes whether a newer image is available for an installed app.
type UpdateInfo struct {
	AppID        int64  `json:"app_id"`
	BlueprintID  string `json:"blueprint_id"`
	CurrentImage string `json:"current_image"`
	LatestDigest string `json:"latest_digest,omitempty"`
	UpdateAvail  bool   `json:"update_available"`
}

var updateClient = &http.Client{Timeout: 10 * time.Second}

// CheckAppUpdates fetches the latest digest for each installed app's image from Docker Hub.
// Called from main.go on a timer (every 6 hours).
func CheckAppUpdates(db *sql.DB) {
	rows, err := db.QueryContext(context.Background(),
		"SELECT id, blueprint_id, image FROM apps WHERE status='running'")
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		var bpID, image string
		if rows.Scan(&id, &bpID, &image) != nil {
			continue
		}
		digest, err := getLatestDigest(image)
		if err != nil {
			slog.Debug("update check failed", "image", image, "err", err)
			continue
		}
		_, _ = db.Exec(
			`INSERT INTO settings(key,value,updated_at) VALUES(?,?,CURRENT_TIMESTAMP)
			 ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=CURRENT_TIMESTAMP`,
			"UPDATE_DIGEST_"+bpID, digest,
		)
	}
}

// AppUpdateStatus returns whether updates are available for all installed apps.
// GET /api/apps/updates
func (h *Handler) AppUpdateStatus(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(),
		"SELECT id, blueprint_id, image FROM apps WHERE status='running'")
	if err != nil {
		jsonOK(w, []UpdateInfo{})
		return
	}
	defer rows.Close()

	var updates []UpdateInfo
	for rows.Next() {
		var id int64
		var bpID, image string
		if rows.Scan(&id, &bpID, &image) != nil {
			continue
		}
		var cached string
		h.db.QueryRowContext(r.Context(),
			"SELECT value FROM settings WHERE key=?", "UPDATE_DIGEST_"+bpID).Scan(&cached) //nolint:errcheck

		currentDigest, _ := getLatestDigest(image)
		avail := cached != "" && currentDigest != "" && cached != currentDigest
		updates = append(updates, UpdateInfo{
			AppID:        id,
			BlueprintID:  bpID,
			CurrentImage: image,
			LatestDigest: currentDigest,
			UpdateAvail:  avail,
		})
	}
	if updates == nil {
		updates = []UpdateInfo{}
	}
	jsonOK(w, updates)
}

// getLatestDigest fetches the content digest of the :latest tag from Docker Hub
// or an OCI-compatible registry. Returns an empty string if unavailable.
func getLatestDigest(image string) (string, error) {
	// Parse image reference: [registry/]name[:tag]
	ref, tag := image, "latest"
	if i := strings.LastIndex(image, ":"); i > strings.LastIndex(image, "/") {
		ref, tag = image[:i], image[i+1:]
	}

	// Only Docker Hub images (no explicit registry host)
	parts := strings.Split(ref, "/")
	if len(parts) > 2 || strings.Contains(parts[0], ".") {
		// Non-Docker-Hub registry — skip for now
		return "", nil
	}

	// Docker Hub anonymous manifest endpoint
	if len(parts) == 1 {
		ref = "library/" + ref
	}
	url := fmt.Sprintf("https://hub.docker.com/v2/repositories/%s/tags/%s", ref, tag)
	req, _ := http.NewRequestWithContext(context.Background(), "GET", url, nil)
	resp, err := updateClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("hub returned %d", resp.StatusCode)
	}

	var result struct {
		Images []struct {
			Digest string `json:"digest"`
		} `json:"images"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if len(result.Images) == 0 {
		return "", nil
	}
	return result.Images[0].Digest, nil
}
