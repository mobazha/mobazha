// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package models

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func validPaymentAttemptSettlementTerms() PaymentAttemptSettlementTerms {
	return PaymentAttemptSettlementTerms{
		Version:              PaymentAttemptSettlementTermsVersion,
		OrderID:              "order-1",
		AttemptID:            "attempt-1",
		AssetID:              "crypto:eip155:1:native",
		FundingAmount:        "1000",
		FundingTargetAddress: "0x4444444444444444444444444444444444444444",
		RouteBindingID:       "route-1",
		BuyerPeerID:          "12D3KooWLSei5eJ8o8mWoS8SsEj5ymL93kFYvNgHA4PpdVhhZyuu",
		SellerPeerID:         "12D3KooWD1GpGf11qVtcDhat8q8rB2du9nohFEFu2DgciUYWY2BC",
		SellerAddress:        "0x1111111111111111111111111111111111111111",
		SellerGrossBasis:     "1000",
		PlatformReleaseFee: PaymentAttemptSettlementFee{
			Address: "0x2222222222222222222222222222222222222222", Amount: "10",
		},
		BuyerCancellationFee: PaymentAttemptSettlementFee{
			Address: "0x2222222222222222222222222222222222222222", Amount: "10",
		},
		Affiliate: &PaymentAttemptAffiliateTerm{
			ReferralSessionID: "referral-1", ProgramID: "program-1",
			PromoterPeerID:    "12D3KooWSsoZBMiQjvPctdqckrAGukta3q7kAZS7cQRwfwbet7zG",
			BuyerPeerID:       "12D3KooWLSei5eJ8o8mWoS8SsEj5ymL93kFYvNgHA4PpdVhhZyuu",
			CommissionRateBPS: 1000, Address: "0x3333333333333333333333333333333333333333",
			Amount: "100", SellerGrossBasis: "1000",
			Lines: []PaymentAttemptAffiliateLineTerm{{
				OrderLineID: "order-1:0", NetMerchandiseAtomic: "1000", CommissionAtomic: "100",
			}},
		},
		DisputePolicy: DisputeScalingSellerAwardProRataFloor,
	}
}

func TestPaymentAttemptSettlementTerms_CanonicalHashIsStable(t *testing.T) {
	terms := validPaymentAttemptSettlementTerms()
	first, firstHash, err := terms.CanonicalBytesAndHash()
	require.NoError(t, err)
	second, secondHash, err := terms.CanonicalBytesAndHash()
	require.NoError(t, err)
	require.Equal(t, first, second)
	require.Equal(t, firstHash, secondHash)
	require.Len(t, firstHash, 64)
}

func TestPaymentAttemptSettlementTerms_PreservesZeroAffiliateAllocation(t *testing.T) {
	terms := validPaymentAttemptSettlementTerms()
	terms.Affiliate.Amount = "0"
	terms.Affiliate.SellerGrossBasis = "1"
	terms.Affiliate.Lines = []PaymentAttemptAffiliateLineTerm{{
		OrderLineID: "order-1:0", NetMerchandiseAtomic: "1", CommissionAtomic: "0",
	}}
	require.NoError(t, terms.Validate())
}

func TestPaymentAttempt_SetSettlementTermsIsImmutable(t *testing.T) {
	attempt := PaymentAttempt{AttemptID: "attempt-1", OrderID: "order-1"}
	terms := validPaymentAttemptSettlementTerms()
	require.NoError(t, attempt.SetSettlementTerms(terms))
	require.NoError(t, attempt.SetSettlementTerms(terms))

	changed := terms
	changed.Affiliate = &PaymentAttemptAffiliateTerm{}
	*changed.Affiliate = *terms.Affiliate
	changed.Affiliate.Address = "0x4444444444444444444444444444444444444444"
	require.ErrorIs(t, attempt.SetSettlementTerms(changed), ErrPaymentAttemptSettlementTermsConflict)
}

func TestPaymentAttempt_SetSettlementTermsRejectsPartialStoredState(t *testing.T) {
	terms := validPaymentAttemptSettlementTerms()
	canonical, hash, err := terms.CanonicalBytesAndHash()
	require.NoError(t, err)

	missingHash := PaymentAttempt{AttemptID: "attempt-1", OrderID: "order-1", SettlementTerms: canonical}
	require.ErrorIs(t, missingHash.SetSettlementTerms(terms), ErrPaymentAttemptSettlementTermsConflict)

	missingTerms := PaymentAttempt{AttemptID: "attempt-1", OrderID: "order-1", SettlementTermsHash: hash}
	require.ErrorIs(t, missingTerms.SetSettlementTerms(terms), ErrPaymentAttemptSettlementTermsConflict)
}

func TestPaymentAttempt_SetSettlementTermsRequiresSelectedModerator(t *testing.T) {
	terms := validPaymentAttemptSettlementTerms()
	terms.ModeratorPeerID = terms.Affiliate.PromoterPeerID
	attempt := PaymentAttempt{AttemptID: terms.AttemptID, OrderID: terms.OrderID, Kind: PaymentAttemptKindCryptoFundingTarget}
	require.ErrorIs(t, attempt.SetSettlementTerms(terms), ErrPaymentAttemptSettlementTermsConflict)

	attempt.ExpectedModeratorPeerID = terms.ModeratorPeerID
	require.NoError(t, attempt.SetSettlementTerms(terms))
}

