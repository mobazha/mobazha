package api

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
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
// /v1/* routes. It supports two authentication modes:
//
//  1. JWT Bearer token (for SaaS proxy-mediated requests, e.g. Mini App):
//     Authorization: Bearer <jwt-token>
//     The JWT must be signed by SaaS Casdoor and the user must be the store
//     owner (via claims.Id == ownerUserID, or legacy peerID fallback).
//
//  2. Basic Auth (for direct admin access):
//     Authorization: Basic <base64(user:pass)>
//     Also supports ?token=basic:<base64> for WebSocket fallback.
//
// JWT is attempted first when a Bearer token is present AND the JWT validator
// is configured. If JWT validation fails, it falls through to Basic Auth.
func (g *Gateway) AuthenticationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

		if g.tryJWTAuth(r) {
			next.ServeHTTP(w, r)
			return
		}

		// If a Bearer token was present but JWT auth failed, reject immediately
		// instead of falling through to Basic Auth or unauthenticated access.
		if g.jwtValidator != nil {
			authHeader := r.Header.Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				ErrorResponse(w, http.StatusUnauthorized, "Invalid or expired token")
				return
			}
		}

		if g.auth.isConfigured() {
			username, password, ok := r.BasicAuth()
			if !ok {
				if tokenParam := r.URL.Query().Get("token"); strings.HasPrefix(tokenParam, "basic:") {
					username, password, ok = parseBasicToken(tokenParam[6:])
				}
			}
			if !ok {
				w.Header().Set("WWW-Authenticate", `Basic realm="Mobazha"`)
				ErrorResponse(w, http.StatusUnauthorized, "Authentication required")
				return
			}
			h := sha256.Sum256([]byte(password))
			providedHash := hex.EncodeToString(h[:])

			if !g.auth.check(username, providedHash) {
				w.Header().Set("WWW-Authenticate", `Basic realm="Mobazha"`)
				ErrorResponse(w, http.StatusUnauthorized, "Invalid credentials")
				return
			}
		} else if g.jwtValidator != nil {
			// JWT validator configured but no Basic Auth: require authentication.
			ErrorResponse(w, http.StatusUnauthorized, "Authentication required")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// tryJWTAuth attempts JWT Bearer token authentication. Returns true if the
// request carries a valid JWT from an authorized admin user.
func (g *Gateway) tryJWTAuth(r *http.Request) bool {
	if g.jwtValidator == nil {
		return false
	}

	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return false
	}
	tokenStr := authHeader[7:]

	claims, err := g.jwtValidator.ValidateToken(tokenStr)
	if err != nil {
		return false
	}

	return g.jwtValidator.IsAdmin(claims)
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
	})
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
