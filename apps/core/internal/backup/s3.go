package backup

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// S3Config holds credentials and location for an S3-compatible bucket.
// Works with AWS S3, Cloudflare R2, Backblaze B2 S3-compat, Wasabi, etc.
type S3Config struct {
	Endpoint  string // e.g. "https://s3.amazonaws.com" or "https://<accountid>.r2.cloudflarestorage.com"
	Bucket    string
	Region    string // "auto" for R2
	AccessKey string
	SecretKey string
	Prefix    string // optional key prefix / folder
}

// UploadToS3 uploads a local file to the configured S3-compatible bucket.
// Returns the object key on success.
func UploadToS3(cfg S3Config, localPath string) (string, error) {
	f, err := os.Open(localPath)
	if err != nil {
		return "", fmt.Errorf("open backup file: %w", err)
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return "", err
	}

	key := filepath.Base(localPath)
	if cfg.Prefix != "" {
		key = strings.TrimSuffix(cfg.Prefix, "/") + "/" + key
	}

	endpoint := strings.TrimSuffix(cfg.Endpoint, "/")
	url := fmt.Sprintf("%s/%s/%s", endpoint, cfg.Bucket, key)
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}

	now := time.Now().UTC()
	dateISO := now.Format("20060102T150405Z")
	dateSh := now.Format("20060102")

	// Hash the file body
	bodyHash, err := sha256File(f)
	if err != nil {
		return "", err
	}
	// Rewind for actual upload
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return "", err
	}

	req, err := http.NewRequest("PUT", url, f)
	if err != nil {
		return "", err
	}
	req.ContentLength = fi.Size()
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("x-amz-date", dateISO)
	req.Header.Set("x-amz-content-sha256", bodyHash)

	// Build canonical request
	signedHeaders, canonicalHeaders := buildHeaders(req.Header, req.Host)
	canonicalRequest := strings.Join([]string{
		"PUT",
		"/" + cfg.Bucket + "/" + key,
		"",
		canonicalHeaders,
		signedHeaders,
		bodyHash,
	}, "\n")

	credScope := dateSh + "/" + cfg.Region + "/s3/aws4_request"
	stringToSign := "AWS4-HMAC-SHA256\n" + dateISO + "\n" + credScope + "\n" + hexSHA256([]byte(canonicalRequest))

	signingKey := deriveSigningKey(cfg.SecretKey, dateSh, cfg.Region, "s3")
	signature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))

	authHeader := fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s,SignedHeaders=%s,Signature=%s",
		cfg.AccessKey, credScope, signedHeaders, signature,
	)
	req.Header.Set("Authorization", authHeader)

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("s3 put: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return "", fmt.Errorf("s3 put returned %d: %s", resp.StatusCode, string(body))
	}

	return key, nil
}

// sha256File returns the hex SHA-256 of a file without loading it fully into memory.
func sha256File(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func hexSHA256(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func hmacSHA256(key, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}

func deriveSigningKey(secret, date, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secret), []byte(date))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(service))
	return hmacSHA256(kService, []byte("aws4_request"))
}

// buildHeaders returns (signedHeaders, canonicalHeaders) sorted canonically.
func buildHeaders(h http.Header, host string) (signed, canonical string) {
	headers := map[string]string{
		"host":                 host,
		"content-type":        h.Get("Content-Type"),
		"x-amz-content-sha256": h.Get("x-amz-content-sha256"),
		"x-amz-date":          h.Get("x-amz-date"),
	}
	keys := make([]string, 0, len(headers))
	for k := range headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	for _, k := range keys {
		sb.WriteString(k)
		sb.WriteByte(':')
		sb.WriteString(strings.TrimSpace(headers[k]))
		sb.WriteByte('\n')
	}
	canonical = sb.String()
	signed = strings.Join(keys, ";")
	return signed, canonical
}
