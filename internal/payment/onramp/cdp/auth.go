// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package cdp

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// KeyAuthenticator signs each request with a short-lived EdDSA JWT minted
// from a CDP v2 (Ed25519) API key.
//
// Per https://docs.cdp.coinbase.com/get-started/authentication/jwt-authentication:
// header {alg: "EdDSA", typ: "JWT", kid: keyID, nonce: 16-hex}, claims
// {sub: keyID, iss: "cdp", aud: ["cdp_service"], nbf: now, exp: now+120s,
// uri: "METHOD host path"}. The exported privateKey is 64 bytes base64:
// seed (32) || public key (32) — exactly Go's ed25519.PrivateKey layout.
type KeyAuthenticator struct {
	keyID string
	key   ed25519.PrivateKey
	now   func() time.Time
}

// keyFile is the JSON shape the CDP console exports.
type keyFile struct {
	ID         string `json:"id"`
	PrivateKey string `json:"privateKey"`
}

// NewKeyAuthenticator builds the authenticator from the raw key material.
func NewKeyAuthenticator(keyID, privateKeyBase64 string) (*KeyAuthenticator, error) {
	if keyID == "" {
		return nil, fmt.Errorf("cdp: key id is required")
	}
	raw, err := base64.StdEncoding.DecodeString(privateKeyBase64)
	if err != nil {
		return nil, fmt.Errorf("cdp: decode private key: %w", err)
	}
	var key ed25519.PrivateKey
	switch len(raw) {
	case ed25519.PrivateKeySize: // 64 bytes: seed || public key
		key = ed25519.PrivateKey(raw)
	case ed25519.SeedSize: // 32 bytes: seed only
		key = ed25519.NewKeyFromSeed(raw)
	default:
		return nil, fmt.Errorf("cdp: ed25519 private key must be 32 or 64 bytes, got %d", len(raw))
	}
	return &KeyAuthenticator{keyID: keyID, key: key, now: time.Now}, nil
}

// LoadKeyAuthenticator reads a CDP console key export ({"id", "privateKey"}).
// Keeping the secret in a file referenced by env avoids both checked-in
// credentials and secrets in shell history.
func LoadKeyAuthenticator(path string) (*KeyAuthenticator, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cdp: read key file: %w", err)
	}
	var kf keyFile
	if err := json.Unmarshal(raw, &kf); err != nil {
		return nil, fmt.Errorf("cdp: parse key file: %w", err)
	}
	return NewKeyAuthenticator(kf.ID, kf.PrivateKey)
}

// Authorize implements Authenticator: mint a fresh 2-minute JWT bound to this
// exact method+host+path and attach it as the Bearer credential.
func (a *KeyAuthenticator) Authorize(req *http.Request) error {
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return fmt.Errorf("cdp: nonce: %w", err)
	}
	now := a.now()
	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, jwt.MapClaims{
		"sub": a.keyID,
		"iss": "cdp",
		"aud": []string{"cdp_service"},
		"nbf": now.Unix(),
		"exp": now.Add(2 * time.Minute).Unix(),
		"uri": req.Method + " " + req.URL.Host + req.URL.Path,
	})
	token.Header["kid"] = a.keyID
	token.Header["nonce"] = hex.EncodeToString(nonce)
	signed, err := token.SignedString(a.key)
	if err != nil {
		return fmt.Errorf("cdp: sign jwt: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+signed)
	return nil
}

var _ Authenticator = (*KeyAuthenticator)(nil)
