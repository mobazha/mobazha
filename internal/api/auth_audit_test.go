package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
)

type bufferAuthAuditSink struct {
	mu     sync.Mutex
	events []AuthAuditEvent
}

func (s *bufferAuthAuditSink) RecordAuthAudit(_ context.Context, event AuthAuditEvent) {
	s.mu.Lock()
	s.events = append(s.events, event)
	s.mu.Unlock()
}

func (s *bufferAuthAuditSink) snapshot() []AuthAuditEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]AuthAuditEvent(nil), s.events...)
}

func TestAuthAuditAttributes_QueryableSchema(t *testing.T) {
	expiresAt := time.Date(2026, 7, 5, 7, 0, 0, 0, time.UTC)
	event := AuthAuditEvent{
		SchemaVersion: authAuditSchemaVersion,
		Type:          AuthAuditSessionCreated, Outcome: "success", ActorID: "admin",
		AuthMethod: "basic", ClientIP: "127.0.0.1",
		RequestMethod: http.MethodPost, RequestPath: "/v1/auth/admin-session",
		SessionExpiresAt: &expiresAt, OccurredAt: expiresAt.Add(-time.Minute),
	}
	attrs := authAuditAttributes(event)
	got := make(map[string]any, len(attrs)/2)
	for i := 0; i < len(attrs); i += 2 {
		key, ok := attrs[i].(string)
		if !ok {
			t.Fatalf("attribute key is not a string: %#v", attrs[i])
		}
		got[key] = attrs[i+1]
	}
	for key, want := range map[string]any{
		"audit.category":       "authentication",
		"audit.schema_version": authAuditSchemaVersion,
		"audit.event":          string(AuthAuditSessionCreated),
		"audit.outcome":        "success",
		"actor.id":             "admin",
		"auth.method":          "basic",
		"client.ip":            "127.0.0.1",
		"http.method":          http.MethodPost,
		"http.path":            "/v1/auth/admin-session",
	} {
		if got[key] != want {
			t.Errorf("attribute %q = %#v, want %#v", key, got[key], want)
		}
	}
	for key := range got {
		for _, forbidden := range []string{"password", "cookie", "token", "csrf", "authorization"} {
			if strings.Contains(strings.ToLower(key), forbidden) {
				t.Errorf("credential-shaped audit attribute %q is present", key)
			}
		}
	}
}

func TestAuthAudit_LoginDeniedDoesNotLeakCredentials(t *testing.T) {
	g := testGateway(t, "testpass")
	audit := &bufferAuthAuditSink{}
	g.authAuditSink = audit
	router := chi.NewMux()
	api := humachi.New(router, huma.DefaultConfig("test", "1.0.0"))
	g.installNodeHumaMiddlewares(api)
	g.registerNodeHumaAuthAdminOperations(api)

	request := httptest.NewRequest(http.MethodPost, "/v1/auth/admin-session", nil)
	request.SetBasicAuth("admin", "super-secret-wrong-password")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("login returned %d, want 401: %s", response.Code, response.Body.String())
	}

	events := audit.snapshot()
	if len(events) != 1 {
		t.Fatalf("expected one denied login audit event, got %#v", events)
	}
	event := events[0]
	if event.Type != AuthAuditLoginDenied || event.Outcome != "denied" ||
		event.Reason != "invalid_credentials" || event.AuthMethod != "basic" {
		t.Fatalf("unexpected denied login audit event: %#v", event)
	}
	encoded, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encoded, []byte("super-secret-wrong-password")) {
		t.Fatalf("audit event leaked the submitted password: %s", encoded)
	}
}

func TestAuthAudit_ExpiredSessionIsDistinguished(t *testing.T) {
	g := testGateway(t, "testpass")
	audit := &bufferAuthAuditSink{}
	g.authAuditSink = audit
	now := time.Date(2026, 7, 5, 6, 0, 0, 0, time.UTC)
	g.adminSessions.ttl = time.Minute
	g.adminSessions.now = func() time.Time { return now }
	token, _, err := g.adminSessions.issue("admin")
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)

	router := chi.NewMux()
	api := humachi.New(router, huma.DefaultConfig("test", "1.0.0"))
	g.installNodeHumaMiddlewares(api)
	g.registerNodeHumaAuthAdminOperations(api)
	request := httptest.NewRequest(http.MethodGet, "/v1/auth/admin-session", nil)
	request.AddCookie(&http.Cookie{Name: AdminSessionCookieName, Value: token})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expired session returned %d, want 401", response.Code)
	}

	events := audit.snapshot()
	if len(events) != 1 || events[0].Type != AuthAuditSessionRejected || events[0].Reason != "expired_session" {
		t.Fatalf("unexpected expired session audit: %#v", events)
	}
}

func TestAuthAudit_PasswordRotationReportsSessionRevocation(t *testing.T) {
	g := testGateway(t, "oldpassword")
	audit := &bufferAuthAuditSink{}
	g.authAuditSink = audit
	for range 2 {
		if _, _, err := g.adminSessions.issue("admin"); err != nil {
			t.Fatal(err)
		}
	}

	body, err := json.Marshal(changePasswordRequest{
		CurrentPassword: "oldpassword",
		NewPassword:     "newpassword123",
	})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/v1/admin/password", bytes.NewReader(body))
	request.AddCookie(&http.Cookie{Name: AdminSessionCookieName, Value: "browser-session-cookie"})
	request = request.WithContext(WithAuthIdentity(request.Context(), &AuthIdentity{UserID: "admin", IsAdmin: true}))
	response := httptest.NewRecorder()
	g.handleChangePassword(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("password rotation returned %d: %s", response.Code, response.Body.String())
	}

	events := audit.snapshot()
	if len(events) != 1 {
		t.Fatalf("expected one password rotation event, got %#v", events)
	}
	event := events[0]
	if event.Type != AuthAuditPasswordRotated || event.Outcome != "success" ||
		event.ActorID != "admin" || event.AuthMethod != "session" || event.RevokedSessions != 2 {
		t.Fatalf("unexpected password rotation audit: %#v", event)
	}
}
