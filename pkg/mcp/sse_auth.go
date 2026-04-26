package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"sync"
	"time"
)

// identityCacheEntry caches identity data for a token hash.
type identityCacheEntry struct {
	identity  *IdentityData
	expiresAt time.Time
}

// IdentityCache caches token -> identity mappings with TTL.
type IdentityCache struct {
	mu      sync.RWMutex
	entries map[string]identityCacheEntry
	ttl     time.Duration
}

// NewIdentityCache creates a cache with the given TTL.
func NewIdentityCache(ttl time.Duration) *IdentityCache {
	return &IdentityCache{
		entries: make(map[string]identityCacheEntry),
		ttl:     ttl,
	}
}

func (c *IdentityCache) Get(tokenHash string) (*IdentityData, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.entries[tokenHash]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return entry.identity, true
}

func (c *IdentityCache) Set(tokenHash string, identity *IdentityData) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[tokenHash] = identityCacheEntry{
		identity:  identity,
		expiresAt: time.Now().Add(c.ttl),
	}
}

// hashToken creates a SHA-256 hash of a token for cache keys (never store raw tokens).
func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// ResolveIdentityFromHeaders extracts the token from request headers,
// checks the cache, and falls back to calling the identity API at identityPath.
// identityPath is deployment-specific and required (e.g. /v1/auth/identity for
// standalone, /platform/v1/auth/identity for hosting).
func ResolveIdentityFromHeaders(headers http.Header, gatewayURL, identityPath string, httpClient *http.Client, cache *IdentityCache) (*IdentityData, error) {
	token := extractBearerToken(headers)
	if token == "" {
		return nil, ErrNoToken
	}

	tokenHash := hashToken(token)
	if identity, ok := cache.Get(tokenHash); ok {
		return identity, nil
	}

	bridge := NewHTTPBridge(gatewayURL, token, "", httpClient)
	identity, err := FetchIdentityFromPath(context.Background(), bridge, identityPath)
	if err != nil {
		return nil, err
	}

	cache.Set(tokenHash, identity)
	return identity, nil
}

var ErrNoToken = &AuthError{Message: "no authorization token provided"}

type AuthError struct {
	Message string
}

func (e *AuthError) Error() string {
	return e.Message
}
