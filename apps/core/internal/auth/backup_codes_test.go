package auth

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
)

// makeUser inserts a user and returns the database plus the new user's id.
func makeUser(t *testing.T) (*sql.DB, int64) {
	t.Helper()
	database := setupTestDB(t)
	if err := dbCreateUser(database, "u@example.com", "password123", "U", ""); err != nil {
		t.Fatal(err)
	}
	var uid int64
	if err := database.QueryRow("SELECT id FROM users WHERE email=?", "u@example.com").Scan(&uid); err != nil {
		t.Fatal(err)
	}
	return database, uid
}

func TestGenerateBackupCodes_RoundTrip(t *testing.T) {
	db, uid := makeUser(t)

	codes, err := GenerateBackupCodes(db, uid)
	if err != nil {
		t.Fatalf("GenerateBackupCodes: %v", err)
	}
	if len(codes) != numBackupCodes {
		t.Fatalf("got %d codes, want %d", len(codes), numBackupCodes)
	}
	for _, c := range codes {
		if len(c) != backupCodeDigits {
			t.Errorf("code %q is %d digits, want %d", c, len(c), backupCodeDigits)
		}
	}

	// Fresh set: all present, none used.
	if total, unused := BackupCodeStatus(db, uid); total != numBackupCodes || unused != numBackupCodes {
		t.Fatalf("status after generate = (%d,%d), want (%d,%d)", total, unused, numBackupCodes, numBackupCodes)
	}

	// A valid code is consumed exactly once.
	if !UseBackupCode(db, uid, codes[0]) {
		t.Fatal("first use of a valid code should succeed")
	}
	if UseBackupCode(db, uid, codes[0]) {
		t.Error("reusing a consumed code must fail")
	}
	if _, unused := BackupCodeStatus(db, uid); unused != numBackupCodes-1 {
		t.Errorf("unused after one use = %d, want %d", unused, numBackupCodes-1)
	}

	// A code that was never issued is rejected.
	if UseBackupCode(db, uid, "0000000000") {
		t.Error("an unknown code must be rejected")
	}

	// Regenerating replaces the old set entirely.
	if _, err := GenerateBackupCodes(db, uid); err != nil {
		t.Fatalf("regenerate: %v", err)
	}
	if total, unused := BackupCodeStatus(db, uid); total != numBackupCodes || unused != numBackupCodes {
		t.Errorf("status after regenerate = (%d,%d), want a fresh set", total, unused)
	}
	// The old consumed code is gone, so it must not work after regeneration.
	if UseBackupCode(db, uid, codes[0]) {
		t.Error("a code from the previous set must not work after regeneration")
	}
}

func TestUseBackupCode_UnknownUser(t *testing.T) {
	db, _ := makeUser(t)
	// No codes exist for user 999.
	if UseBackupCode(db, 999, "0000000000") {
		t.Error("a code for a user with no backup codes must be rejected")
	}
}

func TestTOTPBackupCodeHandlers_RequireSession(t *testing.T) {
	db, _ := makeUser(t)
	h := &Handler{db: db}

	for _, tc := range []struct {
		name    string
		handler func(http.ResponseWriter, *http.Request)
	}{
		{"generate", h.TOTPGenBackupCodes},
		{"status", h.TOTPBackupCodeStatus},
	} {
		rec := httptest.NewRecorder()
		tc.handler(rec, httptest.NewRequest("GET", "/api/auth/totp/backup-codes", nil))
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s without a session cookie = %d, want 401", tc.name, rec.Code)
		}
	}
}
