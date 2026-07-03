package mcp

import (
	"encoding/json"
	"net/http"

	"github.com/mobazha/mobazha/pkg/contracts"
)

// RequireAIUseScope wraps an HTTP handler so every request must carry a
// Bearer token whose identity holds the contracts.ScopeAIUse permission.
//
// This is the front-door gate for the MCP endpoint (/v1/mcp). It runs once per
// HTTP request — before the JSON-RPC handler and well before any tool-level
// scope check inside the bridge call chain.
//
// Identity model (matches /v1/auth/identity contract):
//   - Admin/JWT/Basic identities: the identity endpoint expands their scopes
//     to contracts.AllScopes(), which already includes ScopeAIUse. They pass.
//   - API tokens (mbz_*): scopes are exactly what the token was minted with.
//     The token MUST include "ai:use" or it is rejected with HTTP 403.
//
// gatewayURL/identityPath/httpClient/cache are reused from the same machinery
// as SSEIdentityFunc so hosting and standalone share one implementation.
// Sharing the cache across this guard and SSEIdentityFunc halves the number of
// /v1/auth/identity round-trips per tool-call session.
//
// identityPath is deployment-specific:
//   - Standalone:   "/v1/auth/identity"
//   - Hosting/SaaS: "/platform/v1/auth/identity"
//
// If cache is nil this function panics — the caller is expected to allocate
// one IdentityCache per server lifetime so it can be shared with SSEIdentityFunc.
func RequireAIUseScope(next http.Handler, gatewayURL, identityPath string, httpClient *http.Client, cache *IdentityCache) http.Handler {
	if cache == nil {
		// Caller bug — guard cannot reuse SSEIdentityFunc's cache without one.
		panic("mcp.RequireAIUseScope: cache is required (allocate one per server)")
	}
	required := string(contracts.ScopeAIUse)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity, err := ResolveIdentityFromHeaders(r.Header, gatewayURL, identityPath, httpClient, cache)
		if err != nil || identity == nil {
			writeMCPAuthError(w, http.StatusUnauthorized, "UNAUTHORIZED",
				"authentication required for /v1/mcp")
			return
		}
		if !scopeListContains(identity.Scopes, required) {
			writeMCPAuthError(w, http.StatusForbidden, "FORBIDDEN",
				"missing required scope: ai:use")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// scopeListContains is a small helper kept local so this file has zero
// dependency on internal/api. The slice is short (≤ ~20 entries) so a linear
// scan is faster than building a map.
func scopeListContains(scopes []string, want string) bool {
	for _, s := range scopes {
		if s == want {
			return true
		}
	}
	return false
}

// writeMCPAuthError emits an envelope matching pkg/response so MCP clients see
// the same error shape as the rest of the API. We hand-roll the JSON instead of
// importing pkg/response to keep pkg/mcp dependency-free of HTTP-server packages.
func writeMCPAuthError(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{
			"code":    code,
			"message": msg,
		},
	})
}
