package totp

import (
	"testing"
	"time"
)

func TestGenerateCode_VerifiesRoundTrip(t *testing.T) {
	secret, err := GenerateSecret()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_700_000_000, 0)

	code, err := GenerateCode(secret, now)
	if err != nil {
		t.Fatalf("GenerateCode: %v", err)
	}
	if len(code) != digits {
		t.Errorf("code = %q, want %d digits", code, digits)
	}
	if !Verify(secret, code, now) {
		t.Error("Verify must accept a code produced by GenerateCode at the same time")
	}
}

func TestGenerateCode_RejectsShortSecret(t *testing.T) {
	if _, err := GenerateCode("SHORT", time.Unix(0, 0)); err == nil {
		t.Error("GenerateCode must reject a secret shorter than MinSecretLen")
	}
}

func TestGenerateCode_RejectsInvalidBase32(t *testing.T) {
	// 32 chars but not valid base32 (contains 0/1/8/9 which base32 excludes).
	if _, err := GenerateCode("00000000000000000000000000000001", time.Unix(0, 0)); err == nil {
		t.Error("GenerateCode must reject a non-base32 secret")
	}
}
