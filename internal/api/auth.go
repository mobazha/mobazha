package api

import (
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/mobazha/mobazha/pkg/apitoken"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/response"
)

// AuthCookieName is the name for the authentication cookie
const AuthCookieName = "Mobazha_Auth_Cookie"

const websocketAuthTokenProtocolPrefix = "mbz.auth.b64."

// authState holds the mutable authentication credentials.
// The GatewayConfig Username/Password provide the initial values;
// authState allows runtime changes (password change API) without
// restarting the node.
type authState struct {
	mu           sync.RWMutex
	username     string // plaintext username
	passwordHash string // bcrypt ($2a$/$2b$) or legacy SHA-256 hex
	hashFile     string // if non-empty, password hash is persisted here
	plainFile    string // first-run plaintext file, removed after password change
}

// checkPassword verifies a plaintext password against the stored hash.
// Supports both bcrypt and legacy SHA-256 hex hashes. When a legacy hash
// matches, upgradable is true so the caller can optionally re-hash.
func (s *authState) checkPassword(username, plainPassword string) (ok bool, upgradable bool) {
	s.mu.RLock()
	storedUser := s.username
	storedHash := s.passwordHash
	s.mu.RUnlock()

	if storedUser == "" || storedHash == "" {
		return false, false
	}

	if subtle.ConstantTimeCompare([]byte(username), []byte(storedUser)) != 1 {
		return false, false
	}

	ok, isLegacy := VerifyPassword(plainPassword, storedHash)
	return ok, ok && isLegacy
}

// upgradeHash re-hashes a plaintext password with bcrypt and persists the
// new hash to disk + memory. Best-effort; failures are logged, not fatal.
func (s *authState) upgradeHash(plainPassword string) {
	newHash, err := HashPassword(plainPassword)
	if err != nil {
		log.Warningf("Failed to upgrade password hash: %v", err)
		return
	}
	if s.hashFile != "" {
		if err := os.WriteFile(s.hashFile, []byte(newHash), 0600); err != nil {
			log.Warningf("Failed to persist upgraded hash: %v", err)
			return
		}
	}
	s.mu.Lock()
	s.passwordHash = newHash
	s.mu.Unlock()
}

func (s *authState) isConfigured() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.username != "" && s.passwordHash != ""
}

