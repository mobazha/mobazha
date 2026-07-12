// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package paymentintent

import (
	"crypto/rand"
	"fmt"
	"testing"
	"time"

	libp2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/mobazha/mobazha/pkg/models"
)

func cryptoAttemptFixture(t *testing.T) (
	models.PaymentAttempt,
	models.PaymentRouteBinding,
	models.PaymentAttemptSettlementTerms,
	string,
	[]byte,
	models.PaymentAttemptFundingTarget,
) {
	t.Helper()
	privateKey, publicKey, err := libp2pcrypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	sellerPeerID, err := peer.IDFromPublicKey(publicKey)
	require.NoError(t, err)

	attempt := models.PaymentAttempt{
		AttemptID: "attempt-crypto-1", Kind: models.PaymentAttemptKindCryptoFundingTarget,
		PaymentSessionID: "ps_order-1", OrderID: "order-1", AmountValue: "1000",
		Currency: "crypto:eip155:1/native", RouteBindingID: "route-crypto-1",
		IdempotencyKey: "order-1:crypto:eip155:1:native", State: models.PaymentAttemptPendingExternal,
	}
	route := models.PaymentRouteBinding{
		RouteBindingID: "route-crypto-1", AttemptID: "attempt-crypto-1",
		ContributionID: "builtin-managed-evm", ModuleID: "managed-evm",
		ImplementationGeneration: "safe-v1.4.1", RailKind: "crypto",
		NetworkID: "eip155:1", AssetID: "crypto:eip155:1/native",
		ProtocolVersion: "1", StateSchemaVersion: "1", CreatedAt: time.Now().UTC(),
	}
	terms := models.PaymentAttemptSettlementTerms{
		Version: models.PaymentAttemptSettlementTermsVersion, OrderID: attempt.OrderID,
		AttemptID: attempt.AttemptID, AssetID: route.AssetID, FundingAmount: "1000",
		FundingTargetAddress: "0x3333333333333333333333333333333333333333",
		RouteBindingID:       route.RouteBindingID, SellerPeerID: sellerPeerID.String(),
		SellerAddress: "0x1111111111111111111111111111111111111111", SellerGrossBasis: "1000",
		PlatformReleaseFee:   models.PaymentAttemptSettlementFee{Amount: "0"},
		BuyerCancellationFee: models.PaymentAttemptSettlementFee{Amount: "0"},
		Affiliate: &models.PaymentAttemptAffiliateTerm{
			ReferralSessionID: "referral-1", ProgramID: "program-1",
			PromoterPeerID:    "12D3KooWSsoZBMiQjvPctdqckrAGukta3q7kAZS7cQRwfwbet7zG",
			BuyerPeerID:       "12D3KooWLSei5eJ8o8mWoS8SsEj5ymL93kFYvNgHA4PpdVhhZyuu",
			CommissionRateBPS: 500, Address: "0x2222222222222222222222222222222222222222",
			Amount: "50", SellerGrossBasis: "1000",
			Lines: []models.PaymentAttemptAffiliateLineTerm{{
				OrderLineID: "order-1:0", NetMerchandiseAtomic: "1000", CommissionAtomic: "50",
			}},
		},
		DisputePolicy: models.DisputeScalingSellerAwardProRataFloor,
	}
	payload, err := terms.SellerSigningPayload()
	require.NoError(t, err)
	signature, err := privateKey.Sign(payload)
	require.NoError(t, err)
	target := models.PaymentAttemptFundingTarget{
		Version: models.PaymentAttemptFundingTargetVersion, AttemptID: attempt.AttemptID,
		Type: models.PaymentAttemptFundingTargetAddress, AssetID: terms.AssetID,
		AmountAtomic: terms.FundingAmount, Address: "0x3333333333333333333333333333333333333333",
	}
	return attempt, route, terms, sellerPeerID.String(), signature, target
}

func TestFreezeCryptoPaymentAttempt_PersistsAtomicSnapshotAndAcceptsRetry(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:crypto-attempt-%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.PaymentAttempt{}, &models.PaymentRouteBinding{}))
	attempt, route, terms, signer, signature, target := cryptoAttemptFixture(t)

	require.NoError(t, FreezeCryptoPaymentAttempt(db, attempt, route, terms, signer, signature, target))
	route.CreatedAt = route.CreatedAt.Add(time.Minute)
	require.NoError(t, FreezeCryptoPaymentAttempt(db, attempt, route, terms, signer, signature, target))

	var stored models.PaymentAttempt
	require.NoError(t, db.Where("tenant_id = ? AND attempt_id = ?", "", attempt.AttemptID).First(&stored).Error)
	require.Equal(t, models.PaymentAttemptFundingTargetReady, stored.State)
	storedTerms, err := stored.GetSettlementTerms()
	require.NoError(t, err)
	require.Equal(t, terms, *storedTerms)
	storedTarget, err := stored.GetFundingTarget()
	require.NoError(t, err)
	require.Equal(t, target, *storedTarget)
}

func TestFreezeCryptoPaymentAttempt_RejectsFrozenMutation(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:crypto-attempt-conflict-%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.PaymentAttempt{}, &models.PaymentRouteBinding{}))
	attempt, route, terms, signer, signature, target := cryptoAttemptFixture(t)
	require.NoError(t, FreezeCryptoPaymentAttempt(db, attempt, route, terms, signer, signature, target))

	target.Address = "0x4444444444444444444444444444444444444444"
	require.ErrorIs(
		t,
		FreezeCryptoPaymentAttempt(db, attempt, route, terms, signer, signature, target),
		models.ErrPaymentAttemptSettlementTermsConflict,
	)
}

func TestFreezeCryptoPaymentAttempt_RejectsTargetBeforeValidSellerAuthorization(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:crypto-attempt-auth-%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.PaymentAttempt{}, &models.PaymentRouteBinding{}))
	attempt, route, terms, signer, _, target := cryptoAttemptFixture(t)

	require.Error(t, FreezeCryptoPaymentAttempt(db, attempt, route, terms, signer, []byte("invalid"), target))
	var count int64
	require.NoError(t, db.Model(&models.PaymentAttempt{}).Count(&count).Error)
	require.Zero(t, count)
}
