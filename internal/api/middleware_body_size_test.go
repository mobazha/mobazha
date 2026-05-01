package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestMaxBodySizeMiddleware_JSON_UnderLimit(t *testing.T) {
	r := chi.NewMux()
	r.Use(maxBodySizeMiddleware(1024))
	r.Post("/test", func(w http.ResponseWriter, r *http.Request) {
		buf := new(bytes.Buffer)
		if _, err := buf.ReadFrom(r.Body); err != nil {
			http.Error(w, err.Error(), http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	body := bytes.Repeat([]byte("a"), 512)
	req := httptest.NewRequest("POST", "/test", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 for body under limit, got %d", rr.Code)
	}
}

func TestMaxBodySizeMiddleware_JSON_OverLimit(t *testing.T) {
	r := chi.NewMux()
	r.Use(maxBodySizeMiddleware(1024))
	r.Post("/test", func(w http.ResponseWriter, r *http.Request) {
		buf := new(bytes.Buffer)
		if _, err := buf.ReadFrom(r.Body); err != nil {
			if handleMaxBytesError(w, err) {
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	body := bytes.Repeat([]byte("a"), 2048)
	req := httptest.NewRequest("POST", "/test", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected 413 for body over limit, got %d", rr.Code)
	}
}

func TestMaxBodySizeMiddleware_Multipart_Exempt(t *testing.T) {
	r := chi.NewMux()
	r.Use(maxBodySizeMiddleware(1024))
	r.Post("/upload", func(w http.ResponseWriter, r *http.Request) {
		buf := new(bytes.Buffer)
		if _, err := buf.ReadFrom(r.Body); err != nil {
			http.Error(w, err.Error(), http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	body := bytes.Repeat([]byte("a"), 2048)
	req := httptest.NewRequest("POST", "/upload", bytes.NewReader(body))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=---")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 for multipart (exempt), got %d", rr.Code)
	}
}

func TestMaxBodySizeMiddleware_GET_NoLimit(t *testing.T) {
	r := chi.NewMux()
	r.Use(maxBodySizeMiddleware(1024))
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 for GET, got %d", rr.Code)
	}
}

func TestMaxBodySizeMiddleware_MediaPath_HigherLimit(t *testing.T) {
	r := chi.NewMux()
	r.Use(maxBodySizeMiddleware(1024))
	r.Post("/v1/media/product-images", func(w http.ResponseWriter, r *http.Request) {
		buf := new(bytes.Buffer)
		if _, err := buf.ReadFrom(r.Body); err != nil {
			if handleMaxBytesError(w, err) {
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	body := bytes.Repeat([]byte("a"), 2048)
	req := httptest.NewRequest("POST", "/v1/media/product-images", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 for media path with higher limit, got %d", rr.Code)
	}
}

func TestMaxBodySizeMiddleware_NonMediaPath_DefaultLimit(t *testing.T) {
	r := chi.NewMux()
	r.Use(maxBodySizeMiddleware(1024))
	r.Post("/v1/orders", func(w http.ResponseWriter, r *http.Request) {
		buf := new(bytes.Buffer)
		if _, err := buf.ReadFrom(r.Body); err != nil {
			if handleMaxBytesError(w, err) {
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	body := bytes.Repeat([]byte("a"), 2048)
	req := httptest.NewRequest("POST", "/v1/orders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected 413 for non-media path over default limit, got %d", rr.Code)
	}
}

func TestIsMaxBytesError(t *testing.T) {
	if isMaxBytesError(nil) {
		t.Error("expected false for nil error")
	}

	r := chi.NewMux()
	r.Use(maxBodySizeMiddleware(10))
	var capturedErr error
	r.Post("/test", func(w http.ResponseWriter, r *http.Request) {
		buf := new(bytes.Buffer)
		_, capturedErr = buf.ReadFrom(r.Body)
		if capturedErr != nil {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	body := bytes.Repeat([]byte("a"), 100)
	req := httptest.NewRequest("POST", "/test", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if capturedErr == nil {
		t.Fatal("expected error from reading over-limit body")
	}
	if !isMaxBytesError(capturedErr) {
		t.Errorf("expected isMaxBytesError=true for %v", capturedErr)
	}
}
