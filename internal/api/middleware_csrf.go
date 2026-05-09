package api

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/mobazha/mobazha3.0/pkg/response"
)

// csrfOriginCheckMiddleware validates that state-changing requests (POST, PUT,
// DELETE, PATCH) carry an Origin or Referer header matching the request Host.
//
// This defends standalone/private_distribution deployments against CSRF when the browser
// auto-attaches Basic Auth credentials. Bearer tokens (API tokens or JWTs)
// are explicitly sent by code, never auto-attached by browsers, so they skip
// this check. Requests with no Origin/Referer are assumed to come from
// non-browser clients (curl, SDKs) and are allowed through.
func csrfOriginCheckMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" || r.Method == "HEAD" || r.Method == "OPTIONS" {
			next.ServeHTTP(w, r)
			return
		}

		if strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			next.ServeHTTP(w, r)
			return
		}

		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = refererOrigin(r.Header.Get("Referer"))
		}
		if origin == "" {
			next.ServeHTTP(w, r)
			return
		}

		if originHost(origin) != r.Host {
			response.Error(w, http.StatusForbidden, response.CodeForbidden, "CSRF origin mismatch")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// refererOrigin extracts the origin (scheme://host[:port]) from a Referer header value.
func refererOrigin(referer string) string {
	if referer == "" {
		return ""
	}
	u, err := url.Parse(referer)
	if err != nil || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

// originHost extracts the host[:port] from an origin URL.
func originHost(origin string) string {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return ""
	}
	return u.Host
}
