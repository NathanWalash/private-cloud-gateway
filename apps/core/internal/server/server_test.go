package server_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/NathanWalash/private-cloud-gateway/apps/core/internal/db"
	"github.com/NathanWalash/private-cloud-gateway/apps/core/internal/server"
)

const (
	testEmail    = "test@example.com"
	testPassword = "testpassword123"
	testSecret   = "test-secret-32-chars-for-testing!"
	testLoginURL = "http://home.localtest.me/login"
	testDomain   = "localtest.me"
)

// newTestServer spins up a real in-memory stack: SQLite + migrations + bootstrap user.
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	database, err := db.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	if err := db.Migrate(database); err != nil {
		t.Fatalf("db.Migrate: %v", err)
	}
	if err := db.Bootstrap(database, testEmail, testPassword); err != nil {
		t.Fatalf("db.Bootstrap: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	srv := server.New(database, []byte(testSecret), testLoginURL, testDomain, nil, nil, nil, t.TempDir())
	return httptest.NewServer(srv.Handler())
}

// noRedirect returns an http.Client that stops at the first redirect.
func noRedirect() *http.Client {
	return &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// loginAndGetCookie performs a successful login and returns the session cookie.
func loginAndGetCookie(t *testing.T, ts *httptest.Server) *http.Cookie {
	t.Helper()
	resp, err := noRedirect().PostForm(ts.URL+"/api/auth/login", url.Values{
		"email":    {testEmail},
		"password": {testPassword},
	})
	if err != nil {
		t.Fatalf("login request: %v", err)
	}
	defer resp.Body.Close()
	for _, c := range resp.Cookies() {
		if c.Name == "pcg_session" {
			return c
		}
	}
	t.Fatal("no pcg_session cookie in login response")
	return nil
}

// ── Health check ─────────────────────────────────────────────────────────────

func TestHealthz(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	resp, err := ts.Client().Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /healthz: got %d, want 200", resp.StatusCode)
	}
}

// ── Login page ────────────────────────────────────────────────────────────────

func TestLoginPageRendered(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	resp, err := ts.Client().Get(ts.URL + "/login")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /login: got %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Errorf("GET /login Content-Type: %q, want text/html", ct)
	}
}

// ── Auth guard on root ────────────────────────────────────────────────────────

func TestRoot_RedirectsToLoginWhenUnauthenticated(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	resp, err := noRedirect().Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Errorf("GET / unauthenticated: got %d, want 302", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/login" {
		t.Errorf("GET / redirect Location: %q, want /login", loc)
	}
}

func TestRoot_ServesContentWhenAuthenticated(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	cookie := loginAndGetCookie(t, ts)
	req, _ := http.NewRequestWithContext(context.Background(), "GET", ts.URL+"/", nil)
	req.AddCookie(cookie)
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET / authenticated: got %d, want 200", resp.StatusCode)
	}
}

// ── /api/auth/me ─────────────────────────────────────────────────────────────

func TestMe_Returns401WhenUnauthenticated(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	resp, err := noRedirect().Get(ts.URL + "/api/auth/me")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("GET /api/auth/me unauthenticated: got %d, want 401", resp.StatusCode)
	}
}

func TestMe_Returns200WithEmailWhenAuthenticated(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	cookie := loginAndGetCookie(t, ts)
	req, _ := http.NewRequestWithContext(context.Background(), "GET", ts.URL+"/api/auth/me", nil)
	req.AddCookie(cookie)
	resp, err := noRedirect().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /api/auth/me authenticated: got %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type: %q, want application/json", ct)
	}
}

// ── /api/auth/verify ──────────────────────────────────────────────────────────

func TestVerify_Redirects302WhenNoSession(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	resp, err := noRedirect().Get(ts.URL + "/api/auth/verify")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Errorf("verify unauthenticated: got %d, want 302 (not 401)", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != testLoginURL {
		t.Errorf("verify redirect Location: %q, want %q", loc, testLoginURL)
	}
}

func TestVerify_Returns200WithUserIDWhenAuthenticated(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	cookie := loginAndGetCookie(t, ts)
	req, _ := http.NewRequestWithContext(context.Background(), "GET", ts.URL+"/api/auth/verify", nil)
	req.AddCookie(cookie)
	resp, err := noRedirect().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("verify authenticated: got %d, want 200", resp.StatusCode)
	}
	if h := resp.Header.Get("X-Auth-User-ID"); h == "" {
		t.Error("X-Auth-User-ID header missing on successful verify")
	}
}

