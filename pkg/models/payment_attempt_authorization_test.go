// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package models

import (
	"crypto/rand"
	"strings"
	"testing"

	libp2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
)

func TestNewSettlementAuthorizationContextID_IsRandomCanonical32Bytes(t *testing.T) {
	first, err := NewSettlementAuthorizationContextID()
	require.NoError(t, err)
	second, err := NewSettlementAuthorizationContextID()
	require.NoError(t, err)
	require.Len(t, first, 64)
	require.NotEqual(t, first, second)
	require.True(t, validSettlementAuthorizationContextID(first))
}

func TestSettlementKeyOffer_VerifiesIdentityBoundScope(t *testing.T) {
	contextID, err := NewSettlementAuthorizationContextID()
	require.NoError(t, err)
	privateKey, publicKey, err := libp2pcrypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	participant, err := peer.IDFromPublicKey(publicKey)
	require.NoError(t, err)
	offer := SettlementKeyOffer{
		Version: SettlementAuthorizationVersion, AuthorizationContextID: contextID,
		OrderID: "order-1", AttemptID: "attempt-1", ParticipantPeerID: participant.String(),
		ParticipantRole: SettlementParticipantSeller, RailID: "crypto:eip155:1:native",
		Purpose: "standard-order-participant:seller", PublicKey: []byte("settlement-public-key"),
	}
	payload, err := offer.SigningPayload()
	require.NoError(t, err)
	offer.Signature, err = privateKey.Sign(payload)
	require.NoError(t, err)
	require.NoError(t, offer.Verify())

	tampered := offer
	tampered.RailID = "crypto:eip155:137:native"
	require.Error(t, tampered.Verify())
}

func TestSettlementKeyOffer_BindsEd25519AlgorithmToSolanaRail(t *testing.T) {
	contextID, err := NewSettlementAuthorizationContextID()
	require.NoError(t, err)
	privateKey, publicKey, err := libp2pcrypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	participant, err := peer.IDFromPublicKey(publicKey)
	require.NoError(t, err)
	offer := SettlementKeyOffer{
		Version: SettlementAuthorizationVersion, AuthorizationContextID: contextID,
		OrderID: "order-solana", AttemptID: "attempt-solana", ParticipantPeerID: participant.String(),
		ParticipantRole: SettlementParticipantBuyer, RailID: "crypto:solana:mainnet:native",
		Purpose: "standard-order-participant:buyer", KeyAlgorithm: SettlementKeyAlgorithmEd25519,
		PublicKey: make([]byte, 32), AmountAtomic: "1000", EscrowTimeoutHours: 72, EscrowUnlockUnix: 2000000000,
		BuyerRefundAddress: "34HNCS4M9qWZftHMyMd5MAwxpgfCbvMvbAvRSNZyQpsv",
	}
	offer.PublicKey[0] = 1
	payload, err := offer.SigningPayload()
	require.NoError(t, err)
	offer.Signature, err = privateKey.Sign(payload)
	require.NoError(t, err)
	require.NoError(t, offer.Verify())

	wrongRail := offer
	wrongRail.RailID = "crypto:eip155:1:native"
	require.Error(t, wrongRail.Verify())
	wrongAlgorithm := offer
	wrongAlgorithm.KeyAlgorithm = SettlementKeyAlgorithmSecp256k1
	require.Error(t, wrongAlgorithm.Verify())
}

func TestSettlementKeyOffer_AcceptsOnlyCanonicalResolvableCryptoAssetRails(t *testing.T) {
	contextID, err := NewSettlementAuthorizationContextID()
	require.NoError(t, err)
	_, publicKey, err := libp2pcrypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	participant, err := peer.IDFromPublicKey(publicKey)
	require.NoError(t, err)
	offer := SettlementKeyOffer{
		Version: SettlementAuthorizationVersion, AuthorizationContextID: contextID,
		OrderID: "order-token", AttemptID: "attempt-token", ParticipantPeerID: participant.String(),
		ParticipantRole: SettlementParticipantBuyer,
		RailID:          "crypto:eip155:1:erc20:0x1111111111111111111111111111111111111111",
		Purpose:         "standard-order-participant:buyer",
		KeyAlgorithm:    SettlementKeyAlgorithmSecp256k1,
		PublicKey:       append([]byte{2}, make([]byte, 32)...),
	}
	_, err = offer.SigningPayload()
	require.NoError(t, err)

	for name, railID := range map[string]string{
		"non-canonical case": "CRYPTO:eip155:1:erc20:0x1111111111111111111111111111111111111111",
		"invalid contract":   "crypto:eip155:1:erc20:not-an-address",
		"unknown network":    "crypto:eip155:999999:erc20:0x1111111111111111111111111111111111111111",
	} {
		t.Run(name, func(t *testing.T) {
			invalid := offer
			invalid.RailID = railID
			_, err := invalid.SigningPayload()
			require.ErrorContains(t, err, "canonical crypto asset ID")
		})
	}
}

