// Package api — huma_middleware.go
//
// AH-1.4: Bridges the Node gateway's auth model onto huma's per-operation
// declarative security. The Node supports three auth modes (in priority):
//  1. mbz_ API token → AuthIdentity with IsAPIToken=true + ScopeSet
//  2. Bearer JWT (Casdoor) → AuthIdentity with IsAdmin=true
//  3. Basic Auth → AuthIdentity with IsAdmin=true
//
// huma operations declare their auth requirement via:
//   - Security: nodeAuthSecurity (OR across basicAuth / bearerJWT / apiToken)
//     for owner-only routes
//   - omitting Security for anonymous/public routes
//
// The middleware delegates the actual credential check to the Gateway's
// existing auth helpers (tryAPITokenAuth, tryJWTAuthWith, auth.checkPassword),
// keeping a single source of truth for credential validation.
package api

import (
	"context"
	"crypto/subtle"
	"net"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/mobazha/mobazha3.0/pkg/apitoken"
)

// clientIPKey is the context key for the upstream client IP address
// extracted by nodeHumaClientIPMiddleware. Used by rate-limited Huma
// handlers (e.g. guest checkout) to throttle by IP.
type clientIPKey struct{}

// remoteIPFromHuma extracts the direct peer IP from huma.Context's
// RemoteAddr, deliberately ignoring proxy headers (X-Forwarded-For,
// X-Real-IP). Used for security-sensitive operations where trusting
// proxy headers would allow IP-based bypass (auth rate limiting,
// AllowedIPs, Cookie gate).
func remoteIPFromHuma(ctx huma.Context) string {
	addr := ctx.RemoteAddr()
	if addr == "" {
		return "unknown"
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return host
}

// extractCookieFromHuma parses the Cookie header to find a named cookie's value.
func extractCookieFromHuma(ctx huma.Context, name string) string {
	header := ctx.Header("Cookie")
	if header == "" {
		return ""
	}
	for _, part := range strings.Split(header, ";") {
		part = strings.TrimSpace(part)
		eq := strings.IndexByte(part, '=')
		if eq < 0 {
			continue
		}
		if part[:eq] == name {
			return part[eq+1:]
		}
	}
	return ""
}

// clientIPFromContext returns the upstream client IP previously stored
// by nodeHumaClientIPMiddleware. Falls back to "unknown" when the
// middleware has not run (e.g. unit tests).
func clientIPFromContext(ctx context.Context) string {
	if ip, ok := ctx.Value(clientIPKey{}).(string); ok {
		return ip
	}
	return "unknown"
}

// operationAcceptsAPIToken reports whether the huma operation's declared
// Security includes the apiToken scheme. Operations that omit apiToken
// (e.g. those using adminOnlyAuthSecurity for seed/key export, wallet
// setup wizards, or other "operator at the keyboard" actions) MUST refuse
// mbz_ tokens at runtime — the OpenAPI declaration is otherwise advisory
// only and a wallet:read token would be silently accepted by the broader
// scope middleware (which uses prefix matching against routeScopeMap).
//
// This function is the single runtime gate that turns Operation.Security
// into a real access decision. Without it, a token holding ScopeWalletRead
// could read the 25-word seed via GET /v1/wallet/external_payment/secrets/mnemonic
// because the route prefix-matches GET /v1/wallet → ScopeWalletRead.
func operationAcceptsAPIToken(op *huma.Operation) bool {
	if op == nil {
		return false
	}
	for _, requirement := range op.Security {
		if _, ok := requirement[SecuritySchemeAPIToken]; ok {
			return true
		}
	}
	return false
}

// enforceHumaScope checks API-token scopes against routeScopeMap for the
// given huma operation. Admin identities (Scopes == nil) always pass.
// Returns true if the request should be blocked (error already written).
func enforceHumaScope(api huma.API, ctx huma.Context, op *huma.Operation, identity *AuthIdentity) bool {
	if identity.Scopes == nil {
		return false
	}
	result := matchRouteScope(op.Method, op.Path, identity)
	if result.Allowed {
		return false
	}
	if !result.Matched {
		log.Warningf("[SCOPE_DENIED] api token %d attempted unmapped huma route %s %s",
			identity.TokenID, op.Method, op.Path)
	} else {
		logScopeDenial(identity, result.Scope, op.Path)
	}
	huma.WriteErr(api, ctx, http.StatusForbidden, result.DenyMsg)
	return true
}

// installNodeHumaMiddlewares wires auth and client-IP extraction onto
// the huma API. Origin-meta runs first to capture Host/Auth/TLS for
// bridged handlers. Client-IP runs second so rate-limited handlers can
// read the IP from context even for anonymous operations.
func (g *Gateway) installNodeHumaMiddlewares(api huma.API) {
	api.UseMiddleware(nodeHumaOriginMiddleware())
	api.UseMiddleware(nodeHumaClientIPMiddleware())
	api.UseMiddleware(g.nodeHumaAuthMiddleware(api))
}

// nodeHumaOriginMiddleware captures the original HTTP request metadata
// and ALL headers, then stores them in context so that nodeBridgeRequest
// can faithfully restore them on synthetic requests passed to legacy
// handlers. Using humachi.Unwrap gives us the full http.Header map,
// eliminating any per-header allowlist and ensuring webhook signatures,
// custom headers, and future provider headers are transparently forwarded.
func nodeHumaOriginMiddleware() func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		r, _ := humachi.Unwrap(ctx)
		meta := &originRequestMeta{
			OriginalHeaders: r.Header.Clone(),
			Host:            ctx.Host(),
			RemoteAddr:      ctx.RemoteAddr(),
			IsTLS:           ctx.TLS() != nil,
		}
		newCtx := withOriginMeta(ctx.Context(), meta)
		next(huma.WithContext(ctx, newCtx))
	}
}

