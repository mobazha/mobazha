package api

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/mobazha/mobazha3.0/pkg/apitoken"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/response"
)

// AuthCookieName is the name for the authentication cookie
const AuthCookieName = "Mobazha_Auth_Cookie"

// authState holds the mutable authentication credentials.
// The GatewayConfig Username/Password provide the initial values;
// authState allows runtime changes (password change API) without
// restarting the node.
type authState struct {
	mu           sync.RWMutex
	username     string // plaintext username
	passwordHash string // SHA-256 hex
	hashFile     string // if non-empty, password hash is persisted here
	plainFile    string // first-run plaintext file, removed after password change
}

func (s *authState) check(username, passwordHash string) bool {
	s.mu.RLock()
	storedUser := s.username
	storedHash := s.passwordHash
	s.mu.RUnlock()

	if storedUser == "" || storedHash == "" {
		return false
	}

	userOK := subtle.ConstantTimeCompare([]byte(username), []byte(storedUser)) == 1
	passOK := subtle.ConstantTimeCompare([]byte(passwordHash), []byte(storedHash)) == 1
	return userOK && passOK
}

func (s *authState) isConfigured() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.username != "" && s.passwordHash != ""
}

// AuthenticationMiddleware checks credentials for every request on protected
// /v1/* routes. It supports three authentication modes (checked in order):
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
//  3. Basic Auth (for direct admin access):
//     Authorization: Basic <base64(user:pass)>
//     Also supports ?token=basic:<base64> for WebSocket fallback. Yields an
//     AuthIdentity with IsAdmin=true and Scopes=nil (full access).
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
			bearerVal := authHeader[7:]
			if apitoken.IsAPIToken(bearerVal) {
				identity, ok := g.tryAPITokenAuth(bearerVal)
				if !ok {
					ErrorResponse(w, http.StatusUnauthorized, "Invalid or expired API token")
					return
				}
				next.ServeHTTP(w, r.WithContext(WithAuthIdentity(r.Context(), identity)))
				return
			}
		}

		// 2) JWT Bearer (or ?token= query for WebSocket).
		if identity, ok := g.tryJWTAuthWith(jv, r); ok {
			next.ServeHTTP(w, r.WithContext(WithAuthIdentity(r.Context(), identity)))
			return
		}

		// If a Bearer token was present but neither mbz_ nor a valid JWT, reject.
		if jv != nil && strings.HasPrefix(authHeader, "Bearer ") {
			ErrorResponse(w, http.StatusUnauthorized, "Invalid or expired token")
			return
		}

		// 3) Basic Auth.
		if g.auth.isConfigured() {
			username, password, ok := r.BasicAuth()
			if !ok {
				if tokenParam := r.URL.Query().Get("token"); strings.HasPrefix(tokenParam, "basic:") {
					username, password, ok = parseBasicToken(tokenParam[6:])
				}
			}
			if !ok {
				ErrorResponse(w, http.StatusUnauthorized, "Authentication required")
				return
			}
			h := sha256.Sum256([]byte(password))
			providedHash := hex.EncodeToString(h[:])

			if !g.auth.check(username, providedHash) {
				ErrorResponse(w, http.StatusUnauthorized, "Invalid credentials")
				return
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
//  2. ?token=<jwt> query parameter (WebSocket fallback — browsers cannot set
//     headers on the WebSocket constructor)
func (g *Gateway) tryJWTAuthWith(jv *JWTValidator, r *http.Request) (*AuthIdentity, bool) {
	if jv == nil {
		return nil, false
	}

	var tokenStr string
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		tokenStr = authHeader[7:]
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

	curHash := sha256.Sum256([]byte(req.CurrentPassword))
	curHex := hex.EncodeToString(curHash[:])

	g.auth.mu.RLock()
	storedHash := g.auth.passwordHash
	g.auth.mu.RUnlock()

	if subtle.ConstantTimeCompare([]byte(curHex), []byte(storedHash)) != 1 {
		ErrorResponse(w, http.StatusForbidden, "Current password is incorrect")
		return
	}

	newHash := sha256.Sum256([]byte(req.NewPassword))
	newHex := hex.EncodeToString(newHash[:])

	// Persist to disk first so a restart doesn't revert the change.
	if g.auth.hashFile != "" {
		if err := os.WriteFile(g.auth.hashFile, []byte(newHex), 0600); err != nil {
			log.Errorf("Failed to persist password hash: %v", err)
			ErrorResponse(w, http.StatusInternalServerError, "Failed to save password")
			return
		}
	}

	g.auth.mu.Lock()
	g.auth.passwordHash = newHex
	g.auth.mu.Unlock()

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
