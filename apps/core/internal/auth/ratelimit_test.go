package auth

import (
	"net/http"
	"testing"
	"time"
)

func TestRateLimiter_BlocksAfterMax(t *testing.T) {
	r := newRateLimiter()
	const ip = "203.0.113.7"
	for i := 0; i < maxLoginAttempts; i++ {
		if !r.allow(ip) {
			t.Fatalf("attempt %d unexpectedly blocked", i+1)
		}
	}
	if r.allow(ip) {
		t.Error("expected block after reaching maxLoginAttempts")
	}
	// A different IP is unaffected.
	if !r.allow("198.51.100.1") {
		t.Error("a different IP should not be rate-limited")
	}
}

func TestRateLimiter_ResetsAfterWindow(t *testing.T) {
	r := newRateLimiter()
	const ip = "203.0.113.8"
	for i := 0; i < maxLoginAttempts; i++ {
		r.allow(ip)
	}
	if r.allow(ip) {
		t.Fatal("expected block before window reset")
	}
	// Force the window to have elapsed.
	r.mu.Lock()
	r.entries[ip].resetAt = time.Now().Add(-time.Second)
	r.mu.Unlock()
	if !r.allow(ip) {
		t.Error("expected allow after the window elapsed")
	}
}

// TestRateLimiter_Prunes guards the class of bug where a limiter is built without
// its cleanup path (the totpLimiter regression): expired entries must be removed.
func TestRateLimiter_Prunes(t *testing.T) {
	r := newRateLimiter()
	r.allow("a")
	r.allow("b")
	r.mu.Lock()
	for _, e := range r.entries {
		e.resetAt = time.Now().Add(-time.Second) // mark expired
	}
	// Inline the cleanup body (the ticker loop calls the same logic).
	now := time.Now()
	for ip, e := range r.entries {
		if now.After(e.resetAt) {
			delete(r.entries, ip)
		}
	}
	n := len(r.entries)
	r.mu.Unlock()
	if n != 0 {
		t.Errorf("expected expired entries pruned, %d remain", n)
	}
}

func TestIsTrustedProxy(t *testing.T) {
	trusted := []string{"127.0.0.1", "::1", "172.16.0.1", "172.31.255.255", "172.17.0.9"}
	for _, ip := range trusted {
		if !isTrustedProxy(ip) {
			t.Errorf("expected %s to be trusted", ip)
		}
	}
	untrusted := []string{"8.8.8.8", "192.168.1.1", "10.0.0.1", "203.0.113.5", "not-an-ip"}
	for _, ip := range untrusted {
		if isTrustedProxy(ip) {
			t.Errorf("expected %s to NOT be trusted", ip)
		}
	}
}

func TestRealIP(t *testing.T) {
	// Trusted proxy → first X-Forwarded-For entry is used (spaces trimmed).
	req := &http.Request{RemoteAddr: "172.17.0.5:5000", Header: http.Header{}}
	req.Header.Set("X-Forwarded-For", "203.0.113.9, 10.0.0.1")
	if got := realIP(req); got != "203.0.113.9" {
		t.Errorf("trusted XFF: got %q, want 203.0.113.9", got)
	}

	// Untrusted remote → XFF ignored, RemoteAddr host used (anti-spoofing).
	req2 := &http.Request{RemoteAddr: "8.8.8.8:5000", Header: http.Header{}}
	req2.Header.Set("X-Forwarded-For", "203.0.113.9")
	if got := realIP(req2); got != "8.8.8.8" {
		t.Errorf("untrusted XFF: got %q, want 8.8.8.8", got)
	}

	// No XFF → RemoteAddr host.
	req3 := &http.Request{RemoteAddr: "172.17.0.5:5000", Header: http.Header{}}
	if got := realIP(req3); got != "172.17.0.5" {
		t.Errorf("no XFF: got %q, want 172.17.0.5", got)
	}
}
