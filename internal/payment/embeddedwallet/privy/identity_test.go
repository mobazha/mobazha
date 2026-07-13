// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package privy

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/mobazha/mobazha/pkg/contracts"
)

const (
	testAppID     = "test-app-id"
	testDID       = "did:privy:buyer-1"
	testCasdoor   = "casdoor-sub-1"
	testWalletHex = "0x4444444444444444444444444444444444444444"
)

// fakeDir is an in-memory userDirectory for proving the identity-link security
// logic without a live Privy app.
type fakeDir struct {
	byDID     map[string]*PrivyUser
	bySubject map[string]*PrivyUser
	err       error
}

func (f fakeDir) UserByDID(_ context.Context, did string) (*PrivyUser, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.byDID[did], nil
}

func (f fakeDir) UserByCustomAuthSubject(_ context.Context, subject string) (*PrivyUser, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.bySubject[subject], nil
}

// verifierFixture builds a verifier over a fresh ES256 keypair and a directory
// holding one bound buyer (DID testDID, subject testCasdoor, owning
// testWalletHex). It returns the verifier and the signing key so a test can
// mint tokens (or mint a mismatched one with a different key).
func verifierFixture(t *testing.T) (*IdentityVerifier, *ecdsa.PrivateKey) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	der, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		t.Fatalf("marshal pub: %v", err)
	}
	keyPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))

	user := &PrivyUser{
		DID:               testDID,
		CustomAuthSubject: testCasdoor,
		Wallets:           []PrivyWallet{{ID: "w-eth", Address: testWalletHex, ChainType: "ethereum"}},
	}
	dir := fakeDir{
		byDID:     map[string]*PrivyUser{testDID: user},
		bySubject: map[string]*PrivyUser{testCasdoor: user},
	}
	v, err := newIdentityVerifier(testAppID, keyPEM, dir)
	if err != nil {
		t.Fatalf("new verifier: %v", err)
	}
	return v, priv
}

// mintToken signs a Privy-style access token with the given key and claims.
func mintToken(t *testing.T, key *ecdsa.PrivateKey, iss, aud, sub string, exp time.Time) string {
	t.Helper()
	claims := jwt.RegisteredClaims{
		Issuer:    iss,
		Audience:  jwt.ClaimStrings{aud},
		Subject:   sub,
		ExpiresAt: jwt.NewNumericDate(exp),
		IssuedAt:  jwt.NewNumericDate(time.Now().Add(-time.Minute)),
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodES256, claims).SignedString(key)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}

func goodToken(t *testing.T, key *ecdsa.PrivateKey) string {
	return mintToken(t, key, privyIssuer, testAppID, testDID, time.Now().Add(time.Hour))
}

func TestAuthorizeSignerAcceptsBoundBuyer(t *testing.T) {
	v, key := verifierFixture(t)
	did, err := v.AuthorizeSigner(context.Background(), goodToken(t, key), testCasdoor, testWalletHex)
	if err != nil {
		t.Fatalf("authorize: %v", err)
	}
	if did != testDID {
		t.Fatalf("did = %q, want %q", did, testDID)
	}
	// Address ownership is case-insensitive (Privy returns EIP-55 mixed case).
	if _, err := v.AuthorizeSigner(context.Background(), goodToken(t, key), "", "0X4444444444444444444444444444444444444444"); err != nil {
		t.Fatalf("case-insensitive ownership rejected: %v", err)
	}
}

func TestAuthorizeSignerRejectsBadTokens(t *testing.T) {
	v, key := verifierFixture(t)
	other, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)

	cases := []struct {
		name  string
		token string
	}{
		{"wrong-issuer", mintToken(t, key, "evil.example", testAppID, testDID, time.Now().Add(time.Hour))},
		{"wrong-audience", mintToken(t, key, privyIssuer, "other-app", testDID, time.Now().Add(time.Hour))},
		{"expired", mintToken(t, key, privyIssuer, testAppID, testDID, time.Now().Add(-time.Hour))},
		{"wrong-key", mintToken(t, other, privyIssuer, testAppID, testDID, time.Now().Add(time.Hour))},
		{"empty", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := v.AuthorizeSigner(context.Background(), tc.token, testCasdoor, testWalletHex); !errors.Is(err, ErrAccessTokenInvalid) {
				t.Fatalf("expected ErrAccessTokenInvalid, got %v", err)
			}
		})
	}
}