// AuthenticationMiddleware checks credentials for every request on protected
// /v1/* routes. It supports four authentication modes (checked in order):
//
//  1. mbz_ API token (for AI agents / MCP / programmatic access):
//     Authorization: Bearer mbz_<id>_<secret>
//     Validated against the local SQLite/GORM token store. Yields an
//     AuthIdentity with IsAPIToken=true and a concrete ScopeSet — subject
//     to ScopeEnforcementMiddleware.
//
//  2. JWT Bearer token (for SaaS proxy-mediated requests, e.g. Mini App):
//     Authorization: Bearer <jwt-token>
//     Signed by SaaS Casdoor; the user must be the store owner. Yields an
//     AuthIdentity with IsAdmin=true and Scopes=nil (full access).
//
//  3. Short-lived HttpOnly admin session (for the standalone browser UI):
//     Cookie: Mobazha_Admin_Session=<opaque token>
//     Unsafe methods additionally require X-CSRF-Token. Explicit
//     Authorization credentials always take precedence over this cookie.
//
//  4. Basic Auth (for direct admin access and session creation):
//     Authorization: Basic <base64(user:pass)>
//     Also supports ?token=basic:<base64> for direct WebSocket fallback.
//     Basic credentials are intentionally not accepted through
//     Sec-WebSocket-Protocol; that channel is reserved for JWTs.
//     Yields an AuthIdentity with IsAdmin=true and Scopes=nil (full access).
//
// On success, the AuthIdentity is attached to the request context so that
// ScopeEnforcementMiddleware / RequireScope can make permission decisions.
func (g *Gateway) AuthenticationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// SaaS / SharedRouter: the hosting resolver has already populated
		// AuthIdentity in the request context. Don't overwrite it.
		if GetAuthIdentity(r.Context()) != nil {
			next.ServeHTTP(w, r)
			return
		}

		// isBlocked is enforced only after a credential failure (below);
		// up-front blocking locks out shared-IP deployments (Sovereign / Docker /
		// NAT) where a real operator can't recover after a few stray 401s.
		// Correct credentials reset the counter via ResetAuthFailure().

		if len(g.config.AllowedIPs) > 0 {
			host, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				host = r.RemoteAddr
			}
			if !g.config.AllowedIPs[host] {
				ErrorResponse(w, http.StatusForbidden, "Forbidden")
				return
			}
		}

		if g.config.Cookie != "" {
			cookie, err := r.Cookie(AuthCookieName)
			if err != nil || subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(g.config.Cookie)) != 1 {
				ErrorResponse(w, http.StatusForbidden, "Forbidden")
				return
			}
		}

		authHeader := r.Header.Get("Authorization")
		jv := g.getJWTValidator()

		// 1) mbz_ API token (highest priority — cheap to detect, distinct format).
		if strings.HasPrefix(authHeader, "Bearer ") {
			bearerVal := strings.TrimSpace(authHeader[7:])
			if bearerVal == "" {
				if g.recordAuthFailureAndRateLimited(r) {
					writeAuthRateLimited(w)
					return
				}
				ErrorResponse(w, http.StatusUnauthorized, "Empty Bearer token")
				return
			}
			if apitoken.IsAPIToken(bearerVal) {
				identity, ok := g.tryAPITokenAuth(bearerVal)
				if !ok {
					if g.recordAuthFailureAndRateLimited(r) {
						writeAuthRateLimited(w)
						return
					}
					ErrorResponse(w, http.StatusUnauthorized, "Invalid or expired API token")
					return
				}
				g.ResetAuthFailure(r)
				next.ServeHTTP(w, r.WithContext(WithAuthIdentity(r.Context(), identity)))
				return
			}
			// Non-mbz_ Bearer: fall through to JWT validation below.
		}

		// 2) JWT Bearer (or ?token= query for WebSocket).
		if identity, ok := g.tryJWTAuthWith(jv, r); ok {
			g.ResetAuthFailure(r)
			next.ServeHTTP(w, r.WithContext(WithAuthIdentity(r.Context(), identity)))
			return
		}

		// If a Bearer token was present but neither mbz_ nor a valid JWT, reject.
		if jv != nil && strings.HasPrefix(authHeader, "Bearer ") {
			if g.recordAuthFailureAndRateLimited(r) {
				writeAuthRateLimited(w)
				return
			}
			ErrorResponse(w, http.StatusUnauthorized, "Invalid or expired token")
			return
		}

		// 3) Short-lived browser admin session. Explicit Authorization or
		// WebSocket Basic credentials always take precedence over ambient cookies.
		tokenParam := r.URL.Query().Get("token")
		if authHeader == "" && !strings.HasPrefix(tokenParam, "basic:") {
			if _, session, ok := g.adminSessionFromRequest(r); ok {
				if adminSessionRequiresCSRF(r.Method) &&
					!csrfTokenMatches(session.CSRFToken, r.Header.Get(AdminSessionCSRFHeader)) {
					ErrorResponse(w, http.StatusForbidden, "Invalid or missing CSRF token")
					return
				}
				g.ResetAuthFailure(r)
				next.ServeHTTP(w, r.WithContext(WithAuthIdentity(r.Context(), adminSessionIdentity(session))))
				return
			}
		}

		// 4) Basic Auth.
		if g.auth.isConfigured() {
			username, password, ok := r.BasicAuth()
			if !ok {
				if strings.HasPrefix(tokenParam, "basic:") {
					username, password, ok = parseBasicToken(tokenParam[6:])
				}
			}
			if !ok {
				ErrorResponse(w, http.StatusUnauthorized, "Authentication required")
				return
			}

			matched, upgradable := g.auth.checkPassword(username, password)
			if !matched {
				if g.recordAuthFailureAndRateLimited(r) {
					writeAuthRateLimited(w)
					return
				}
				ErrorResponse(w, http.StatusUnauthorized, "Invalid credentials")
				return
			}
			g.ResetAuthFailure(r)
			if upgradable {
				go g.auth.upgradeHash(password)
			}
			identity := &AuthIdentity{
				UserID:  username,
				Scopes:  nil,
				IsAdmin: true,
			}
			next.ServeHTTP(w, r.WithContext(WithAuthIdentity(r.Context(), identity)))
			return
		}

		if jv != nil {
			// JWT validator configured but no Basic Auth and no Bearer: require auth.
			ErrorResponse(w, http.StatusUnauthorized, "Authentication required")
			return
		}

		// Fully open node (no auth configured): synthesize an admin identity so
		// downstream middleware (ScopeEnforcementMiddleware) sees Scopes == nil
		// and lets the request through.
		identity := &AuthIdentity{
			UserID:  "anonymous",
			Scopes:  nil,
			IsAdmin: true,
		}
		next.ServeHTTP(w, r.WithContext(WithAuthIdentity(r.Context(), identity)))
	})
}