func TestPaymentAttemptAuthorizationBundle_CanonicalizesAndRequiresCompleteOffers(t *testing.T) {
	contextID, err := NewSettlementAuthorizationContextID()
	require.NoError(t, err)
	privateKey, publicKey, err := libp2pcrypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	seller, err := peer.IDFromPublicKey(publicKey)
	require.NoError(t, err)
	buyerPrivateKey, buyerPublicKey, err := libp2pcrypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	buyer, err := peer.IDFromPublicKey(buyerPublicKey)
	require.NoError(t, err)
	offer := SettlementKeyOffer{
		Version: SettlementAuthorizationVersion, AuthorizationContextID: contextID,
		OrderID: "order-1", AttemptID: "attempt-1", ParticipantPeerID: seller.String(),
		ParticipantRole: SettlementParticipantSeller, RailID: "crypto:eip155:1:native",
		Purpose: "standard-order-participant:seller", PublicKey: []byte("settlement-public-key"),
	}
	payload, err := offer.SigningPayload()
	require.NoError(t, err)
	offer.Signature, err = privateKey.Sign(payload)
	require.NoError(t, err)
	buyerOffer := SettlementKeyOffer{
		Version: SettlementAuthorizationVersion, AuthorizationContextID: contextID,
		OrderID: "order-1", AttemptID: "attempt-1", ParticipantPeerID: buyer.String(),
		ParticipantRole: SettlementParticipantBuyer, RailID: "crypto:eip155:1:native",
		Purpose: "standard-order-participant:buyer", PublicKey: []byte("buyer-settlement-public-key"),
	}
	buyerPayload, err := buyerOffer.SigningPayload()
	require.NoError(t, err)
	buyerOffer.Signature, err = buyerPrivateKey.Sign(buyerPayload)
	require.NoError(t, err)
	bundle := PaymentAttemptAuthorizationBundle{
		Version: SettlementAuthorizationVersion, AuthorizationContextID: contextID,
		OrderID: "order-1", AttemptID: "attempt-1", RailID: "crypto:eip155:1:native",
		SettlementTermsHash: strings.Repeat("a", 64), FundingTargetHash: strings.Repeat("b", 64),
		RequiredRoles: []SettlementParticipantRole{SettlementParticipantBuyer, SettlementParticipantSeller},
		Offers:        []SettlementKeyOffer{buyerOffer, offer}, SellerTermsSigner: seller.String(),
		SellerTermsSignature: []byte("seller-terms-signature"),
	}
	first, firstHash, err := bundle.CanonicalBytesAndHash()
	require.NoError(t, err)
	second, secondHash, err := bundle.CanonicalBytesAndHash()
	require.NoError(t, err)
	require.Equal(t, first, second)
	require.Equal(t, firstHash, secondHash)
	reversedRoles := bundle
	reversedRoles.RequiredRoles = []SettlementParticipantRole{SettlementParticipantSeller, SettlementParticipantBuyer}
	_, reversedHash, err := reversedRoles.CanonicalBytesAndHash()
	require.NoError(t, err)
	require.Equal(t, firstHash, reversedHash)

	duplicateKey := bundle
	duplicateKey.Offers = append([]SettlementKeyOffer(nil), bundle.Offers...)
	duplicateKey.Offers[1].PublicKey = append([]byte(nil), duplicateKey.Offers[0].PublicKey...)
	duplicatePayload, err := duplicateKey.Offers[1].SigningPayload()
	require.NoError(t, err)
	duplicateKey.Offers[1].Signature, err = privateKey.Sign(duplicatePayload)
	require.NoError(t, err)
	require.ErrorContains(t, duplicateKey.Validate(), "public key is reused")

	mismatchedSeller := bundle
	mismatchedSeller.SellerTermsSigner = buyer.String()
	require.ErrorContains(t, mismatchedSeller.Validate(), "seller signer does not match seller offer")

	bundle.RequiredRoles = append(bundle.RequiredRoles, SettlementParticipantModerator)
	require.Error(t, bundle.Validate())
}