func TestAuthorizeSignerRejectsBindingMismatch(t *testing.T) {
	v, key := verifierFixture(t)
	// A valid token, but the Core-authenticated subject is a different buyer.
	if _, err := v.AuthorizeSigner(context.Background(), goodToken(t, key), "someone-else", testWalletHex); !errors.Is(err, ErrBuyerBindingMismatch) {
		t.Fatalf("expected ErrBuyerBindingMismatch, got %v", err)
	}
}

func TestAuthorizeSignerRejectsForeignWallet(t *testing.T) {
	v, key := verifierFixture(t)
	if _, err := v.AuthorizeSigner(context.Background(), goodToken(t, key), testCasdoor, "0x9999999999999999999999999999999999999999"); !errors.Is(err, ErrWalletNotOwnedByBuyer) {
		t.Fatalf("expected ErrWalletNotOwnedByBuyer, got %v", err)
	}
}

func TestResolveBuyerWallet(t *testing.T) {
	v, _ := verifierFixture(t)
	w, err := v.ResolveBuyerWallet(context.Background(), testCasdoor, contracts.ChainFamilyEVM)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if w.Address != testWalletHex {
		t.Fatalf("address = %q, want %q", w.Address, testWalletHex)
	}

	// No embedded wallet for a family the buyer has not provisioned.
	if _, err := v.ResolveBuyerWallet(context.Background(), testCasdoor, contracts.ChainFamilySolana); !errors.Is(err, ErrBuyerWalletNotFound) {
		t.Fatalf("expected ErrBuyerWalletNotFound, got %v", err)
	}

	// Unknown subject: directory returns nil, binding cannot be proven.
	if _, err := v.ResolveBuyerWallet(context.Background(), "unknown", contracts.ChainFamilyEVM); !errors.Is(err, ErrBuyerBindingMismatch) {
		t.Fatalf("expected ErrBuyerBindingMismatch, got %v", err)
	}

	// No subject at all: refused as missing buyer authorization.
	if _, err := v.ResolveBuyerWallet(context.Background(), "", contracts.ChainFamilyEVM); !errors.Is(err, contracts.ErrEmbeddedWalletNoBuyerAuthorization) {
		t.Fatalf("expected ErrEmbeddedWalletNoBuyerAuthorization, got %v", err)
	}
}

func TestNewIdentityVerifierRejectsBadKey(t *testing.T) {
	if _, err := newIdentityVerifier(testAppID, "not a pem", fakeDir{}); !errors.Is(err, ErrIdentityNotConfigured) {
		t.Fatalf("expected ErrIdentityNotConfigured, got %v", err)
	}
}

// TestJWKSVerifier proves the JWKS path: keys are fetched from the endpoint and
// selected by the token's kid, and a token signed by the matching key verifies.
func TestJWKSVerifier(t *testing.T) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	const kid = "sig-1"
	jwksDoc, _ := json.Marshal(map[string]any{
		"keys": []map[string]string{{
			"kty": "EC", "crv": "P-256", "alg": "ES256", "kid": kid,
			"x": base64.RawURLEncoding.EncodeToString(priv.PublicKey.X.Bytes()),
			"y": base64.RawURLEncoding.EncodeToString(priv.PublicKey.Y.Bytes()),
		}},
	})
	var fetches int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fetches++
		_, _ = w.Write(jwksDoc)
	}))
	defer srv.Close()

	user := &PrivyUser{DID: testDID, CustomAuthSubject: testCasdoor, Wallets: []PrivyWallet{{Address: testWalletHex, ChainType: "ethereum"}}}
	dir := fakeDir{byDID: map[string]*PrivyUser{testDID: user}}
	v, err := newIdentityVerifierJWKS(testAppID, srv.URL, srv.Client(), dir)
	if err != nil {
		t.Fatalf("new jwks verifier: %v", err)
	}

	tok := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.RegisteredClaims{
		Issuer: privyIssuer, Audience: jwt.ClaimStrings{testAppID}, Subject: testDID,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	})
	tok.Header["kid"] = kid
	signed, err := tok.SignedString(priv)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, err := v.AuthorizeSigner(context.Background(), signed, testCasdoor, testWalletHex); err != nil {
		t.Fatalf("jwks authorize: %v", err)
	}
	// A second verify reuses the cached key — no re-fetch.
	if _, err := v.AuthorizeSigner(context.Background(), signed, testCasdoor, testWalletHex); err != nil {
		t.Fatalf("jwks authorize (cached): %v", err)
	}
	if fetches != 1 {
		t.Fatalf("expected 1 JWKS fetch (cached thereafter), got %d", fetches)
	}
}

