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

// The lifecycle target: a small, static image that reliably serves 200 at "/".
const (
	lifecycleBlueprint = "excalidraw"
	lifecycleSubdomain = "draw"
)

// installClient allows a long timeout — installing pulls the image synchronously.
var installClient = &http.Client{
	Timeout: 5 * time.Minute,
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

type appEntry struct {
	ID          int64  `json:"id"`
	BlueprintID string `json:"blueprint_id"`
	Status      string `json:"status"`
}

func listApps(t *testing.T, cookie string) []appEntry {
	t.Helper()
	resp := get(t, "/api/apps", "Cookie", cookie)
	b := body(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/api/apps: got %d body=%s", resp.StatusCode, b)
	}
	var apps []appEntry
	if err := json.Unmarshal([]byte(b), &apps); err != nil {
		t.Fatalf("/api/apps: bad JSON: %s", b)
	}
	return apps
}

func findApp(t *testing.T, cookie, blueprintID string) (appEntry, bool) {
	t.Helper()
	for _, a := range listApps(t, cookie) {
		if a.BlueprintID == blueprintID {
			return a, true
		}
	}
	return appEntry{}, false
}

// appRequest hits an app subdomain through Caddy without depending on DNS for
// the subdomain: it connects to the base host but sets the Host header to
// <subdomain>.<domain>, which is what Caddy routes on.
func appRequest(t *testing.T, subdomain, cookie string) *http.Response {
	t.Helper()
	u, err := url.Parse(baseURL)
	if err != nil {
		t.Fatalf("parse baseURL: %v", err)
	}
	parts := strings.SplitN(u.Host, ".", 2)
	suffix := u.Host
	if len(parts) == 2 {
		suffix = parts[1]
	}
	req, _ := http.NewRequest("GET", baseURL+"/", nil)
	req.Host = subdomain + "." + suffix
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("app request to %s: %v", req.Host, err)
	}
	return resp
}

func uninstallApp(t *testing.T, cookie string, id int64) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodDelete, baseURL+"/api/apps/"+strconv.FormatInt(id, 10), nil)
	req.Header.Set("Cookie", cookie)
	resp, err := client.Do(req)
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

func TestE2E_AppLifecycle(t *testing.T) {
	requireCreds(t)
	cookie := login(t)

	// Clean slate — remove any leftover from a prior run.
	if a, ok := findApp(t, cookie, lifecycleBlueprint); ok {
		uninstallApp(t, cookie, a.ID)
		waitUntil(t, "leftover app removed", 60*time.Second, func() bool {
			_, ok := findApp(t, cookie, lifecycleBlueprint)
			return !ok
		})
	}

	// Before install: the subdomain must not route (catch-all → redirect to home).
	if resp := appRequest(t, lifecycleSubdomain, cookie); resp.StatusCode == http.StatusOK {
		resp.Body.Close()
		t.Fatalf("%s.* served 200 before install — unexpected", lifecycleSubdomain)
	} else {
		resp.Body.Close()
	}

	// Install (synchronous: pulls image, creates + starts container).
	t.Logf("installing %s (pulling image, may take a while)…", lifecycleBlueprint)
	ireq, _ := http.NewRequest(http.MethodPost, baseURL+"/api/apps/install",
		strings.NewReader(`{"blueprint_id":"`+lifecycleBlueprint+`"}`))
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

	// The container must reach "running" as reported from Docker.
	var appID int64
	waitUntil(t, "app running", 120*time.Second, func() bool {
		a, ok := findApp(t, cookie, lifecycleBlueprint)
		if ok {
			appID = a.ID
		}
		return ok && a.Status == "running"
	})
	t.Logf("%s is running (id=%d)", lifecycleBlueprint, appID)

	// Routable through Caddy with a valid session (proxied → 200, not 302).
	waitUntil(t, "app routable through Caddy", 60*time.Second, func() bool {
		resp := appRequest(t, lifecycleSubdomain, cookie)
		resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	})
	t.Logf("%s.* is routable and returns 200", lifecycleSubdomain)

	// Uninstall → container removed → subdomain stops routing.
	uninstallApp(t, cookie, appID)
	waitUntil(t, "app removed from list", 60*time.Second, func() bool {
		_, ok := findApp(t, cookie, lifecycleBlueprint)
		return !ok
	})
	waitUntil(t, "subdomain stops routing after uninstall", 30*time.Second, func() bool {
		resp := appRequest(t, lifecycleSubdomain, cookie)
		resp.Body.Close()
		return resp.StatusCode != http.StatusOK
	})
	t.Logf("%s uninstalled and no longer routing — lifecycle complete", lifecycleBlueprint)
}
