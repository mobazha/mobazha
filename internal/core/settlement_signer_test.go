// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package core

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"strings"
	"testing"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	btcecdsa "github.com/btcsuite/btcd/btcec/v2/ecdsa"
	"github.com/ethereum/go-ethereum/crypto"
	libp2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mobazha/mobazha/internal/core/guest"
	"github.com/mobazha/mobazha/internal/core/paymentintent"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/models"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

type settlementSigningWallet struct {
	iwallet.Wallet
	capturedKey    []byte
	capturedScript []byte
	signatures     []iwallet.EscrowSignature
}

func (*settlementSigningWallet) EstimateEscrowFee(int, int, int, iwallet.FeeLevel) (iwallet.Amount, error) {
	return iwallet.NewAmount(1), nil
}

func (*settlementSigningWallet) CreateMultisigAddress(
	[]btcec.PublicKey, []byte, int,
) (iwallet.Address, []byte, error) {
	return iwallet.Address{}, nil, nil
}

func (w *settlementSigningWallet) SignMultisigTransaction(
	_ iwallet.Transaction,
	key btcec.PrivateKey,
	redeemScript []byte,
) ([]iwallet.EscrowSignature, error) {
	w.capturedKey = append([]byte(nil), key.Serialize()...)
	w.capturedScript = append([]byte(nil), redeemScript...)
	return w.signatures, nil
}

func (*settlementSigningWallet) BuildAndSend(
	iwallet.Tx,
	iwallet.Transaction,
	[][]iwallet.EscrowSignature,
	[]byte,
	iwallet.OrderFinishType,
) (iwallet.TransactionID, error) {
	return "", nil
}

type settlementSigningWalletOperator struct {
	contracts.WalletOperator
	wallet iwallet.Wallet
}

func (o settlementSigningWalletOperator) WalletForCurrencyCode(string) (iwallet.Wallet, error) {
	return o.wallet, nil
}

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

func TestLocalSettlementSigner_SignsUTXOMultisigWithoutExposingDerivedKey(t *testing.T) {
	wallet := &settlementSigningWallet{
		signatures: []iwallet.EscrowSignature{{Index: 0, Signature: []byte("chain-signature")}},
	}
	signer := newLocalSettlementSigner(
		newFileKeyProvider(nil, settlementTestPrivateKey(t, 1), nil, nil, nil),
		settlementSigningWalletOperator{wallet: wallet},
	)
	utxoSigner, ok := signer.(contracts.UTXOSettlementSigner)
	require.True(t, ok)
	keyRef := contracts.SettlementKeyRef{
		TenantID: "tenant-a", RailID: "BTC",
		Purpose: "standard-order-participant:buyer", ReferenceID: "authorization-context-1",
	}
	publicKey, err := signer.PublicKey(t.Context(), keyRef)
	require.NoError(t, err)
	request := contracts.UTXOMultisigSettlementSignRequest{
		KeyRef: keyRef, OrderID: "order-1", AttemptID: "attempt-1", Action: "cancel", Sequence: 1,
		TermsHash: strings.Repeat("a", 64), CoinCode: "BTC",
		Transaction: iwallet.Transaction{
			From: []iwallet.SpendInfo{{ID: []byte{1}}},
			To:   []iwallet.SpendInfo{{ID: []byte{2}}},
		},
		RedeemScript: []byte{3, 4, 5},
	}
	signatures, err := utxoSigner.SignUTXOMultisig(t.Context(), request)
	require.NoError(t, err)
	require.Equal(t, wallet.signatures, signatures)
	require.Equal(t, request.RedeemScript, wallet.capturedScript)
	derivedKey, _ := btcec.PrivKeyFromBytes(wallet.capturedKey)
	require.Equal(t, publicKey, derivedKey.PubKey().SerializeCompressed())
}

func TestLocalSettlementSigner_RejectsUTXOSigningWithoutWalletCapability(t *testing.T) {
	signer := newLocalSettlementSigner(newFileKeyProvider(nil, settlementTestPrivateKey(t, 1), nil, nil, nil))
	utxoSigner := signer.(contracts.UTXOSettlementSigner)
	_, err := utxoSigner.SignUTXOMultisig(context.Background(), contracts.UTXOMultisigSettlementSignRequest{})
	require.Error(t, err)
}