// TestLivePrivyJWKSFetch fetches the real app JWKS and asserts at least one
// ES256 key parses. It validates the JWKS half of the identity link against the
// live dev app without needing a buyer login. Gated on PRIVY_JWKS_URL.
func TestLivePrivyJWKSFetch(t *testing.T) {
	url := os.Getenv("PRIVY_JWKS_URL")
	if url == "" {
		t.Skip("set PRIVY_JWKS_URL to fetch the live Privy app JWKS")
	}
	keys, err := fetchJWKS(context.Background(), http.DefaultClient, url)
	if err != nil {
		t.Fatalf("fetch live jwks: %v", err)
	}
	if len(keys) == 0 {
		t.Fatalf("live JWKS returned no ES256 keys")
	}
	for kid := range keys {
		t.Logf("live Privy ES256 signing key: kid=%q", kid)
	}
}

// TestProviderProductionPaths wires the verifier into the Provider and drives
// both production operations: EnsureWallet resolves the bound buyer's wallet
// from the directory, and SignTypedData(SchemeUserJWT) verifies the buyer token
// and signs on the buyer's authority (carrying the access token to Privy).
func TestProviderProductionPaths(t *testing.T) {
	v, key := verifierFixture(t)

	fakeSig := "0x" + "11" + strings.Repeat("22", 63) + "1b" // 65 bytes, V=27
	var sawAccessToken bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/rpc") {
			if r.Header.Get("privy-access-token") != "" {
				sawAccessToken = true
			}
			_, _ = io.WriteString(w, `{"data":{"signature":"`+fakeSig+`"}}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	client := NewClient("app", "secret", srv.URL, srv.Client())
	p, err := New(Config{Client: client, Verifier: v})
	if err != nil {
		t.Fatalf("new: %v", err)
	}

	// EnsureWallet resolves the buyer's embedded wallet through the identity link.
	wallet, err := p.EnsureWallet(context.Background(), contracts.EnsureWalletRequest{
		Buyer: contracts.BuyerRef{Subject: testCasdoor}, RailID: "crypto:eip155:1:native",
	})
	if err != nil {
		t.Fatalf("ensure wallet: %v", err)
	}
	if wallet.Address != testWalletHex {
		t.Fatalf("resolved address %q, want %q", wallet.Address, testWalletHex)
	}

	// Sign with a valid buyer token: the provider authorizes then signs.
	req := signRequest(wallet, SchemeUserJWT)
	req.Authorization.Token = goodToken(t, key)
	sig, err := p.SignTypedData(context.Background(), req)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if len(sig.Signature) != 65 {
		t.Fatalf("expected 65-byte signature, got %d", len(sig.Signature))
	}
	if !sawAccessToken {
		t.Fatalf("provider did not carry the buyer access token to Privy")
	}

	// A token for a wallet the buyer does not own is refused before any rpc.
	foreign := wallet
	foreign.Address = "0x9999999999999999999999999999999999999999"
	badReq := signRequest(foreign, SchemeUserJWT)
	badReq.Authorization.Token = goodToken(t, key)
	if _, err := p.SignTypedData(context.Background(), badReq); !errors.Is(err, ErrWalletNotOwnedByBuyer) {
		t.Fatalf("expected ErrWalletNotOwnedByBuyer, got %v", err)
	}
}
