// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package models

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func validCrossCurrencyFundingBasis() PaymentAttemptFundingBasis {
	return PaymentAttemptFundingBasis{
		Version: PaymentAttemptFundingBasisVersion, OrderID: "order-funding-basis", AttemptID: "attempt-funding-basis",
		AuthorizationContextID: strings.Repeat("b", 64),
		OrderOpenHash:          strings.Repeat("a", 64), PricingCurrency: "USD", PricingAmount: "4900", PricingDivisibility: 2,
		PaymentAssetID: "crypto:eip155:1:native", PaymentCurrency: "ETH", PaymentDivisibility: 18,
		ConversionRequired: true, ExchangeRate: "250000", ExchangeRateBase: "ETH", ExchangeRateQuote: "USD",
		ExchangeRateQuoteDivisibility: 2, RateSourceUpdatedUnix: 1784015970,
		RoundingPolicy: PaymentAttemptFundingRoundingCeilV1, PaymentSubtotal: "19600000000000000",
		ProviderOrNetworkCost: "0", PlatformPaymentCost: "0", BuyerPaymentTotal: "19600000000000000",
		QuoteID: "quote-funding-basis", QuotePolicyVersion: PaymentSelectionQuotePilotZeroFeeV1,
		QuoteIssuer: "buyer-proposal-core", IssuedAtUnix: 1784016000, ExpiresAtUnix: 1784016900,
	}
}

func TestPaymentAttemptFundingBasis_CanonicalHashIsStable(t *testing.T) {
	basis := validCrossCurrencyFundingBasis()
	first, firstHash, err := basis.CanonicalBytesAndHash()
	require.NoError(t, err)
	second, secondHash, err := basis.CanonicalBytesAndHash()
	require.NoError(t, err)
	require.Equal(t, first, second)
	require.Equal(t, firstHash, secondHash)
	require.Len(t, firstHash, 64)
}

func TestPaymentAttemptFundingBasis_RejectsEconomicMismatch(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*PaymentAttemptFundingBasis)
	}{
		{name: "wrong orientation", mutate: func(b *PaymentAttemptFundingBasis) { b.ExchangeRateBase = "USD" }},
		{name: "underfunded conversion", mutate: func(b *PaymentAttemptFundingBasis) { b.PaymentSubtotal = "19599999999999999" }},
		{name: "cost not in total", mutate: func(b *PaymentAttemptFundingBasis) { b.ProviderOrNetworkCost = "1" }},
		{name: "expired at issue", mutate: func(b *PaymentAttemptFundingBasis) { b.ExpiresAtUnix = b.IssuedAtUnix }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			basis := validCrossCurrencyFundingBasis()
			tt.mutate(&basis)
			require.Error(t, basis.Validate())
		})
	}
}

func TestPaymentAttemptFundingBasis_FreezesOnAttempt(t *testing.T) {
	basis := validCrossCurrencyFundingBasis()
	attempt := PaymentAttempt{
		Kind: PaymentAttemptKindCryptoFundingTarget, OrderID: basis.OrderID, AttemptID: basis.AttemptID,
		AuthorizationContextID: basis.AuthorizationContextID,
		Currency:               basis.PaymentAssetID, AmountValue: basis.BuyerPaymentTotal,
	}
	require.NoError(t, attempt.SetFundingBasis(basis))
	require.NoError(t, attempt.SetFundingBasis(basis), "byte-identical retry must be idempotent")
	stored, err := attempt.GetFundingBasis()
	require.NoError(t, err)
	require.Equal(t, basis, *stored)

	changed := basis
	changed.QuoteID = "different-quote"
	require.ErrorIs(t, attempt.SetFundingBasis(changed), ErrPaymentAttemptSettlementTermsConflict)

	attempt.FundingBasisHash = strings.Repeat("0", 64)
	_, err = attempt.GetFundingBasis()
	require.ErrorIs(t, err, ErrPaymentAttemptSettlementTermsConflict)
}

func TestPaymentAttemptFundingBasis_BindsQuoteBoundSettlementTerms(t *testing.T) {
	terms := validPaymentAttemptSettlementTerms()
	basis := validCrossCurrencyFundingBasis()
	basis.OrderID = terms.OrderID
	basis.AttemptID = terms.AttemptID
	basis.PricingCurrency = "ETH"
	basis.PricingAmount = terms.FundingAmount
	basis.PricingDivisibility = 18
	basis.ConversionRequired = false
	basis.ExchangeRate = "1000000000000000000"
	basis.ExchangeRateBase = "ETH"
	basis.ExchangeRateQuote = "ETH"
	basis.ExchangeRateQuoteDivisibility = 18
	basis.PaymentSubtotal = terms.FundingAmount
	basis.BuyerPaymentTotal = terms.FundingAmount

	attempt := PaymentAttempt{
		Kind: PaymentAttemptKindCryptoFundingTarget, OrderID: basis.OrderID, AttemptID: basis.AttemptID,
		AuthorizationContextID: basis.AuthorizationContextID,
		Currency:               basis.PaymentAssetID, AmountValue: basis.BuyerPaymentTotal,
	}
	require.NoError(t, attempt.SetFundingBasis(basis))
	terms.Version = PaymentAttemptSettlementTermsQuoteBoundVersion
	terms.FundingBasisHash = attempt.FundingBasisHash
	require.NoError(t, attempt.SetSettlementTerms(terms))

	stored, err := attempt.GetSettlementTerms()
	require.NoError(t, err)
	require.Equal(t, attempt.FundingBasisHash, stored.FundingBasisHash)

	tampered := terms
	tampered.FundingBasisHash = strings.Repeat("b", 64)
	require.ErrorIs(t, attempt.SetSettlementTerms(tampered), ErrPaymentAttemptSettlementTermsConflict)
}
