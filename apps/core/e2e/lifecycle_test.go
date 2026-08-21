//go:build e2e

// Container-lifecycle end-to-end test: proves an app is actually installed,
// its container spun up and made routable through Caddy, then torn down and
// removed. This is the "instances spin up and down" check the unit tests can't
// give — it drives the real API against a real Docker daemon, so it's slow and
// lives in the manual/per-release E2E job, not per-PR CI.
package e2e_test

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Default lifecycle target when E2E_APPS is unset: a small, static image.
const lifecycleBlueprint = "excalidraw"

// installClient allows a long timeout — installing pulls the image synchronously.
var installClient = &http.Client{
	Timeout:   5 * time.Minute,
	Transport: e2eTransport(),
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

// pollClient is used for the readiness polling loops. It has a generous timeout
// so a momentarily busy server (containers pulling/starting, health checks
// running) doesn't fail the poll — the loop just retries on the next tick.
var pollClient = &http.Client{
	Timeout:   30 * time.Second,
	Transport: e2eTransport(),
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

type appEntry struct {
	ID           int64  `json:"id"`
	BlueprintID  string `json:"blueprint_id"`
	Subdomain    string `json:"subdomain"`
	Status       string `json:"status"`
	HealthStatus string `json:"health_status"`
}

// listApps fetches /api/apps for polling. It returns (nil, false) on any
// transient error or non-200 so callers can simply retry on the next tick
// rather than failing the whole test on a momentary hiccup.
func listApps(cookie string) ([]appEntry, bool) {
	req, _ := http.NewRequest(http.MethodGet, baseURL+"/api/apps", nil)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Cookie", cookie)
	resp, err := pollClient.Do(req)
	if err != nil {
		return nil, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, false
	}
	var apps []appEntry
	if json.NewDecoder(resp.Body).Decode(&apps) != nil {
		return nil, false
	}
	return apps, true
}

func findApp(cookie, blueprintID string) (appEntry, bool) {
	apps, ok := listApps(cookie)
	if !ok {
		return appEntry{}, false
	}
	for _, a := range apps {
		if a.BlueprintID == blueprintID {
			return a, true
		}
	}
	return appEntry{}, false
}

// appRequest issues an authed GET to an app subdomain via Caddy using
// Host-header routing (no DNS needed) and returns the status and Location.
// ok is false on a transient error so pollers can just retry.
func appRequest(subdomain, cookie string) (status int, location string, ok bool) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return 0, "", false
	}
	suffix := u.Host
	if parts := strings.SplitN(u.Host, ".", 2); len(parts) == 2 {
		suffix = parts[1]
	}
	req, _ := http.NewRequest(http.MethodGet, baseURL+"/", nil)
	req.Host = subdomain + "." + suffix
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	resp, err := pollClient.Do(req)
	if err != nil {
		return 0, "", false
	}
	defer resp.Body.Close()
	return resp.StatusCode, resp.Header.Get("Location"), true
}

// isCatchAll reports whether a response is Caddy's catch-all redirect to the
// dashboard (i.e. no route exists for that subdomain).
func isCatchAll(status int, location string) bool {
	return status == http.StatusFound && strings.Contains(location, "home.")
}

func uninstallApp(t *testing.T, cookie string, id int64) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodDelete, baseURL+"/api/apps/"+strconv.FormatInt(id, 10), nil)
	req.Header.Set("Cookie", cookie)
	resp, err := pollClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE app %d: %v", id, err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		t.Fatalf("DELETE app %d: got %d", id, resp.StatusCode)
	}
}

// waitUntil polls fn every interval until it returns true or the deadline hits.
func waitUntil(t *testing.T, what string, timeout time.Duration, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(2 * time.Second)
	}
	t.Fatalf("timed out after %s waiting for: %s", timeout, what)
}

