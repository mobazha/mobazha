package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/response"
)

// AuthIdentity holds the resolved identity and permission set for the current request.
// It is stored in the request context via authIdentityKey.
//
// Standalone-mode counterpart of mobazha_hosting/api/auth_scoped.go's AuthIdentity.
// Differences:
//   - No hostDB / Casdoor dependencies (this runs on a single-node deployment).
//   - JWT (admin) and Basic Auth produce identities with Scopes == nil, meaning
//     "full access" — the same convention used by the SaaS gateway so the same
//     ScopeEnforcementMiddleware semantics apply.
//   - mbz_ API tokens always have a concrete ScopeSet built from the persisted
//     scope list, and are subject to deny-by-default route checks.
type AuthIdentity struct {
	// UserID is a stable identifier for audit/log purposes. For JWT auth this is
	// the Casdoor user ID (claims.Id); for Basic Auth it is the configured
	// admin username; for API tokens it is fmt.Sprintf("api_token:%d", tokenID).
	UserID string

	// PeerID is the local node's peerID when known (JWT puts it in
	// claims.Properties["peerID"]). Optional.
	PeerID string

	// Scopes is the set of granted scopes. nil means "full access" (admin JWT /
	// Basic Auth). API tokens always have a concrete set.
	Scopes contracts.ScopeSet

	// IsAPIToken distinguishes API tokens from session JWTs / Basic Auth.
	IsAPIToken bool

	// IsAdmin is true for JWT admins or Basic Auth users (the only humans that
	// can hit owner-only routes on a standalone node).
	IsAdmin bool

	// TokenID is the database ID of the API token (0 for non-token auth).
	TokenID int64
}

// HasScope returns true if the identity has the given scope.
// Identities with Scopes == nil (admin JWT / Basic Auth) always pass.
func (a *AuthIdentity) HasScope(s contracts.Scope) bool {
	if a == nil {
		return false
	}
	if a.Scopes == nil {
		return true
	}
	return a.Scopes.Has(s)
}

// HasAnyScope returns true if the identity has any of the given scopes.
func (a *AuthIdentity) HasAnyScope(scopes ...contracts.Scope) bool {
	if a == nil {
		return false
	}
	if a.Scopes == nil {
		return true
	}
	return a.Scopes.HasAny(scopes...)
}

// authIdentityCtxKey is the context key for AuthIdentity values.
// Defined as a private type to avoid cross-package collisions.
type authIdentityCtxKey struct{}

// WithAuthIdentity attaches an AuthIdentity to the request context.
func WithAuthIdentity(ctx context.Context, id *AuthIdentity) context.Context {
	return context.WithValue(ctx, authIdentityCtxKey{}, id)
}

// GetAuthIdentity retrieves the AuthIdentity from the request context.
// Returns nil if no identity is set (unauthenticated / public route).
func GetAuthIdentity(ctx context.Context) *AuthIdentity {
	v, _ := ctx.Value(authIdentityCtxKey{}).(*AuthIdentity)
	return v
}

// RequireScope returns a middleware that enforces the given scope on the request.
// Returns 401 if no identity is present, 403 if the scope is missing.
//
// Note: most routes are gated by ScopeEnforcementMiddleware via the routeScopeMap;
// RequireScope is reserved for handlers that need a stronger or different scope
// than the route prefix would imply.
func RequireScope(scope contracts.Scope) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			identity := GetAuthIdentity(r.Context())
			if identity == nil {
				response.Error(w, http.StatusUnauthorized, response.CodeUnauthorized,
					"authentication required")
				return
			}
			if !identity.HasScope(scope) {
				logScopeDenial(identity, scope, r.URL.Path)
				response.Error(w, http.StatusForbidden, response.CodeForbidden,
					fmt.Sprintf("missing required scope: %s", scope))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireAnyScope returns a middleware that enforces at least one of the given scopes.
func RequireAnyScope(scopes ...contracts.Scope) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			identity := GetAuthIdentity(r.Context())
			if identity == nil {
				response.Error(w, http.StatusUnauthorized, response.CodeUnauthorized,
					"authentication required")
				return
			}
			if !identity.HasAnyScope(scopes...) {
				names := make([]string, len(scopes))
				for i, s := range scopes {
					names[i] = string(s)
				}
				response.Error(w, http.StatusForbidden, response.CodeForbidden,
					fmt.Sprintf("requires one of: %s", strings.Join(names, ", ")))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func logScopeDenial(identity *AuthIdentity, scope contracts.Scope, path string) {
	tokenType := "jwt"
	if identity.IsAPIToken {
		tokenType = fmt.Sprintf("api_token(id=%d)", identity.TokenID)
	}
	log.Warningf("[SCOPE_DENIED] user=%s type=%s scope=%s path=%s",
		identity.UserID, tokenType, scope, path)
}
