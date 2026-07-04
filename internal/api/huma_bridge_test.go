package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNodeBridgeRecorderBinaryPreservesDownload(t *testing.T) {
	rr := httptest.NewRecorder()
	rr.Header().Set("Content-Type", "application/gzip")
	rr.Header().Set("Content-Disposition", `attachment; filename="mobazha-diag.tar.gz"`)
	rr.WriteHeader(http.StatusOK)
	_, _ = rr.Write([]byte{0x1f, 0x8b, 0x08, 0x00})

	out, err := nodeBridgeRecorderBinary(rr)
	if err != nil {
		t.Fatalf("nodeBridgeRecorderBinary: %v", err)
	}
	if out.ContentType != "application/gzip" {
		t.Fatalf("Content-Type = %q, want application/gzip", out.ContentType)
	}
	if out.ContentDisposition != `attachment; filename="mobazha-diag.tar.gz"` {
		t.Fatalf("Content-Disposition = %q", out.ContentDisposition)
	}
	if got, want := string(out.Body), string([]byte{0x1f, 0x8b, 0x08, 0x00}); got != want {
		t.Fatalf("Body = %v, want %v", []byte(got), []byte(want))
	}
}

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
