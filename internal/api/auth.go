package api

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
)

// AuthCookieName is the name for the authentication cookie
const AuthCookieName = "Mobazha_Auth_Cookie"

// AuthenticationMiddleware is a function which will be called for each request.
// It checks if the IP is on the whitelist and validates either the cookie
// authentication or basic authentication, if set in the config.
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
			if err != nil {
				ErrorResponse(w, http.StatusForbidden, "Forbidden")
				return
			}
			if g.config.Cookie != cookie.Value {
				ErrorResponse(w, http.StatusForbidden, "Forbidden")
				return
			}
		}
		if g.config.Username != "" && g.config.Password != "" {
			username, password, ok := r.BasicAuth()
			h := sha256.Sum256([]byte(password))
			password = hex.EncodeToString(h[:])
			if !ok || username != g.config.Username || password != g.config.Password {
				ErrorResponse(w, http.StatusForbidden, "Forbidden")
				return
			}
		}
		next.ServeHTTP(w, r)
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