func TestLocalSettlementSigner_SignsRawEVMDigestWithAttemptKey(t *testing.T) {
	signer := newLocalSettlementSigner(newFileKeyProvider(nil, settlementTestPrivateKey(t, 1), nil, nil, nil))
	keyRef := contracts.SettlementKeyRef{
		TenantID: "tenant-a", RailID: "crypto:eip155:1:native",
		Purpose: "standard-order-participant:seller", ReferenceID: "authorization-context-1",
	}
	publicKey, err := signer.PublicKey(t.Context(), keyRef)
	require.NoError(t, err)
	parsedKey, err := btcec.ParsePubKey(publicKey)
	require.NoError(t, err)
	wantAddress := crypto.PubkeyToAddress(*parsedKey.ToECDSA())
	digest := [32]byte{1, 2, 3, 4}

	address, signature, err := signer.(contracts.EVMSettlementSigner).SignEVMDigest(
		t.Context(),
		contracts.EVMDigestSettlementSignRequest{
			KeyRef: keyRef, OrderID: "order-1", AttemptID: "attempt-1", Action: "complete",
			Sequence: 1, TermsHash: strings.Repeat("a", 64), ChainID: 1, Digest: digest,
		},
	)
	require.NoError(t, err)
	require.Equal(t, wantAddress, address)
	require.Len(t, signature, 65)
	recoverySignature := append([]byte(nil), signature...)
	recoverySignature[64] -= 27
	recovered, err := crypto.SigToPub(digest[:], recoverySignature)
	require.NoError(t, err)
	require.Equal(t, wantAddress, crypto.PubkeyToAddress(*recovered))
}

func TestLocalSettlementSigner_SignsSolanaMessageWithAttemptKey(t *testing.T) {
	signer := newLocalSettlementSigner(newFileKeyProvider(nil, settlementTestPrivateKey(t, 1), nil, nil, nil))
	keyRef := contracts.SettlementKeyRef{
		TenantID: "tenant-a", RailID: "crypto:solana:mainnet:native",
		Purpose: "standard-order-participant:moderator", ReferenceID: "authorization-context-solana",
	}
	publicKey, err := signer.(contracts.SolanaSettlementSigner).SolanaPublicKey(t.Context(), keyRef)
	require.NoError(t, err)
	require.Len(t, publicKey, ed25519.PublicKeySize)
	message := []byte("canonical anchor settlement message")
	signedPublicKey, signature, err := signer.(contracts.SolanaSettlementSigner).SignSolanaMessage(
		t.Context(), contracts.SolanaMessageSettlementSignRequest{
			KeyRef: keyRef, OrderID: "order-solana", AttemptID: "attempt-solana", Action: "dispute_release",
			Sequence: 1, TermsHash: strings.Repeat("a", 64),
			ProgramAddress: "11111111111111111111111111111112",
			EscrowAddress:  "11111111111111111111111111111113", Message: message,
		},
	)
	require.NoError(t, err)
	require.Equal(t, publicKey, signedPublicKey)
	require.True(t, ed25519.Verify(ed25519.PublicKey(publicKey), message, signature))
}

func TestIssueSettlementKeyOffer_SelectsSolanaEd25519Key(t *testing.T) {
	identityPrivateKey, _, err := libp2pcrypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	marshaled, err := libp2pcrypto.MarshalPrivateKey(identityPrivateKey)
	require.NoError(t, err)
	identitySigner, err := contracts.NewKeyPairSignerFromMarshaledKey(marshaled)
	require.NoError(t, err)
	settlementSigner := newLocalSettlementSigner(
		newFileKeyProvider(nil, settlementTestPrivateKey(t, 1), nil, nil, nil),
	)
	contextID := strings.Repeat("a", 64)
	offer, err := paymentintent.IssueSettlementKeyOffer(
		t.Context(), identitySigner, settlementSigner,
		contracts.SettlementKeyRef{
			TenantID: "tenant-solana", RailID: "crypto:solana:mainnet:native",
			Purpose: contracts.StandardOrderSettlementKeyPurpose, ReferenceID: contextID,
		},
		"order-solana", "attempt-solana", models.SettlementParticipantBuyer,
	)
	require.NoError(t, err)
	require.Equal(t, models.SettlementKeyAlgorithmEd25519, offer.KeyAlgorithm)
	require.Len(t, offer.PublicKey, ed25519.PublicKeySize)
	require.NoError(t, offer.Verify())
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
