// Package api — huma_middleware.go
//
// AH-1.4: Bridges the Node gateway's auth model onto huma's per-operation
// declarative security. The Node supports three auth modes (in priority):
//   1. mbz_ API token → AuthIdentity with IsAPIToken=true + ScopeSet
//   2. Bearer JWT (Casdoor) → AuthIdentity with IsAdmin=true
//   3. Basic Auth → AuthIdentity with IsAdmin=true
//
// huma operations declare their auth requirement via:
//   - Security: []map[string][]string{{SecuritySchemeNodeAuth: {}}}
//     for owner-only routes
//   - omitting Security for anonymous/public routes
//
// The middleware delegates the actual credential check to the Gateway's
// existing auth helpers (tryAPITokenAuth, tryJWTAuthWith, auth.check),
// keeping a single source of truth for credential validation.
package api

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/mobazha/mobazha3.0/pkg/apitoken"
)

// installNodeHumaMiddlewares wires auth onto the huma API.
func (g *Gateway) installNodeHumaMiddlewares(api huma.API) {
	api.UseMiddleware(g.nodeHumaAuthMiddleware(api))
}

// nodeHumaAuthMiddleware enforces Operation.Security for Node API routes.
func (g *Gateway) nodeHumaAuthMiddleware(api huma.API) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		op := ctx.Operation()
		if op == nil || len(op.Security) == 0 {
			next(ctx)
			return
		}

		// SaaS SharedRouter may have already set AuthIdentity at the
		// mux level. Don't overwrite.
		if GetAuthIdentity(ctx.Context()) != nil {
			next(ctx)
			return
		}

		authHeader := ctx.Header("Authorization")
		jv := g.getJWTValidator()

		// 1) mbz_ API token
		if strings.HasPrefix(authHeader, "Bearer ") {
			bearerVal := strings.TrimSpace(authHeader[7:])
			if bearerVal != "" && apitoken.IsAPIToken(bearerVal) {
				identity, ok := g.tryAPITokenAuth(bearerVal)
				if !ok {
					huma.WriteErr(api, ctx, http.StatusUnauthorized, "Invalid or expired API token")
					return
				}
				next(huma.WithContext(ctx, WithAuthIdentity(ctx.Context(), identity)))
				return
			}
		}

		// 2) JWT Bearer
		if jv != nil {
			var tokenStr string
			if strings.HasPrefix(authHeader, "Bearer ") {
				tokenStr = strings.TrimSpace(authHeader[7:])
			} else if qp := ctx.Query("token"); qp != "" && !strings.HasPrefix(qp, "basic:") {
				tokenStr = qp
			}
			if tokenStr != "" {
				if identity, ok := g.tryJWTAuthWith(jv, buildMinimalRequest(authHeader, ctx)); ok {
					next(huma.WithContext(ctx, WithAuthIdentity(ctx.Context(), identity)))
					return
				}
				// Bearer was present but invalid → hard fail
				huma.WriteErr(api, ctx, http.StatusUnauthorized, "Invalid or expired token")
				return
			}
		}

		// If a Bearer token was present but neither mbz_ nor valid JWT, reject.
		if jv != nil && strings.HasPrefix(authHeader, "Bearer ") {
			huma.WriteErr(api, ctx, http.StatusUnauthorized, "Invalid or expired token")
			return
		}

		// 3) Basic Auth
		if g.auth.isConfigured() {
			username, password, ok := parseBasicFromHuma(ctx)
			if !ok {
				if tokenParam := ctx.Query("token"); strings.HasPrefix(tokenParam, "basic:") {
					username, password, ok = parseBasicToken(tokenParam[6:])
				}
			}
			if !ok {
				huma.WriteErr(api, ctx, http.StatusUnauthorized, "Authentication required")
				return
			}
			h := sha256.Sum256([]byte(password))
			providedHash := hex.EncodeToString(h[:])
			if !g.auth.check(username, providedHash) {
				huma.WriteErr(api, ctx, http.StatusUnauthorized, "Invalid credentials")
				return
			}
			identity := &AuthIdentity{
				UserID:  username,
				Scopes:  nil,
				IsAdmin: true,
			}
			next(huma.WithContext(ctx, WithAuthIdentity(ctx.Context(), identity)))
			return
		}

		if jv != nil {
			huma.WriteErr(api, ctx, http.StatusUnauthorized, "Authentication required")
			return
		}

		// Fully open node — synthesize admin identity
		identity := &AuthIdentity{
			UserID:  "anonymous",
			Scopes:  nil,
			IsAdmin: true,
		}
		next(huma.WithContext(ctx, WithAuthIdentity(ctx.Context(), identity)))
	}
}

// parseBasicFromHuma extracts Basic Auth credentials from huma.Context.
func parseBasicFromHuma(ctx huma.Context) (string, string, bool) {
	auth := ctx.Header("Authorization")
	const prefix = "Basic "
	if len(auth) < len(prefix) || !strings.EqualFold(auth[:len(prefix)], prefix) {
		return "", "", false
	}
	return parseBasicToken(auth[len(prefix):])
}

// buildMinimalRequest constructs a minimal *http.Request with the
// Authorization header so tryJWTAuthWith can parse it.
func buildMinimalRequest(authHeader string, ctx huma.Context) *http.Request {
	r, _ := http.NewRequestWithContext(ctx.Context(), http.MethodGet, "/", nil)
	if authHeader != "" {
		r.Header.Set("Authorization", authHeader)
	}
	if qp := ctx.Query("token"); qp != "" {
		r.URL.RawQuery = "token=" + qp
	}
	return r
}