func TestPaymentAttempt_GetSettlementTermsRejectsTampering(t *testing.T) {
	attempt := PaymentAttempt{AttemptID: "attempt-1", OrderID: "order-1"}
	require.NoError(t, attempt.SetSettlementTerms(validPaymentAttemptSettlementTerms()))
	attempt.SettlementTerms = append([]byte(nil), attempt.SettlementTerms...)
	attempt.SettlementTerms[len(attempt.SettlementTerms)-2] = 'x'
	_, err := attempt.GetSettlementTerms()
	require.Error(t, err)
}

func TestPaymentAttempt_GetSettlementTermsRejectsAnotherAttemptTerms(t *testing.T) {
	attempt := PaymentAttempt{AttemptID: "attempt-1", OrderID: "order-1"}
	require.NoError(t, attempt.SetSettlementTerms(validPaymentAttemptSettlementTerms()))
	attempt.AttemptID = "attempt-2"
	_, err := attempt.GetSettlementTerms()
	require.ErrorIs(t, err, ErrPaymentAttemptSettlementTermsConflict)
}

func TestPaymentAttemptSettlementTerms_PersistRoundTrip(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:payment-attempt-terms-%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&PaymentAttempt{}))

	attempt := PaymentAttempt{
		TenantID: "tenant-1", AttemptID: "attempt-1", Kind: PaymentAttemptKindProviderSession,
		PaymentSessionID: "ps_order-1", OrderID: "order-1", RouteBindingID: "route-1",
		IdempotencyKey: "payment-attempt-terms-round-trip", State: PaymentAttemptPendingExternal,
		SellerTermsSignature: []byte("seller-signature"), PlatformTermsSignature: []byte("platform-signature"),
	}
	require.NoError(t, attempt.SetSettlementTerms(validPaymentAttemptSettlementTerms()))
	require.NoError(t, db.Create(&attempt).Error)

	var stored PaymentAttempt
	require.NoError(t, db.Where("tenant_id = ? AND attempt_id = ?", attempt.TenantID, attempt.AttemptID).First(&stored).Error)
	terms, err := stored.GetSettlementTerms()
	require.NoError(t, err)
	require.Equal(t, attempt.SettlementTermsHash, stored.SettlementTermsHash)
	require.Equal(t, attempt.SellerTermsSignature, stored.SellerTermsSignature)
	require.Equal(t, attempt.PlatformTermsSignature, stored.PlatformTermsSignature)
	require.Equal(t, validPaymentAttemptSettlementTerms(), *terms)
}

func TestPaymentAttempt_AuthorizationContextUniqueOnlyWhenPresent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:payment-attempt-context-%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&PaymentAttempt{}))

	firstWithoutContext := PaymentAttempt{TenantID: "tenant-1", AttemptID: "attempt-1", IdempotencyKey: "idempotency-1"}
	secondWithoutContext := PaymentAttempt{TenantID: "tenant-1", AttemptID: "attempt-2", IdempotencyKey: "idempotency-2"}
	require.NoError(t, db.Create(&firstWithoutContext).Error)
	require.NoError(t, db.Create(&secondWithoutContext).Error)

	contextID, err := NewSettlementAuthorizationContextID()
	require.NoError(t, err)
	firstWithContext := PaymentAttempt{TenantID: "tenant-1", AttemptID: "attempt-3", IdempotencyKey: "idempotency-3", AuthorizationContextID: contextID}
	secondWithContext := PaymentAttempt{TenantID: "tenant-1", AttemptID: "attempt-4", IdempotencyKey: "idempotency-4", AuthorizationContextID: contextID}
	require.NoError(t, db.Create(&firstWithContext).Error)
	require.Error(t, db.Create(&secondWithContext).Error)
}

func TestPaymentAttemptSettlementTerms_RejectsNonCanonicalOrUnsafeAmounts(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*PaymentAttemptSettlementTerms)
	}{
		{name: "leading zero", mutate: func(terms *PaymentAttemptSettlementTerms) { terms.FundingAmount = "01000" }},
		{name: "deductions consume seller gross", mutate: func(terms *PaymentAttemptSettlementTerms) { terms.PlatformReleaseFee.Amount = "900" }},
		{name: "positive fee without address", mutate: func(terms *PaymentAttemptSettlementTerms) { terms.BuyerCancellationFee.Address = "" }},
		{name: "cancel fee consumes funding", mutate: func(terms *PaymentAttemptSettlementTerms) { terms.BuyerCancellationFee.Amount = "1000" }},
		{name: "affiliate line rate mismatch", mutate: func(terms *PaymentAttemptSettlementTerms) { terms.Affiliate.Lines[0].CommissionAtomic = "99" }},
		{name: "affiliate line basis mismatch", mutate: func(terms *PaymentAttemptSettlementTerms) { terms.Affiliate.Lines[0].NetMerchandiseAtomic = "999" }},
		{name: "buyer equals seller", mutate: func(terms *PaymentAttemptSettlementTerms) { terms.BuyerPeerID = terms.SellerPeerID }},
		{name: "moderator equals seller", mutate: func(terms *PaymentAttemptSettlementTerms) { terms.ModeratorPeerID = terms.SellerPeerID }},
		{name: "affiliate buyer mismatch", mutate: func(terms *PaymentAttemptSettlementTerms) { terms.Affiliate.BuyerPeerID = terms.SellerPeerID }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			terms := validPaymentAttemptSettlementTerms()
			test.mutate(&terms)
			require.Error(t, terms.Validate())
		})
	}
}
