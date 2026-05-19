package auth

import (
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const cookieName = "pcg_session"

// Handler provides HTTP handlers for login, logout, and Caddy forward-auth verify.
type Handler struct {
	db           *sql.DB
	loginURL     string
	cookieDomain string
}

func NewHandler(db *sql.DB, loginURL, cookieDomain string) *Handler {
	return &Handler{db: db, loginURL: loginURL, cookieDomain: cookieDomain}
}

// LoginPage serves the HTML login form.
func (h *Handler) LoginPage(w http.ResponseWriter, r *http.Request) {
	errMsg := ""
	if r.URL.Query().Get("error") == "1" {
		errMsg = `<p class="error">Invalid email or password.</p>`
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, loginPageHTML, errMsg)
}

// Login validates credentials, sets the session cookie, and redirects.
// Rate-limited to 10 attempts per IP per minute.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	ip := realIP(r)

	if !loginLimiter.allow(ip) {
		slog.Warn("login rate limited", "ip", ip)
		http.Error(w, "too many requests", http.StatusTooManyRequests)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, h.loginURL+"?error=1", http.StatusSeeOther)
		return
	}

	email := r.FormValue("email")
	password := r.FormValue("password")

	var userID int64
	var hash string
	err := h.db.QueryRow(
		"SELECT id, password FROM users WHERE email = ?", email,
	).Scan(&userID, &hash)
	if err == sql.ErrNoRows {
		slog.Info("login failed", "reason", "user not found", "ip", ip)
		auditLog(h.db, "login.fail", email, "user not found")
		http.Redirect(w, r, h.loginURL+"?error=1", http.StatusSeeOther)
		return
	}
	if err != nil {
		slog.Error("login db error", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		slog.Info("login failed", "reason", "wrong password", "ip", ip)
		auditLog(h.db, "login.fail", email, "wrong password")
		http.Redirect(w, r, h.loginURL+"?error=1", http.StatusSeeOther)
		return
	}

	sessionID, err := createSession(h.db, userID)
	if err != nil {
		slog.Error("create session error", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    sessionID,
		Path:     "/",
		Domain:   h.cookieDomain,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(sessionTTL),
	})

	slog.Info("login success", "ip", ip)
	auditLog(h.db, "login.success", email, "")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// Logout clears the session cookie and invalidates the server-side session.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(cookieName); err == nil {
		deleteSession(h.db, cookie.Value)
		auditLog(h.db, "logout", "", "")
		slog.Info("logout", "ip", realIP(r))
	}

	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		Domain:   h.cookieDomain,
		HttpOnly: true,
		MaxAge:   -1,
	})

	http.Redirect(w, r, h.loginURL, http.StatusSeeOther)
}

// Verify is the Caddy forward-auth endpoint.
// Returns 200 + X-Auth-User-ID for a valid session.
// Returns 302 to loginURL for an invalid or missing session — Caddy passes
// this redirect straight through to the browser.
func (h *Handler) Verify(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		http.Redirect(w, r, h.loginURL, http.StatusFound)
		return
	}

	userID, err := validateSession(h.db, cookie.Value)
	if err != nil {
		slog.Error("verify session error", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if userID == 0 {
		http.Redirect(w, r, h.loginURL, http.StatusFound)
		return
	}

	w.Header().Set("X-Auth-User-ID", fmt.Sprintf("%d", userID))
	w.WriteHeader(http.StatusOK)
}

// RequireAuth wraps a handler, redirecting to /login if there is no valid session.
// Used for routes served directly by Go Core (e.g. the dashboard root).
func (h *Handler) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(cookieName)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		userID, err := validateSession(h.db, cookie.Value)
		if err != nil || userID == 0 {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		next(w, r)
	}
}

const loginPageHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Private Cloud Gateway — Login</title>
  <style>
    *, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
    body {
      min-height: 100vh;
      display: flex;
      align-items: center;
      justify-content: center;
      background: #0f1117;
      font-family: system-ui, sans-serif;
      color: #e2e8f0;
    }
    .card {
      width: 100%%;
      max-width: 380px;
      background: #1a1d27;
      border: 1px solid #2d3148;
      border-radius: 12px;
      padding: 2.5rem 2rem;
    }
    h1 { font-size: 1.25rem; font-weight: 600; margin-bottom: 0.25rem; }
    .subtitle { font-size: 0.875rem; color: #64748b; margin-bottom: 2rem; }
    label { display: block; font-size: 0.8125rem; color: #94a3b8; margin-bottom: 0.375rem; }
    input {
      width: 100%%;
      padding: 0.625rem 0.75rem;
      background: #0f1117;
      border: 1px solid #2d3148;
      border-radius: 6px;
      color: #e2e8f0;
      font-size: 0.9375rem;
      margin-bottom: 1.25rem;
      outline: none;
    }
    input:focus { border-color: #6366f1; }
    button {
      width: 100%%;
      padding: 0.625rem;
      background: #6366f1;
      border: none;
      border-radius: 6px;
      color: #fff;
      font-size: 0.9375rem;
      font-weight: 500;
      cursor: pointer;
    }
    button:hover { background: #5254cc; }
    .error { color: #f87171; font-size: 0.875rem; margin-bottom: 1rem; }
  </style>
</head>
<body>
  <div class="card">
    <h1>Private Cloud Gateway</h1>
    <p class="subtitle">Sign in to your private cloud</p>
    %s
    <form method="POST" action="/api/auth/login">
      <label for="email">Email</label>
      <input type="email" id="email" name="email" autocomplete="email" required>
      <label for="password">Password</label>
      <input type="password" id="password" name="password" autocomplete="current-password" required>
      <button type="submit">Sign in</button>
    </form>
  </div>
</body>
</html>
`
