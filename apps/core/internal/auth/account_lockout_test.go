package auth

import "testing"

func TestAccountGuard(t *testing.T) {
	g := newAccountGuard()
	key := "user@example.com"

	if g.locked(key) {
		t.Fatal("account should start unlocked")
	}

	// Failures below the threshold do not lock.
	for i := 1; i < maxAccountFails; i++ {
		if g.recordFail(key) {
			t.Fatalf("locked too early at failure %d", i)
		}
		if g.locked(key) {
			t.Fatalf("reported locked too early at failure %d", i)
		}
	}

	// The threshold failure trips the lock, reported exactly once.
	if !g.recordFail(key) {
		t.Fatal("threshold failure should trip the lock")
	}
	if !g.locked(key) {
		t.Fatal("account should be locked after the threshold")
	}

	// An unrelated account is unaffected.
	if g.locked("someone-else@example.com") {
		t.Error("unrelated account must not be locked")
	}

	// A successful login clears the failure streak.
	g.recordSuccess(key)
	if g.locked(key) {
		t.Error("recordSuccess must clear the lock")
	}
}
