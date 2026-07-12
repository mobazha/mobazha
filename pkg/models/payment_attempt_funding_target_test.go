// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package models

import (
	"crypto/rand"
	"testing"

	libp2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
)

func signedSettlementAttempt(t *testing.T) (*PaymentAttempt, PaymentAttemptSettlementTerms) {
	t.Helper()
	privateKey, publicKey, err := libp2pcrypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	sellerPeerID, err := peer.IDFromPublicKey(publicKey)
	require.NoError(t, err)
	buyerPrivateKey, buyerPublicKey, err := libp2pcrypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	buyerPeerID, err := peer.IDFromPublicKey(buyerPublicKey)
	require.NoError(t, err)

	attempt := &PaymentAttempt{
		AttemptID: "attempt-1", OrderID: "order-1", Kind: PaymentAttemptKindCryptoFundingTarget,
		PaymentSessionID: "ps_order-1", Currency: "crypto:eip155:1:native", RouteBindingID: "route-1", State: PaymentAttemptPendingExternal,
	}
	terms := validPaymentAttemptSettlementTerms()
	terms.BuyerPeerID = buyerPeerID.String()
	terms.SellerPeerID = sellerPeerID.String()
	terms.Affiliate.BuyerPeerID = buyerPeerID.String()
	terms.FundingTargetAddress = "0xescrow"
	require.NoError(t, attempt.SetSettlementTerms(terms))
	payload, err := terms.SellerSigningPayload()
	require.NoError(t, err)
	signature, err := privateKey.Sign(payload)
	require.NoError(t, err)
	require.NoError(t, attempt.SetSellerTermsAuthorization(sellerPeerID.String(), signature))
	target := PaymentAttemptFundingTarget{
		Version: PaymentAttemptFundingTargetVersion, AttemptID: attempt.AttemptID,
		Type: PaymentAttemptFundingTargetAddress, AssetID: terms.AssetID,
		AmountAtomic: terms.FundingAmount, Address: "0xescrow",
	}
	_, targetHash, err := target.CanonicalBytesAndHash()
	require.NoError(t, err)
	contextID, err := NewSettlementAuthorizationContextID()
	require.NoError(t, err)
	require.NoError(t, attempt.SetAuthorizationContextID(contextID))
	offer := SettlementKeyOffer{
		Version: SettlementAuthorizationVersion, AuthorizationContextID: contextID,
		OrderID: attempt.OrderID, AttemptID: attempt.AttemptID, ParticipantPeerID: sellerPeerID.String(),
		ParticipantRole: SettlementParticipantSeller, RailID: terms.AssetID,
		Purpose: "standard-order-participant:seller", PublicKey: []byte("seller-settlement-public-key"),
	}
	offerPayload, err := offer.SigningPayload()
	require.NoError(t, err)
	offer.Signature, err = privateKey.Sign(offerPayload)
	require.NoError(t, err)
	buyerOffer := SettlementKeyOffer{
		Version: SettlementAuthorizationVersion, AuthorizationContextID: contextID,
		OrderID: attempt.OrderID, AttemptID: attempt.AttemptID, ParticipantPeerID: buyerPeerID.String(),
		ParticipantRole: SettlementParticipantBuyer, RailID: terms.AssetID,
		Purpose: "standard-order-participant:buyer", PublicKey: []byte("buyer-settlement-public-key"),
	}
	buyerOfferPayload, err := buyerOffer.SigningPayload()
	require.NoError(t, err)
	buyerOffer.Signature, err = buyerPrivateKey.Sign(buyerOfferPayload)
	require.NoError(t, err)
	_, termsHash, err := terms.CanonicalBytesAndHash()
	require.NoError(t, err)
	require.NoError(t, attempt.SetAuthorizationBundle(PaymentAttemptAuthorizationBundle{
		Version: SettlementAuthorizationVersion, AuthorizationContextID: contextID,
		OrderID: attempt.OrderID, AttemptID: attempt.AttemptID, RailID: terms.AssetID,
		SettlementTermsHash: termsHash, FundingTargetHash: targetHash,
		RequiredRoles: []SettlementParticipantRole{SettlementParticipantBuyer, SettlementParticipantSeller},
		Offers:        []SettlementKeyOffer{buyerOffer, offer}, SellerTermsSigner: sellerPeerID.String(),
		SellerTermsSignature: signature,
	}))
	return attempt, terms
}

