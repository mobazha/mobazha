package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/mobazha/mobazha/pkg/response"
)

const defaultMaxBodySize int64 = 1 << 20 // 1 MB
const mediaMaxBodySize int64 = 32 << 20  // 32 MB — base64 JSON image uploads (~20 MB raw art photos × 1.34 + JSON overhead)

// largeBodyPaths lists URL path prefixes that carry base64-encoded media
// inside JSON bodies and therefore need a higher body size limit.
var largeBodyPaths = []string{
	"/v1/media/",
}

// maxBodySizeMiddleware limits request body size for non-multipart requests.
// Multipart requests (file uploads, ZIP imports) are exempt because their
// handlers manage limits individually — some need up to 300 MB.
// Paths in largeBodyPaths get a higher limit to accommodate base64 image payloads.
func maxBodySizeMiddleware(limit int64) func(http.Handler) http.Handler {
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

			effectiveLimit := limit
			for _, prefix := range largeBodyPaths {
				if strings.HasPrefix(r.URL.Path, prefix) {
					effectiveLimit = mediaMaxBodySize
					break
				}
			}

			r.Body = http.MaxBytesReader(w, r.Body, effectiveLimit)
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
