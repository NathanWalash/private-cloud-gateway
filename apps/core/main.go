package main

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	apiPkg "github.com/NathanWalash/private-cloud-gateway/apps/core/internal/api"
	"github.com/NathanWalash/private-cloud-gateway/apps/core/internal/backup"
	"github.com/NathanWalash/private-cloud-gateway/apps/core/internal/blueprint"
	"github.com/NathanWalash/private-cloud-gateway/apps/core/internal/caddy"
	"github.com/NathanWalash/private-cloud-gateway/apps/core/internal/db"
	"github.com/NathanWalash/private-cloud-gateway/apps/core/internal/docker"
	"github.com/NathanWalash/private-cloud-gateway/apps/core/internal/server"
	"github.com/NathanWalash/private-cloud-gateway/apps/core/web"
)

func main() {
	cfg := config{
		env:               getenv("CLOUD_CORE_ENV", "production"),
		dbPath:            getenv("CLOUD_CORE_DATABASE_PATH", "./data/cloud-core.db"),
		sessionSecret:     mustGetenv("CLOUD_CORE_SESSION_SECRET"),
		port:              getenv("CLOUD_CORE_PORT", "8080"),
		loginURL:          getenv("CLOUD_CORE_LOGIN_URL", "http://home.localtest.me/login"),
		cookieDomain:      getenv("CLOUD_CORE_COOKIE_DOMAIN", "localtest.me"),
		// Bootstrap vars are optional — the in-app setup wizard is preferred.
		bootstrapEmail:    os.Getenv("CLOUD_CORE_BOOTSTRAP_EMAIL"),
		bootstrapPassword: os.Getenv("CLOUD_CORE_BOOTSTRAP_PASSWORD"),
		caddyAdmin:        getenv("CLOUD_CORE_CADDY_ADMIN", "caddy:2019"),
		blueprintDir:      getenv("CLOUD_CORE_BLUEPRINT_DIR", "/blueprints"),
		adminEmail:        os.Getenv("CLOUD_CORE_ADMIN_EMAIL"),
		backupSchedule:    getenv("CLOUD_CORE_BACKUP_SCHEDULE", ""), // e.g. "24h", "12h" — empty disables
	}

	setupLogging(cfg.env)

	database, err := db.Open(cfg.dbPath)
	if err != nil {
		slog.Error("open database", "err", err)
		os.Exit(1)
	}
	defer database.Close()

	if err := db.Migrate(database); err != nil {
		slog.Error("run migrations", "err", err)
		os.Exit(1)
	}

	// Bootstrap is only used if set in .env and no users exist yet.
	// The in-app setup wizard is the preferred first-run path.
	if cfg.bootstrapEmail != "" && cfg.bootstrapPassword != "" {
		if err := db.Bootstrap(database, cfg.bootstrapEmail, cfg.bootstrapPassword); err != nil {
			slog.Info("bootstrap skipped", "reason", err.Error())
		} else {
			slog.Info("bootstrap user created", "email", cfg.bootstrapEmail)
		}
	}

	var dm *docker.Manager
	dm, err = docker.New()
	if err != nil {
		slog.Warn("docker unavailable — app install/lifecycle disabled", "err", err)
	} else {
		defer dm.Close()
		slog.Info("docker connected")
	}

	var cm *caddy.Manager
	if cfg.env == "production" && cfg.adminEmail != "" {
		cm = caddy.NewProduction(cfg.caddyAdmin, cfg.cookieDomain, cfg.adminEmail)
		slog.Info("caddy running in production HTTPS mode", "domain", cfg.cookieDomain)
	} else {
		cm = caddy.New(cfg.caddyAdmin, cfg.cookieDomain, cfg.loginURL)
	}

	srv := server.New(
		database,
		[]byte(cfg.sessionSecret),
		cfg.loginURL,
		cfg.cookieDomain,
		web.FS(),
		dm,
		cm,
		cfg.blueprintDir,
	)

	// Re-register Caddy routes for all installed apps on every startup.
	reregisterRoutes(database, cm)

	// Health polling — updates app status from Docker every 30 seconds.
	if dm != nil {
		go runHealthPolling(database, dm)
		slog.Info("health polling enabled")
	}

	// Monitor polling — checks all registered URLs every 2 minutes.
	go func() {
		ticker := time.NewTicker(2 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			apiPkg.PollAllMonitors(database)
		}
	}()

	// Start scheduled backup goroutine if CLOUD_CORE_BACKUP_SCHEDULE is set.
	if cfg.backupSchedule != "" {
		interval, err := time.ParseDuration(cfg.backupSchedule)
		if err != nil {
			slog.Warn("invalid backup schedule, disabling", "value", cfg.backupSchedule)
		} else {
			go runScheduledBackups(interval, cfg.dbPath, cfg.blueprintDir, dm)
			slog.Info("scheduled backups enabled", "interval", interval)
		}
	}

	if err := srv.ListenAndServe(":" + cfg.port); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

func setupLogging(env string) {
	var handler slog.Handler
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	if env == "development" {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}
	slog.SetDefault(slog.New(handler))
}

type config struct {
	env               string
	dbPath            string
	sessionSecret     string
	port              string
	loginURL          string
	cookieDomain      string
	bootstrapEmail    string
	bootstrapPassword string
	caddyAdmin        string
	blueprintDir      string
	adminEmail        string // Let's Encrypt email (production only)
	backupSchedule    string // duration string, e.g. "24h"
}

// runScheduledBackups runs backups on a fixed interval until the process exits.
func runScheduledBackups(interval time.Duration, dbPath, blueprintDir string, dm *docker.Manager) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		slog.Info("running scheduled backup...")

		backupDir := os.Getenv("CLOUD_CORE_BACKUP_DIR")
		if backupDir == "" {
			backupDir = "/backups"
		}
		if err := os.MkdirAll(backupDir, 0o700); err != nil {
			slog.Error("scheduled backup: cannot create dir", "err", err)
			continue
		}

		passphrase := os.Getenv("CLOUD_CORE_BACKUP_PASSPHRASE")
		destPath := filepath.Join(backupDir, backup.FileName(time.Now()))

		// Collect volumes from running apps via blueprints directory
		var volumes []backup.AppVolume
		var vr backup.VolumeReader
		if dm != nil {
			entries, _ := os.ReadDir(blueprintDir)
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				data, err := os.ReadFile(filepath.Join(blueprintDir, e.Name()))
				if err != nil {
					continue
				}
				bp, err := blueprint.Parse(data)
				if err != nil || !bp.Backup.Enabled {
					continue
				}
				containerName := bp.ContainerName()
				status := dm.Status(context.Background(), containerName)
				if status != "running" {
					continue
				}
				for _, p := range bp.Backup.ContainerPaths {
					volumes = append(volumes, backup.AppVolume{
						AppID:         bp.ID,
						ContainerName: containerName,
						ContainerPath: p,
					})
				}
			}
			vr = func(cn, cp string) (io.ReadCloser, error) {
				return dm.CopyFromContainer(context.Background(), cn, cp)
			}
		}

		if err := backup.Create(dbPath, blueprintDir, destPath, passphrase, volumes, vr); err != nil {
			slog.Error("scheduled backup failed", "err", err)
		} else {
			slog.Info("scheduled backup completed", "file", filepath.Base(destPath), "volumes", len(volumes))
		}
	}
}

