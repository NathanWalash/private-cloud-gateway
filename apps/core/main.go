package main

import (
	"context"
	"database/sql"
	"log/slog"
	"os"

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
	// Routes are lost when Caddy restarts; this keeps them in sync with the DB.
	reregisterRoutes(database, cm)

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