// tryJWTAuthWith attempts JWT Bearer token authentication using the provided
// validator snapshot. Returns the resolved AuthIdentity (with full-access
// Scopes==nil and IsAdmin=true) and true on success.
//
// Token sources (checked in order):
//  1. Authorization: Bearer <token> header
//  2. Sec-WebSocket-Protocol mbz.auth.b64.* (WebSocket — keeps token out of URL/logs)
//  3. ?token=<jwt> query parameter (WebSocket fallback — browsers cannot set
//     headers on the WebSocket constructor)
func (g *Gateway) tryJWTAuthWith(jv *JWTValidator, r *http.Request) (*AuthIdentity, bool) {
	if jv == nil {
		return nil, false
	}

	var tokenStr string
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		tokenStr = strings.TrimSpace(authHeader[7:])
	} else if wsToken := tokenFromWebSocketProtocol(r); wsToken != "" {
		tokenStr = wsToken
	} else if qp := r.URL.Query().Get("token"); qp != "" && !strings.HasPrefix(qp, "basic:") {
		tokenStr = qp
	}

	if tokenStr == "" {
		return nil, false
	}

	claims, err := jv.ValidateToken(tokenStr)
	if err != nil {
		return nil, false
	}

	if !jv.IsAdmin(claims) {
		return nil, false
	}

	return &AuthIdentity{
		UserID:  claims.Id,
		PeerID:  claims.PeerID(),
		Scopes:  nil, // admin: full access
		IsAdmin: true,
	}, true
}

// tryJWTSubjectWith validates a JWT and returns its subject identity without
// granting admin/full-access privileges. Use only on explicitly public routes
// that perform their own resource ownership checks.
func (g *Gateway) tryJWTSubjectWith(jv *JWTValidator, r *http.Request) (*AuthIdentity, bool) {
	if jv == nil {
		return nil, false
	}

	var tokenStr string
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		tokenStr = strings.TrimSpace(authHeader[7:])
	} else if qp := r.URL.Query().Get("token"); qp != "" && !strings.HasPrefix(qp, "basic:") {
		tokenStr = qp
	}

	if tokenStr == "" {
		return nil, false
	}

	claims, err := jv.ValidateToken(tokenStr)
	if err != nil {
		return nil, false
	}

	return &AuthIdentity{
		UserID:  claims.Id,
		PeerID:  claims.PeerID(),
		Scopes:  contracts.NewScopeSet([]contracts.Scope{contracts.ScopePurchasesRead}),
		IsAdmin: false,
	}, true
}

// tryJWTCasdoorBuyerWith validates a Casdoor JWT and returns a buyer identity
// for cross-store marketplace checkout on external standalone stores.
// Scopes are nil (same as hosting's identityFromJWTClaims bridge) so buyer
// checkout routes pass scope enforcement without admin privileges.
func (g *Gateway) tryJWTCasdoorBuyerWith(jv *JWTValidator, r *http.Request) (*AuthIdentity, bool) {
	if jv == nil {
		return nil, false
	}

	var tokenStr string
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		tokenStr = strings.TrimSpace(authHeader[7:])
	} else if qp := r.URL.Query().Get("token"); qp != "" && !strings.HasPrefix(qp, "basic:") {
		tokenStr = qp
	}
	if tokenStr == "" || apitoken.IsAPIToken(tokenStr) {
		return nil, false
	}

	claims, err := jv.ValidateToken(tokenStr)
	if err != nil {
		return nil, false
	}

	return &AuthIdentity{
		UserID:  claims.Id,
		PeerID:  claims.PeerID(),
		IsAdmin: false,
	}, true
}

