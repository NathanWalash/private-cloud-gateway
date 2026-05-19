package main

import (
	"log/slog"
	"os"

	"github.com/NathanWalash/private-cloud-gateway/apps/core/internal/db"
	"github.com/NathanWalash/private-cloud-gateway/apps/core/internal/server"
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

	srv := server.New(database, []byte(cfg.sessionSecret), cfg.loginURL, cfg.cookieDomain)
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
