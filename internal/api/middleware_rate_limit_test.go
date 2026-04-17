package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimiter_AllowWithinLimit(t *testing.T) {
	rl := newRateLimiter(3, time.Hour)

	for i := 0; i < 3; i++ {
		if !rl.allow("1.2.3.4") {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}
}

func TestRateLimiter_BlockAfterLimit(t *testing.T) {
	rl := newRateLimiter(2, time.Hour)

	rl.allow("1.2.3.4")
	rl.allow("1.2.3.4")

	if rl.allow("1.2.3.4") {
		t.Fatal("third request should be blocked")
	}
}

func TestRateLimiter_DifferentIPsIndependent(t *testing.T) {
	rl := newRateLimiter(1, time.Hour)

	if !rl.allow("1.1.1.1") {
		t.Fatal("first IP should be allowed")
	}
	if !rl.allow("2.2.2.2") {
		t.Fatal("second IP should be allowed independently")
	}
	if rl.allow("1.1.1.1") {
		t.Fatal("first IP should be blocked after limit")
	}
}

func TestRateLimiter_WindowReset(t *testing.T) {
	rl := newRateLimiter(1, 50*time.Millisecond)

	if !rl.allow("1.2.3.4") {
		t.Fatal("first request should be allowed")
	}
	if rl.allow("1.2.3.4") {
		t.Fatal("second request should be blocked")
	}

	time.Sleep(60 * time.Millisecond)

	if !rl.allow("1.2.3.4") {
		t.Fatal("request after window reset should be allowed")
	}
}

func TestRateLimiter_Cleanup(t *testing.T) {
	rl := newRateLimiter(1, 10*time.Millisecond)

	rl.allow("stale-ip")
	time.Sleep(30 * time.Millisecond)
	rl.cleanup()

	rl.mu.Lock()
	_, exists := rl.counters["stale-ip"]
	rl.mu.Unlock()

	if exists {
		t.Fatal("stale entry should be cleaned up")
	}
}

func TestExtractClientIP_XForwardedFor(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Forwarded-For", "10.0.0.1, 172.16.0.1, 192.168.1.1")

	ip := extractClientIP(r)
	if ip != "10.0.0.1" {
		t.Fatalf("expected 10.0.0.1, got %s", ip)
	}
}

func TestExtractClientIP_XForwardedForSingle(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Forwarded-For", "10.0.0.1")

	ip := extractClientIP(r)
	if ip != "10.0.0.1" {
		t.Fatalf("expected 10.0.0.1, got %s", ip)
	}
}

func TestExtractClientIP_XRealIP(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Real-IP", "10.0.0.2")

	ip := extractClientIP(r)
	if ip != "10.0.0.2" {
		t.Fatalf("expected 10.0.0.2, got %s", ip)
	}
}

func TestExtractClientIP_RemoteAddr(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "192.168.1.100:12345"

	ip := extractClientIP(r)
	if ip != "192.168.1.100" {
		t.Fatalf("expected 192.168.1.100, got %s", ip)
	}
}

func TestGuestOrderRateLimitMiddleware_Returns429(t *testing.T) {
	rl := newRateLimiter(1, time.Hour)

	handler := guestOrderRateLimitMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}, rl)

	r1 := httptest.NewRequest("POST", "/v1/guest/orders", nil)
	r1.RemoteAddr = "10.0.0.1:1234"
	w1 := httptest.NewRecorder()
	handler(w1, r1)
	if w1.Code != http.StatusOK {
		t.Fatalf("first request: expected 200, got %d", w1.Code)
	}

	r2 := httptest.NewRequest("POST", "/v1/guest/orders", nil)
	r2.RemoteAddr = "10.0.0.1:1234"
	w2 := httptest.NewRecorder()
	handler(w2, r2)
	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request: expected 429, got %d", w2.Code)
	}
}
