package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAdminSessionStoreLifecycle(t *testing.T) {
	now := time.Date(2026, 7, 5, 5, 0, 0, 0, time.UTC)
	store := newAdminSessionStore(10 * time.Minute)
	store.now = func() time.Time { return now }

	token, record, err := store.issue("admin")
	if err != nil {
		t.Fatal(err)
	}
	if token == "" || record.CSRFToken == "" || token == record.CSRFToken {
		t.Fatal("session and CSRF secrets must be non-empty and independent")
	}
	if record.UserID != "admin" || !record.ExpiresAt.Equal(now.Add(10*time.Minute)) {
		t.Fatalf("unexpected session record: %#v", record)
	}
	if _, exists := store.sessions[sessionTokenHash(token)]; !exists {
		t.Fatal("session must be indexed by its hash")
	}
	if got, ok := store.get(token); !ok || got.CSRFToken != record.CSRFToken {
		t.Fatalf("issued session was not retrievable: %#v, %v", got, ok)
	}

	now = now.Add(10 * time.Minute)
	if _, ok := store.get(token); ok {
		t.Fatal("expired session must be rejected")
	}
}

func TestAdminSessionStoreRevoke(t *testing.T) {
	store := newAdminSessionStore(time.Minute)
	first, _, err := store.issue("admin")
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := store.issue("admin")
	if err != nil {
		t.Fatal(err)
	}

	store.revoke(first)
	if _, ok := store.get(first); ok {
		t.Error("revoked session must be rejected")
	}
	if _, ok := store.get(second); !ok {
		t.Error("revoking one session must not revoke another")
	}
	store.revokeAll()
	if _, ok := store.get(second); ok {
		t.Error("revokeAll must reject all sessions")
	}
}

func TestAdminSessionCookieSecurityAttributes(t *testing.T) {
	g := &Gateway{config: &GatewayConfig{}}
	expiresAt := time.Now().Add(time.Minute)

	plainReq := httptest.NewRequest(http.MethodGet, "http://localhost/v1/auth/admin-session", nil)
	plain := g.adminSessionCookie("secret", expiresAt, plainReq)
	for _, attribute := range []string{"HttpOnly", "SameSite=Strict", "Path=/"} {
		if !strings.Contains(plain, attribute) {
			t.Errorf("cookie missing %q: %s", attribute, plain)
		}
	}
	if strings.Contains(plain, "Secure") {
		t.Fatalf("local HTTP cookie must remain usable without Secure: %s", plain)
	}

	tlsReq := httptest.NewRequest(http.MethodGet, "https://example.com/v1/auth/admin-session", nil)
	secure := g.adminSessionCookie("secret", expiresAt, tlsReq)
	if !strings.Contains(secure, "Secure") {
		t.Fatalf("TLS cookie must include Secure: %s", secure)
	}

	expired := g.expiredAdminSessionCookie(tlsReq)
	for _, attribute := range []string{"Max-Age=0", "HttpOnly", "Secure", "SameSite=Strict", "Path=/"} {
		if !strings.Contains(expired, attribute) {
			t.Errorf("expired cookie missing %q: %s", attribute, expired)
		}
	}
}
