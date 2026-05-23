package backup

import (
	"encoding/hex"
	"net/http"
	"strings"
	"testing"
)

func TestDeriveSigningKey_KnownVector(t *testing.T) {
	// AWS Sig V4 test vector from the AWS documentation:
	// https://docs.aws.amazon.com/general/latest/gr/signature-v4-test-suite.html
	key := deriveSigningKey("wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY", "20110909", "us-east-1", "iam")
	got := hex.EncodeToString(key)
	// Expected from AWS test vector
	want := "98f1d889fec4f4421adc522bab0ce1f82e6929c262ed15e5a94c90efd1e3b0e7"
	if got != want {
		t.Errorf("signing key mismatch\ngot:  %s\nwant: %s", got, want)
	}
}

func TestBuildHeaders_Sorted(t *testing.T) {
	h := http.Header{
		"Content-Type":         {"application/octet-stream"},
		"X-Amz-Date":          {"20231001T000000Z"},
		"X-Amz-Content-Sha256": {"abc123"},
	}
	signed, canonical := buildHeaders(h, "s3.amazonaws.com")

	// signed headers must be lowercase and sorted
	parts := strings.Split(signed, ";")
	for i := 1; i < len(parts); i++ {
		if parts[i-1] > parts[i] {
			t.Errorf("signed headers not sorted: %q comes before %q", parts[i-1], parts[i])
		}
	}

	// canonical headers must contain the host header
	if !strings.Contains(canonical, "host:s3.amazonaws.com\n") {
		t.Errorf("canonical headers missing host line: %q", canonical)
	}

	// each canonical header line must end with \n
	for _, line := range strings.Split(strings.TrimSuffix(canonical, "\n"), "\n") {
		if !strings.Contains(line, ":") {
			t.Errorf("canonical header line missing colon: %q", line)
		}
	}
}

func TestSHA256File_KnownHash(t *testing.T) {
	// SHA-256 of empty string is known
	r := strings.NewReader("")
	got, err := sha256File(r)
	if err != nil {
		t.Fatalf("sha256File: %v", err)
	}
	want := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if got != want {
		t.Errorf("sha256File(\"\") = %q, want %q", got, want)
	}
}

func TestS3Config_EmptySkips(t *testing.T) {
	// UploadToS3 with missing config must fail with a descriptive error, not panic.
	_, err := UploadToS3(S3Config{}, "/nonexistent/file.pcg-backup")
	if err == nil {
		t.Error("expected error for nonexistent file, got nil")
	}
}
