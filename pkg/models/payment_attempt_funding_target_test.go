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

	attempt := &PaymentAttempt{
		AttemptID: "attempt-1", OrderID: "order-1", Kind: PaymentAttemptKindCryptoFundingTarget,
		PaymentSessionID: "ps_order-1", RouteBindingID: "route-1", State: PaymentAttemptPendingExternal,
	}
	terms := validPaymentAttemptSettlementTerms()
	terms.SellerPeerID = sellerPeerID.String()
	terms.FundingTargetAddress = "0xescrow"
	require.NoError(t, attempt.SetSettlementTerms(terms))
	payload, err := terms.SellerSigningPayload()
	require.NoError(t, err)
	signature, err := privateKey.Sign(payload)
	require.NoError(t, err)
	require.NoError(t, attempt.SetSellerTermsAuthorization(sellerPeerID.String(), signature))
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
