// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package core

import (
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/models"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"gorm.io/gorm"
)

const maxSellerSettlementRateAge = 15 * time.Minute

func buildBuyerPaymentAttemptFundingBasis(
	db *gorm.DB,
	order *models.Order,
	orderOpen *pb.OrderOpen,
	attempt models.PaymentAttempt,
	quoteID, buyerPeerID string,
) (models.PaymentAttemptFundingBasis, error) {
	if db == nil || order == nil || orderOpen == nil || strings.TrimSpace(quoteID) == "" {
		return models.PaymentAttemptFundingBasis{}, fmt.Errorf("quote-bound settlement authorization requires a payment-selection quote")
	}
	var quote models.PaymentSelectionQuote
	if err := db.Where(
		"tenant_id = ? AND quote_id = ? AND order_id = ?", strings.TrimSpace(order.TenantID), quoteID, order.ID.String(),
	).First(&quote).Error; err != nil {
		return models.PaymentAttemptFundingBasis{}, fmt.Errorf("load quote-bound settlement funding basis: %w", err)
	}
	orderHash, err := order.OrderOpenCanonicalHash()
	if err != nil {
		return models.PaymentAttemptFundingBasis{}, fmt.Errorf("hash signed order open: %w", err)
	}
	basis := models.PaymentAttemptFundingBasis{
		Version: models.PaymentAttemptFundingBasisVersion, OrderID: order.ID.String(), AttemptID: attempt.AttemptID,
		AuthorizationContextID: attempt.AuthorizationContextID,
		OrderOpenHash:          orderHash, PricingCurrency: quote.PricingCurrency, PricingAmount: quote.PricingAmount,
		PricingDivisibility: quote.PricingDivisibility, PaymentAssetID: quote.PaymentCoin,
		PaymentCurrency: quote.PaymentCurrency, PaymentDivisibility: quote.PaymentDivisibility,
		ConversionRequired: quote.ConversionRequired, ExchangeRate: quote.ExchangeRate,
		ExchangeRateBase: quote.ExchangeRateBase, ExchangeRateQuote: quote.ExchangeRateQuote,
		ExchangeRateQuoteDivisibility: quote.ExchangeRateQuoteDivisibility,
		RateSourceUpdatedUnix:         quote.RateSourceUpdatedAt.UTC().Unix(),
		RoundingPolicy:                models.PaymentAttemptFundingRoundingCeilV1,
		PaymentSubtotal:               quote.PaymentSubtotal, ProviderOrNetworkCost: quote.ProviderOrNetworkCost,
		PlatformPaymentCost: quote.PlatformPaymentCost, BuyerPaymentTotal: quote.BuyerPaymentTotal,
		QuoteID: quote.QuoteID, QuotePolicyVersion: quote.PolicyVersion, QuoteIssuer: buyerPeerID,
		IssuedAtUnix: quote.CreatedAt.UTC().Unix(), ExpiresAtUnix: quote.ExpiresAt.UTC().Unix(),
	}
	if quote.PaymentCoin != attempt.Currency || quote.BuyerPaymentTotal != attempt.AmountValue ||
		quote.PricingCurrency != strings.ToUpper(strings.TrimSpace(orderOpen.PricingCoin)) ||
		quote.PricingAmount != strings.TrimSpace(orderOpen.Amount) || quote.QuoteID != strings.TrimSpace(quoteID) {
		return models.PaymentAttemptFundingBasis{}, models.ErrPaymentAttemptSettlementTermsConflict
	}
	if err := basis.Validate(); err != nil {
		return models.PaymentAttemptFundingBasis{}, err
	}
	return basis, nil
}

func validateSellerPaymentAttemptFundingBasis(
	basis models.PaymentAttemptFundingBasis,
	order *models.Order,
	orderOpen *pb.OrderOpen,
	buyerPeerID string,
	exchange contracts.ExchangeRateService,
	now time.Time,
) error {
	if err := basis.Validate(); err != nil {
		return err
	}
	if order == nil || orderOpen == nil || basis.OrderID != order.ID.String() || basis.QuoteIssuer != buyerPeerID ||
		basis.PricingCurrency != strings.ToUpper(strings.TrimSpace(orderOpen.PricingCoin)) ||
		basis.PricingAmount != strings.TrimSpace(orderOpen.Amount) || basis.ExpiresAtUnix <= now.UTC().Unix() {
		return models.ErrPaymentAttemptSettlementTermsConflict
	}
	orderHash, err := order.OrderOpenCanonicalHash()
	if err != nil || basis.OrderOpenHash != orderHash {
		return models.ErrPaymentAttemptSettlementTermsConflict
	}
	if basis.QuotePolicyVersion != models.PaymentSelectionQuotePilotZeroFeeV1 ||
		basis.ProviderOrNetworkCost != "0" || basis.PlatformPaymentCost != "0" ||
		basis.PaymentSubtotal != basis.BuyerPaymentTotal {
		return fmt.Errorf("seller quote policy does not admit proposed payment costs")
	}
	if !basis.ConversionRequired {
		return nil
	}
	if exchange == nil {
		return fmt.Errorf("seller exchange-rate policy is unavailable")
	}
	rate, err := exchange.GetRate(
		models.CurrencyCode(basis.PaymentCurrency), models.CurrencyCode(basis.PricingCurrency), true,
	)
	if err != nil {
		return fmt.Errorf("load seller exchange-rate policy: %w", err)
	}
	rateInt := big.Int(rate)
	if rateInt.Sign() <= 0 {
		return fmt.Errorf("seller exchange-rate policy returned a non-positive rate")
	}
	rateUpdatedAt := exchange.LastUpdated(models.CurrencyCode(basis.PaymentCurrency)).UTC()
	// GetRate above is allowed to refresh the seller's provider cache. Compare
	// its source timestamp with a clock sample taken after that refresh, while
	// retaining an injected future `now` used by deterministic callers/tests.
	rateCheckedAt := time.Now().UTC()
	if requestedAt := now.UTC(); requestedAt.After(rateCheckedAt) {
		rateCheckedAt = requestedAt
	}
	if basis.ExpiresAtUnix <= rateCheckedAt.Unix() {
		return models.ErrPaymentAttemptSettlementTermsConflict
	}
	if rateUpdatedAt.IsZero() || rateUpdatedAt.After(rateCheckedAt) || rateCheckedAt.Sub(rateUpdatedAt) >= maxSellerSettlementRateAge {
		return fmt.Errorf("seller exchange-rate policy snapshot is stale")
	}
	pricingAmount, ok := new(big.Int).SetString(basis.PricingAmount, 10)
	if !ok {
		return models.ErrPaymentAttemptSettlementTermsConflict
	}
	paymentScale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(basis.PaymentDivisibility)), nil)
	numerator := new(big.Int).Mul(pricingAmount, paymentScale)
	minimum, remainder := new(big.Int), new(big.Int)
	minimum.QuoRem(numerator, &rateInt, remainder)
	if remainder.Sign() > 0 {
		minimum.Add(minimum, big.NewInt(1))
	}
	proposed, ok := new(big.Int).SetString(basis.PaymentSubtotal, 10)
	if !ok || proposed.Cmp(minimum) < 0 {
		return fmt.Errorf("buyer funding proposal is below seller exchange-rate policy minimum")
	}
	return nil
}
