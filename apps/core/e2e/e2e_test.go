//go:build e2e

// End-to-end tests for Private Cloud Gateway.
// These tests run against a live stack and are NOT part of the standard CI pipeline.
//
// Run with:
//   E2E_BASE_URL=http://home.localtest.me \
//   E2E_EMAIL=you@example.com \
//   E2E_PASSWORD=yourpassword \
//   go test -v -tags e2e ./e2e/...
package e2e_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

var (
	baseURL   = getenv("E2E_BASE_URL", "http://home.localtest.me")
	filesURL  = getenv("E2E_FILES_URL", "http://files.localtest.me")
	testEmail = getenv("E2E_EMAIL", "")
	testPass  = getenv("E2E_PASSWORD", "")
)

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// client is a test HTTP client that does not follow redirects.
var client = &http.Client{
	Timeout: 10 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

func get(t *testing.T, path string, headers ...string) *http.Response {
	t.Helper()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", baseURL+path, nil)
	req.Header.Set("Accept", "application/json")
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	return resp
}

func post(t *testing.T, path string, body any, headers ...string) *http.Response {
	t.Helper()
	b, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(context.Background(), "POST", baseURL+path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	return resp
}

func body(t *testing.T, resp *http.Response) string {
	t.Helper()
	b, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(b)
}

func sessionCookie(resp *http.Response) string {
	for _, c := range resp.Cookies() {
		if c.Name == "pcg_session" {
			return c.Name + "=" + c.Value
		}
	}
	return ""
}

// ── Stack health ──────────────────────────────────────────────────────────────

func TestE2E_Healthz(t *testing.T) {
	resp := get(t, "/healthz")
	body(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("healthz: got %d, want 200", resp.StatusCode)
	}
}

func TestE2E_SetupEndpointResponds(t *testing.T) {
	resp := get(t, "/api/auth/setup")
	b := body(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/api/auth/setup: got %d", resp.StatusCode)
	}
	if !strings.Contains(b, "needs_setup") {
		t.Errorf("/api/auth/setup body missing needs_setup: %s", b)
	}
}

// ── Unauthenticated access ────────────────────────────────────────────────────

func TestE2E_SPAServed(t *testing.T) {
	resp := get(t, "/")
	body(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /: got %d, want 200", resp.StatusCode)
	}
}

func TestE2E_ApiRequiresAuth(t *testing.T) {
	for _, path := range []string{"/api/auth/me", "/api/apps", "/api/status", "/api/blueprints"} {
		resp := get(t, path)
		body(t, resp)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("GET %s unauthed: got %d, want 401", path, resp.StatusCode)
		}
	}
}

func TestE2E_VerifyRedirects(t *testing.T) {
	resp := get(t, "/api/auth/verify")
	body(t, resp)
	if resp.StatusCode != http.StatusFound {
		t.Errorf("/api/auth/verify unauthed: got %d, want 302", resp.StatusCode)
	}
}

func TestE2E_ProtectedSubdomainRedirects(t *testing.T) {
	req, _ := http.NewRequestWithContext(context.Background(), "GET", filesURL+"/", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Skipf("cannot reach %s: %v (add to /etc/hosts)", filesURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Errorf("files.* unauthed: got %d, want 302", resp.StatusCode)
	}
}

// ── Authenticated flows (require E2E_EMAIL + E2E_PASSWORD) ───────────────────

func requireCreds(t *testing.T) {
	t.Helper()
	if testEmail == "" || testPass == "" {
		t.Skip("set E2E_EMAIL and E2E_PASSWORD to run authenticated tests")
	}
}

func login(t *testing.T) string {
	t.Helper()
	resp := post(t, "/api/auth/login", map[string]string{
		"email":    testEmail,
		"password": testPass,
	})
	body(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login: got %d, want 200", resp.StatusCode)
	}
	cookie := sessionCookie(resp)
	if cookie == "" {
		t.Fatal("login: no pcg_session cookie in response")
	}
	return cookie
}

func TestE2E_LoginAndMe(t *testing.T) {
	requireCreds(t)
	cookie := login(t)

	resp := get(t, "/api/auth/me", "Cookie", cookie)
	b := body(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/api/auth/me: got %d body=%s", resp.StatusCode, b)
	}
	var me map[string]any
	json.Unmarshal([]byte(b), &me)
	if me["email"] != testEmail {
		t.Errorf("me.email: got %v, want %s", me["email"], testEmail)
	}
}

func TestE2E_StatusEndpoint(t *testing.T) {
	requireCreds(t)
	cookie := login(t)

	resp := get(t, "/api/status", "Cookie", cookie)
	b := body(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/api/status: got %d", resp.StatusCode)
	}
	var s map[string]any
	json.Unmarshal([]byte(b), &s)
	if s["uptime"] == nil {
		t.Errorf("/api/status missing uptime: %s", b)
	}
	if s["version"] == nil {
		t.Errorf("/api/status missing version: %s", b)
	}
}

func TestE2E_BlueprintsNonEmpty(t *testing.T) {
	requireCreds(t)
	cookie := login(t)

	resp := get(t, "/api/blueprints", "Cookie", cookie)
	b := body(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/api/blueprints: got %d", resp.StatusCode)
	}
	var bps []any
	json.Unmarshal([]byte(b), &bps)
	if len(bps) == 0 {
		t.Error("/api/blueprints: expected at least one blueprint, got empty array")
	}
	t.Logf("found %d blueprints", len(bps))
}

func TestE2E_AppsReturnsArray(t *testing.T) {
	requireCreds(t)
	cookie := login(t)

	resp := get(t, "/api/apps", "Cookie", cookie)
	b := body(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/api/apps: got %d", resp.StatusCode)
	}
	var apps []any
	if err := json.Unmarshal([]byte(b), &apps); err != nil {
		t.Errorf("/api/apps: not valid JSON array: %s", b)
	}
}

func TestE2E_BackupList(t *testing.T) {
	requireCreds(t)
	cookie := login(t)

	resp := get(t, "/api/backup/list", "Cookie", cookie)
	b := body(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/api/backup/list: got %d", resp.StatusCode)
	}
	var list []any
	if err := json.Unmarshal([]byte(b), &list); err != nil {
		t.Errorf("/api/backup/list: not valid JSON array: %s", b)
	}
}

func TestE2E_BackupCreate(t *testing.T) {
	requireCreds(t)
	cookie := login(t)

	resp := post(t, "/api/backup/create", nil, "Cookie", cookie)
	b := body(t, resp)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("/api/backup/create: got %d body=%s", resp.StatusCode, b)
	}
	var result map[string]any
	json.Unmarshal([]byte(b), &result)
	if result["name"] == nil {
		t.Errorf("/api/backup/create missing name: %s", b)
	}
	t.Logf("backup created: %s (%v bytes)", result["name"], result["size"])
}

func TestE2E_LogoutInvalidatesSession(t *testing.T) {
	requireCreds(t)
	cookie := login(t)

	// Logout
	resp := post(t, "/api/auth/logout", nil, "Cookie", cookie)
	body(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("logout: got %d, want 200", resp.StatusCode)
	}

	// Session should now be invalid
	resp2 := get(t, "/api/auth/me", "Cookie", cookie)
	body(t, resp2)
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Errorf("post-logout /api/auth/me: got %d, want 401", resp2.StatusCode)
	}
}

func TestE2E_InstallRejectsUnknownBlueprint(t *testing.T) {
	requireCreds(t)
	cookie := login(t)

	resp := post(t, "/api/apps/install", map[string]string{"blueprint_id": "nonexistent-app-xyz"}, "Cookie", cookie)
	b := body(t, resp)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("install unknown blueprint: got %d body=%s, want 404", resp.StatusCode, b)
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// Compile-time check that package-level vars are used.
var _ = fmt.Sprintf
