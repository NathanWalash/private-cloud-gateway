package netguard

import (
	"net"
	"testing"
)

func TestBlocked(t *testing.T) {
	blocked := []string{"127.0.0.1", "10.0.0.5", "192.168.1.1", "169.254.169.254", "::1", "0.0.0.0", "fd00::1"}
	for _, s := range blocked {
		if !Blocked(net.ParseIP(s)) {
			t.Errorf("%s should be blocked", s)
		}
	}
	allowed := []string{"8.8.8.8", "1.1.1.1", "93.184.216.34"}
	for _, s := range allowed {
		if Blocked(net.ParseIP(s)) {
			t.Errorf("%s should be allowed", s)
		}
	}
}

func TestValidateURL(t *testing.T) {
	bad := []string{
		"not a url",
		"ftp://example.com",
		"http://127.0.0.1/x",
		"https://169.254.169.254/latest/meta-data",
		"http://10.0.0.1",
	}
	for _, u := range bad {
		if err := ValidateURL(u); err == nil {
			t.Errorf("ValidateURL(%q) should fail", u)
		}
	}
	good := []string{"https://example.com/hook", "http://hooks.slack.com/services/x"}
	for _, u := range good {
		if err := ValidateURL(u); err != nil {
			t.Errorf("ValidateURL(%q) should pass: %v", u, err)
		}
	}
}
