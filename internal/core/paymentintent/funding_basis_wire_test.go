// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package paymentintent

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mobazha/mobazha/pkg/models"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func paymentIntentFundingBasis(attempt models.PaymentAttempt) models.PaymentAttemptFundingBasis {
	return models.PaymentAttemptFundingBasis{
		Version: models.PaymentAttemptFundingBasisVersion, OrderID: attempt.OrderID, AttemptID: attempt.AttemptID,
		AuthorizationContextID: attempt.AuthorizationContextID,
		OrderOpenHash:          strings.Repeat("a", 64), PricingCurrency: "ETH", PricingAmount: attempt.AmountValue,
		PricingDivisibility: 18, PaymentAssetID: attempt.Currency, PaymentCurrency: "ETH", PaymentDivisibility: 18,
		ConversionRequired: false, ExchangeRate: "1000000000000000000", ExchangeRateBase: "ETH", ExchangeRateQuote: "ETH",
		ExchangeRateQuoteDivisibility: 18, RateSourceUpdatedUnix: 1784015970,
		RoundingPolicy: models.PaymentAttemptFundingRoundingCeilV1, PaymentSubtotal: attempt.AmountValue,
		ProviderOrNetworkCost: "0", PlatformPaymentCost: "0", BuyerPaymentTotal: attempt.AmountValue,
		QuoteID: "quote-wire", QuotePolicyVersion: models.PaymentSelectionQuotePilotZeroFeeV1,
		QuoteIssuer: "buyer-proposal-core", IssuedAtUnix: 1784016000, ExpiresAtUnix: 1784016900,
	}
}

func TestFundingBasisProposalWire_RoundTripsCanonicalSnapshot(t *testing.T) {
	attempt, _, _, _, _, _, _ := cryptoAttemptFixture(t)
	basis := paymentIntentFundingBasis(attempt)
	wire, err := FundingBasisProposalToProto(basis)
	require.NoError(t, err)
	roundTrip, err := FundingBasisProposalFromProto(wire)
	require.NoError(t, err)
	require.Equal(t, basis, roundTrip)

	wire.AttemptID = "another-attempt"
	_, err = FundingBasisProposalFromProto(wire)
	require.ErrorIs(t, err, models.ErrPaymentAttemptSettlementTermsConflict)
}

func TestRetainReceivedFundingBasisProposal_IsIdempotentAndImmutable(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:funding-basis-inbox-%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.PaymentAttemptFundingBasisProposalRecord{}))
	attempt, _, _, _, _, _, _ := cryptoAttemptFixture(t)
	basis := paymentIntentFundingBasis(attempt)

	for range 2 {
		require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
			return RetainReceivedFundingBasisProposalInTransaction(tx, "tenant-buyer", basis)
		}))
	}
	loaded, err := LoadRetainedFundingBasisProposal(db, "tenant-buyer", basis.AttemptID)
	require.NoError(t, err)
	require.Equal(t, basis, loaded)

	mutated := basis
	mutated.QuoteID = "different-quote"
	require.ErrorIs(t, db.Transaction(func(tx *gorm.DB) error {
		return RetainReceivedFundingBasisProposalInTransaction(tx, "tenant-buyer", mutated)
	}), models.ErrPaymentAttemptSettlementTermsConflict)
}

func TestRetainReceivedSettlementAuthorization_RejectsV1V2Downgrade(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:settlement-authorization-downgrade-%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.PaymentAttempt{}, &models.PaymentRouteBinding{}, &models.PaymentAttemptSettlementOffer{},
		&models.PaymentAttemptSettlementAuthorizationRecord{},
	))
	attempt, route, terms, _, _, bundle, target := cryptoAttemptFixture(t)
	attempt, err = CreateCryptoPaymentAttemptDraft(db, attempt, route)
	require.NoError(t, err)
	basis := paymentIntentFundingBasis(attempt)
	_, err = BindCryptoPaymentAttemptFundingBasis(db, attempt.TenantID, attempt.AttemptID, basis)
	require.NoError(t, err)

	v1 := models.PaymentAttemptSettlementAuthorization{
		Version: models.SettlementAuthorizationVersion, Terms: terms, Target: target, Authorization: bundle,
	}
	require.ErrorIs(t, db.Transaction(func(tx *gorm.DB) error {
		return RetainReceivedSettlementAuthorizationInTransaction(tx, attempt.TenantID, v1)
	}), models.ErrPaymentAttemptSettlementTermsConflict)
}
