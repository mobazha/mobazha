package api

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net/http"
	"sync"
	"time"
)

const (
	// AdminSessionCookieName is the short-lived standalone administrator session cookie.
	AdminSessionCookieName = "Mobazha_Admin_Session"
	// AdminSessionCSRFHeader carries the per-session CSRF proof on unsafe requests.
	AdminSessionCSRFHeader = "X-CSRF-Token"
	defaultAdminSessionTTL = 30 * time.Minute
	maxAdminSessions       = 64
)

type adminSessionRecord struct {
	UserID    string
	CSRFToken string
	ExpiresAt time.Time
}

type adminSessionStore struct {
	mu       sync.Mutex
	ttl      time.Duration
	now      func() time.Time
	sessions map[[sha256.Size]byte]adminSessionRecord
}

func newAdminSessionStore(ttl time.Duration) *adminSessionStore {
	if ttl <= 0 {
		ttl = defaultAdminSessionTTL
	}
	return &adminSessionStore{
		ttl:      ttl,
		now:      time.Now,
		sessions: make(map[[sha256.Size]byte]adminSessionRecord),
	}
}

func randomSessionSecret() (string, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return "", fmt.Errorf("read secure random bytes: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(secret), nil
}

func sessionTokenHash(token string) [sha256.Size]byte {
	return sha256.Sum256([]byte(token))
}

func (s *adminSessionStore) issue(userID string) (string, adminSessionRecord, error) {
	token, err := randomSessionSecret()
	if err != nil {
		return "", adminSessionRecord{}, err
	}
	csrfToken, err := randomSessionSecret()
	if err != nil {
		return "", adminSessionRecord{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	s.removeExpiredLocked(now)
	if len(s.sessions) >= maxAdminSessions {
		s.removeOldestLocked()
	}
	record := adminSessionRecord{
		UserID:    userID,
		CSRFToken: csrfToken,
		ExpiresAt: now.Add(s.ttl),
	}
	s.sessions[sessionTokenHash(token)] = record
	return token, record, nil
}

func (s *adminSessionStore) get(token string) (adminSessionRecord, bool) {
	if token == "" {
		return adminSessionRecord{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := sessionTokenHash(token)
	record, ok := s.sessions[key]
	if !ok {
		return adminSessionRecord{}, false
	}
	if !record.ExpiresAt.After(s.now()) {
		delete(s.sessions, key)
		return adminSessionRecord{}, false
	}
	return record, true
}

func (s *adminSessionStore) revoke(token string) {
	if token == "" {
		return
	}
	s.mu.Lock()
	delete(s.sessions, sessionTokenHash(token))
	s.mu.Unlock()
}

func (s *adminSessionStore) revokeAll() {
	s.mu.Lock()
	clear(s.sessions)
	s.mu.Unlock()
}

func (s *adminSessionStore) removeExpiredLocked(now time.Time) {
	for key, record := range s.sessions {
		if !record.ExpiresAt.After(now) {
			delete(s.sessions, key)
		}
	}
}

func (s *adminSessionStore) removeOldestLocked() {
	var (
		oldestKey [sha256.Size]byte
		oldest    time.Time
		found     bool
	)
	for key, record := range s.sessions {
		if !found || record.ExpiresAt.Before(oldest) {
			oldestKey = key
			oldest = record.ExpiresAt
			found = true
		}
	}
	if found {
		delete(s.sessions, oldestKey)
	}
}

func (g *Gateway) ensureAdminSessionStore() *adminSessionStore {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.adminSessions == nil {
		var ttl time.Duration
		if g.config != nil {
			ttl = g.config.AdminSessionTTL
		}
		g.adminSessions = newAdminSessionStore(ttl)
	}
	return g.adminSessions
}

func (g *Gateway) adminSessionFromRequest(r *http.Request) (string, adminSessionRecord, bool) {
	cookie, err := r.Cookie(AdminSessionCookieName)
	if err != nil {
		return "", adminSessionRecord{}, false
	}
	record, ok := g.ensureAdminSessionStore().get(cookie.Value)
	return cookie.Value, record, ok
}

func adminSessionIdentity(record adminSessionRecord) *AuthIdentity {
	return &AuthIdentity{
		UserID:  record.UserID,
		Scopes:  nil,
		IsAdmin: true,
	}
}

func csrfTokenMatches(expected, actual string) bool {
	if expected == "" || actual == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expected), []byte(actual)) == 1
}

func adminSessionRequiresCSRF(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	default:
		return true
	}
}

func (g *Gateway) adminSessionCookie(token string, expiresAt time.Time, r *http.Request) string {
	secure := r != nil && r.TLS != nil
	if g.config != nil && g.config.UseSSL {
		secure = true
	}
	return (&http.Cookie{
		Name:     AdminSessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		MaxAge:   max(1, int(time.Until(expiresAt).Seconds())),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	}).String()
}

func (g *Gateway) expiredAdminSessionCookie(r *http.Request) string {
	secure := r != nil && r.TLS != nil
	if g.config != nil && g.config.UseSSL {
		secure = true
	}
	return (&http.Cookie{
		Name:     AdminSessionCookieName,
		Path:     "/",
		Expires:  time.Unix(1, 0).UTC(),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	}).String()
}