// nodeHumaClientIPMiddleware extracts the upstream client IP from
// X-Forwarded-For / X-Real-IP headers and stores it in the request
// context. Huma handlers read it via clientIPFromContext(ctx).
func nodeHumaClientIPMiddleware() func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		ip := extractClientIPFromHumaContext(ctx)
		newCtx := context.WithValue(ctx.Context(), clientIPKey{}, ip)
		next(huma.WithContext(ctx, newCtx))
	}
}

// extractClientIPFromHumaContext mirrors extractClientIP but reads
// from huma.Context headers / RemoteAddr instead of *http.Request.
func extractClientIPFromHumaContext(ctx huma.Context) string {
	if xff := ctx.Header("X-Forwarded-For"); xff != "" {
		for i, c := range xff {
			if c == ',' {
				return strings.TrimSpace(xff[:i])
			}
		}
		return strings.TrimSpace(xff)
	}
	if xri := ctx.Header("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	if addr := ctx.RemoteAddr(); addr != "" {
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			return addr
		}
		return host
	}
	return "unknown"
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

		// Security layers — mirror legacy AuthenticationMiddleware behavior.
		// Use RemoteAddr directly (not proxy headers) to prevent XFF bypass.
		peerIP := remoteIPFromHuma(ctx)

		// No up-front isBlocked check — see AuthenticationMiddleware.
		// The limiter is enforced only after a credential failure
		// below; valid credentials reset via resetIP.

		if len(g.config.AllowedIPs) > 0 && !g.config.AllowedIPs[peerIP] {
			huma.WriteErr(api, ctx, http.StatusForbidden, "Forbidden")
			return
		}

		if g.config.Cookie != "" {
			cookieVal := extractCookieFromHuma(ctx, AuthCookieName)
			if subtle.ConstantTimeCompare([]byte(cookieVal), []byte(g.config.Cookie)) != 1 {
				huma.WriteErr(api, ctx, http.StatusForbidden, "Forbidden")
				return
			}
		}

		authHeader := ctx.Header("Authorization")
		jv := g.getJWTValidator()

		// 1) mbz_ API token
		if strings.HasPrefix(authHeader, "Bearer ") {
			bearerVal := strings.TrimSpace(authHeader[7:])
			if bearerVal != "" && apitoken.IsAPIToken(bearerVal) {
				// Honor the operation's declared Security: if apiToken
				// is not listed (e.g. adminOnlyAuthSecurity), refuse
				// without even validating the token. This is what
				// makes the spec a real access control instead of a
				// documentation hint, and prevents wallet:read tokens
				// from reaching seed/key export endpoints that happen
				// to share a prefix with read-only wallet routes in
				// routeScopeMap.
				if !operationAcceptsAPIToken(op) {
					huma.WriteErr(api, ctx, http.StatusUnauthorized,
						"API tokens are not accepted for this operation; use Basic Auth or Bearer JWT")
					return
				}
				identity, ok := g.tryAPITokenAuth(bearerVal)
				if !ok {
					if g.recordHumaAuthFailureAndRateLimited(ctx) {
						writeHumaAuthRateLimited(api, ctx)
						return
					}
					huma.WriteErr(api, ctx, http.StatusUnauthorized, "Invalid or expired API token")
					return
				}
				g.resetHumaAuthFailure(ctx)
				if enforceHumaScope(api, ctx, op, identity) {
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
					g.resetHumaAuthFailure(ctx)
					next(huma.WithContext(ctx, WithAuthIdentity(ctx.Context(), identity)))
					return
				}
				// Bearer was present but invalid → hard fail
				if g.recordHumaAuthFailureAndRateLimited(ctx) {
					writeHumaAuthRateLimited(api, ctx)
					return
				}
				huma.WriteErr(api, ctx, http.StatusUnauthorized, "Invalid or expired token")
				return
			}
		}

		// If a Bearer token was present but neither mbz_ nor valid JWT, reject.
		if jv != nil && strings.HasPrefix(authHeader, "Bearer ") {
			if g.recordHumaAuthFailureAndRateLimited(ctx) {
				writeHumaAuthRateLimited(api, ctx)
				return
			}
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
			matched, upgradable := g.auth.checkPassword(username, password)
			if !matched {
				if g.recordHumaAuthFailureAndRateLimited(ctx) {
					writeHumaAuthRateLimited(api, ctx)
					return
				}
				huma.WriteErr(api, ctx, http.StatusUnauthorized, "Invalid credentials")
				return
			}
			g.resetHumaAuthFailure(ctx)
			if upgradable {
				go g.auth.upgradeHash(password)
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

		// SECURITY: when neither Basic Auth nor a JWT validator is
		// configured, the node has no way to authenticate anyone — earlier
		// code synthesized an "anonymous IsAdmin" identity to make bare
		// dev nodes curl-friendly, which silently granted full admin to
		// any caller on every owner-only route (Security: nodeAuthSecurity).
		// On misconfigured production deployments this is a critical
		// privilege escalation. Standalone nodes auto-generate admin
		// credentials via EnsureStandaloneAuth, so legitimate self-hosted
		// deployments never reach this branch; only intentionally bare
		// configurations (DataDir empty, auth explicitly disabled) do, and
		// those must explicitly opt in by setting AllowedIPs / configuring
		// auth before exposing owner-only routes.
		huma.WriteErr(api, ctx, http.StatusUnauthorized,
			"Authentication required — configure an admin password or JWT validator")
	}
}

func (g *Gateway) recordHumaAuthFailureAndRateLimited(ctx huma.Context) bool {
	if g.authLimiter == nil {
		return false
	}
	peerIP := remoteIPFromHuma(ctx)
	g.authLimiter.recordFailure(peerIP)
	return g.authLimiter.isBlocked(peerIP)
}

func (g *Gateway) resetHumaAuthFailure(ctx huma.Context) {
	if g.authLimiter != nil {
		g.authLimiter.resetIP(remoteIPFromHuma(ctx))
	}
}

func writeHumaAuthRateLimited(api huma.API, ctx huma.Context) {
	ctx.SetHeader("Retry-After", "900")
	huma.WriteErr(api, ctx, http.StatusTooManyRequests,
		"Too many authentication failures. Try again later.")
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
