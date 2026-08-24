// Package netguard provides SSRF defenses shared by any code that makes HTTP
// requests to user-configured URLs (uptime monitors, notification webhooks).
// It blocks connections to loopback, private, link-local (incl. cloud metadata),
// and unspecified addresses — checked at dial time, so a hostname that resolves
// or later rebinds to an internal address is still refused.
package netguard

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"syscall"
	"time"
)

// Blocked reports whether an IP is one that user-driven requests must not reach:
// loopback, private (RFC1918 / IPv6 ULA), link-local (incl. 169.254.169.254),
// or the unspecified address.
func Blocked(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified()
}

// ValidateURL rejects URLs that are malformed, use a non-HTTP(S) scheme, or
// (when the host is a literal IP) point at a blocked address. Hostnames are
// re-checked at dial time by GuardedClient, which defeats DNS rebinding.
func ValidateURL(raw string) error {
	u, err := url.ParseRequestURI(raw)
	if err != nil {
		return errors.New("invalid url")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("url must use http or https")
	}
	host := u.Hostname()
	if host == "" {
		return errors.New("invalid url")
	}
	if ip := net.ParseIP(host); ip != nil && Blocked(ip) {
		return errors.New("url points to a private or reserved address")
	}
	return nil
}

// GuardedClient returns an HTTP client whose dialer rejects connections to
// blocked IPs at connect time — after DNS resolution and on every redirect hop.
func GuardedClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{
		Timeout: timeout,
		Control: func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			ip := net.ParseIP(host)
			if ip == nil {
				return fmt.Errorf("unresolvable address %q", host)
			}
			if Blocked(ip) {
				return fmt.Errorf("blocked address %s", ip)
			}
			return nil
		},
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: &http.Transport{DialContext: dialer.DialContext},
	}
}
