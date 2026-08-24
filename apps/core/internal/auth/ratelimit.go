package auth

import (
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	maxLoginAttempts = 10
	loginWindow      = time.Minute
	cleanupInterval  = 5 * time.Minute

	// Per-account lockout: after this many consecutive failures for one email,
	// that account is locked for accountLockDuration regardless of source IP
	// (the IP limiter alone doesn't stop a distributed/botnet attack on one user).
	maxAccountFails     = 5
	accountLockDuration = 15 * time.Minute
)

type rateLimiter struct {
	mu      sync.Mutex
	entries map[string]*rlEntry
}

type rlEntry struct {
	attempts int
	resetAt  time.Time
}

var loginLimiter = newRateLimiter()

// totpLimiter guards TOTP verification (codes are brute-forceable). It MUST be
// built via newRateLimiter so its cleanup goroutine runs — a bare struct literal
// would leak one map entry per distinct IP forever.
var totpLimiter = newRateLimiter()

var accountLocker = newAccountGuard()

// ResetLimiters clears all rate limiter state. Intended for use in tests only.
func ResetLimiters() {
	loginLimiter.mu.Lock()
	loginLimiter.entries = make(map[string]*rlEntry)
	loginLimiter.mu.Unlock()
	totpLimiter.mu.Lock()
	totpLimiter.entries = make(map[string]*rlEntry)
	totpLimiter.mu.Unlock()
	accountLocker.mu.Lock()
	accountLocker.entries = make(map[string]*acctEntry)
	accountLocker.mu.Unlock()
}

// accountGuard tracks consecutive login failures per account (email) and locks
// the account for a cooldown once the threshold is hit.
type accountGuard struct {
	mu      sync.Mutex
	entries map[string]*acctEntry
}

type acctEntry struct {
	fails     int
	lockUntil time.Time
}

func newAccountGuard() *accountGuard {
	g := &accountGuard{entries: make(map[string]*acctEntry)}
	go g.cleanup()
	return g
}

func (g *accountGuard) cleanup() {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()
	for range ticker.C {
		g.mu.Lock()
		now := time.Now()
		for k, e := range g.entries {
			// Drop any entry whose lock window has passed. A tripped entry keeps
			// fails >= maxAccountFails, so a "&& fails < max" guard would never
			// reclaim it — this sweeps those too, since it's no longer locked.
			if now.After(e.lockUntil) {
				delete(g.entries, k)
			}
		}
		g.mu.Unlock()
	}
}

// locked reports whether the account is currently in its lockout window.
func (g *accountGuard) locked(key string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	e, ok := g.entries[key]
	return ok && time.Now().Before(e.lockUntil)
}

// recordFail registers a failed attempt and returns true if this failure just
// tripped the lockout (so the caller can alert exactly once).
func (g *accountGuard) recordFail(key string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := time.Now()
	e, ok := g.entries[key]
	if !ok {
		e = &acctEntry{}
		g.entries[key] = e
	}
	// A previous lock has expired — start a fresh failure streak.
	if e.fails >= maxAccountFails && now.After(e.lockUntil) {
		e.fails = 0
	}
	e.fails++
	if e.fails == maxAccountFails {
		e.lockUntil = now.Add(accountLockDuration)
		return true
	}
	return false
}

// recordSuccess clears failure state after a successful login.
func (g *accountGuard) recordSuccess(key string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.entries, key)
}

func newRateLimiter() *rateLimiter {
	r := &rateLimiter{entries: make(map[string]*rlEntry)}
	go r.cleanup()
	return r
}

// cleanup removes expired entries on a regular interval to prevent unbounded growth.
func (r *rateLimiter) cleanup() {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()
	for range ticker.C {
		r.mu.Lock()
		now := time.Now()
		for ip, e := range r.entries {
			if now.After(e.resetAt) {
				delete(r.entries, ip)
			}
		}
		r.mu.Unlock()
	}
}

// allow returns true if the IP is within the rate limit window.
func (r *rateLimiter) allow(ip string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	e, ok := r.entries[ip]
	if !ok || now.After(e.resetAt) {
		r.entries[ip] = &rlEntry{attempts: 1, resetAt: now.Add(loginWindow)}
		return true
	}
	if e.attempts >= maxLoginAttempts {
		return false
	}
	e.attempts++
	return true
}

// realIP extracts the real client IP.
// X-Forwarded-For is only trusted when the connection comes from localhost
// (i.e. from Caddy running in the same Docker network). Direct connections
// use RemoteAddr directly to prevent spoofing.
func realIP(r *http.Request) string {
	remoteHost, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		remoteHost = r.RemoteAddr
	}

	// Trust X-Forwarded-For only from Caddy (private Docker network ranges).
	if isTrustedProxy(remoteHost) {
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			// Take the first (client) address from the chain.
			for _, part := range splitComma(fwd) {
				if ip := net.ParseIP(trimSpace(part)); ip != nil {
					return ip.String()
				}
			}
		}
	}
	return remoteHost
}

// isTrustedProxy reports whether ip is a Docker bridge network address
// (172.16.0.0/12) or localhost — i.e. a known-safe proxy.
// trustedProxyNets are the source ranges whose X-Forwarded-For header is trusted
// (i.e. the reverse proxy in front of core). Configurable via
// CLOUD_CORE_TRUSTED_PROXY_CIDR (comma-separated CIDRs) so an operator can pin it
// to the exact caddy↔core subnet instead of the broad Docker default; loopback
// is always trusted for local dev. Defaults to Docker's 172.16.0.0/12.
var trustedProxyNets = parseTrustedProxies(os.Getenv("CLOUD_CORE_TRUSTED_PROXY_CIDR"))

func parseTrustedProxies(env string) []*net.IPNet {
	specs := strings.TrimSpace(env)
	if specs == "" {
		specs = "172.16.0.0/12"
	}
	var nets []*net.IPNet
	for _, c := range strings.Split(specs, ",") {
		if c = strings.TrimSpace(c); c == "" {
			continue
		}
		if _, n, err := net.ParseCIDR(c); err == nil {
			nets = append(nets, n)
		}
	}
	return nets
}

func isTrustedProxy(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	if parsed.IsLoopback() {
		return true
	}
	for _, n := range trustedProxyNets {
		if n.Contains(parsed) {
			return true
		}
	}
	return false
}

func splitComma(s string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	return append(parts, s[start:])
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && s[start] == ' ' {
		start++
	}
	for end > start && s[end-1] == ' ' {
		end--
	}
	return s[start:end]
}
