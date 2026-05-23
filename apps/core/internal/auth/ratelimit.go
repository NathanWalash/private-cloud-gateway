package auth

import (
	"net"
	"net/http"
	"sync"
	"time"
)

const (
	maxLoginAttempts = 10
	loginWindow      = time.Minute
	cleanupInterval  = 5 * time.Minute
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

// totpLimiter is stricter — TOTP codes can be brute-forced (10^6 possibilities).
var totpLimiter = &rateLimiter{entries: make(map[string]*rlEntry)}

// ResetLimiters clears all rate limiter state. Intended for use in tests only.
func ResetLimiters() {
	loginLimiter.mu.Lock()
	loginLimiter.entries = make(map[string]*rlEntry)
	loginLimiter.mu.Unlock()
	totpLimiter.mu.Lock()
	totpLimiter.entries = make(map[string]*rlEntry)
	totpLimiter.mu.Unlock()
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
func isTrustedProxy(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	if parsed.IsLoopback() {
		return true
	}
	// Docker default bridge: 172.16.0.0/12
	_, docker, _ := net.ParseCIDR("172.16.0.0/12")
	return docker.Contains(parsed)
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
