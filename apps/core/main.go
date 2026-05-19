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
		bootstrapEmail:    os.Getenv("CLOUD_CORE_BOOTSTRAP_EMAIL"),
		bootstrapPassword: os.Getenv("CLOUD_CORE_BOOTSTRAP_PASSWORD"),
		caddyAdmin:        getenv("CLOUD_CORE_CADDY_ADMIN", "caddy:2019"),
		blueprintDir:      getenv("CLOUD_CORE_BLUEPRINT_DIR", "/blueprints"),
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

	if cfg.bootstrapEmail != "" && cfg.bootstrapPassword != "" {
		if err := db.Bootstrap(database, cfg.bootstrapEmail, cfg.bootstrapPassword); err != nil {
			slog.Info("bootstrap skipped", "reason", err.Error())
		} else {
			slog.Info("bootstrap user created", "email", cfg.bootstrapEmail)
		}
	}

	// Docker manager — optional: logs a warning if Docker socket is unavailable.
	var dm *docker.Manager
	dm, err = docker.New()
	if err != nil {
		slog.Warn("docker unavailable — app install/lifecycle disabled", "err", err)
	} else {
		defer dm.Close()
		slog.Info("docker connected")
	}

	// Caddy manager — for dynamic route registration.
	cm := caddy.New(cfg.caddyAdmin, cfg.cookieDomain, cfg.loginURL)

	// Re-register Caddy routes for all apps that were running before restart.
	// Routes are lost when Caddy or Core restarts; this ensures they are always in sync.
	reregisterRoutes(database, cm)

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

// reregisterRoutes re-registers Caddy routes for all apps in the DB.
// Called on startup so routes survive Core or Caddy restarts.
func reregisterRoutes(database *sql.DB, cm *caddy.Manager) {
	rows, err := database.QueryContext(context.Background(),
		"SELECT subdomain, container_name, internal_port FROM apps WHERE status = 'running'")
	if err != nil {
		slog.Warn("reregister routes: db query failed", "err", err)
		return
	}
	defer rows.Close()

	var count int
	for rows.Next() {
		var subdomain, containerName string
		var internalPort int
		if err := rows.Scan(&subdomain, &containerName, &internalPort); err != nil {
			continue
		}
		if err := cm.RegisterApp(context.Background(), subdomain, containerName, internalPort); err != nil {
			slog.Warn("reregister route failed", "subdomain", subdomain, "err", err)
		} else {
			count++
		}
	}
	if count > 0 {
		slog.Info("caddy routes re-registered", "count", count)
	}
}
