// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package privy

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"sync"
	"time"
)

// jwksCache fetches and caches a Privy app's ES256 access-token signing keys
// from its JWKS endpoint, selecting by key id. Keys are fetched lazily on first
// use and re-fetched once when a token presents an unknown kid (a rotation),
// bounded by a short cooldown so an attacker-chosen kid cannot drive unbounded
// fetches.
type jwksCache struct {
	url    string
	client *http.Client

	mu          sync.RWMutex
	keys        map[string]*ecdsa.PublicKey
	lastFetch   time.Time
	refetchWait time.Duration
}

func newJWKSCache(url string, client *http.Client) *jwksCache {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &jwksCache{url: url, client: client, refetchWait: 30 * time.Second}
}

// key returns the public key for kid, fetching the JWKS if needed. When kid is
// empty and exactly one key is cached, that key is returned (single-key apps).
func (j *jwksCache) key(kid string) (*ecdsa.PublicKey, error) {
	if k, ok := j.lookup(kid); ok {
		return k, nil
	}
	if err := j.refresh(); err != nil {
		return nil, err
	}
	if k, ok := j.lookup(kid); ok {
		return k, nil
	}
	return nil, fmt.Errorf("%w: no signing key for kid %q", ErrAccessTokenInvalid, kid)
}

func (j *jwksCache) lookup(kid string) (*ecdsa.PublicKey, bool) {
	j.mu.RLock()
	defer j.mu.RUnlock()
	if kid != "" {
		k, ok := j.keys[kid]
		return k, ok
	}
	if len(j.keys) == 1 {
		for _, k := range j.keys {
			return k, true
		}
	}
	return nil, false
}

func (j *jwksCache) refresh() error {
	j.mu.Lock()
	defer j.mu.Unlock()
	// Coalesce: if another goroutine just fetched, don't hammer the endpoint.
	if len(j.keys) > 0 && time.Since(j.lastFetch) < j.refetchWait {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	keys, err := fetchJWKS(ctx, j.client, j.url)
	if err != nil {
		return err
	}
	j.keys = keys
	j.lastFetch = time.Now()
	return nil
}

// jwkKey is one JSON Web Key (EC subset) from a JWKS document.
type jwkKey struct {
	Kty string `json:"kty"`
	Crv string `json:"crv"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

func fetchJWKS(ctx context.Context, client *http.Client, url string) (map[string]*ecdsa.PublicKey, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("privy: build jwks request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("privy: fetch jwks: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("privy: jwks endpoint returned %d", resp.StatusCode)
	}
	payload, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("privy: read jwks: %w", err)
	}
	var doc struct {
		Keys []jwkKey `json:"keys"`
	}
	if err := json.Unmarshal(payload, &doc); err != nil {
		return nil, fmt.Errorf("privy: decode jwks: %w", err)
	}
	out := make(map[string]*ecdsa.PublicKey, len(doc.Keys))
	for _, k := range doc.Keys {
		// Privy signs access tokens with ES256 (EC P-256); ignore other keys.
		if k.Kty != "EC" || (k.Alg != "" && k.Alg != "ES256") {
			continue
		}
		pub, err := k.toECDSA()
		if err != nil {
			return nil, err
		}
		out[k.Kid] = pub
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("privy: jwks contained no ES256 EC keys")
	}
	return out, nil
}

// toECDSA converts an EC P-256 JWK into an ECDSA public key.
func (k jwkKey) toECDSA() (*ecdsa.PublicKey, error) {
	if k.Crv != "P-256" {
		return nil, fmt.Errorf("privy: unsupported JWK curve %q (want P-256)", k.Crv)
	}
	xb, err := base64.RawURLEncoding.DecodeString(k.X)
	if err != nil {
		return nil, fmt.Errorf("privy: decode JWK x: %w", err)
	}
	yb, err := base64.RawURLEncoding.DecodeString(k.Y)
	if err != nil {
		return nil, fmt.Errorf("privy: decode JWK y: %w", err)
	}
	return &ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     new(big.Int).SetBytes(xb),
		Y:     new(big.Int).SetBytes(yb),
	}, nil
}
