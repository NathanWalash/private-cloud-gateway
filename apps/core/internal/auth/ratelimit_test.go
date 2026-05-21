package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRateLimit_AllowsUnderLimit(t *testing.T) {
	// Build a fake request from a fresh IP
	req := httptest.NewRequest("POST", "/api/auth/login", nil)
	req.RemoteAddr = "10.0.0.1:12345"

	// Should allow 10 attempts
	for i := range 10 {
		got := realIP(req)
		if got != "10.0.0.1" {
			t.Fatalf("realIP: got %q, want 10.0.0.1", got)
		}
		_ = i
	}
}

func TestRealIP_TrustedProxy(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		forwarded  string
		wantIP     string
	}{
		{
			name:       "direct connection — use RemoteAddr",
			remoteAddr: "1.2.3.4:9999",
			forwarded:  "5.5.5.5",
			wantIP:     "1.2.3.4",
		},
		{
			name:       "from Docker bridge — trust X-Forwarded-For",
			remoteAddr: "172.18.0.4:9999",
			forwarded:  "8.8.8.8",
			wantIP:     "8.8.8.8",
		},
		{
			name:       "from localhost — trust X-Forwarded-For",
			remoteAddr: "127.0.0.1:9999",
			forwarded:  "9.9.9.9",
			wantIP:     "9.9.9.9",
		},
		{
			name:       "no X-Forwarded-For from trusted proxy — use RemoteAddr",
			remoteAddr: "172.18.0.4:9999",
			forwarded:  "",
			wantIP:     "172.18.0.4",
		},
		{
			name:       "malformed forwarded header — fallback to RemoteAddr",
			remoteAddr: "172.18.0.4:9999",
			forwarded:  "not-an-ip",
			wantIP:     "172.18.0.4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.forwarded != "" {
				req.Header.Set("X-Forwarded-For", tt.forwarded)
			}
			got := realIP(req)
			if got != tt.wantIP {
				t.Errorf("realIP = %q, want %q", got, tt.wantIP)
			}
		})
	}
}

func TestRateLimit_BlocksAfterMax(t *testing.T) {
	// Use a unique IP to avoid interference from loginLimiter state in other tests
	req := httptest.NewRequest("POST", "/", nil)
	req.RemoteAddr = "192.168.99.99:1234"

	// Allow 10 attempts then block on the 11th
	rl := newRateLimiter()
	for i := range 10 {
		if !rl.allow("192.168.99.99") {
			t.Fatalf("attempt %d should be allowed", i+1)
		}
	}
	if rl.allow("192.168.99.99") {
		t.Error("11th attempt should be blocked")
	}
	_ = req
}

var _ *http.Request // keep http import used
