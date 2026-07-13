// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package privy

import (
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/mobazha/mobazha/pkg/contracts"
)

// This file is the Casdoor->Privy identity link (RFC-0012 batch 3 / strategy
// doc §4). Its job is a single security property: before an embedded wallet is
// admitted into a moderated-escrow attempt as the buyer's co-signer, prove the
// wallet genuinely belongs to the Core-authenticated buyer.
//
// The trust chain is:
//
//	Casdoor `sub` (Core-authenticated)  ─ custom-auth link ─▶  Privy user (DID)  ─ owns ─▶  embedded wallet
//
// Privy is configured (dashboard: JWKS URL + user-id claim) to trust Casdoor
// JWTs as a custom auth provider, so a buyer's Privy user carries a linked
// `custom_auth` account whose `custom_user_id` is the Casdoor `sub`. The
// verifier here re-proves that link server-side; it never trusts the client's
// claim of who it is.
//
// The pure security logic (access-token verification, subject binding, wallet
// ownership) is behind the userDirectory seam and is exercised by unit tests
// with a fake directory. The concrete Privy REST directory is in this file too,
// but its endpoint/field specifics are confirmed by the env-gated live test
// against a real dev app, not asserted here.

// privyIssuer is the `iss` claim on a genuine Privy access token.
const privyIssuer = "privy.io"

// Identity-link sentinel errors.
var (
	// ErrIdentityNotConfigured marks a provider whose Casdoor->Privy link is not
	// configured (no verification key). Production buyer-authorized paths stay
	// fail-closed (ErrProductionAuthNotWired) in that case.
	ErrIdentityNotConfigured = errors.New("privy: identity link is not configured")

	// ErrAccessTokenInvalid marks a Privy access token that failed signature,
	// issuer, audience, or expiry verification, or resolves to no user.
	ErrAccessTokenInvalid = errors.New("privy: access token is invalid")

	// ErrBuyerBindingMismatch marks a Privy user whose linked custom-auth subject
	// is not the Core-authenticated Casdoor subject. Proving this mismatch is the
	// whole point of the link: it stops a buyer admitting someone else's wallet.
	ErrBuyerBindingMismatch = errors.New("privy: Privy user is not bound to the authenticated Casdoor subject")

	// ErrWalletNotOwnedByBuyer marks a signing request whose authenticated buyer
	// does not own the wallet being signed with.
	ErrWalletNotOwnedByBuyer = errors.New("privy: authenticated buyer does not own this wallet")

	// ErrBuyerWalletNotFound marks a bound buyer with no embedded wallet for the
	// requested rail's chain family yet (the client provisions it at login).
	ErrBuyerWalletNotFound = errors.New("privy: buyer has no embedded wallet for this rail")
)

// PrivyWallet is an embedded wallet linked to a Privy user.
type PrivyWallet struct {
	ID        string
	Address   string
	ChainType string // Privy chain_type: "ethereum", "solana", ...
}

// PrivyUser is the subset of a Privy user record the identity link consumes.
type PrivyUser struct {
	// DID is the user's Privy identifier ("did:privy:...").
	DID string
	// CustomAuthSubject is the `custom_user_id` of the user's linked custom-auth
	// account — the Casdoor `sub` when the custom auth provider fronts Casdoor.
	CustomAuthSubject string
	// Wallets are the user's embedded (privy-client) wallets.
	Wallets []PrivyWallet
}

// userDirectory reads Privy user records. The concrete restUserDirectory speaks
// Privy's REST API; tests substitute a fake. Keeping this a seam lets the
// security logic be proven without a live Privy app.
type userDirectory interface {
	// UserByDID resolves a user by their Privy DID.
	UserByDID(ctx context.Context, did string) (*PrivyUser, error)
	// UserByCustomAuthSubject resolves the user whose linked custom-auth account
	// has custom_user_id == subject (the Casdoor sub).
	UserByCustomAuthSubject(ctx context.Context, subject string) (*PrivyUser, error)
}

