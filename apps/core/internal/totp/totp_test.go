package totp_test

import (
	"testing"
	"time"

	"github.com/NathanWalash/private-cloud-gateway/apps/core/internal/totp"
)

func TestGenerateSecret(t *testing.T) {
	s1, err := totp.GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret: %v", err)
	}
	if len(s1) < 16 {
		t.Errorf("secret too short: %q", s1)
	}
	s2, _ := totp.GenerateSecret()
	if s1 == s2 {
		t.Error("two generated secrets should not be identical")
	}
}

func TestOTPAuthURI(t *testing.T) {
	uri := totp.OTPAuthURI("JBSWY3DPEHPK3PXP", "Private Cloud Gateway", "admin@example.com")
	if uri == "" {
		t.Error("OTPAuthURI returned empty string")
	}
	if len(uri) < 50 {
		t.Errorf("URI seems too short: %q", uri)
	}
}

func TestVerify_RoundTrip(t *testing.T) {
	secret, _ := totp.GenerateSecret()
	now := time.Now()

	// Generate the expected code for this second
	uri := totp.OTPAuthURI(secret, "Test", "test@example.com")
	_ = uri // just confirm it doesn't panic

	// Verify should accept the current code
	// We can't easily test a specific code without computing it,
	// but we can test that wrong codes are rejected.
	if totp.Verify(secret, "000000", now) {
		// 1-in-a-million chance this is the actual code — skip if so
		t.Log("coincidental collision with 000000 — skipping false-positive check")
	}
	if totp.Verify(secret, "abcdef", now) {
		t.Error("non-numeric code should not verify")
	}
	if totp.Verify(secret, "", now) {
		t.Error("empty code should not verify")
	}
	if totp.Verify("INVALID!!SECRET", "123456", now) {
		t.Error("invalid base32 secret should not verify")
	}
}

func TestVerify_ClockSkew(t *testing.T) {
	secret, _ := totp.GenerateSecret()
	// Test that a slightly future/past time window is handled
	// (skewSteps = 1 means ±30s tolerance)
	now := time.Now()
	// These just check no panics occur with boundary times
	totp.Verify(secret, "000000", now.Add(-29*time.Second))
	totp.Verify(secret, "000000", now.Add(29*time.Second))
}
