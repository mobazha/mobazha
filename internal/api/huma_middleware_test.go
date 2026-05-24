//go:build !private_distribution

package api

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
)

// TestOperationAcceptsAPIToken verifies that the runtime gate correctly
// reflects what the OpenAPI declaration says about apiToken acceptance.
// This is the single check that prevents a wallet:read mbz_ token from
// reaching seed/key export endpoints whose path happens to prefix-match
// a wallet read scope in routeScopeMap.
func TestOperationAcceptsAPIToken(t *testing.T) {
	cases := []struct {
		name string
		sec  []map[string][]string
		want bool
	}{
		{
			name: "nodeAuthSecurity (basic+jwt+apiToken) accepts",
			sec:  nodeAuthSecurity,
			want: true,
		},
		{
			name: "adminOnlyAuthSecurity (basic+jwt only) refuses",
			sec:  adminOnlyAuthSecurity,
			want: false,
		},
		{
			name: "nil security refuses (no API token allowed by default)",
			sec:  nil,
			want: false,
		},
		{
			name: "empty security refuses",
			sec:  []map[string][]string{},
			want: false,
		},
		{
			name: "single basic-only refuses",
			sec:  []map[string][]string{{SecuritySchemeBasicAuth: {}}},
			want: false,
		},
		{
			name: "single bearer-only refuses",
			sec:  []map[string][]string{{SecuritySchemeBearerJWT: {}}},
			want: false,
		},
		{
			name: "single apiToken-only accepts",
			sec:  []map[string][]string{{SecuritySchemeAPIToken: {}}},
			want: true,
		},
		{
			name: "mixed requirement (apiToken alongside others) accepts",
			sec: []map[string][]string{
				{SecuritySchemeBasicAuth: {}},
				{SecuritySchemeAPIToken: {}, SecuritySchemeBasicAuth: {}},
			},
			want: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			op := &huma.Operation{Security: tc.sec}
			got := operationAcceptsAPIToken(op)
			if got != tc.want {
				t.Errorf("operationAcceptsAPIToken: got %v, want %v", got, tc.want)
			}
		})
	}
}

// TestOperationAcceptsAPIToken_NilOp guards against accidental nil-deref
// when callers pass a nil operation pointer (defense-in-depth).
func TestOperationAcceptsAPIToken_NilOp(t *testing.T) {
	if operationAcceptsAPIToken(nil) {
		t.Fatal("nil operation must not accept API tokens")
	}
}

// TestAdminOnlyAuthSecurity_ConstantShape pins the shape of the
// adminOnlyAuthSecurity constant so future edits cannot silently add
// apiToken into the OR-list. If this test fails, audit every endpoint
// using adminOnlyAuthSecurity — its scope assumptions just changed.
func TestAdminOnlyAuthSecurity_ConstantShape(t *testing.T) {
	if operationAcceptsAPIToken(&huma.Operation{Security: adminOnlyAuthSecurity}) {
		t.Fatal("adminOnlyAuthSecurity must NOT include apiToken — " +
			"endpoints relying on it (EXTERNAL_PAYMENT seed export, wallet setup, " +
			"view-only export, transfer history) would become reachable " +
			"via mbz_ wallet:read tokens via routeScopeMap prefix match")
	}

	// Sanity: must include at least one human-driven scheme so the
	// endpoint is reachable at all.
	hasHuman := false
	for _, req := range adminOnlyAuthSecurity {
		if _, ok := req[SecuritySchemeBasicAuth]; ok {
			hasHuman = true
		}
		if _, ok := req[SecuritySchemeBearerJWT]; ok {
			hasHuman = true
		}
	}
	if !hasHuman {
		t.Fatal("adminOnlyAuthSecurity must include basicAuth and/or bearerJWT")
	}
}

func TestNodeHumaAuthMiddleware_JWTWebSocketProtocol(t *testing.T) {
	const localPeerID = "QmLocalPeerID1234567890ABCDEF1234567890"

	certPEM, privKey := generateTestRSACert()
	validator, err := NewJWTValidator(certPEM, localPeerID, "")
	if err != nil {
		t.Fatalf("NewJWTValidator: %v", err)
	}

	gateway := &Gateway{
		config:       &GatewayConfig{},
		jwtValidator: validator,
	}

	r := chi.NewMux()
	api := humachi.New(r, huma.DefaultConfig("test", "1.0.0"))
	gateway.installNodeHumaMiddlewares(api)

	type output struct {
		Body struct {
			OK bool `json:"ok"`
		}
	}
	huma.Register(api, huma.Operation{
		OperationID: "test-huma-ws-jwt",
		Method:      http.MethodGet,
		Path:        "/secure",
		Security:    adminOnlyAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*output, error) {
		out := &output{}
		out.Body.OK = true
		return out, nil
	})

	ts := httptest.NewServer(r)
	defer ts.Close()

	token := signToken(&JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		Properties: map[string]string{"peerID": localPeerID},
	}, privKey)
	encoded := base64.RawURLEncoding.EncodeToString([]byte(token))

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/secure", nil)
	req.Header.Set("Sec-WebSocket-Protocol", "mbz.auth.v1, mbz.auth.b64."+encoded)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected Huma JWT auth via websocket protocol to pass, got %d", resp.StatusCode)
	}
}
