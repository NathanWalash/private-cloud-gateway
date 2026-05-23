package server_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// jsonGet performs a GET with Accept: application/json so RequireAuth returns 401 not 302.
func jsonGet(t *testing.T, ts *httptest.Server, path string, cookie *http.Cookie) *http.Response {
	t.Helper()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", ts.URL+path, nil)
	req.Header.Set("Accept", "application/json")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	resp, err := noRedirect().Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	return resp
}

// ── GET /api/apps/updates ─────────────────────────────────────────────────────

func TestAppUpdates_RequiresAuth(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	resp := jsonGet(t, ts, "/api/apps/updates", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("GET /api/apps/updates unauthenticated: got %d, want 401", resp.StatusCode)
	}
}

func TestAppUpdates_ReturnsArrayWhenAuthenticated(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	cookie := loginAndGetCookie(t, ts)
	resp := jsonGet(t, ts, "/api/apps/updates", cookie)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /api/apps/updates authenticated: got %d, want 200", resp.StatusCode)
	}
	var body []interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Errorf("response is not a JSON array: %v", err)
	}
}

// ── GET /api/apps/events ──────────────────────────────────────────────────────

func TestAppEvents_RequiresAuth(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	resp := jsonGet(t, ts, "/api/apps/events", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("GET /api/apps/events unauthenticated: got %d, want 401", resp.StatusCode)
	}
}

func TestAppEvents_ContentType(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	cookie := loginAndGetCookie(t, ts)

	// Use a context with cancel so the SSE connection is closed after we verify headers.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/api/apps/events", nil)
	req.AddCookie(cookie)
	// SSE requires Accept: text/event-stream but RequireAuth also checks Accept for JSON guard.
	// No Accept header means RequireAuth redirects on auth failure — we're authenticated so this is fine.

	resp, err := noRedirect().Do(req)
	if err != nil && ctx.Err() == nil {
		t.Fatal(err)
	}
	if resp == nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/apps/events: got %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type: %q, want text/event-stream", ct)
	}

	// Read the keep-alive comment (": connected\n\n")
	buf := make([]byte, 64)
	n, _ := io.ReadAtLeast(resp.Body, buf, 1)
	if n == 0 {
		t.Error("expected keep-alive comment, got nothing")
	}
	cancel() // close the SSE connection
}

// ── GET /api/backup/last-run ──────────────────────────────────────────────────

func TestBackupLastRun_RequiresAuth(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	resp := jsonGet(t, ts, "/api/backup/last-run", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("GET /api/backup/last-run unauthenticated: got %d, want 401", resp.StatusCode)
	}
}

func TestBackupLastRun_ReturnsNullWhenNeverRun(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	cookie := loginAndGetCookie(t, ts)
	resp := jsonGet(t, ts, "/api/backup/last-run", cookie)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/backup/last-run: got %d, want 200", resp.StatusCode)
	}
	var body struct {
		LastRun interface{} `json:"last_run"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.LastRun != nil {
		t.Errorf("expected last_run to be null on fresh DB, got %v", body.LastRun)
	}
}

// ── GET /api/audit pagination ─────────────────────────────────────────────────

func TestAuditLog_PaginationParams(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	cookie := loginAndGetCookie(t, ts)
	resp := jsonGet(t, ts, "/api/audit?limit=5&offset=0", cookie)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/audit: got %d, want 200", resp.StatusCode)
	}
	var body struct {
		Entries []interface{} `json:"entries"`
		Total   int           `json:"total"`
		Limit   int           `json:"limit"`
		Offset  int           `json:"offset"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Limit != 5 {
		t.Errorf("limit: got %d, want 5", body.Limit)
	}
	if body.Offset != 0 {
		t.Errorf("offset: got %d, want 0", body.Offset)
	}
}
