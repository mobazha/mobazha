package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/mobazha/mobazha3.0/pkg/response"
)

const defaultMaxBodySize int64 = 1 << 20 // 1 MB

// maxBodySizeMiddleware limits request body size for non-multipart requests.
// Multipart requests (file uploads, ZIP imports) are exempt because their
// handlers manage limits individually — some need up to 300 MB.
func maxBodySizeMiddleware(limit int64) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodDelete:
				next.ServeHTTP(w, r)
				return
			}

			ct := r.Header.Get("Content-Type")
			if strings.HasPrefix(ct, "multipart/") {
				next.ServeHTTP(w, r)
				return
			}

			r.Body = http.MaxBytesReader(w, r.Body, limit)
			next.ServeHTTP(w, r)
		})
	}
}

// isMaxBytesError checks whether the error is from exceeding MaxBytesReader limit.
func isMaxBytesError(err error) bool {
	if err == nil {
		return false
	}
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		return true
	}
	return strings.Contains(err.Error(), "http: request body too large")
}

// handleMaxBytesError sends a 413 response if err is a MaxBytesError.
// Returns true if it handled the error, false otherwise.
func handleMaxBytesError(w http.ResponseWriter, err error) bool {
	if isMaxBytesError(err) {
		response.Error(w, http.StatusRequestEntityTooLarge, response.CodePayloadTooLarge, "request body too large")
		return true
	}
	return false
}
