package api

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/NathanWalash/private-cloud-gateway/apps/core/internal/blueprint"
	"github.com/NathanWalash/private-cloud-gateway/apps/core/internal/notify"
)

// RunAppHealthChecks polls the health endpoint of every running app.
// Called on a timer from main.go — every 60 seconds.
func RunAppHealthChecks(db *sql.DB, blueprintDir string, notifier *notify.Service) {
	rows, err := db.QueryContext(context.Background(),
		"SELECT id, blueprint_id, container_name, health_status FROM apps WHERE status='running'")
	if err != nil {
		return
	}
	defer rows.Close()

	client := &http.Client{Timeout: 10 * time.Second}

	for rows.Next() {
		var id int64
		var bpID, containerName, prevHealth string
		if rows.Scan(&id, &bpID, &containerName, &prevHealth) != nil {
			continue
		}

		// Load blueprint to get health endpoint
		bpPath := filepath.Join(blueprintDir, bpID+".yaml")
		data, err := os.ReadFile(bpPath)
		if err != nil {
			continue
		}
		bp, err := blueprint.Parse(data)
		if err != nil || bp.Health.Path == "" {
			continue
		}

		// Determine the app's URL (inside Docker network via container name)
		appURL := fmt.Sprintf("http://%s:%d%s", containerName, bp.Route.InternalPort, bp.Health.Path)

		newHealth := "healthy"
		req, _ := http.NewRequestWithContext(context.Background(), "GET", appURL, nil)
		resp, err := client.Do(req)
		if err != nil {
			newHealth = "unreachable"
		} else {
			resp.Body.Close()
			expected := bp.Health.ExpectedStatus
			if expected == 0 {
				expected = http.StatusOK
			}
			if resp.StatusCode != expected {
				newHealth = fmt.Sprintf("unhealthy (%d)", resp.StatusCode)
			}
		}

		_, _ = db.Exec(
			"UPDATE apps SET health_status=?, health_checked=CURRENT_TIMESTAMP WHERE id=?",
			newHealth, id)

		// Notify on state transitions
		if notifier != nil && prevHealth != newHealth {
			if newHealth != "healthy" {
				notifier.Notify(context.Background(), notify.EventAppHealthBad,
					"App health degraded: "+bp.Name,
					fmt.Sprintf("Status: %s\nURL: %s", newHealth, appURL))
			} else if prevHealth != "unknown" {
				notifier.Notify(context.Background(), notify.EventAppHealthOK,
					"App recovered: "+bp.Name, "")
			}
			slog.Info("app health changed", "app", bpID, "from", prevHealth, "to", newHealth)
		}
	}
}

