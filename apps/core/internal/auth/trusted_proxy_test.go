package auth

import (
	"net"
	"testing"
)

func TestParseTrustedProxies(t *testing.T) {
	// Empty → Docker default; contains a 172.x address, not a public one.
	def := parseTrustedProxies("")
	if !contains(def, "172.17.0.5") {
		t.Error("default should trust 172.16.0.0/12")
	}
	if contains(def, "8.8.8.8") {
		t.Error("default must not trust a public address")
	}

	// A tightened, custom list only trusts what's listed.
	custom := parseTrustedProxies("10.1.0.0/16, 192.168.5.0/24")
	if !contains(custom, "10.1.2.3") || !contains(custom, "192.168.5.9") {
		t.Error("custom CIDRs should be trusted")
	}
	if contains(custom, "172.17.0.5") {
		t.Error("a tightened list must not trust the old Docker default range")
	}

	// Malformed entries are skipped, valid ones kept.
	mixed := parseTrustedProxies("garbage, 10.9.0.0/16")
	if len(mixed) != 1 || !contains(mixed, "10.9.0.1") {
		t.Errorf("malformed entries should be skipped, got %d nets", len(mixed))
	}
}

func contains(nets []*net.IPNet, ip string) bool {
	p := net.ParseIP(ip)
	for _, n := range nets {
		if n.Contains(p) {
			return true
		}
	}
	return false
}