// ── Login ─────────────────────────────────────────────────────────────────────

func TestLogin_SetsCookieWithCorrectAttributes(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	resp, err := noRedirect().PostForm(ts.URL+"/api/auth/login", url.Values{
		"email":    {testEmail},
		"password": {testPassword},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("login: got %d, want 303", resp.StatusCode)
	}
	var cookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "pcg_session" {
			cookie = c
		}
	}
	if cookie == nil {
		t.Fatal("no pcg_session cookie set after login")
	}
	if !cookie.HttpOnly {
		t.Error("pcg_session must be HttpOnly")
	}
	if cookie.Domain != testDomain {
		t.Errorf("pcg_session Domain: %q, want %q — wrong domain breaks cross-subdomain auth", cookie.Domain, testDomain)
	}
	if cookie.Value == "" {
		t.Error("pcg_session value must not be empty")
	}
}

func TestLogin_RedirectsToErrorOnWrongPassword(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	resp, err := noRedirect().PostForm(ts.URL+"/api/auth/login", url.Values{
		"email":    {testEmail},
		"password": {"wrong"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("wrong password: got %d, want 303", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); !strings.Contains(loc, "error=1") {
		t.Errorf("wrong password Location: %q, want ?error=1", loc)
	}
	for _, c := range resp.Cookies() {
		if c.Name == "pcg_session" {
			t.Error("pcg_session cookie must not be set on failed login")
		}
	}
}

func TestLogin_RedirectsToErrorOnUnknownEmail(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	resp, err := noRedirect().PostForm(ts.URL+"/api/auth/login", url.Values{
		"email":    {"nobody@example.com"},
		"password": {"anything"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if !strings.Contains(resp.Header.Get("Location"), "error=1") {
		t.Error("unknown email should redirect to login?error=1")
	}
}

// ── Logout ────────────────────────────────────────────────────────────────────

func TestLogout_ClearsCookieAndInvalidatesSession(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	cookie := loginAndGetCookie(t, ts)

	// Logout
	req, _ := http.NewRequestWithContext(context.Background(), "POST", ts.URL+"/api/auth/logout", nil)
	req.AddCookie(cookie)
	logoutResp, err := noRedirect().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer logoutResp.Body.Close()
	if logoutResp.StatusCode != http.StatusSeeOther {
		t.Errorf("logout: got %d, want 303", logoutResp.StatusCode)
	}

	// Session cookie should be cleared (MaxAge -1)
	for _, c := range logoutResp.Cookies() {
		if c.Name == "pcg_session" && c.MaxAge >= 0 && !c.Expires.IsZero() {
			t.Error("logout should clear pcg_session cookie")
		}
	}

	// Old session should now be invalid
	req2, _ := http.NewRequestWithContext(context.Background(), "GET", ts.URL+"/api/auth/verify", nil)
	req2.AddCookie(cookie)
	verifyResp, err := noRedirect().Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	defer verifyResp.Body.Close()
	if verifyResp.StatusCode != http.StatusFound {
		t.Errorf("verify after logout: got %d, want 302 (session should be gone)", verifyResp.StatusCode)
	}
}

// ── App install (no Docker) ───────────────────────────────────────────────────

// TestInstall_Returns503WhenDockerUnavailable verifies that calling the install
// endpoint without a Docker manager returns 503 rather than panicking.
// This was a real production bug: nil Manager caused a panic instead of an error.
func TestInstall_Returns503WhenDockerUnavailable(t *testing.T) {
	ts := newTestServer(t) // server.New passes nil for docker manager
	defer ts.Close()
	cookie := loginAndGetCookie(t, ts)

	body := strings.NewReader(`{"blueprint_id":"test-app"}`)
	req, _ := http.NewRequestWithContext(context.Background(), "POST", ts.URL+"/api/apps/install", body)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)

	resp, err := noRedirect().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("install without docker: got %d, want 503", resp.StatusCode)
	}
}