func TestPaymentAttempt_SetSellerTermsAuthorizationRejectsWrongSigner(t *testing.T) {
	attempt, terms := signedSettlementAttempt(t)
	otherPrivateKey, otherPublicKey, err := libp2pcrypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	otherPeerID, err := peer.IDFromPublicKey(otherPublicKey)
	require.NoError(t, err)
	payload, err := terms.SellerSigningPayload()
	require.NoError(t, err)
	otherSignature, err := otherPrivateKey.Sign(payload)
	require.NoError(t, err)

	attempt.SellerTermsSigner = ""
	attempt.SellerTermsSignature = nil
	require.Error(t, attempt.SetSellerTermsAuthorization(otherPeerID.String(), otherSignature))
}

func TestPaymentAttempt_SetFundingTargetRequiresVerifiedTerms(t *testing.T) {
	attempt := &PaymentAttempt{AttemptID: "attempt-1", OrderID: "order-1"}
	target := PaymentAttemptFundingTarget{
		Version: PaymentAttemptFundingTargetVersion, AttemptID: "attempt-1",
		Type: PaymentAttemptFundingTargetAddress, AssetID: "crypto:eip155:1:native",
		AmountAtomic: "1000", Address: "0xescrow",
	}
	require.Error(t, attempt.SetFundingTarget(target))
}

func TestPaymentAttempt_SetFundingTargetIsImmutableAndBoundToTerms(t *testing.T) {
	attempt, terms := signedSettlementAttempt(t)
	target := PaymentAttemptFundingTarget{
		Version: PaymentAttemptFundingTargetVersion, AttemptID: attempt.AttemptID,
		Type: PaymentAttemptFundingTargetAddress, AssetID: terms.AssetID,
		AmountAtomic: terms.FundingAmount, Address: "0xescrow",
	}
	require.NoError(t, attempt.SetFundingTarget(target))
	require.NoError(t, attempt.SetFundingTarget(target))
	require.Equal(t, PaymentAttemptFundingTargetReady, attempt.State)

	stored, err := attempt.GetFundingTarget()
	require.NoError(t, err)
	require.Equal(t, target, *stored)

	changed := target
	changed.Address = "0xdifferent"
	require.ErrorIs(t, attempt.SetFundingTarget(changed), ErrPaymentAttemptSettlementTermsConflict)

	wrongAsset := target
	wrongAsset.AssetID = "crypto:eip155:1/erc20:0x1"
	require.Error(t, (&PaymentAttempt{
		AttemptID: attempt.AttemptID, OrderID: attempt.OrderID,
		SettlementTerms: attempt.SettlementTerms, SettlementTermsHash: attempt.SettlementTermsHash,
		SellerTermsSigner: attempt.SellerTermsSigner, SellerTermsSignature: attempt.SellerTermsSignature,
	}).SetFundingTarget(wrongAsset))
}

func TestPaymentAttempt_GetFundingTargetRejectsTampering(t *testing.T) {
	attempt, terms := signedSettlementAttempt(t)
	require.NoError(t, attempt.SetFundingTarget(PaymentAttemptFundingTarget{
		Version: PaymentAttemptFundingTargetVersion, AttemptID: attempt.AttemptID,
		Type: PaymentAttemptFundingTargetAddress, AssetID: terms.AssetID,
		AmountAtomic: terms.FundingAmount, Address: "0xescrow",
	}))
	attempt.FundingTarget = append([]byte(nil), attempt.FundingTarget...)
	attempt.FundingTarget[len(attempt.FundingTarget)-2] = 'x'
	_, err := attempt.GetFundingTarget()
	require.Error(t, err)
}

func TestPaymentAttempt_GetAuthorizationBundleRejectsWrongAttemptContext(t *testing.T) {
	attempt, _ := signedSettlementAttempt(t)
	otherContextID, err := NewSettlementAuthorizationContextID()
	require.NoError(t, err)
	attempt.AuthorizationContextID = otherContextID
	_, err = attempt.GetAuthorizationBundle()
	require.Error(t, err)
}
