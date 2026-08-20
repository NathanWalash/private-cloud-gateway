package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/NathanWalash/private-cloud-gateway/apps/core/internal/db"
)

// TestListenAndServeHandlerIsWrapped guards against a regression where the
// production server served the raw mux instead of s.Handler(), which silently
// dropped the security-header and body-limit middleware. It asserts the handler
// that ListenAndServe actually runs (via httpServer) carries those headers —
// unlike the other tests, which exercise s.Handler() directly.
func TestListenAndServeHandlerIsWrapped(t *testing.T) {
	database, err := db.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := db.Migrate(database); err != nil {
		t.Fatalf("db.Migrate: %v", err)
	}

	srv := New(database, []byte("test-secret"), "http://home.localtest.me/login",
		"localtest.me", "http", "", nil, nil, nil, t.TempDir())

	handler := srv.httpServer(":0").Handler
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if got := rec.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Errorf("served handler missing security middleware: X-Frame-Options = %q, want DENY", got)
	}
}
