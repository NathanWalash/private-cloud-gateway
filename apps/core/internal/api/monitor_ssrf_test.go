package api

import (
	"testing"
)

// IP-level blocking is tested directly in internal/netguard (TestBlocked). This
// file covers the monitor-facing URL validation wrapper.
func TestValidateMonitorURL(t *testing.T) {
	bad := []string{
		"",                            // empty
		"not a url",                   // malformed
		"ftp://example.com",           // wrong scheme
		"file:///etc/passwd",          // wrong scheme
		"http://169.254.169.254/meta", // metadata IP literal
		"http://127.0.0.1:8080/admin", // loopback IP literal
		"http://10.0.0.1/",            // private IP literal
	}
	for _, u := range bad {
		if err := validateMonitorURL(u); err == nil {
			t.Errorf("expected %q to be rejected", u)
		}
	}

	good := []string{
		"https://example.com",
		"http://example.com:8080/health",
		"https://home.mydomain.com/status",
	}
	for _, u := range good {
		if err := validateMonitorURL(u); err != nil {
			t.Errorf("expected %q to be accepted, got %v", u, err)
		}
	}
}