// TestE2E_AppLifecycle verifies the full lifecycle for each blueprint listed in
// E2E_APPS (comma-separated; defaults to "excalidraw"). Each app is a subtest,
// so `E2E_APPS=gitea,vaultwarden go test ...` verifies exactly those.
//
// The health assertion uses each blueprint's OWN declared health check (the core
// pings the container at its health.path and compares to expected_status), so it
// works uniformly across apps that return 200, 401, or a deep API path — no
// per-app assertions needed.
func TestE2E_AppLifecycle(t *testing.T) {
	requireCreds(t)
	cookie := login(t)

	apps := strings.Split(getenv("E2E_APPS", lifecycleBlueprint), ",")
	for _, id := range apps {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		t.Run(id, func(t *testing.T) { verifyAppLifecycle(t, cookie, id) })
	}
}

func verifyAppLifecycle(t *testing.T, cookie, bpID string) {
	// Clean slate — remove any leftover from a prior run.
	if a, ok := findApp(cookie, bpID); ok {
		uninstallApp(t, cookie, a.ID)
		waitUntil(t, "leftover removed", 60*time.Second, func() bool {
			_, ok := findApp(cookie, bpID)
			return !ok
		})
	}

	// Install (synchronous: pulls image, creates + starts container).
	t.Logf("installing %s (pulling image, may take a while)…", bpID)
	ireq, _ := http.NewRequest(http.MethodPost, baseURL+"/api/apps/install",
		strings.NewReader(`{"blueprint_id":"`+bpID+`"}`))
	ireq.Header.Set("Content-Type", "application/json")
	ireq.Header.Set("Cookie", cookie)
	iresp, err := installClient.Do(ireq)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	iresp.Body.Close()
	if iresp.StatusCode != http.StatusCreated {
		t.Fatalf("install: got %d, want 201", iresp.StatusCode)
	}

	// Container reaches "running" (Docker state).
	var app appEntry
	waitUntil(t, "container running", 120*time.Second, func() bool {
		a, ok := findApp(cookie, bpID)
		if ok {
			app = a
		}
		return ok && a.Status == "running"
	})
	t.Logf("%s running (id=%d, subdomain=%s)", bpID, app.ID, app.Subdomain)

	// Caddy route exists: an authed request must NOT fall through to the
	// catch-all (which 302-redirects to home.*). Any other response means a
	// route was created and Caddy is proxying to the container.
	waitUntil(t, "Caddy route exists", 30*time.Second, func() bool {
		status, loc, ok := appRequest(app.Subdomain, cookie)
		return ok && !isCatchAll(status, loc)
	})
	t.Logf("%s.* is routed through Caddy", app.Subdomain)

	// App becomes healthy per its OWN blueprint health check (runs on a ~60s
	// timer against the container, comparing to the blueprint's expected_status).
	waitUntil(t, "app healthy per blueprint health check", 180*time.Second, func() bool {
		a, ok := findApp(cookie, bpID)
		return ok && a.HealthStatus == "healthy"
	})
	t.Logf("%s is healthy", bpID)

	// Uninstall → removed from list → subdomain stops routing.
	uninstallApp(t, cookie, app.ID)
	waitUntil(t, "removed from list", 60*time.Second, func() bool {
		_, ok := findApp(cookie, bpID)
		return !ok
	})
	// The "files" subdomain is special in dev mode: the whoami test route
	// reclaims it once no app uses it, so it keeps serving instead of falling
	// through to the catch-all. Skip the stops-routing assertion there.
	if app.Subdomain == "files" {
		t.Logf("%s uninstalled (skipping stops-routing check — dev whoami reclaims files.*)", bpID)
	} else {
		waitUntil(t, "subdomain stops routing", 30*time.Second, func() bool {
			status, loc, ok := appRequest(app.Subdomain, cookie)
			return ok && isCatchAll(status, loc)
		})
		t.Logf("%s uninstalled and no longer routing — lifecycle complete", bpID)
	}
}
