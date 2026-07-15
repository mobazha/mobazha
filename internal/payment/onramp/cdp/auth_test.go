// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package cdp

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// The JWT must match CDP's v2 contract exactly: EdDSA over {sub: keyID,
// iss: "cdp", aud: ["cdp_service"], nbf, exp: +120s, uri: "METHOD host
// path"}, with kid and a nonce in the header, carried as a Bearer credential.
func TestKeyAuthenticatorSignsTheCDPContract(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	auth, err := NewKeyAuthenticator("key-id-1", base64.StdEncoding.EncodeToString(priv))
	if err != nil {
		t.Fatalf("NewKeyAuthenticator: %v", err)
	}
	frozen := time.Unix(1_800_000_000, 0)
	auth.now = func() time.Time { return frozen }

	req, _ := http.NewRequest(http.MethodPost, "https://api.developer.coinbase.com/onramp/v1/token", nil)
	if err := auth.Authorize(req); err != nil {
		t.Fatalf("Authorize: %v", err)
	}

	header := req.Header.Get("Authorization")
	if len(header) < 8 || header[:7] != "Bearer " {
		t.Fatalf("Authorization = %q, want Bearer token", header)
	}

	parsed, err := jwt.Parse(header[7:], func(tok *jwt.Token) (interface{}, error) {
		if tok.Method != jwt.SigningMethodEdDSA {
			t.Fatalf("alg = %v, want EdDSA", tok.Method.Alg())
		}
		return pub, nil
	}, jwt.WithTimeFunc(func() time.Time { return frozen }), jwt.WithAudience("cdp_service"))
	if err != nil || !parsed.Valid {
		t.Fatalf("verify jwt: %v", err)
	}
	if parsed.Header["kid"] != "key-id-1" {
		t.Fatalf("kid = %v", parsed.Header["kid"])
	}
	if nonce, _ := parsed.Header["nonce"].(string); len(nonce) != 32 {
		t.Fatalf("nonce = %v, want 16 random bytes hex-encoded", parsed.Header["nonce"])
	}

	claims := parsed.Claims.(jwt.MapClaims)
	if claims["sub"] != "key-id-1" || claims["iss"] != "cdp" {
		t.Fatalf("claims = %+v", claims)
	}
	if claims["uri"] != "POST api.developer.coinbase.com/onramp/v1/token" {
		t.Fatalf("uri = %v", claims["uri"])
	}
	aud, _ := claims["aud"].([]interface{})
	if len(aud) != 1 || aud[0] != "cdp_service" {
		t.Fatalf("aud = %v", claims["aud"])
	}
	if int64(claims["exp"].(float64))-int64(claims["nbf"].(float64)) != 120 {
		t.Fatalf("exp-nbf = %v, want 120s", claims)
	}
}

// The console export is {"id", "privateKey": base64(seed||pub)}; a 32-byte
// seed-only key must also work, and junk must fail loudly.
func TestLoadKeyAuthenticator(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "key.json")
	blob, _ := json.Marshal(map[string]string{
		"id":         "key-id-2",
		"privateKey": base64.StdEncoding.EncodeToString(priv),
	})
	if err := os.WriteFile(path, blob, 0o600); err != nil {
		t.Fatalf("write key file: %v", err)
	}
	if _, err := LoadKeyAuthenticator(path); err != nil {
		t.Fatalf("LoadKeyAuthenticator: %v", err)
	}

	if _, err := NewKeyAuthenticator("k", base64.StdEncoding.EncodeToString(priv.Seed())); err != nil {
		t.Fatalf("32-byte seed must be accepted: %v", err)
	}
	if _, err := NewKeyAuthenticator("k", base64.StdEncoding.EncodeToString([]byte("short"))); err == nil {
		t.Fatal("junk key material must be rejected")
	}
	if _, err := NewKeyAuthenticator("", base64.StdEncoding.EncodeToString(priv)); err == nil {
		t.Fatal("empty key id must be rejected")
	}
}
