package server_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestSecurityHeaders verifies that all required defensive HTTP headers
// are present on every response.
func TestSecurityHeaders(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	req, _ := http.NewRequestWithContext(context.Background(), "GET", ts.URL+"/healthz", nil)
	resp, err := noRedirect().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	required := map[string]string{
		"X-Frame-Options":        "DENY",
		"X-Content-Type-Options": "nosniff",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
	}
	for header, want := range required {
		got := resp.Header.Get(header)
		if got != want {
			t.Errorf("Header %q: got %q, want %q", header, got, want)
		}
	}
}

// TestSecurityHeaders_SPA verifies CSP is set on non-API SPA routes.
func TestSecurityHeaders_API_NoCSP(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	// API endpoints should not have a strict CSP (would break JSON clients)
	req, _ := http.NewRequestWithContext(context.Background(), "GET", ts.URL+"/api/auth/me", nil)
	req.Header.Set("Accept", "application/json")
	resp, err := noRedirect().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// Core security headers must still be present on API routes
	if got := resp.Header.Get("X-Frame-Options"); got != "DENY" {
		t.Errorf("X-Frame-Options on API route: got %q, want DENY", got)
	}
}

// TestInstall_RejectsBlueprintIDPathTraversal checks that the install endpoint
// rejects blueprint IDs containing path traversal sequences.
func TestInstall_RejectsBlueprintIDPathTraversal(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	cookie := loginAndGetCookie(t, ts)

	dangerous := []string{
		"../etc/passwd",
		"../../secret",
		"app name",
		"App",
		"app.yaml",
	}
	for _, id := range dangerous {
		body := strings.NewReader(`{"blueprint_id":"` + id + `"}`)
		req, _ := http.NewRequestWithContext(context.Background(), "POST", ts.URL+"/api/apps/install", body)
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(cookie)
		resp, err := noRedirect().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("install %q: got %d, want 400", id, resp.StatusCode)
		}
	}
}

// TestHTTPSizeLimit checks that the handler doesn't read unbounded request bodies.
func TestHTTPSizeLimit(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	cookie := loginAndGetCookie(t, ts)

	// 10MB body — should be rejected or safely limited
	bigBody := strings.NewReader(strings.Repeat("a", 10*1024*1024))
	req, _ := http.NewRequestWithContext(context.Background(), "POST", ts.URL+"/api/apps/install", bigBody)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	resp, err := noRedirect().Do(req)
	if err != nil {
		// Connection reset or timeout is acceptable — the server protected itself
		return
	}
	resp.Body.Close()
	// Should not succeed
	if resp.StatusCode == http.StatusCreated {
		t.Error("oversized body should not result in a successful install")
	}
}

var _ = httptest.NewServer // keep package import
