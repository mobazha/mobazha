package api

import (
	"context"
	"crypto/tls"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
)

// stubHumaCtx is a minimal huma.Context for testing IP extraction.
type stubHumaCtx struct {
	headers    map[string]string
	remoteAddr string
}

func (s *stubHumaCtx) Operation() *huma.Operation { return nil }
func (s *stubHumaCtx) Context() context.Context   { return context.Background() }
func (s *stubHumaCtx) TLS() *tls.ConnectionState  { return nil }
func (s *stubHumaCtx) Version() huma.ProtoVersion { return huma.ProtoVersion{} }
func (s *stubHumaCtx) Method() string             { return "GET" }
func (s *stubHumaCtx) Host() string               { return "localhost" }
func (s *stubHumaCtx) RemoteAddr() string         { return s.remoteAddr }
func (s *stubHumaCtx) URL() url.URL               { return url.URL{} }
func (s *stubHumaCtx) Param(string) string        { return "" }
func (s *stubHumaCtx) Query(string) string        { return "" }
func (s *stubHumaCtx) Header(name string) string {
	if s.headers == nil {
		return ""
	}
	return s.headers[name]
}
func (s *stubHumaCtx) EachHeader(func(string, string))            {}
func (s *stubHumaCtx) BodyReader() io.Reader                      { return nil }
func (s *stubHumaCtx) GetMultipartForm() (*multipart.Form, error) { return nil, nil }
func (s *stubHumaCtx) SetReadDeadline(time.Time) error            { return nil }
func (s *stubHumaCtx) SetStatus(int)                              {}
func (s *stubHumaCtx) Status() int                                { return 0 }
func (s *stubHumaCtx) SetHeader(string, string)                   {}
func (s *stubHumaCtx) AppendHeader(string, string)                {}
func (s *stubHumaCtx) BodyWriter() io.Writer                      { return nil }

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

func TestClientIPFromContext_Roundtrip(t *testing.T) {
	ctx := context.WithValue(context.Background(), clientIPKey{}, "10.0.0.99")
	if got := clientIPFromContext(ctx); got != "10.0.0.99" {
		t.Fatalf("expected 10.0.0.99, got %s", got)
	}
}

func TestClientIPFromContext_Missing(t *testing.T) {
	if got := clientIPFromContext(context.Background()); got != "unknown" {
		t.Fatalf("expected unknown, got %s", got)
	}
}

func TestExtractClientIPFromHumaContext_RemoteAddr(t *testing.T) {
	ctx := &stubHumaCtx{remoteAddr: "192.168.1.50:9999"}
	got := extractClientIPFromHumaContext(ctx)
	if got != "192.168.1.50" {
		t.Fatalf("expected 192.168.1.50, got %s", got)
	}
}

func TestExtractClientIPFromHumaContext_XFF(t *testing.T) {
	ctx := &stubHumaCtx{
		headers:    map[string]string{"X-Forwarded-For": "1.2.3.4, 10.0.0.1"},
		remoteAddr: "10.0.0.1:1234",
	}
	got := extractClientIPFromHumaContext(ctx)
	if got != "1.2.3.4" {
		t.Fatalf("expected 1.2.3.4, got %s", got)
	}
}

func TestExtractClientIPFromHumaContext_NoHeaders_NoAddr(t *testing.T) {
	ctx := &stubHumaCtx{}
	got := extractClientIPFromHumaContext(ctx)
	if got != "unknown" {
		t.Fatalf("expected unknown, got %s", got)
	}
}
