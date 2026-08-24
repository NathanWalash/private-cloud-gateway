package backup

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"io"
	"testing"

	"golang.org/x/crypto/pbkdf2"
)

const testPass = "correct horse battery staple"

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	plain := []byte("the quick brown fox — cloud-core.db bytes")
	var enc bytes.Buffer
	if err := encryptStream(bytes.NewReader(plain), &enc, testPass); err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	// The new format carries the versioned header and the current iteration count.
	if !bytes.HasPrefix(enc.Bytes(), kdfMagic) {
		t.Fatal("ciphertext missing KDF magic header")
	}

	r, err := decryptStream(bytes.NewReader(enc.Bytes()), testPass)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	got, _ := io.ReadAll(r)
	if !bytes.Equal(got, plain) {
		t.Errorf("round-trip mismatch: got %q", got)
	}

	// Wrong passphrase must fail (GCM auth), not return garbage.
	if _, err := decryptStream(bytes.NewReader(enc.Bytes()), "wrong"); err == nil {
		t.Error("decrypt with wrong passphrase must fail")
	}
}

// TestDecrypt_LegacyFormat proves pre-v0.8.3 backups (header-less, salt-first,
// 100k iterations) still decrypt after the format upgrade.
func TestDecrypt_LegacyFormat(t *testing.T) {
	plain := []byte("legacy backup payload")

	salt := make([]byte, saltLen)
	_, _ = rand.Read(salt)
	key := pbkdf2.Key([]byte(testPass), salt, legacyIterations, keyLen, sha256.New)
	block, _ := aes.NewCipher(key)
	gcm, _ := cipher.NewGCM(block)
	nonce := make([]byte, gcm.NonceSize())
	_, _ = rand.Read(nonce)

	// Old format: [salt][nonce+ciphertext], NO header.
	blob := append(append([]byte{}, salt...), gcm.Seal(nonce, nonce, plain, nil)...)

	r, err := decryptStream(bytes.NewReader(blob), testPass)
	if err != nil {
		t.Fatalf("legacy decrypt: %v", err)
	}
	got, _ := io.ReadAll(r)
	if !bytes.Equal(got, plain) {
		t.Errorf("legacy round-trip mismatch: got %q", got)
	}
}

// TestDecrypt_RejectsInsaneIterations guards against a crafted header forcing a
// multi-billion-round PBKDF2 (decrypt DoS).
func TestDecrypt_RejectsInsaneIterations(t *testing.T) {
	header := append([]byte{}, kdfMagic...)
	header = append(header, kdfVersion)
	header = append(header, 0xff, 0xff, 0xff, 0xff) // ~4.3 billion iterations
	header = append(header, make([]byte, saltLen)...)
	header = append(header, make([]byte, 64)...) // some ciphertext bytes

	if _, err := decryptStream(bytes.NewReader(header), testPass); err == nil {
		t.Error("decrypt must reject an absurd iteration count")
	}
}
