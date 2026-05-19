package auth

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
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

// LoginPage serves the HTML login form (used as a fallback if JS fails).
func (h *Handler) LoginPage(w http.ResponseWriter, r *http.Request) {
	errMsg := ""
	if r.URL.Query().Get("error") == "1" {
		errMsg = `<p class="error">Invalid email or password.</p>`
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprintf(w, loginPageHTML, errMsg)
}

// Me returns the authenticated user's info including first/last name.
// GET /api/auth/me — returns 200+JSON when authenticated, 401 when not.
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	userID, err := validateSession(h.db, cookie.Value)
	if err != nil {
		slog.Error("me: validate session", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if userID == 0 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var email, firstName, lastName string
	if err := h.db.QueryRow(
		"SELECT email, first_name, last_name FROM users WHERE id = ?", userID,
	).Scan(&email, &firstName, &lastName); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	b, _ := json.Marshal(map[string]any{
		"id":         userID,
		"email":      email,
		"first_name": firstName,
		"last_name":  lastName,
	})
	_, _ = w.Write(b)
}

// NeedsSetup returns whether the system has been configured yet.
// GET /api/auth/setup
func (h *Handler) NeedsSetup(w http.ResponseWriter, _ *http.Request) {
	var count int
	h.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count) //nolint:errcheck
	w.Header().Set("Content-Type", "application/json")
	_, _ = fmt.Fprintf(w, `{"needs_setup":%v}`, count == 0)
}

// Setup creates the first admin account. Only works when no users exist.
// POST /api/auth/setup
func (h *Handler) Setup(w http.ResponseWriter, r *http.Request) {
	var count int
	h.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count) //nolint:errcheck
	if count > 0 {
		http.Error(w, `{"error":"already configured"}`, http.StatusConflict)
		return
	}

	var req struct {
		Email     string `json:"email"`
		Password  string `json:"password"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}
	if req.Email == "" || req.Password == "" || req.FirstName == "" {
		http.Error(w, `{"error":"email, password, and first_name are required"}`, http.StatusBadRequest)
		return
	}
	if len(req.Password) < 8 {
		http.Error(w, `{"error":"password must be at least 8 characters"}`, http.StatusBadRequest)
		return
	}

	if err := dbCreateUser(h.db, req.Email, req.Password, req.FirstName, req.LastName); err != nil {
		slog.Error("setup: create user failed", "err", err)
		http.Error(w, `{"error":"failed to create account"}`, http.StatusInternalServerError)
		return
	}

	slog.Info("first admin created via setup wizard", "email", req.Email)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

// Login handles login from either an HTML form (form-encoded) or the React app (JSON).
// Form: redirects on success/failure. JSON: returns 200/401 with JSON body.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	ip := realIP(r)

	if !loginLimiter.allow(ip) {
		slog.Warn("login rate limited", "ip", ip)
		if isJSON(r) {
			w.Header().Set("Content-Type", "application/json")
			http.Error(w, `{"error":"too many requests"}`, http.StatusTooManyRequests)
		} else {
			http.Error(w, "too many requests", http.StatusTooManyRequests)
		}
		return
	}

	email, password, ok := parseLoginBody(r)
	if !ok {
		if isJSON(r) {
			http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		} else {
			http.Redirect(w, r, h.loginURL+"?error=1", http.StatusSeeOther)
		}
		return
	}

	var userID int64
	var hash string
	err := h.db.QueryRow(
		"SELECT id, password FROM users WHERE email = ?", email,
	).Scan(&userID, &hash)
	if err == sql.ErrNoRows {
		slog.Info("login failed", "reason", "user not found", "ip", ip)
		auditLog(h.db, "login.fail", email, "user not found")
		h.loginError(w, r)
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
		h.loginError(w, r)
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

	if isJSON(r) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	} else {
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

// Logout clears the session and redirects (form) or returns JSON (API).
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(cookieName); err == nil {
		_ = deleteSession(h.db, cookie.Value)
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

	if isJSON(r) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	} else {
		http.Redirect(w, r, h.loginURL, http.StatusSeeOther)
	}
}

// Verify is the Caddy forward-auth endpoint.
// Returns 200 + X-Auth-User-ID on valid session, 302 to loginURL otherwise.
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

// RequireAuth wraps a handler, redirecting to /login (302) if no valid session.
// Used to guard API endpoints and the dashboard root.
func (h *Handler) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(cookieName)
		if err != nil {
			if isJSON(r) {
				w.Header().Set("Content-Type", "application/json")
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			} else {
				http.Redirect(w, r, "/login", http.StatusFound)
			}
			return
		}
		userID, err := validateSession(h.db, cookie.Value)
		if err != nil || userID == 0 {
			if isJSON(r) {
				w.Header().Set("Content-Type", "application/json")
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			} else {
				http.Redirect(w, r, "/login", http.StatusFound)
			}
			return
		}
		next(w, r)
	}
}

// loginError returns 401 JSON or redirects to /login depending on the request type.
func (h *Handler) loginError(w http.ResponseWriter, r *http.Request) {
	if isJSON(r) {
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
	} else {
		http.Redirect(w, r, h.loginURL+"?error=1", http.StatusSeeOther)
	}
}

// isJSON reports whether the request expects a JSON response.
func isJSON(r *http.Request) bool {
	ct := r.Header.Get("Content-Type")
	acc := r.Header.Get("Accept")
	return strings.Contains(ct, "application/json") ||
		strings.Contains(acc, "application/json")
}

// parseLoginBody reads email/password from either JSON or form body.
func parseLoginBody(r *http.Request) (email, password string, ok bool) {
	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		var req struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return "", "", false
		}
		return req.Email, req.Password, true
	}
	if err := r.ParseForm(); err != nil {
		return "", "", false
	}
	return r.FormValue("email"), r.FormValue("password"), true
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
