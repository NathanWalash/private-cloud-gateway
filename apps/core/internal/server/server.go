package server

import (
	"context"
	"database/sql"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/NathanWalash/private-cloud-gateway/apps/core/internal/api"
	"github.com/NathanWalash/private-cloud-gateway/apps/core/internal/auth"
)

// Server wraps the HTTP mux and holds shared dependencies.
type Server struct {
	mux *http.ServeMux
}

// New wires up all routes and returns a ready Server.
// staticFS is the embedded web app; pass nil to use the placeholder fallback.
func New(db *sql.DB, secret []byte, loginURL, cookieDomain string, staticFS fs.FS) *Server {
	mux := http.NewServeMux()

	authHandler := auth.NewHandler(db, loginURL, cookieDomain)
	apiHandler := api.NewHandler(db, "0.1.0")

	// ── Auth ────────────────────────────────────────────────────────────────
	mux.HandleFunc("GET /login", authHandler.LoginPage)
	mux.HandleFunc("POST /api/auth/login", authHandler.Login)
	mux.HandleFunc("POST /api/auth/logout", authHandler.Logout)
	mux.HandleFunc("GET /api/auth/verify", authHandler.Verify)
	mux.HandleFunc("GET /api/auth/me", authHandler.Me)

	// ── API ─────────────────────────────────────────────────────────────────
	mux.HandleFunc("GET /api/status", authHandler.RequireAuth(apiHandler.Status))
	mux.HandleFunc("GET /api/apps", authHandler.RequireAuth(apiHandler.Apps))

	// ── Health check ────────────────────────────────────────────────────────
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})

	// ── Static web app ──────────────────────────────────────────────────────
	if staticFS != nil {
		// SPA mode: serve the Vite app and fall back to index.html for all
		// non-API routes so React Router handles client-side navigation.
		fileServer := http.FileServer(http.FS(staticFS))
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				http.NotFound(w, r)
				return
			}
			_, err := staticFS.(fs.StatFS).Stat(strings.TrimPrefix(r.URL.Path, "/"))
			if err != nil {
				r2 := r.Clone(r.Context())
				r2.URL.Path = "/"
				fileServer.ServeHTTP(w, r2)
				return
			}
			fileServer.ServeHTTP(w, r)
		})
	} else {
		// No SPA — used in tests. Auth guard on root for backward compatibility.
		mux.HandleFunc("GET /", authHandler.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/" {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte("<!DOCTYPE html><html><body>ok</body></html>"))
		}))
	}

	return &Server{mux: mux}
}

// Handler returns the HTTP handler — used by tests with httptest.NewServer.
func (s *Server) Handler() http.Handler {
	return s.mux
}

// ListenAndServe starts the HTTP server with timeouts and handles SIGINT/SIGTERM
// for graceful shutdown.
func (s *Server) ListenAndServe(addr string) error {
	srv := &http.Server{
		Addr:         addr,
		Handler:      s.mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	done := make(chan struct{})
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		sig := <-quit
		slog.Info("shutting down", "signal", sig)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			slog.Error("shutdown error", "err", err)
		}
		close(done)
	}()

	slog.Info("listening", "addr", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	<-done
	return nil
}