func TestPaymentAttemptSettlementAuthorization_RejectsSellerMutationOfModeratorFacts(t *testing.T) {
	contextID, err := NewSettlementAuthorizationContextID()
	require.NoError(t, err)
	roles := []SettlementParticipantRole{
		SettlementParticipantBuyer, SettlementParticipantSeller, SettlementParticipantModerator,
	}
	privateKeys := make([]libp2pcrypto.PrivKey, 0, len(roles))
	peerIDs := make([]peer.ID, 0, len(roles))
	for range roles {
		privateKey, publicKey, err := libp2pcrypto.GenerateEd25519Key(rand.Reader)
		require.NoError(t, err)
		peerID, err := peer.IDFromPublicKey(publicKey)
		require.NoError(t, err)
		privateKeys = append(privateKeys, privateKey)
		peerIDs = append(peerIDs, peerID)
	}
	const (
		orderID         = "order-moderated-bindings"
		attemptID       = "attempt-moderated-bindings"
		railID          = "crypto:bip122:000000000019d6689c085ae165831e93:native"
		fundingAmount   = "100000"
		moderatorAddr   = "bc1qmoderatoroffer000000000000000000000000"
		moderatorAmount = "100"
		timeoutHours    = uint32(72)
	)
	offers := make([]SettlementKeyOffer, 0, len(roles))
	for i, role := range roles {
		offer := SettlementKeyOffer{
			Version: SettlementAuthorizationVersion, AuthorizationContextID: contextID,
			OrderID: orderID, AttemptID: attemptID, ParticipantPeerID: peerIDs[i].String(),
			ParticipantRole: role, RailID: railID, Purpose: "standard-order-participant:" + string(role),
			PublicKey:               []byte("settlement-key-" + string(role)),
			ExpectedModeratorPeerID: peerIDs[2].String(), AmountAtomic: fundingAmount,
			EscrowTimeoutHours: timeoutHours,
		}
		if role == SettlementParticipantModerator {
			offer.ModeratorPayoutAddress = moderatorAddr
			offer.ModeratorFeeAmount = moderatorAmount
		}
		payload, err := offer.SigningPayload()
		require.NoError(t, err)
		offer.Signature, err = privateKeys[i].Sign(payload)
		require.NoError(t, err)
		offers = append(offers, offer)
	}
	terms := PaymentAttemptSettlementTerms{
		Version: PaymentAttemptSettlementTermsVersion, OrderID: orderID, AttemptID: attemptID,
		AssetID: railID, FundingAmount: fundingAmount, FundingTargetAddress: "bc1qfundingtarget0000000000000000000000000",
		RouteBindingID: "route-moderated-bindings", BuyerPeerID: peerIDs[0].String(), SellerPeerID: peerIDs[1].String(),
		ModeratorPeerID: peerIDs[2].String(), ModeratorFee: &PaymentAttemptSettlementFee{Address: moderatorAddr, Amount: moderatorAmount},
		EscrowTimeoutHours: timeoutHours, SellerAddress: "bc1qsellerpayout00000000000000000000000000",
		SellerGrossBasis: fundingAmount, PlatformReleaseFee: PaymentAttemptSettlementFee{Amount: "0"},
		BuyerCancellationFee: PaymentAttemptSettlementFee{Amount: "0"}, DisputePolicy: DisputeScalingSellerAwardProRataFloor,
	}
	target := PaymentAttemptFundingTarget{
		Version: PaymentAttemptFundingTargetVersion, AttemptID: attemptID, Type: PaymentAttemptFundingTargetAddress,
		AssetID: railID, AmountAtomic: fundingAmount, Address: terms.FundingTargetAddress, RedeemScriptHex: "51",
	}
	_, targetHash, err := target.CanonicalBytesAndHash()
	require.NoError(t, err)

	buildAuthorization := func(t *testing.T, candidate PaymentAttemptSettlementTerms) PaymentAttemptSettlementAuthorization {
		t.Helper()
		payload, err := candidate.SellerSigningPayload()
		require.NoError(t, err)
		signature, err := privateKeys[1].Sign(payload)
		require.NoError(t, err)
		_, termsHash, err := candidate.CanonicalBytesAndHash()
		require.NoError(t, err)
		return PaymentAttemptSettlementAuthorization{
			Version: SettlementAuthorizationVersion, Terms: candidate, Target: target,
			Authorization: PaymentAttemptAuthorizationBundle{
				Version: SettlementAuthorizationVersion, AuthorizationContextID: contextID,
				OrderID: orderID, AttemptID: attemptID, RailID: railID,
				SettlementTermsHash: termsHash, FundingTargetHash: targetHash,
				RequiredRoles: []SettlementParticipantRole{SettlementParticipantBuyer, SettlementParticipantSeller, SettlementParticipantModerator},
				Offers:        offers, SellerTermsSigner: peerIDs[1].String(), SellerTermsSignature: signature,
			},
		}
	}
	require.NoError(t, buildAuthorization(t, terms).Validate())

	tests := []struct {
		name   string
		mutate func(*PaymentAttemptSettlementTerms)
	}{
		{name: "payout address", mutate: func(candidate *PaymentAttemptSettlementTerms) {
			candidate.ModeratorFee = &PaymentAttemptSettlementFee{Address: candidate.SellerAddress, Amount: moderatorAmount}
		}},
		{name: "fee amount", mutate: func(candidate *PaymentAttemptSettlementTerms) {
			candidate.ModeratorFee = &PaymentAttemptSettlementFee{Address: moderatorAddr, Amount: "1"}
		}},
		{name: "escrow timeout", mutate: func(candidate *PaymentAttemptSettlementTerms) {
			candidate.EscrowTimeoutHours = 48
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := terms
			test.mutate(&candidate)
			err := buildAuthorization(t, candidate).Validate()
			require.ErrorIs(t, err, ErrPaymentAttemptSettlementTermsConflict)
		})
	}
}
