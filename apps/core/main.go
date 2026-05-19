package main

import (
	"log"
	"os"

	"github.com/NathanWalash/private-cloud-gateway/apps/core/internal/db"
	"github.com/NathanWalash/private-cloud-gateway/apps/core/internal/server"
)

func main() {
	cfg := config{
		dbPath:            getenv("CLOUD_CORE_DATABASE_PATH", "./data/cloud-core.db"),
		sessionSecret:     mustGetenv("CLOUD_CORE_SESSION_SECRET"),
		port:              getenv("CLOUD_CORE_PORT", "8080"),
		loginURL:          getenv("CLOUD_CORE_LOGIN_URL", "http://home.localhost/login"),
		cookieDomain:      getenv("CLOUD_CORE_COOKIE_DOMAIN", "localhost"),
		bootstrapEmail:    os.Getenv("CLOUD_CORE_BOOTSTRAP_EMAIL"),
		bootstrapPassword: os.Getenv("CLOUD_CORE_BOOTSTRAP_PASSWORD"),
	}

	database, err := db.Open(cfg.dbPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer database.Close()

	if err := db.Migrate(database); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	if cfg.bootstrapEmail != "" && cfg.bootstrapPassword != "" {
		if err := db.Bootstrap(database, cfg.bootstrapEmail, cfg.bootstrapPassword); err != nil {
			log.Printf("bootstrap: %v", err)
		}
	}

	srv := server.New(database, []byte(cfg.sessionSecret), cfg.loginURL, cfg.cookieDomain)
	log.Printf("Cloud Core listening on :%s", cfg.port)
	if err := srv.ListenAndServe(":" + cfg.port); err != nil {
		log.Fatalf("server: %v", err)
	}
}

type config struct {
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
		log.Fatalf("%s must be set", key)
	}
	return v
}
