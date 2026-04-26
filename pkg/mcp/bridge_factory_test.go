package mcp

import (
	"net/http"
	"testing"

	gomcp "github.com/mark3labs/mcp-go/mcp"
)

func TestStaticBridgeFactory_ReturnsSameBridge(t *testing.T) {
	bridge := &mockBridge{}
	bf := StaticBridgeFactory(bridge)

	req1 := gomcp.CallToolRequest{}
	req2 := gomcp.CallToolRequest{}

	b1 := bf(req1)
	b2 := bf(req2)

	if b1 != b2 {
		t.Error("StaticBridgeFactory should return the same bridge instance")
	}
	if b1 != bridge {
		t.Error("StaticBridgeFactory should return the wrapped bridge")
	}
}

func TestSSEBridgeFactory_ExtractsToken(t *testing.T) {
	bf := SSEBridgeFactory("http://localhost:8080", nil)

	header := http.Header{}
	header.Set("Authorization", "Bearer my-secret-token")
	header.Set("X-Store-PeerID", "QmPeer123")

	req := gomcp.CallToolRequest{}
	req.Header = header

	b := bf(req)
	httpBridge, ok := b.(*HTTPBridge)
	if !ok {
		t.Fatal("expected *HTTPBridge")
	}
	if httpBridge.token != "my-secret-token" {
		t.Errorf("expected token 'my-secret-token', got '%s'", httpBridge.token)
	}
	if httpBridge.peerID != "QmPeer123" {
		t.Errorf("expected peerID 'QmPeer123', got '%s'", httpBridge.peerID)
	}
	if httpBridge.gatewayURL != "http://localhost:8080" {
		t.Errorf("expected gateway URL 'http://localhost:8080', got '%s'", httpBridge.gatewayURL)
	}
}

func TestSSEBridgeFactory_NilHeaders(t *testing.T) {
	bf := SSEBridgeFactory("http://localhost:8080", nil)

	req := gomcp.CallToolRequest{}
	b := bf(req)
	httpBridge, ok := b.(*HTTPBridge)
	if !ok {
		t.Fatal("expected *HTTPBridge")
	}
	if httpBridge.token != "" {
		t.Errorf("expected empty token, got '%s'", httpBridge.token)
	}
	if httpBridge.peerID != "" {
		t.Errorf("expected empty peerID, got '%s'", httpBridge.peerID)
	}
}

func TestExtractBearerToken(t *testing.T) {
	tests := []struct {
		name     string
		header   http.Header
		expected string
	}{
		{"nil header", nil, ""},
		{"no auth header", http.Header{}, ""},
		{"bearer token", http.Header{"Authorization": {"Bearer abc123"}}, "abc123"},
		{"raw token", http.Header{"Authorization": {"raw-token-no-prefix"}}, "raw-token-no-prefix"},
		{"empty bearer", http.Header{"Authorization": {"Bearer "}}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractBearerToken(tt.header)
			if got != tt.expected {
				t.Errorf("extractBearerToken() = %q, want %q", got, tt.expected)
			}
		})
	}
}
