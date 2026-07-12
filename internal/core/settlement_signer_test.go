// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package core

import (
	"strings"
	"testing"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	btcecdsa "github.com/btcsuite/btcd/btcec/v2/ecdsa"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mobazha/mobazha/internal/core/guest"
	"github.com/mobazha/mobazha/pkg/contracts"
)

func TestLocalSettlementSigner_DeterministicAndStableContextSeparated(t *testing.T) {
	root := settlementTestPrivateKey(t, 1)
	signer := newLocalSettlementSigner(newFileKeyProvider(nil, root, nil, nil, nil))
	ref := contracts.SettlementKeyRef{
		TenantID: "tenant-a", RailID: "ETH", Purpose: "guest-managed-escrow-owner", ReferenceID: "owner-v1",
	}

	first, err := signer.PublicKey(t.Context(), ref)
	require.NoError(t, err)
	second, err := signer.PublicKey(t.Context(), ref)
	require.NoError(t, err)
	assert.Equal(t, first, second)

	otherTenant := ref
	otherTenant.TenantID = "tenant-b"
	other, err := signer.PublicKey(t.Context(), otherTenant)
	require.NoError(t, err)
	assert.Equal(t, first, other)
	otherRail := ref
	otherRail.RailID = "BSC"
	other, err = signer.PublicKey(t.Context(), otherRail)
	require.NoError(t, err)
	assert.NotEqual(t, first, other)

	buyerRole := ref
	buyerRole.Purpose = "standard-order-participant:buyer"
	buyerKey, err := signer.PublicKey(t.Context(), buyerRole)
	require.NoError(t, err)
	sellerRole := ref
	sellerRole.Purpose = "standard-order-participant:seller"
	sellerKey, err := signer.PublicKey(t.Context(), sellerRole)
	require.NoError(t, err)
	assert.NotEqual(t, buyerKey, sellerKey)
}

func TestLocalSettlementSigner_TenantIDDoesNotAffectKeyOrSignatureDigest(t *testing.T) {
	signer := newLocalSettlementSigner(newFileKeyProvider(nil, settlementTestPrivateKey(t, 1), nil, nil, nil))
	request := contracts.SettlementSignRequest{
		KeyRef: contracts.SettlementKeyRef{
			TenantID: "tenant-a", RailID: "ETH", Purpose: "safe-owner", ReferenceID: "order-1",
		},
		Domain: "safe-execute-v1", OrderID: "order-1", AttemptID: "attempt-1", Action: "safe-execute",
		TermsHash: strings.Repeat("a", 64), Payload: []byte("canonical transaction plan"),
	}

	firstPublicKey, err := signer.PublicKey(t.Context(), request.KeyRef)
	require.NoError(t, err)
	firstDigest := settlementSignatureDigest(request)

	otherTenant := request
	otherTenant.KeyRef.TenantID = "tenant-b"
	secondPublicKey, err := signer.PublicKey(t.Context(), otherTenant.KeyRef)
	require.NoError(t, err)
	secondDigest := settlementSignatureDigest(otherTenant)

	assert.Equal(t, firstPublicKey, secondPublicKey)
	assert.Equal(t, firstDigest, secondDigest)
}

func TestLocalSettlementSigner_DifferentRootsProduceDifferentPublicKeys(t *testing.T) {
	ref := contracts.SettlementKeyRef{
		TenantID: "tenant-a", RailID: "ETH", Purpose: "safe-owner", ReferenceID: "order-1",
	}
	firstSigner := newLocalSettlementSigner(newFileKeyProvider(nil, settlementTestPrivateKey(t, 1), nil, nil, nil))
	secondSigner := newLocalSettlementSigner(newFileKeyProvider(nil, settlementTestPrivateKey(t, 2), nil, nil, nil))

	firstPublicKey, err := firstSigner.PublicKey(t.Context(), ref)
	require.NoError(t, err)
	secondPublicKey, err := secondSigner.PublicKey(t.Context(), ref)
	require.NoError(t, err)

	assert.NotEqual(t, firstPublicKey, secondPublicKey)
}

func TestLocalSettlementSigner_SignatureBindsDomainPayloadAndReference(t *testing.T) {
	root := settlementTestPrivateKey(t, 1)
	signer := newLocalSettlementSigner(newFileKeyProvider(nil, root, nil, nil, nil))
	request := contracts.SettlementSignRequest{
		KeyRef: contracts.SettlementKeyRef{
			TenantID: "tenant-a", RailID: "ETH", Purpose: "safe-owner", ReferenceID: "order-1",
		},
		Domain: "safe-execute-v1", OrderID: "order-1", AttemptID: "attempt-1", Action: "safe-execute",
		TermsHash: strings.Repeat("a", 64), Payload: []byte("canonical transaction plan"),
	}
	publicKey, err := signer.PublicKey(t.Context(), request.KeyRef)
	require.NoError(t, err)
	signature, err := signer.Sign(t.Context(), request)
	require.NoError(t, err)
	parsedKey, err := btcec.ParsePubKey(publicKey)
	require.NoError(t, err)
	parsedSignature, err := btcecdsa.ParseDERSignature(signature)
	require.NoError(t, err)
	digest := settlementSignatureDigest(request)
	assert.True(t, parsedSignature.Verify(digest[:], parsedKey))

	tampered := request
	tampered.Payload = []byte("different plan")
	tamperedDigest := settlementSignatureDigest(tampered)
	assert.False(t, parsedSignature.Verify(tamperedDigest[:], parsedKey))
	tampered = request
	tampered.AttemptID = "attempt-2"
	tamperedDigest = settlementSignatureDigest(tampered)
	assert.False(t, parsedSignature.Verify(tamperedDigest[:], parsedKey))
}

func TestGuestManagedEscrowOwner_UsesSettlementSignerNotEVMProfileKey(t *testing.T) {
	profileKey := settlementTestPrivateKey(t, 1)
	settlementRoot := settlementTestPrivateKey(t, 2)
	signer := newLocalSettlementSigner(newFileKeyProvider(profileKey, settlementRoot, nil, nil, nil))
	resolver := &guest.NodeEVMSellerOwnerResolver{Signer: signer, TenantID: "tenant-a"}

	owner, err := resolver.SellerEVMOwnerAddress(t.Context(), "ETH")
	require.NoError(t, err)
	profileOwner := crypto.PubkeyToAddress(*profileKey.PubKey().ToECDSA())
	assert.NotEqual(t, profileOwner, owner)

	ownerRetry, err := resolver.SellerEVMOwnerAddress(t.Context(), "ETH")
	require.NoError(t, err)
	assert.Equal(t, owner, ownerRetry)
	otherRailOwner, err := resolver.SellerEVMOwnerAddress(t.Context(), "BSC")
	require.NoError(t, err)
	assert.NotEqual(t, owner, otherRailOwner)
}

func settlementTestPrivateKey(t *testing.T, seed byte) *btcec.PrivateKey {
	t.Helper()
	keyBytes := make([]byte, 32)
	keyBytes[len(keyBytes)-1] = seed
	key, _ := btcec.PrivKeyFromBytes(keyBytes)
	require.NotNil(t, key)
	return key
}
