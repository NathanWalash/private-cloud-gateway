package api

import (
	"net"
	"testing"
)

func TestBlockedIP(t *testing.T) {
	blocked := []string{
		"127.0.0.1",       // loopback
		"::1",             // loopback v6
		"10.1.2.3",        // private
		"192.168.0.5",     // private
		"172.16.4.4",      // private
		"169.254.169.254", // cloud metadata (link-local)
		"fe80::1",         // link-local v6
		"fc00::1",         // unique-local v6
		"0.0.0.0",         // unspecified
	}
	for _, s := range blocked {
		if !blockedIP(net.ParseIP(s)) {
			t.Errorf("expected %s to be blocked", s)
		}
	}

	allowed := []string{"8.8.8.8", "1.1.1.1", "93.184.216.34", "2606:2800:220:1::"}
	for _, s := range allowed {
		if blockedIP(net.ParseIP(s)) {
			t.Errorf("expected %s to be allowed", s)
		}
	}
}

func TestValidateMonitorURL(t *testing.T) {
	bad := []string{
		"",                              // empty
		"not a url",                     // malformed
		"ftp://example.com",             // wrong scheme
		"file:///etc/passwd",            // wrong scheme
		"http://169.254.169.254/meta",   // metadata IP literal
		"http://127.0.0.1:8080/admin",   // loopback IP literal
		"http://10.0.0.1/",              // private IP literal
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
