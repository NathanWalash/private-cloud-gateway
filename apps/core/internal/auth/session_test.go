package auth

import "testing"

func TestNewSessionID_UniqueAndLong(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 200; i++ {
		id, err := newSessionID()
		if err != nil {
			t.Fatal(err)
		}
		if len(id) < 32 {
			t.Fatalf("session id too short (%d chars) — not enough entropy", len(id))
		}
		if seen[id] {
			t.Fatal("duplicate session id generated")
		}
		seen[id] = true
	}
}

func TestSessionRoundTrip(t *testing.T) {
	db := setupTestDB(t)
	if err := dbCreateUser(db, "u@example.com", "password123", "U", ""); err != nil {
		t.Fatal(err)
	}
	var uid int64
	if err := db.QueryRow("SELECT id FROM users WHERE email=?", "u@example.com").Scan(&uid); err != nil {
		t.Fatal(err)
	}

	sid, err := createSession(db, uid)
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}
	if sid == "" {
		t.Fatal("createSession returned empty id")
	}

	gotUID, err := validateSession(db, sid)
	if err != nil {
		t.Fatalf("validateSession: %v", err)
	}
	if gotUID != uid {
		t.Errorf("validateSession returned uid %d, want %d", gotUID, uid)
	}

	// Deleting the session invalidates it — validateSession returns uid 0
	// (no row) rather than an error.
	if err := deleteSession(db, sid); err != nil {
		t.Fatalf("deleteSession: %v", err)
	}
	if gotUID, err := validateSession(db, sid); err != nil || gotUID != 0 {
		t.Errorf("after deletion: got (%d, %v), want (0, nil)", gotUID, err)
	}

	// An unknown session id resolves to no user.
	if gotUID, err := validateSession(db, "does-not-exist"); err != nil || gotUID != 0 {
		t.Errorf("unknown id: got (%d, %v), want (0, nil)", gotUID, err)
	}
}
