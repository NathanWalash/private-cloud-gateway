package auth

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const cookieName = "pcg_session"

// Handler provides HTTP handlers for login, logout, and Caddy forward-auth verify.
type Handler struct {
	db       *sql.DB
	loginURL string
}

func NewHandler(db *sql.DB, loginURL string) *Handler {
	return &Handler{db: db, loginURL: loginURL}
}

// LoginPage serves the minimal HTML login form.
func (h *Handler) LoginPage(w http.ResponseWriter, r *http.Request) {
	errMsg := ""
	if r.URL.Query().Get("error") == "1" {
		errMsg = `<p class="error">Invalid email or password.</p>`
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, loginPageHTML, errMsg)
}

// Login handles form submission from the login page.
// On success it sets the session cookie and redirects to /.
// On failure it redirects back to /login?error=1.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
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
		auditLog(h.db, "login.fail", email, "user not found")
		http.Redirect(w, r, h.loginURL+"?error=1", http.StatusSeeOther)
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		auditLog(h.db, "login.fail", email, "wrong password")
		http.Redirect(w, r, h.loginURL+"?error=1", http.StatusSeeOther)
		return
	}

	sessionID, err := createSession(h.db, userID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(sessionTTL),
	})

	auditLog(h.db, "login.success", email, "")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// Logout clears the session and redirects to the login page.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(cookieName); err == nil {
		deleteSession(h.db, cookie.Value)
		auditLog(h.db, "logout", "", "")
	}

	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})

	http.Redirect(w, r, h.loginURL, http.StatusSeeOther)
}

// Verify is the Caddy forward-auth endpoint.
// Returns 200 for valid sessions, 401 for invalid or missing sessions.
// Caddy is expected to handle the 401 redirect via handle_errors in the Caddyfile.
func (h *Handler) Verify(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	userID, err := validateSession(h.db, cookie.Value)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if userID == 0 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Downstream services can use this header to identify the caller.
	w.Header().Set("X-Auth-User-ID", fmt.Sprintf("%d", userID))
	w.WriteHeader(http.StatusOK)
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
