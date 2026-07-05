package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
)

func TestAdminSessionHumaLifecycle(t *testing.T) {
	g := testGateway(t, "testpass")
	audit := &bufferAuthAuditSink{}
	g.authAuditSink = audit
	router := chi.NewMux()
	api := humachi.New(router, huma.DefaultConfig("test", "1.0.0"))
	g.installNodeHumaMiddlewares(api)
	g.registerNodeHumaAuthAdminOperations(api)
	server := httptest.NewServer(router)
	defer server.Close()

	loginReq, err := http.NewRequest(http.MethodPost, server.URL+"/v1/auth/admin-session", nil)
	if err != nil {
		t.Fatal(err)
	}
	loginReq.SetBasicAuth("admin", "testpass")
	loginResp, err := http.DefaultClient.Do(loginReq)
	if err != nil {
		t.Fatal(err)
	}
	defer loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(loginResp.Body)
		t.Fatalf("login returned %d: %s", loginResp.StatusCode, body)
	}
	cookies := loginResp.Cookies()
	if len(cookies) != 1 || cookies[0].Name != AdminSessionCookieName {
		t.Fatalf("expected administrator session cookie, got %#v", cookies)
	}
	if !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatalf("session cookie is missing security attributes: %#v", cookies[0])
	}
	var login adminSessionStatus
	if err := json.NewDecoder(loginResp.Body).Decode(&login); err != nil {
		t.Fatal(err)
	}
	if !login.Authenticated || login.CSRFToken == "" || login.ExpiresAt == nil {
		t.Fatalf("unexpected login response: %#v", login)
	}

	getReq, _ := http.NewRequest(http.MethodGet, server.URL+"/v1/auth/admin-session", nil)
	getReq.AddCookie(cookies[0])
	getResp, err := http.DefaultClient.Do(getReq)
	if err != nil {
		t.Fatal(err)
	}
	if getResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(getResp.Body)
		getResp.Body.Close()
		t.Fatalf("session restore returned %d: %s", getResp.StatusCode, body)
	}
	var restored adminSessionStatus
	if err := json.NewDecoder(getResp.Body).Decode(&restored); err != nil {
		getResp.Body.Close()
		t.Fatal(err)
	}
	getResp.Body.Close()
	if restored.CSRFToken != login.CSRFToken {
		t.Fatalf("restore returned a different CSRF token: %#v", restored)
	}

	logoutReq, _ := http.NewRequest(http.MethodDelete, server.URL+"/v1/auth/admin-session", nil)
	logoutReq.AddCookie(cookies[0])
	logoutResp, err := http.DefaultClient.Do(logoutReq)
	if err != nil {
		t.Fatal(err)
	}
	logoutResp.Body.Close()
	if logoutResp.StatusCode != http.StatusForbidden {
		t.Fatalf("logout without CSRF returned %d, want 403", logoutResp.StatusCode)
	}

	logoutReq, _ = http.NewRequest(http.MethodDelete, server.URL+"/v1/auth/admin-session", nil)
	logoutReq.AddCookie(cookies[0])
	logoutReq.Header.Set(AdminSessionCSRFHeader, login.CSRFToken)
	logoutResp, err = http.DefaultClient.Do(logoutReq)
	if err != nil {
		t.Fatal(err)
	}
	logoutResp.Body.Close()
	if logoutResp.StatusCode != http.StatusOK {
		t.Fatalf("logout with CSRF returned %d, want 200", logoutResp.StatusCode)
	}

	getReq, _ = http.NewRequest(http.MethodGet, server.URL+"/v1/auth/admin-session", nil)
	getReq.AddCookie(cookies[0])
	getResp, err = http.DefaultClient.Do(getReq)
	if err != nil {
		t.Fatal(err)
	}
	getResp.Body.Close()
	if getResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("revoked session returned %d, want 401", getResp.StatusCode)
	}

	events := audit.snapshot()
	wantTypes := []AuthAuditEventType{
		AuthAuditSessionCreated,
		AuthAuditSessionRestored,
		AuthAuditSessionCSRFDenied,
		AuthAuditSessionRevoked,
		AuthAuditSessionRejected,
	}
	if len(events) != len(wantTypes) {
		t.Fatalf("unexpected audit events: %#v", events)
	}
	for i, wantType := range wantTypes {
		if events[i].Type != wantType {
			t.Errorf("audit event %d type = %q, want %q", i, events[i].Type, wantType)
		}
		if events[i].SchemaVersion != authAuditSchemaVersion || events[i].OccurredAt.IsZero() {
			t.Errorf("audit event %d is missing schema metadata: %#v", i, events[i])
		}
	}
	if events[0].AuthMethod != "basic" || events[0].ActorID != "admin" {
		t.Errorf("unexpected session creation audit: %#v", events[0])
	}
	if events[2].Reason != "csrf_mismatch" || events[2].Outcome != "denied" {
		t.Errorf("unexpected CSRF audit: %#v", events[2])
	}
	if events[3].RevokedSessions != 1 {
		t.Errorf("logout must report one revoked session: %#v", events[3])
	}
	if events[4].Reason != "unknown_session" {
		t.Errorf("unexpected rejected session audit: %#v", events[4])
	}
	encoded, err := json.Marshal(events)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encoded, []byte(cookies[0].Value)) || bytes.Contains(encoded, []byte(login.CSRFToken)) {
		t.Fatalf("audit events leaked session credential material: %s", encoded)
	}
}