// IdentityVerifier proves the Casdoor->Privy trust chain server-side. It is
// constructed only when the provider is configured with a way to obtain the
// app's Privy access-token signing key (its JWKS endpoint, or a static
// verification key); otherwise the production paths stay fail-closed.
type IdentityVerifier struct {
	appID string
	// resolveKey returns the ES256 public key for a token's `kid` header. It is
	// backed by either the app JWKS endpoint (with rotation) or one static key.
	resolveKey func(kid string) (*ecdsa.PublicKey, error)
	dir        userDirectory
}

// newIdentityVerifier builds a verifier from the app id, the app's Privy access-
// token verification key (PEM-encoded ECDSA public key, from the Privy
// dashboard), and a user directory. This is the offline path: no key rotation,
// no network at verify time. Prefer newIdentityVerifierJWKS for real apps.
func newIdentityVerifier(appID, verificationKeyPEM string, dir userDirectory) (*IdentityVerifier, error) {
	if strings.TrimSpace(verificationKeyPEM) == "" {
		return nil, fmt.Errorf("%w: missing verification key", ErrIdentityNotConfigured)
	}
	key, err := parseECDSAPublicKeyPEM(verificationKeyPEM)
	if err != nil {
		return nil, err
	}
	return newIdentityVerifierWithResolver(appID, func(string) (*ecdsa.PublicKey, error) { return key, nil }, dir)
}

// newIdentityVerifierJWKS builds a verifier that fetches the app's ES256 signing
// keys from its Privy JWKS endpoint (e.g.
// https://auth.privy.io/api/v1/apps/{app_id}/jwks.json), caching them and
// selecting by the token's `kid`. This is the production path and tolerates key
// rotation.
func newIdentityVerifierJWKS(appID, jwksURL string, httpClient *http.Client, dir userDirectory) (*IdentityVerifier, error) {
	if strings.TrimSpace(jwksURL) == "" {
		return nil, fmt.Errorf("%w: missing JWKS url", ErrIdentityNotConfigured)
	}
	cache := newJWKSCache(jwksURL, httpClient)
	return newIdentityVerifierWithResolver(appID, cache.key, dir)
}

func newIdentityVerifierWithResolver(appID string, resolveKey func(kid string) (*ecdsa.PublicKey, error), dir userDirectory) (*IdentityVerifier, error) {
	if strings.TrimSpace(appID) == "" {
		return nil, fmt.Errorf("%w: missing app id", ErrIdentityNotConfigured)
	}
	if resolveKey == nil {
		return nil, fmt.Errorf("%w: missing key resolver", ErrIdentityNotConfigured)
	}
	if dir == nil {
		return nil, fmt.Errorf("%w: missing user directory", ErrIdentityNotConfigured)
	}
	return &IdentityVerifier{appID: appID, resolveKey: resolveKey, dir: dir}, nil
}

// verifiedToken carries the Privy DID proven by a valid access token.
type verifiedToken struct {
	DID string
}

// verifyAccessToken cryptographically verifies a Privy access token: ES256
// signature against the app verification key, issuer privy.io, audience equal
// to the app id, and a required, unexpired exp. It returns the token's subject
// (the Privy DID). It contacts no network — this is pure crypto.
func (v *IdentityVerifier) verifyAccessToken(token string) (verifiedToken, error) {
	if strings.TrimSpace(token) == "" {
		return verifiedToken{}, ErrAccessTokenInvalid
	}
	parsed, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodECDSA); !ok {
			return nil, fmt.Errorf("unexpected signing method %v", t.Header["alg"])
		}
		kid, _ := t.Header["kid"].(string)
		return v.resolveKey(kid)
	},
		jwt.WithValidMethods([]string{"ES256"}),
		jwt.WithIssuer(privyIssuer),
		jwt.WithAudience(v.appID),
		jwt.WithExpirationRequired(),
	)
	if err != nil || !parsed.Valid {
		return verifiedToken{}, fmt.Errorf("%w: %v", ErrAccessTokenInvalid, err)
	}
	sub, err := parsed.Claims.GetSubject()
	if err != nil || strings.TrimSpace(sub) == "" {
		return verifiedToken{}, fmt.Errorf("%w: missing subject", ErrAccessTokenInvalid)
	}
	return verifiedToken{DID: sub}, nil
}

