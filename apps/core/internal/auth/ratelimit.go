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
)

type rateLimiter struct {
	mu      sync.Mutex
	entries map[string]*rlEntry
}

type rlEntry struct {
	attempts int
	resetAt  time.Time
}

var loginLimiter = &rateLimiter{entries: make(map[string]*rlEntry)}

// allow returns true if the request IP is within the rate limit window.
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

// realIP extracts the client IP, respecting X-Forwarded-For from Caddy.
func realIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		// Take the first (client) address in the chain.
		if ip, _, err := net.SplitHostPort(fwd); err == nil {
			return ip
		}
		return fwd
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
