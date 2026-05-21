// Package totp implements RFC 6238 Time-based One-Time Passwords.
// No external dependencies — uses only the standard library.
package totp

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // SHA1 required by RFC 4226 HOTP spec
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"time"
)

const (
	digits   = 6
	stepSecs = 30
	// Allow ±1 step to handle clock skew between client and server.
	skewSteps = 1
)

// GenerateSecret returns a random base32-encoded secret for a new TOTP credential.
func GenerateSecret() (string, error) {
	b := make([]byte, 20) // 160-bit secret
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate secret: %w", err)
	}
	return base32.StdEncoding.EncodeToString(b), nil
}

// OTPAuthURI builds the otpauth:// URI that authenticator apps scan (or accept as text).
// issuer is the app name shown in the authenticator (e.g. "Private Cloud Gateway").
func OTPAuthURI(secret, issuer, accountName string) string {
	return fmt.Sprintf(
		"otpauth://totp/%s:%s?secret=%s&issuer=%s&algorithm=SHA1&digits=%d&period=%d",
		issuer, accountName, secret, issuer, digits, stepSecs,
	)
}

// MinSecretLen is the minimum accepted base32-encoded secret length (160 bits = 32 chars).
// Shorter secrets are brute-forceable and are rejected.
const MinSecretLen = 32

// Verify checks whether code is valid for the given base32 secret at time t.
// Accepts codes from the previous, current, and next step window.
func Verify(secret, code string, t time.Time) bool {
	secret = strings.ToUpper(strings.TrimSpace(secret))
	if len(secret) < MinSecretLen {
		return false
	}
	key, err := base32.StdEncoding.DecodeString(secret)
	if err != nil {
		return false
	}
	code = strings.TrimSpace(code)
	step := t.Unix() / stepSecs
	for i := -skewSteps; i <= skewSteps; i++ {
		if hotp(key, step+int64(i)) == code {
			return true
		}
	}
	return false
}

// hotp computes HOTP(key, counter) as a zero-padded decimal string.
func hotp(key []byte, counter int64) string {
	msg := make([]byte, 8)
	binary.BigEndian.PutUint64(msg, uint64(counter)) //nolint:gosec
	h := hmac.New(sha1.New, key)
	_, _ = h.Write(msg)
	sum := h.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	code := int(sum[offset]&0x7f)<<24 |
		int(sum[offset+1])<<16 |
		int(sum[offset+2])<<8 |
		int(sum[offset+3])
	code %= int(math.Pow10(digits))
	return fmt.Sprintf("%0*d", digits, code)
}
