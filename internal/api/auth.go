package api

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
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

// AuthenticationMiddleware checks IP allowlist, cookie, and basic-auth
// credentials for every request on the /v1/* router.
func (g *Gateway) AuthenticationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(g.config.AllowedIPs) > 0 {
			remoteAddr := strings.Split(r.RemoteAddr, ":")
			if !g.config.AllowedIPs[remoteAddr[0]] {
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

		if g.auth.isConfigured() {
			username, password, ok := r.BasicAuth()
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
		}

		next.ServeHTTP(w, r)
	})
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
