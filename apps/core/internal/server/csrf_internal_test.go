package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCSRFGuard(t *testing.T) {
	const domain = "example.com"
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	h := csrfGuard(domain, next)

	call := func(method, origin string) int {
		req := httptest.NewRequest(method, "/api/apps/install", nil)
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}

	// Cross-site mutating requests are blocked.
	if code := call(http.MethodPost, "https://evil.com"); code != http.StatusForbidden {
		t.Errorf("cross-site POST: got %d, want 403", code)
	}
	// A look-alike domain must not slip through the suffix check.
	if code := call(http.MethodPost, "https://notexample.com"); code != http.StatusForbidden {
		t.Errorf("look-alike POST: got %d, want 403", code)
	}
	// Same-site (subdomain) is allowed.
	if code := call(http.MethodPost, "https://home.example.com"); code != http.StatusOK {
		t.Errorf("same-site subdomain POST: got %d, want 200", code)
	}
	// Bare apex origin is allowed.
	if code := call(http.MethodPost, "https://example.com"); code != http.StatusOK {
		t.Errorf("apex POST: got %d, want 200", code)
	}
	// No Origin/Referer (non-browser client) is allowed.
	if code := call(http.MethodPost, ""); code != http.StatusOK {
		t.Errorf("no-origin POST: got %d, want 200", code)
	}
	// Safe methods are never blocked, even cross-site.
	if code := call(http.MethodGet, "https://evil.com"); code != http.StatusOK {
		t.Errorf("cross-site GET: got %d, want 200", code)
	}
}
