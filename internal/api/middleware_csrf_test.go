package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestCSRFMiddleware_AllowsSafeMethod(t *testing.T) {
	for _, method := range []string{"GET", "HEAD", "OPTIONS"} {
		req := httptest.NewRequest(method, "http://localhost:5104/v1/listings", nil)
		req.Header.Set("Origin", "http://evil.example.com")
		rr := httptest.NewRecorder()
		csrfOriginCheckMiddleware(okHandler()).ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("%s: expected 200, got %d", method, rr.Code)
		}
	}
}

func TestCSRFMiddleware_AllowsBearerToken(t *testing.T) {
	for _, auth := range []string{"Bearer mbz_abc123", "Bearer eyJhbGciOiJSUzI1NiJ9.xxx"} {
		req := httptest.NewRequest("POST", "http://localhost:5104/v1/listings", nil)
		req.Host = "localhost:5104"
		req.Header.Set("Authorization", auth)
		req.Header.Set("Origin", "http://evil.example.com")
		rr := httptest.NewRecorder()
		csrfOriginCheckMiddleware(okHandler()).ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("Bearer %q: expected 200, got %d", auth, rr.Code)
		}
	}
}

func TestCSRFMiddleware_AllowsSameOrigin(t *testing.T) {
	req := httptest.NewRequest("POST", "http://localhost:5104/v1/listings", nil)
	req.Host = "localhost:5104"
	req.Header.Set("Origin", "http://localhost:5104")
	rr := httptest.NewRecorder()
	csrfOriginCheckMiddleware(okHandler()).ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("same-origin POST: expected 200, got %d", rr.Code)
	}
}

func TestCSRFMiddleware_AllowsSameReferer(t *testing.T) {
	req := httptest.NewRequest("DELETE", "http://localhost:5104/v1/listings/slug", nil)
	req.Host = "localhost:5104"
	req.Header.Set("Referer", "http://localhost:5104/admin/products")
	rr := httptest.NewRecorder()
	csrfOriginCheckMiddleware(okHandler()).ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("same-referer DELETE: expected 200, got %d", rr.Code)
	}
}

func TestCSRFMiddleware_BlocksCrossOrigin(t *testing.T) {
	req := httptest.NewRequest("POST", "http://localhost:5104/v1/listings", nil)
	req.Host = "localhost:5104"
	req.Header.Set("Origin", "http://evil.example.com")
	rr := httptest.NewRecorder()
	csrfOriginCheckMiddleware(okHandler()).ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("cross-origin POST with Basic Auth: expected 403, got %d", rr.Code)
	}
}

func TestCSRFMiddleware_BlocksCrossReferer(t *testing.T) {
	req := httptest.NewRequest("PUT", "http://localhost:5104/v1/profiles", nil)
	req.Host = "localhost:5104"
	req.Header.Set("Referer", "http://evil.example.com/page")
	rr := httptest.NewRecorder()
	csrfOriginCheckMiddleware(okHandler()).ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("cross-referer PUT: expected 403, got %d", rr.Code)
	}
}

func TestCSRFMiddleware_AllowsNoOriginNoReferer(t *testing.T) {
	req := httptest.NewRequest("POST", "http://localhost:5104/v1/listings", nil)
	req.Host = "localhost:5104"
	rr := httptest.NewRecorder()
	csrfOriginCheckMiddleware(okHandler()).ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("no origin/referer (curl): expected 200, got %d", rr.Code)
	}
}

func TestCSRFMiddleware_SaaSNotAffected(t *testing.T) {
	// SaaS uses shared router (NewSharedRouter), not newV1Router.
	// CSRF middleware is only registered in newV1Router when csrfEnabled=true.
	// This test verifies that the middleware itself doesn't block Bearer
	// requests, which is the SaaS auth mechanism.
	req := httptest.NewRequest("POST", "http://app.mobazha.org/v1/listings", nil)
	req.Host = "app.mobazha.org"
	req.Header.Set("Authorization", "Bearer eyJhbGciOiJSUzI1NiJ9.payload.sig")
	req.Header.Set("Origin", "http://app.mobazha.org")
	rr := httptest.NewRecorder()
	csrfOriginCheckMiddleware(okHandler()).ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("SaaS Bearer POST: expected 200, got %d", rr.Code)
	}
}
