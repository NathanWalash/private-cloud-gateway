package auth

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/NathanWalash/private-cloud-gateway/apps/core/internal/totp"
)

const totpPendingTTL = 5 * time.Minute
const totpIssuer = "Private Cloud Gateway"

// TOTPStatus returns whether TOTP is enabled for the authenticated user.
// GET /api/auth/totp/status
func (h *Handler) TOTPStatus(w http.ResponseWriter, r *http.Request) {
	cookie, _ := r.Cookie(cookieName)
	if cookie == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	userID, _ := validateSession(h.db, cookie.Value)
	if userID == 0 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var secret *string
	h.db.QueryRowContext(r.Context(), "SELECT totp_secret FROM users WHERE id=?", userID).Scan(&secret)
	w.Header().Set("Content-Type", "application/json")
	_, _ = fmt.Fprintf(w, `{"enabled":%v}`, secret != nil && *secret != "")
}

// TOTPSetup generates a new TOTP secret and returns the otpauth URI.
// The secret is NOT saved until TOTPConfirm succeeds.
// POST /api/auth/totp/setup
func (h *Handler) TOTPSetup(w http.ResponseWriter, r *http.Request) {
	cookie, _ := r.Cookie(cookieName)
	if cookie == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	userID, _ := validateSession(h.db, cookie.Value)
	if userID == 0 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var email string
	h.db.QueryRowContext(r.Context(), "SELECT email FROM users WHERE id=?", userID).Scan(&email)

	secret, err := totp.GenerateSecret()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	uri := totp.OTPAuthURI(secret, totpIssuer, email)
	w.Header().Set("Content-Type", "application/json")
	b, _ := json.Marshal(map[string]string{"secret": secret, "uri": uri})
	_, _ = w.Write(b)
}

// TOTPConfirm saves the TOTP secret after verifying the first code.
// POST /api/auth/totp/confirm  body: {secret, code}
func (h *Handler) TOTPConfirm(w http.ResponseWriter, r *http.Request) {
	cookie, _ := r.Cookie(cookieName)
	if cookie == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	userID, _ := validateSession(h.db, cookie.Value)
	if userID == 0 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		Secret string `json:"secret"`
		Code   string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Secret == "" || req.Code == "" {
		http.Error(w, `{"error":"secret and code required"}`, http.StatusBadRequest)
		return
	}

	if !totp.Verify(req.Secret, req.Code, time.Now()) {
		http.Error(w, `{"error":"invalid code"}`, http.StatusUnauthorized)
		return
	}

	_, err := h.db.ExecContext(r.Context(),
		"UPDATE users SET totp_secret=? WHERE id=?", req.Secret, userID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	auditLog(h.db, "totp.enabled", "", "")
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"enabled"}`))
}

// TOTPDisable removes TOTP from the account.
// POST /api/auth/totp/disable  body: {code}
func (h *Handler) TOTPDisable(w http.ResponseWriter, r *http.Request) {
	cookie, _ := r.Cookie(cookieName)
	if cookie == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	userID, _ := validateSession(h.db, cookie.Value)
	if userID == 0 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		Code string `json:"code"`
	}
	json.NewDecoder(r.Body).Decode(&req) //nolint:errcheck

	// Require a valid TOTP code to disable (prevents accidental disable)
	var secret string
	h.db.QueryRowContext(r.Context(), "SELECT COALESCE(totp_secret,'') FROM users WHERE id=?", userID).Scan(&secret)
	if secret != "" && !totp.Verify(secret, req.Code, time.Now()) {
		http.Error(w, `{"error":"invalid code"}`, http.StatusUnauthorized)
		return
	}

	_, _ = h.db.ExecContext(r.Context(), "UPDATE users SET totp_secret=NULL WHERE id=?", userID)
	auditLog(h.db, "totp.disabled", "", "")
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"disabled"}`))
}

// TOTPVerify handles the second step of login when TOTP is enabled.
// POST /api/auth/totp/verify  body: {token, code}
func (h *Handler) TOTPVerify(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"token"`
		Code  string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Token == "" || req.Code == "" {
		http.Error(w, `{"error":"token and code required"}`, http.StatusBadRequest)
		return
	}

	// Look up the pending token
	var userID int64
	err := h.db.QueryRowContext(r.Context(),
		"SELECT user_id FROM totp_pending WHERE token=? AND expires_at > CURRENT_TIMESTAMP",
		req.Token).Scan(&userID)
	if err != nil {
		http.Error(w, `{"error":"token expired or invalid"}`, http.StatusUnauthorized)
		return
	}

	// Verify TOTP code
	var secret string
	h.db.QueryRowContext(r.Context(), "SELECT COALESCE(totp_secret,'') FROM users WHERE id=?", userID).Scan(&secret)
	if !totp.Verify(secret, req.Code, time.Now()) {
		http.Error(w, `{"error":"invalid code"}`, http.StatusUnauthorized)
		return
	}

	// Consume the token
	_, _ = h.db.ExecContext(r.Context(), "DELETE FROM totp_pending WHERE token=?", req.Token)

	// Create a full session
	sessionID, err := createSession(h.db, userID)
	if err != nil {
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
	auditLog(h.db, "login.totp.success", "", "")
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

// newTOTPToken creates a short-lived token for the TOTP pending table.
func newTOTPToken() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// storeTOTPPending saves a pending TOTP auth state.
func storeTOTPPending(h *Handler, userID int64) (string, error) {
	token, err := newTOTPToken()
	if err != nil {
		return "", err
	}
	_, err = h.db.Exec(
		"INSERT INTO totp_pending(token, user_id, expires_at) VALUES(?, ?, ?)",
		token, userID, time.Now().Add(totpPendingTTL),
	)
	return token, err
}
