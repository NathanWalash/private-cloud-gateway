package api_test

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/NathanWalash/private-cloud-gateway/apps/core/internal/api"
	"github.com/NathanWalash/private-cloud-gateway/apps/core/internal/version"
)

// TestStatusReportsVersion verifies /api/status echoes the build version, so the
// dashboard and operators can see exactly which build is running.
func TestStatusReportsVersion(t *testing.T) {
	// Status only reads startTime + version, so nil deps are fine here.
	h := api.NewHandler(nil, version.Version, nil, nil, "", "", "http")

	rec := httptest.NewRecorder()
	h.Status(rec, httptest.NewRequest("GET", "/api/status", nil))

	body := rec.Body.String()
	want := `"version":"` + version.Version + `"`
	if !strings.Contains(body, want) {
		t.Errorf("status body %q does not contain %q", body, want)
	}
}
