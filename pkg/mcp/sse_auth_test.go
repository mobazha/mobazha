package mcp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestIdentityCache_GetSet(t *testing.T) {
	cache := NewIdentityCache(5 * time.Minute)

	identity := &IdentityData{
		UserID: "user-1",
		PeerID: "QmPeer1",
		Scopes: []string{"listings:read"},
	}

	cache.Set("hash-abc", identity)

	got, ok := cache.Get("hash-abc")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if got.UserID != "user-1" {
		t.Errorf("expected user-1, got %s", got.UserID)
	}
}

func TestIdentityCache_Miss(t *testing.T) {
	cache := NewIdentityCache(5 * time.Minute)

	_, ok := cache.Get("nonexistent")
	if ok {
		t.Error("expected cache miss")
	}
}

func TestIdentityCache_Expiry(t *testing.T) {
	cache := NewIdentityCache(1 * time.Millisecond)

	identity := &IdentityData{UserID: "user-1"}
	cache.Set("hash-abc", identity)

	time.Sleep(5 * time.Millisecond)

	_, ok := cache.Get("hash-abc")
	if ok {
		t.Error("expected cache miss after expiry")
	}
}

func TestHashToken(t *testing.T) {
	h1 := hashToken("token-a")
	h2 := hashToken("token-a")
	h3 := hashToken("token-b")

	if h1 != h2 {
		t.Error("same token should produce same hash")
	}
	if h1 == h3 {
		t.Error("different tokens should produce different hashes")
	}
	if len(h1) != 64 {
		t.Errorf("SHA-256 hex should be 64 chars, got %d", len(h1))
	}
}

func TestResolveIdentityFromHeaders_CacheHit(t *testing.T) {
	cache := NewIdentityCache(5 * time.Minute)

	expected := &IdentityData{
		UserID: "cached-user",
		PeerID: "QmCached",
		Scopes: []string{"listings:read"},
	}
	tokenHash := hashToken("my-token")
	cache.Set(tokenHash, expected)

	headers := http.Header{}
	headers.Set("Authorization", "Bearer my-token")

	got, err := ResolveIdentityFromHeaders(headers, "http://unused", "/platform/v1/auth/identity", nil, cache)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.UserID != "cached-user" {
		t.Errorf("expected cached-user, got %s", got.UserID)
	}
}

func TestResolveIdentityFromHeaders_NoToken(t *testing.T) {
	cache := NewIdentityCache(5 * time.Minute)

	_, err := ResolveIdentityFromHeaders(http.Header{}, "http://unused", "/platform/v1/auth/identity", nil, cache)
	if err == nil {
		t.Error("expected error for no token")
	}
	if err != ErrNoToken {
		t.Errorf("expected ErrNoToken, got %v", err)
	}
}

func TestResolveIdentityFromHeaders_APIFallback(t *testing.T) {
	identityResp := map[string]interface{}{
		"data": map[string]interface{}{
			"user_id": "api-user",
			"peer_id": "QmAPI",
			"scopes":  []string{"orders:read"},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/platform/v1/auth/identity" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(identityResp)
	}))
	defer srv.Close()

	cache := NewIdentityCache(5 * time.Minute)
	headers := http.Header{}
	headers.Set("Authorization", "Bearer fresh-token")

	got, err := ResolveIdentityFromHeaders(headers, srv.URL, "/platform/v1/auth/identity", nil, cache)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.UserID != "api-user" {
		t.Errorf("expected api-user, got %s", got.UserID)
	}

	tokenHash := hashToken("fresh-token")
	cached, ok := cache.Get(tokenHash)
	if !ok {
		t.Fatal("expected identity to be cached after API call")
	}
	if cached.UserID != "api-user" {
		t.Errorf("cached identity mismatch: %s", cached.UserID)
	}
}
