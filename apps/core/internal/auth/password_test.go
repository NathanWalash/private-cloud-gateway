package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestChangePassword_RequiresSession(t *testing.T) {
	db, _ := makeUser(t)
	h := &Handler{db: db}
	rec := httptest.NewRecorder()
	h.ChangePassword(rec, httptest.NewRequest("POST", "/api/auth/password", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("no session = %d, want 401", rec.Code)
	}
}

func TestChangePassword_Validation(t *testing.T) {
	db, _ := makeUser(t)
	h := &Handler{db: db}

	cases := []struct {
		name, body string
		want       int
	}{
		{"invalid json", "not json", http.StatusBadRequest},
		{"missing fields", `{"current_password":"","new_password":""}`, http.StatusBadRequest},
		{"too short", `{"current_password":"password123","new_password":"short"}`, http.StatusBadRequest},
		{"wrong current", `{"current_password":"wrong","new_password":"newpassword1"}`, http.StatusUnauthorized},
	}
	for _, tc := range cases {
		rec := httptest.NewRecorder()
		h.ChangePassword(rec, authedReq(t, db, "POST", "/api/auth/password", tc.body))
		if rec.Code != tc.want {
			t.Errorf("%s: got %d, want %d (%s)", tc.name, rec.Code, tc.want, rec.Body.String())
		}
	}
}

func TestChangePassword_Succeeds(t *testing.T) {
	db, uid := makeUser(t)
	h := &Handler{db: db}

	// A second session that must be invalidated by the password change.
	otherSid, _ := createSession(db, uid)

	rec := httptest.NewRecorder()
	h.ChangePassword(rec, authedReq(t, db, "POST", "/api/auth/password",
		`{"current_password":"password123","new_password":"a-brand-new-pass"}`))
	if rec.Code != 200 {
		t.Fatalf("change = %d, body=%s", rec.Code, rec.Body.String())
	}

	// New password hash is stored and verifies.
	var hash string
	_ = db.QueryRow("SELECT password FROM users WHERE id=?", uid).Scan(&hash)
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte("a-brand-new-pass")) != nil {
		t.Error("new password does not verify against the stored hash")
	}

	// All sessions were cleared for safety (validateSession returns uid 0 for a
	// session that no longer exists).
	if got, _ := validateSession(db, otherSid); got != 0 {
		t.Error("other sessions must be invalidated after a password change")
	}
}
