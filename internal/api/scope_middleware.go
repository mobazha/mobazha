package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/response"
)

// ScopeEnforcementMiddleware enforces fine-grained permission checks for
// API tokens (mbz_*) using the routeScopeMap.
//
// Policy:
//   - If no AuthIdentity is set → pass through (the route is either public
//     or the upstream AuthenticationMiddleware already 401'd; do not double-handle).
//   - If identity.Scopes == nil → admin/JWT/Basic Auth, full access; pass through.
//   - Otherwise (API token): match the request against routeScopeMap. If a
//     prefix matches and the required scope is granted, allow. If a prefix
//     matches but the scope is missing, deny 403. If no prefix matches, deny
//     403 (deny-by-default).
//
// This is the single layer where API-token scope enforcement happens for
// REST routes. MCP tools that proxy to these routes will receive the same
// 403 response from the loopback bridge — there is no second enforcement
// layer inside the MCP server.
func (g *Gateway) ScopeEnforcementMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity := GetAuthIdentity(r.Context())
		if identity == nil || identity.Scopes == nil {
			next.ServeHTTP(w, r)
			return
		}

		key := r.Method + " " + r.URL.Path
		var matched bool
		for _, rs := range routeScopeMap {
			if !strings.HasPrefix(key, rs.pattern) {
				continue
			}
			matched = true
			// ScopeAny ("") = global metadata route; any authenticated
			// identity (including API tokens) may pass without holding a
			// specific permission.
			if rs.scope == contracts.ScopeAny {
				break
			}
			if !identity.HasScope(rs.scope) {
				logScopeDenial(identity, rs.scope, r.URL.Path)
				response.Error(w, http.StatusForbidden, response.CodeForbidden,
					fmt.Sprintf("missing required scope: %s", rs.scope))
				return
			}
			break
		}

		if !matched {
			log.Warningf("[SCOPE_DENIED] api token %d attempted unmapped route %s %s",
				identity.TokenID, r.Method, r.URL.Path)
			response.Error(w, http.StatusForbidden, response.CodeForbidden,
				"this route is not accessible to API tokens")
			return
		}

		next.ServeHTTP(w, r)
	})
}
