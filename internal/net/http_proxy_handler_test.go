package net

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	peer "github.com/libp2p/go-libp2p/core/peer"
)

func TestHTTPProxyHandler_ProxyGETRequest(t *testing.T) {
	localServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"path": r.URL.Path, "method": r.Method})
	}))
	defer localServer.Close()

	trustedPeer, _ := peer.Decode("12D3KooWDMhdm5yrvtrbkshXFjkqLedHLzBQ1WBhkJyzNDXNhWps")
	handler := NewHTTPProxyHandler([]peer.ID{trustedPeer}, localServer.URL)

	reqBuf := &bytes.Buffer{}
	req, _ := http.NewRequest("GET", "/v1/listings", nil)
	req.Write(reqBuf)

	respBuf := &bytes.Buffer{}
	handler.proxyRequest(reqBuf, respBuf, trustedPeer)

	resp, err := http.ReadResponse(bufio.NewReader(respBuf), req)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body)
	if body["path"] != "/v1/listings" {
		t.Errorf("expected path=/v1/listings, got=%s", body["path"])
	}
	if body["method"] != "GET" {
		t.Errorf("expected method=GET, got=%s", body["method"])
	}
}

func TestHTTPProxyHandler_ProxyPOSTWithBody(t *testing.T) {
	var receivedBody string
	localServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		receivedBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"created": "true"})
	}))
	defer localServer.Close()

	trustedPeer, _ := peer.Decode("12D3KooWDMhdm5yrvtrbkshXFjkqLedHLzBQ1WBhkJyzNDXNhWps")
	handler := NewHTTPProxyHandler([]peer.ID{trustedPeer}, localServer.URL)

	reqBody := `{"title":"new product"}`
	reqBuf := &bytes.Buffer{}
	req, _ := http.NewRequest("POST", "/v1/listings", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer valid-jwt")
	req.ContentLength = int64(len(reqBody))
	req.Write(reqBuf)

	respBuf := &bytes.Buffer{}
	handler.proxyRequest(reqBuf, respBuf, trustedPeer)

	resp, err := http.ReadResponse(bufio.NewReader(respBuf), req)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected 201, got %d", resp.StatusCode)
	}
	if receivedBody != reqBody {
		t.Errorf("expected body=%q, got=%q", reqBody, receivedBody)
	}
}

func TestHTTPProxyHandler_ForwardsHeaders(t *testing.T) {
	var receivedAuth, receivedVia, receivedPeer string
	localServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		receivedVia = r.Header.Get("X-Forwarded-Via")
		receivedPeer = r.Header.Get("X-Forwarded-Peer")
		w.WriteHeader(http.StatusOK)
	}))
	defer localServer.Close()

	trustedPeer, _ := peer.Decode("12D3KooWDMhdm5yrvtrbkshXFjkqLedHLzBQ1WBhkJyzNDXNhWps")
	handler := NewHTTPProxyHandler([]peer.ID{trustedPeer}, localServer.URL)

	reqBuf := &bytes.Buffer{}
	req, _ := http.NewRequest("GET", "/v1/profile", nil)
	req.Header.Set("Authorization", "Bearer my-jwt-token")
	req.Write(reqBuf)

	respBuf := &bytes.Buffer{}
	handler.proxyRequest(reqBuf, respBuf, trustedPeer)

	if receivedAuth != "Bearer my-jwt-token" {
		t.Errorf("Authorization not forwarded, got=%s", receivedAuth)
	}
	if receivedVia != "libp2p" {
		t.Errorf("X-Forwarded-Via not set, got=%s", receivedVia)
	}
	if receivedPeer != trustedPeer.String() {
		t.Errorf("X-Forwarded-Peer not set, got=%s", receivedPeer)
	}
}

func TestHTTPProxyHandler_LocalAPIDown(t *testing.T) {
	trustedPeer, _ := peer.Decode("12D3KooWDMhdm5yrvtrbkshXFjkqLedHLzBQ1WBhkJyzNDXNhWps")
	handler := NewHTTPProxyHandler([]peer.ID{trustedPeer}, "http://127.0.0.1:1") // unreachable port

	reqBuf := &bytes.Buffer{}
	req, _ := http.NewRequest("GET", "/v1/listings", nil)
	req.Write(reqBuf)

	respBuf := &bytes.Buffer{}
	handler.proxyRequest(reqBuf, respBuf, trustedPeer)

	resp, err := http.ReadResponse(bufio.NewReader(respBuf), req)
	if err != nil {
		t.Fatalf("failed to read error response: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("expected 502, got %d", resp.StatusCode)
	}
}

func TestHTTPProxyHandler_TrustCheck(t *testing.T) {
	trustedPeer, _ := peer.Decode("12D3KooWDMhdm5yrvtrbkshXFjkqLedHLzBQ1WBhkJyzNDXNhWps")
	untrustedPeer, _ := peer.Decode("12D3KooWRm1AqnMBqFCy4NKmEFj2shBCTUgDr3RnBhBNx9e2X3Kq")
	handler := NewHTTPProxyHandler([]peer.ID{trustedPeer}, "http://localhost:5102")

	if !handler.trustedPeers[trustedPeer] {
		t.Error("expected trustedPeer to be in trust list")
	}
	if handler.trustedPeers[untrustedPeer] {
		t.Error("expected untrustedPeer to not be in trust list")
	}
}

func TestHTTPProxyHandler_AllMethodsAllowed(t *testing.T) {
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD"}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			var receivedMethod string
			localServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedMethod = r.Method
				w.WriteHeader(http.StatusOK)
			}))
			defer localServer.Close()

			trustedPeer, _ := peer.Decode("12D3KooWDMhdm5yrvtrbkshXFjkqLedHLzBQ1WBhkJyzNDXNhWps")
			handler := NewHTTPProxyHandler([]peer.ID{trustedPeer}, localServer.URL)

			reqBuf := &bytes.Buffer{}
			req, _ := http.NewRequest(method, "/v1/test", nil)
			req.Write(reqBuf)

			respBuf := &bytes.Buffer{}
			handler.proxyRequest(reqBuf, respBuf, trustedPeer)

			if receivedMethod != method {
				t.Errorf("expected %s, got %s", method, receivedMethod)
			}
		})
	}
}
