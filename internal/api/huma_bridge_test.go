//go:build !private_distribution

package api

import (
	"context"
	"net/http"
	"testing"
)

// TestNodeBridgeRequestWithOptionalAuth_BareNode_DoesNotInjectAdmin guards
// against regression of the "anonymous IsAdmin" fallback that previously
// granted full admin privileges to unauthenticated callers on bare dev
// nodes (no admin password + no JWT validator configured). See the SECURITY
// comment in nodeBridgeRequestWithOptionalAuth for the threat model.
func TestNodeBridgeRequestWithOptionalAuth_BareNode_DoesNotInjectAdmin(t *testing.T) {
	gateway := &Gateway{
		config: &GatewayConfig{},
		// auth not configured (zero authState), jwtValidator == nil — the
		// exact bare-node misconfiguration the fix addresses.
	}

	meta := &originRequestMeta{
		OriginalHeaders: http.Header{},
		Host:            "localhost",
		RemoteAddr:      "192.0.2.1:12345",
	}
	ctx := withOriginMeta(context.Background(), meta)

	req := gateway.nodeBridgeRequestWithOptionalAuth(
		ctx, http.MethodGet, "/v1/orders/anyOrder/digital-assets", nil,
	)

	if got := GetAuthIdentity(req.Context()); got != nil {
		t.Fatalf("expected no AuthIdentity on bare node (no admin password + no JWT validator), got %+v", got)
	}
}

// TestNodeBridgeRequestWithOptionalAuth_NoAuthHeader_NoIdentity verifies the
// optional-auth bridge does not invent an identity when the request carries
// no credentials and admin auth is configured.
func TestNodeBridgeRequestWithOptionalAuth_NoAuthHeader_NoIdentity(t *testing.T) {
	gateway := &Gateway{
		config: &GatewayConfig{},
		auth: authState{
			username:     "admin",
			passwordHash: "1c8bfe8f801d79745c4631d09fff36c82aa37fc4cce4fc946683d7b336b63032",
		},
	}

	meta := &originRequestMeta{
		OriginalHeaders: http.Header{},
		Host:            "localhost",
		RemoteAddr:      "192.0.2.1:12345",
	}
	ctx := withOriginMeta(context.Background(), meta)

	req := gateway.nodeBridgeRequestWithOptionalAuth(
		ctx, http.MethodGet, "/v1/orders/anyOrder/digital-assets", nil,
	)

	if got := GetAuthIdentity(req.Context()); got != nil {
		t.Fatalf("expected no AuthIdentity when no credentials are supplied, got %+v", got)
	}
}
