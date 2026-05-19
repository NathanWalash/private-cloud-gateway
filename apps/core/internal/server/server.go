package server

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/NathanWalash/private-cloud-gateway/apps/core/internal/auth"
)

// Server wraps the HTTP mux and holds shared dependencies.
type Server struct {
	mux *http.ServeMux
}

// New wires up all routes and returns a ready Server.
func New(db *sql.DB, secret []byte, loginURL, cookieDomain string) *Server {
	mux := http.NewServeMux()
	authHandler := auth.NewHandler(db, loginURL, cookieDomain)

	// Auth routes
	mux.HandleFunc("GET /login", authHandler.LoginPage)
	mux.HandleFunc("POST /api/auth/login", authHandler.Login)
	mux.HandleFunc("POST /api/auth/logout", authHandler.Logout)

	// Caddy forward-auth endpoint — called server-to-server, not by the browser.
	mux.HandleFunc("GET /api/auth/verify", authHandler.Verify)

	// Health check — used by Docker HEALTHCHECK and load balancers.
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})

	// Dashboard root — requires a valid session.
	// Replaced by the Vite app in Milestone 2.
	mux.HandleFunc("GET /", authHandler.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!DOCTYPE html><html><body style="background:#0f1117;color:#e2e8f0;font-family:system-ui;padding:2rem">
<h1>Private Cloud Gateway</h1><p>Dashboard coming in Milestone 2.</p>
<p><a href="/api/auth/logout" style="color:#6366f1">Sign out</a></p>
</body></html>`))
	}))

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