// runHealthPolling updates app statuses from Docker on a 30-second loop.
func runHealthPolling(database *sql.DB, dm *docker.Manager) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		statuses := dm.StatusAll(context.Background())
		if len(statuses) == 0 {
			continue
		}
		rows, err := database.QueryContext(context.Background(), "SELECT id, container_name FROM apps")
		if err != nil {
			continue
		}
		for rows.Next() {
			var id int64
			var name string
			if rows.Scan(&id, &name) != nil {
				continue
			}
			if status, ok := statuses[name]; ok {
				_, _ = database.ExecContext(context.Background(),
					"UPDATE apps SET status=?, updated_at=CURRENT_TIMESTAMP WHERE id=? AND status!=?",
					status, id, status)
			} else {
				_, _ = database.ExecContext(context.Background(),
					"UPDATE apps SET status='missing', updated_at=CURRENT_TIMESTAMP WHERE id=?", id)
			}
		}
		rows.Close()
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func mustGetenv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		slog.Error("required env var not set", "var", key)
		os.Exit(1)
	}
	return v
}

// reregisterRoutes pushes all installed app routes to Caddy on startup.
// This keeps routes in sync when Core or Caddy restarts.
func reregisterRoutes(database *sql.DB, cm *caddy.Manager) {
	rows, err := database.QueryContext(context.Background(),
		"SELECT subdomain, container_name, internal_port FROM apps ORDER BY id")
	if err != nil {
		slog.Warn("reregister routes: db query failed", "err", err)
		return
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

	if err := cm.ReloadAll(context.Background(), routes); err != nil {
		slog.Warn("reregister routes: caddy reload failed", "err", err)
		return
	}
	slog.Info("caddy routes synced on startup", "count", len(routes))
}