// ResolveBuyerWallet returns the buyer's embedded wallet for a chain family,
// resolved from the Core-authenticated Casdoor subject through the custom-auth
// link. It signs nothing and creates nothing: the wallet is provisioned by the
// buyer's own client at custom-auth login; the server only resolves which
// address is theirs so it can be admitted as an escrow co-owner.
func (v *IdentityVerifier) ResolveBuyerWallet(ctx context.Context, casdoorSubject string, family contracts.ChainFamily) (PrivyWallet, error) {
	if strings.TrimSpace(casdoorSubject) == "" {
		return PrivyWallet{}, contracts.ErrEmbeddedWalletNoBuyerAuthorization
	}
	user, err := v.dir.UserByCustomAuthSubject(ctx, casdoorSubject)
	if err != nil {
		return PrivyWallet{}, err
	}
	// The directory query is by subject, but re-assert the binding on the
	// returned record: never trust that the lookup key round-tripped.
	if user == nil || user.CustomAuthSubject != casdoorSubject {
		return PrivyWallet{}, ErrBuyerBindingMismatch
	}
	w, ok := walletForFamily(user.Wallets, family)
	if !ok {
		return PrivyWallet{}, ErrBuyerWalletNotFound
	}
	return w, nil
}

// AuthorizeSigner verifies a buyer-supplied Privy access token and confirms the
// authenticated Privy user owns walletAddress (and, when casdoorSubject is
// non-empty, is bound to it). It returns the proven DID for the provider to
// pass to Privy's user-authorized signing call. This is the sign-time gate that
// makes a produced signature attributable to the admitted buyer.
func (v *IdentityVerifier) AuthorizeSigner(ctx context.Context, accessToken, casdoorSubject, walletAddress string) (string, error) {
	vt, err := v.verifyAccessToken(accessToken)
	if err != nil {
		return "", err
	}
	user, err := v.dir.UserByDID(ctx, vt.DID)
	if err != nil {
		return "", err
	}
	if user == nil {
		return "", fmt.Errorf("%w: token resolves to no user", ErrAccessTokenInvalid)
	}
	if strings.TrimSpace(casdoorSubject) != "" && user.CustomAuthSubject != casdoorSubject {
		return "", ErrBuyerBindingMismatch
	}
	if !ownsWallet(user.Wallets, walletAddress) {
		return "", ErrWalletNotOwnedByBuyer
	}
	return vt.DID, nil
}

// parseECDSAPublicKeyPEM parses a PEM-encoded PKIX ECDSA public key (the Privy
// app access-token verification key).
func parseECDSAPublicKeyPEM(pemStr string) (*ecdsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(strings.TrimSpace(pemStr)))
	if block == nil {
		return nil, fmt.Errorf("%w: verification key is not PEM", ErrIdentityNotConfigured)
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrIdentityNotConfigured, err)
	}
	key, ok := parsed.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("%w: verification key is not ECDSA", ErrIdentityNotConfigured)
	}
	return key, nil
}

// chainTypeForFamily maps a contract chain family to Privy's chain_type.
func chainTypeForFamily(family contracts.ChainFamily) string {
	switch family {
	case contracts.ChainFamilySolana:
		return "solana"
	default:
		return "ethereum"
	}
}

// walletForFamily returns the first embedded wallet matching the chain family.
func walletForFamily(wallets []PrivyWallet, family contracts.ChainFamily) (PrivyWallet, bool) {
	want := chainTypeForFamily(family)
	for _, w := range wallets {
		if strings.EqualFold(w.ChainType, want) {
			return w, true
		}
	}
	return PrivyWallet{}, false
}

// ownsWallet reports whether any of the user's wallets has the given address
// (case-insensitive; EVM addresses are hex, Privy returns EIP-55 mixed case).
func ownsWallet(wallets []PrivyWallet, address string) bool {
	address = strings.TrimSpace(address)
	if address == "" {
		return false
	}
	for _, w := range wallets {
		if strings.EqualFold(w.Address, address) {
			return true
		}
	}
	return false
}
