package auth

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/NathanWalash/private-cloud-gateway/apps/core/internal/totp"
)

// authedReq builds a request carrying a valid session cookie for uid.
func authedReq(t *testing.T, db *sql.DB, method, path, body string) *http.Request {
	t.Helper()
	var uid int64
	_ = db.QueryRow("SELECT id FROM users WHERE email=?", "u@example.com").Scan(&uid)
	sid, err := createSession(db, uid)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.AddCookie(&http.Cookie{Name: cookieName, Value: sid})
	return req
}

func TestTOTPStatus_ReflectsSecret(t *testing.T) {
	db, uid := makeUser(t)
	h := &Handler{db: db, cookieDomain: "example.com"}

	rec := httptest.NewRecorder()
	h.TOTPStatus(rec, authedReq(t, db, "GET", "/api/auth/totp/status", ""))
	if !strings.Contains(rec.Body.String(), `"enabled":false`) {
		t.Errorf("status before setup = %s, want enabled:false", rec.Body.String())
	}

	if _, err := db.Exec("UPDATE users SET totp_secret=? WHERE id=?", "SECRET", uid); err != nil {
		t.Fatal(err)
	}
	rec = httptest.NewRecorder()
	h.TOTPStatus(rec, authedReq(t, db, "GET", "/api/auth/totp/status", ""))
	if !strings.Contains(rec.Body.String(), `"enabled":true`) {
		t.Errorf("status after setting secret = %s, want enabled:true", rec.Body.String())
	}
}

// TestTOTP_SetupConfirmDisableFlow walks the full enable/disable lifecycle with
// real TOTP codes.
func TestTOTP_SetupConfirmDisableFlow(t *testing.T) {
	db, _ := makeUser(t)
	h := &Handler{db: db, cookieDomain: "example.com"}

	// Setup returns a fresh secret + otpauth URI (not yet saved).
	rec := httptest.NewRecorder()
	h.TOTPSetup(rec, authedReq(t, db, "POST", "/api/auth/totp/setup", ""))
	if rec.Code != 200 {
		t.Fatalf("setup = %d", rec.Code)
	}
	var setup struct{ Secret, URI string }
	if err := json.Unmarshal(rec.Body.Bytes(), &setup); err != nil {
		t.Fatal(err)
	}
	if setup.Secret == "" || !strings.Contains(setup.URI, "otpauth://") {
		t.Fatalf("setup payload = %+v", setup)
	}

	now := time.Now()
	code, _ := totp.GenerateCode(setup.Secret, now)

	// Confirm with a wrong code is rejected and does NOT enable TOTP.
	rec = httptest.NewRecorder()
	h.TOTPConfirm(rec, authedReq(t, db, "POST", "/api/auth/totp/confirm",
		`{"secret":"`+setup.Secret+`","code":"000000"}`))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("confirm with wrong code = %d, want 401", rec.Code)
	}

	// Confirm with the right code enables it and persists the secret.
	rec = httptest.NewRecorder()
	h.TOTPConfirm(rec, authedReq(t, db, "POST", "/api/auth/totp/confirm",
		`{"secret":"`+setup.Secret+`","code":"`+code+`"}`))
	if rec.Code != 200 {
		t.Fatalf("confirm with valid code = %d, body=%s", rec.Code, rec.Body.String())
	}
	var saved string
	_ = db.QueryRow("SELECT totp_secret FROM users WHERE email='u@example.com'").Scan(&saved)
	if saved != setup.Secret {
		t.Error("secret not persisted after confirm")
	}

	// Disable requires a valid code; wrong code is refused.
	rec = httptest.NewRecorder()
	h.TOTPDisable(rec, authedReq(t, db, "POST", "/api/auth/totp/disable", `{"code":"000000"}`))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("disable with wrong code = %d, want 401", rec.Code)
	}

	code, _ = totp.GenerateCode(setup.Secret, time.Now())
	rec = httptest.NewRecorder()
	h.TOTPDisable(rec, authedReq(t, db, "POST", "/api/auth/totp/disable", `{"code":"`+code+`"}`))
	if rec.Code != 200 {
		t.Fatalf("disable with valid code = %d", rec.Code)
	}
	var after sql.NullString
	_ = db.QueryRow("SELECT totp_secret FROM users WHERE email='u@example.com'").Scan(&after)
	if after.Valid && after.String != "" {
		t.Error("secret should be cleared after disable")
	}
}

// TestTOTPVerify_CompletesLogin covers the second login factor: a pending token
// plus a valid code yields a session cookie.
func TestTOTPVerify_CompletesLogin(t *testing.T) {
	db, uid := makeUser(t)
	h := &Handler{db: db, cookieDomain: "example.com"}

	secret, _ := totp.GenerateSecret()
	if _, err := db.Exec("UPDATE users SET totp_secret=? WHERE id=?", secret, uid); err != nil {
		t.Fatal(err)
	}
	token, err := storeTOTPPending(h, uid)
	if err != nil {
		t.Fatal(err)
	}

	// Bad token → 401.
	rec := httptest.NewRecorder()
	h.TOTPVerify(rec, httptest.NewRequest("POST", "/api/auth/totp/verify", strings.NewReader(
		`{"token":"deadbeef","code":"000000"}`)))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("verify with bad token = %d, want 401", rec.Code)
	}

	// Valid token + valid code → 200 and a session cookie is set.
	code, _ := totp.GenerateCode(secret, time.Now())
	rec = httptest.NewRecorder()
	h.TOTPVerify(rec, httptest.NewRequest("POST", "/api/auth/totp/verify", strings.NewReader(
		`{"token":"`+token+`","code":"`+code+`"}`)))
	if rec.Code != 200 {
		t.Fatalf("verify with valid code = %d, body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get("Set-Cookie"), cookieName) {
		t.Error("expected a session cookie after successful TOTP verify")
	}
	// The pending token is single-use — consumed on success.
	var n int
	_ = db.QueryRow("SELECT COUNT(*) FROM totp_pending WHERE token=?", token).Scan(&n)
	if n != 0 {
		t.Error("pending token should be deleted after use")
	}
}
