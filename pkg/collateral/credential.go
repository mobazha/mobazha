// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package collateral

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	libp2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

const (
	AllocationCredentialVersionV1 = "1"
	MaxAllocationCredentialTTL    = 15 * time.Minute
)

// AllocationCredential is a seller-Core signed, audience-bound projection of
// one active allocation. It is transport evidence, not a buyer-local account.
type AllocationCredential struct {
	CredentialID         string              `json:"credentialID"`
	Version              string              `json:"version"`
	IssuerPeerID         string              `json:"issuerPeerID"`
	AudiencePeerID       string              `json:"audiencePeerID"`
	PolicyID             string              `json:"policyID"`
	PolicyVersion        string              `json:"policyVersion"`
	ExtensionRevision    uint64              `json:"extensionRevision"`
	ExtensionDigest      string              `json:"extensionDigest"`
	AccountExpiresAtUnix int64               `json:"accountExpiresAtUnix"`
	IssuedAtUnix         int64               `json:"issuedAtUnix"`
	ExpiresAtUnix        int64               `json:"expiresAtUnix"`
	Allocation           AllocationReference `json:"allocation"`
	IssuerPublicKey      []byte              `json:"issuerPublicKey"`
	Signature            []byte              `json:"signature"`
}

type allocationCredentialBody struct {
	CredentialID         string              `json:"credentialID"`
	Version              string              `json:"version"`
	IssuerPeerID         string              `json:"issuerPeerID"`
	AudiencePeerID       string              `json:"audiencePeerID"`
	PolicyID             string              `json:"policyID"`
	PolicyVersion        string              `json:"policyVersion"`
	ExtensionRevision    uint64              `json:"extensionRevision"`
	ExtensionDigest      string              `json:"extensionDigest"`
	AccountExpiresAtUnix int64               `json:"accountExpiresAtUnix"`
	IssuedAtUnix         int64               `json:"issuedAtUnix"`
	ExpiresAtUnix        int64               `json:"expiresAtUnix"`
	Allocation           AllocationReference `json:"allocation"`
	IssuerPublicKey      []byte              `json:"issuerPublicKey"`
}

func (c AllocationCredential) SignableBytes() ([]byte, error) {
	body := allocationCredentialBody{
		CredentialID: c.CredentialID, Version: c.Version, IssuerPeerID: c.IssuerPeerID,
		AudiencePeerID: c.AudiencePeerID, PolicyID: c.PolicyID, PolicyVersion: c.PolicyVersion,
		ExtensionRevision: c.ExtensionRevision, ExtensionDigest: c.ExtensionDigest, AccountExpiresAtUnix: c.AccountExpiresAtUnix,
		IssuedAtUnix: c.IssuedAtUnix, ExpiresAtUnix: c.ExpiresAtUnix,
		Allocation: c.Allocation, IssuerPublicKey: append([]byte(nil), c.IssuerPublicKey...),
	}
	return json.Marshal(body)
}

// Verify validates identity, audience, freshness, scope, and signature.
func (c AllocationCredential) Verify(expectedAudience string, now time.Time) error {
	for _, value := range []string{c.CredentialID, c.Version, c.IssuerPeerID, c.AudiencePeerID, c.PolicyID, c.PolicyVersion, c.ExtensionDigest} {
		if value == "" || value != strings.TrimSpace(value) {
			return fmt.Errorf("collateral allocation credential identity, parties, and policy are required and canonical")
		}
	}
	if c.Version != AllocationCredentialVersionV1 || c.ExtensionRevision == 0 {
		return fmt.Errorf("collateral allocation credential version or extension revision is unsupported")
	}
	digestHex := strings.TrimPrefix(c.ExtensionDigest, "sha256:")
	digest, err := hex.DecodeString(digestHex)
	if len(c.ExtensionDigest) != len("sha256:")+64 || len(digest) != 32 || err != nil || hex.EncodeToString(digest) != digestHex {
		return fmt.Errorf("collateral allocation credential extension digest is invalid")
	}
	if expectedAudience == "" || c.AudiencePeerID != strings.TrimSpace(expectedAudience) {
		return fmt.Errorf("collateral allocation credential audience mismatch")
	}
	if err := c.Allocation.Validate(); err != nil {
		return fmt.Errorf("collateral allocation credential: %w", err)
	}
	if c.Allocation.State != AllocationActive || c.IssuerPeerID != c.Allocation.PrincipalID {
		return fmt.Errorf("collateral allocation credential issuer or allocation state mismatch")
	}
	issuedAt := time.Unix(c.IssuedAtUnix, 0).UTC()
	expiresAt := time.Unix(c.ExpiresAtUnix, 0).UTC()
	accountExpiresAt := time.Unix(c.AccountExpiresAtUnix, 0).UTC()
	if c.IssuedAtUnix <= 0 || !expiresAt.After(issuedAt) || expiresAt.Sub(issuedAt) > MaxAllocationCredentialTTL ||
		expiresAt.After(accountExpiresAt) || now.Before(issuedAt.Add(-time.Minute)) || !now.Before(expiresAt) {
		return fmt.Errorf("collateral allocation credential is not fresh or exceeds account expiry")
	}
	if len(c.IssuerPublicKey) != ed25519.PublicKeySize || len(c.Signature) != ed25519.SignatureSize {
		return fmt.Errorf("collateral allocation credential public key or signature size is invalid")
	}
	publicKey, err := libp2pcrypto.UnmarshalEd25519PublicKey(c.IssuerPublicKey)
	if err != nil {
		return fmt.Errorf("decode collateral allocation credential public key: %w", err)
	}
	issuerID, err := peer.IDFromPublicKey(publicKey)
	if err != nil || issuerID.String() != c.IssuerPeerID {
		return fmt.Errorf("collateral allocation credential public key does not match issuer")
	}
	signable, err := c.SignableBytes()
	if err != nil {
		return err
	}
	verified, err := publicKey.Verify(signable, c.Signature)
	if err != nil || !verified {
		return fmt.Errorf("collateral allocation credential signature is invalid")
	}
	return nil
}
