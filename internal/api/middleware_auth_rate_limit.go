package api

import (
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/response"
)

const (
	authRateLimitMaxFailures = 5
	authRateLimitWindow      = 15 * time.Minute
	authRateLimitCleanupTick = 5 * time.Minute
)

type authFailureRecord struct {
	count    int
	windowAt time.Time
}

type authRateLimiter struct {
	mu      sync.Mutex
	records map[string]*authFailureRecord
	stopCh  chan struct{}
}

func newAuthRateLimiter() *authRateLimiter {
	l := &authRateLimiter{
		records: make(map[string]*authFailureRecord),
		stopCh:  make(chan struct{}),
	}
	go l.cleanupLoop()
	return l
}

func (l *authRateLimiter) stop() {
	close(l.stopCh)
}

func (l *authRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(authRateLimitCleanupTick)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			l.cleanup()
		case <-l.stopCh:
			return
		}
	}
}

func (l *authRateLimiter) isBlocked(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	rec, ok := l.records[ip]
	if !ok {
		return false
	}
	if time.Since(rec.windowAt) > authRateLimitWindow {
		delete(l.records, ip)
		return false
	}
	return rec.count >= authRateLimitMaxFailures
}

func (l *authRateLimiter) recordFailure(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	rec, ok := l.records[ip]
	if !ok || time.Since(rec.windowAt) > authRateLimitWindow {
		l.records[ip] = &authFailureRecord{count: 1, windowAt: time.Now()}
		return
	}
	rec.count++
}

func (l *authRateLimiter) resetIP(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.records, ip)
}

// cleanup removes expired entries. Called lazily — not on every request.
func (l *authRateLimiter) cleanup() {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	for ip, rec := range l.records {
		if now.Sub(rec.windowAt) > authRateLimitWindow {
			delete(l.records, ip)
		}
	}
}

// remoteIP extracts the IP from r.RemoteAddr, ignoring X-Forwarded-For.
// For security-sensitive operations (auth rate limiting) we must not trust
// spoofable proxy headers.
func remoteIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// AuthRateLimitMiddleware returns 429 when an IP exceeds the failure threshold.
// Wrap around auth-protected handlers. Call RecordAuthFailure from the auth
// layer on credential mismatch, and ResetAuthFailure on success.
func (g *Gateway) AuthRateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if g.authLimiter.isBlocked(remoteIP(r)) {
			w.Header().Set("Retry-After", "900")
			response.Error(w, http.StatusTooManyRequests, response.CodeRateLimited,
				"Too many authentication failures. Try again later.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (g *Gateway) RecordAuthFailure(r *http.Request) {
	if g.authLimiter != nil {
		g.authLimiter.recordFailure(remoteIP(r))
	}
}

func (g *Gateway) ResetAuthFailure(r *http.Request) {
	if g.authLimiter != nil {
		g.authLimiter.resetIP(remoteIP(r))
	}
}
