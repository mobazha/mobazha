package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/response"
)

// scopeMatch is the result of matching a request against routeScopeMap.
type scopeMatch struct {
	Matched bool
	Scope   contracts.Scope
	Allowed bool
	DenyMsg string
}

// matchRouteScope checks method+path against routeScopeMap and evaluates
// whether the given identity has the required scope. This is the single
// source of truth for route-level scope decisions, shared by both
// ScopeEnforcementMiddleware (legacy mux routes) and nodeHumaAuthMiddleware
// (huma routes).
func matchRouteScope(method, path string, identity *AuthIdentity) scopeMatch {
	if identity == nil || identity.Scopes == nil {
		return scopeMatch{Matched: true, Allowed: true}
	}

	key := method + " " + path
	for _, rs := range routeScopeMap {
		if !strings.HasPrefix(key, rs.pattern) {
			continue
		}
		if rs.scope == contracts.ScopeAny {
			return scopeMatch{Matched: true, Scope: rs.scope, Allowed: true}
		}
		if !identity.HasScope(rs.scope) {
			return scopeMatch{
				Matched: true,
				Scope:   rs.scope,
				Allowed: false,
				DenyMsg: fmt.Sprintf("missing required scope: %s", rs.scope),
			}
		}
		return scopeMatch{Matched: true, Scope: rs.scope, Allowed: true}
	}

	return scopeMatch{
		Matched: false,
		Allowed: false,
		DenyMsg: "this route is not accessible to API tokens",
	}
}

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

		result := matchRouteScope(r.Method, r.URL.Path, identity)
		if !result.Allowed {
			if !result.Matched {
				log.Warningf("[SCOPE_DENIED] api token %d attempted unmapped route %s %s",
					identity.TokenID, r.Method, r.URL.Path)
			} else {
				logScopeDenial(identity, result.Scope, r.URL.Path)
			}
			response.Error(w, http.StatusForbidden, response.CodeForbidden, result.DenyMsg)
			return
		}

		next.ServeHTTP(w, r)
	})
}