// tryAPITokenAuth validates an mbz_ prefixed API token against the local store.
// Returns the resolved AuthIdentity (carrying the token's persisted ScopeSet)
// and true on success; (nil, false) if the token is missing, revoked, expired,
// or has an invalid signature.
func (g *Gateway) tryAPITokenAuth(rawToken string) (*AuthIdentity, bool) {
	store := g.getTokenStore()
	if store == nil {
		return nil, false
	}

	prefix, err := apitoken.ExtractPrefix(rawToken)
	if err != nil {
		return nil, false
	}

	token, err := store.FindByPrefix(prefix)
	if err != nil || token == nil {
		return nil, false
	}

	if !token.IsActive() {
		return nil, false
	}

	if !apitoken.Verify(rawToken, token.TokenHash) {
		return nil, false
	}

	go store.TouchUsage(token.ID)

	return &AuthIdentity{
		UserID:     fmt.Sprintf("api_token:%d", token.ID),
		Scopes:     contracts.NewScopeSet(contracts.ParseScopes(token.Scopes)),
		IsAPIToken: true,
		TokenID:    token.ID,
	}, true
}

// parseBasicToken decodes a base64-encoded "user:pass" string.
// Used for WebSocket token query parameter fallback.
func parseBasicToken(encoded string) (username, password string, ok bool) {
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", "", false
	}
	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func tokenFromWebSocketProtocol(r *http.Request) string {
	for _, raw := range r.Header.Values("Sec-WebSocket-Protocol") {
		for _, protocol := range strings.Split(raw, ",") {
			protocol = strings.TrimSpace(protocol)
			if !strings.HasPrefix(protocol, websocketAuthTokenProtocolPrefix) {
				continue
			}
			encoded := strings.TrimPrefix(protocol, websocketAuthTokenProtocolPrefix)
			tokenBytes, err := base64.RawURLEncoding.DecodeString(encoded)
			if err != nil {
				tokenBytes, err = base64.URLEncoding.DecodeString(encoded)
			}
			if err == nil && len(tokenBytes) > 0 {
				token := string(tokenBytes)
				if strings.HasPrefix(token, "basic:") {
					continue
				}
				return token
			}
		}
	}
	return ""
}

type changePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

// handleChangePassword allows the owner to update the API password at runtime.
// The endpoint is registered on /v1/admin/password and protected by the
// same AuthenticationMiddleware that guards all /v1/* routes.
func (g *Gateway) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		ErrorResponse(w, http.StatusMethodNotAllowed, "POST required")
		return
	}

	if !g.auth.isConfigured() {
		ErrorResponse(w, http.StatusConflict, "No credentials configured")
		return
	}

	var req changePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.CurrentPassword == "" || req.NewPassword == "" {
		ErrorResponse(w, http.StatusBadRequest, "Both currentPassword and newPassword are required")
		return
	}

	if len(req.NewPassword) < 8 {
		ErrorResponse(w, http.StatusBadRequest, "New password must be at least 8 characters")
		return
	}

	if len(req.NewPassword) > 128 {
		ErrorResponse(w, http.StatusBadRequest, "New password must be at most 128 characters")
		return
	}

	g.auth.mu.RLock()
	storedHash := g.auth.passwordHash
	g.auth.mu.RUnlock()

	curOK, _ := VerifyPassword(req.CurrentPassword, storedHash)
	if !curOK {
		ErrorResponse(w, http.StatusForbidden, "Current password is incorrect")
		return
	}

	newHash, err := HashPassword(req.NewPassword)
	if err != nil {
		log.Errorf("Failed to hash new password: %v", err)
		ErrorResponse(w, http.StatusInternalServerError, "Failed to save password")
		return
	}

	if g.auth.hashFile != "" {
		if err := os.WriteFile(g.auth.hashFile, []byte(newHash), 0600); err != nil {
			log.Errorf("Failed to persist password hash: %v", err)
			ErrorResponse(w, http.StatusInternalServerError, "Failed to save password")
			return
		}
	}

	g.auth.mu.Lock()
	g.auth.passwordHash = newHash
	g.auth.mu.Unlock()
	g.ensureAdminSessionStore().revokeAll()

	if g.auth.plainFile != "" {
		if err := os.Remove(g.auth.plainFile); err != nil && !os.IsNotExist(err) {
			log.Warningf("Failed to remove plaintext password file: %v", err)
		}
	}

	response.Success(w, map[string]bool{"success": true})
}

func (g *Gateway) CORSAllowAllOriginsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		if r.Method == http.MethodOptions {
			return
		}
		next.ServeHTTP(w, r)
	})
}
