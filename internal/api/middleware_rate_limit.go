package api

import (
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/mobazha/mobazha/pkg/response"
)

type rateLimiter struct {
	mu       sync.Mutex
	counters map[string]*ipCounter
	limit    int
	window   time.Duration
}

type ipCounter struct {
	count    int
	windowAt time.Time
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		counters: make(map[string]*ipCounter),
		limit:    limit,
		window:   window,
	}
}

func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	c, ok := rl.counters[ip]
	if !ok || now.Sub(c.windowAt) > rl.window {
		rl.counters[ip] = &ipCounter{count: 1, windowAt: now}
		return true
	}
	c.count++
	return c.count <= rl.limit
}

// cleanup removes stale entries older than 2x the window to prevent unbounded growth.
func (rl *rateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cutoff := time.Now().Add(-2 * rl.window)
	for ip, c := range rl.counters {
		if c.windowAt.Before(cutoff) {
			delete(rl.counters, ip)
		}
	}
}

// startCleanup runs periodic cleanup in a background goroutine.
// Stops when the channel is closed.
func (rl *rateLimiter) startCleanup(stop <-chan struct{}) {
	go func() {
		ticker := time.NewTicker(rl.window)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				rl.cleanup()
			case <-stop:
				return
			}
		}
	}()
}

// guestOrderRateLimitMiddleware wraps a handler with per-IP rate limiting.
// Returns 429 Too Many Requests when the limit is exceeded.
func guestOrderRateLimitMiddleware(next http.HandlerFunc, limiter *rateLimiter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := extractClientIP(r)
		if !limiter.allow(ip) {
			response.Error(w, http.StatusTooManyRequests, response.CodeRateLimited,
				"Rate limit exceeded. Please try again later.")
			return
		}
		next.ServeHTTP(w, r)
	}
}

func extractClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		for i, c := range xff {
			if c == ',' {
				return xff[:i]
			}
		}
		return xff
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
