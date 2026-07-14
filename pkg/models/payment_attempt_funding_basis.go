// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package models

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

const (
	// PaymentAttemptFundingBasisVersion is the first quote-bound funding-basis
	// encoding. It is versioned independently from settlement authorization v1.
	PaymentAttemptFundingBasisVersion = 1
	// PaymentAttemptFundingRoundingCeilV1 prevents a conversion from
	// underfunding the signed pricing amount by a fractional payment atomic unit.
	PaymentAttemptFundingRoundingCeilV1 = "ceil_to_payment_atomic_v1"
)

// PaymentAttemptFundingBasis is the immutable, seller-authorizable derivation
// of one attempt's payment amount from the signed order pricing amount.
// SettlementKeyOffer deliberately does not contain these economic facts.
type PaymentAttemptFundingBasis struct {
	Version                       uint32 `json:"version"`
	OrderID                       string `json:"orderID"`
	AttemptID                     string `json:"attemptID"`
	AuthorizationContextID        string `json:"authorizationContextID"`
	OrderOpenHash                 string `json:"orderOpenHash"`
	PricingCurrency               string `json:"pricingCurrency"`
	PricingAmount                 string `json:"pricingAmount"`
	PricingDivisibility           uint   `json:"pricingDivisibility"`
	PaymentAssetID                string `json:"paymentAssetID"`
	PaymentCurrency               string `json:"paymentCurrency"`
	PaymentDivisibility           uint   `json:"paymentDivisibility"`
	ConversionRequired            bool   `json:"conversionRequired"`
	ExchangeRate                  string `json:"exchangeRate"`
	ExchangeRateBase              string `json:"exchangeRateBase"`
	ExchangeRateQuote             string `json:"exchangeRateQuote"`
	ExchangeRateQuoteDivisibility uint   `json:"exchangeRateQuoteDivisibility"`
	RateSourceUpdatedUnix         int64  `json:"rateSourceUpdatedUnix"`
	RoundingPolicy                string `json:"roundingPolicy"`
	PaymentSubtotal               string `json:"paymentSubtotal"`
	ProviderOrNetworkCost         string `json:"providerOrNetworkCost"`
	PlatformPaymentCost           string `json:"platformPaymentCost"`
	BuyerPaymentTotal             string `json:"buyerPaymentTotal"`
	QuoteID                       string `json:"quoteID"`
	QuotePolicyVersion            string `json:"quotePolicyVersion"`
	QuoteIssuer                   string `json:"quoteIssuer"`
	IssuedAtUnix                  int64  `json:"issuedAtUnix"`
	ExpiresAtUnix                 int64  `json:"expiresAtUnix"`
}

// CanonicalBytesAndHash validates and encodes the funding basis before
// computing its SHA-256 commitment.
func (b PaymentAttemptFundingBasis) CanonicalBytesAndHash() ([]byte, string, error) {
	if err := b.Validate(); err != nil {
		return nil, "", err
	}
	canonical, err := json.Marshal(b)
	if err != nil {
		return nil, "", fmt.Errorf("encode payment attempt funding basis: %w", err)
	}
	digest := sha256.Sum256(canonical)
	return canonical, hex.EncodeToString(digest[:]), nil
}

// Validate verifies identity, currency orientation, exact integer totals, and
// quote lifetime without consulting mutable exchange-rate state.
func (b PaymentAttemptFundingBasis) Validate() error {
	if b.Version != PaymentAttemptFundingBasisVersion ||
		strings.TrimSpace(b.OrderID) == "" || strings.TrimSpace(b.AttemptID) == "" ||
		!validSettlementAuthorizationContextID(b.AuthorizationContextID) ||
		strings.TrimSpace(b.QuoteID) == "" || strings.TrimSpace(b.QuotePolicyVersion) == "" ||
		strings.TrimSpace(b.QuoteIssuer) == "" {
		return fmt.Errorf("invalid payment attempt funding basis identity")
	}
	orderOpenHash := strings.TrimSpace(b.OrderOpenHash)
	if !validCanonicalSHA256Hex(orderOpenHash) {
		return fmt.Errorf("invalid signed order-open hash")
	}
	coin := iwallet.CoinType(strings.TrimSpace(b.PaymentAssetID))
	if err := coin.ValidateCanonicalPaymentCoin(); err != nil {
		return fmt.Errorf("invalid payment asset ID: %w", err)
	}
	paymentCode, err := coin.PricingCurrencyCode()
	if err != nil || paymentCode != b.PaymentCurrency {
		return fmt.Errorf("payment currency does not match payment asset")
	}
	pricingCurrency, err := CurrencyDefinitions.Lookup(strings.TrimSpace(b.PricingCurrency))
	if err != nil || pricingCurrency.Code.String() != b.PricingCurrency || pricingCurrency.Divisibility != b.PricingDivisibility {
		return fmt.Errorf("invalid pricing currency definition")
	}
	paymentCurrency, err := CurrencyDefinitions.Lookup(strings.TrimSpace(b.PaymentCurrency))
	if err != nil || paymentCurrency.Code.String() != b.PaymentCurrency || paymentCurrency.Divisibility != b.PaymentDivisibility {
		return fmt.Errorf("invalid payment currency definition")
	}
	conversionRequired := b.PricingCurrency != b.PaymentCurrency
	if b.ConversionRequired != conversionRequired || b.ExchangeRateBase != b.PaymentCurrency ||
		b.ExchangeRateQuote != b.PricingCurrency || b.ExchangeRateQuoteDivisibility != b.PricingDivisibility {
		return fmt.Errorf("invalid funding-basis conversion orientation")
	}
	if b.RoundingPolicy != PaymentAttemptFundingRoundingCeilV1 {
		return fmt.Errorf("invalid funding-basis rounding policy")
	}
	pricingAmount, err := settlementAtomicAmount(b.PricingAmount, true)
	if err != nil {
		return fmt.Errorf("invalid pricing amount: %w", err)
	}
	rate, err := settlementAtomicAmount(b.ExchangeRate, true)
	if err != nil {
		return fmt.Errorf("invalid exchange rate: %w", err)
	}
	subtotal, err := settlementAtomicAmount(b.PaymentSubtotal, true)
	if err != nil {
		return fmt.Errorf("invalid payment subtotal: %w", err)
	}
	providerCost, err := settlementAtomicAmount(b.ProviderOrNetworkCost, false)
	if err != nil {
		return fmt.Errorf("invalid provider or network cost: %w", err)
	}
	platformCost, err := settlementAtomicAmount(b.PlatformPaymentCost, false)
	if err != nil {
		return fmt.Errorf("invalid platform payment cost: %w", err)
	}
	total, err := settlementAtomicAmount(b.BuyerPaymentTotal, true)
	if err != nil {
		return fmt.Errorf("invalid buyer payment total: %w", err)
	}
	wantTotal := new(big.Int).Add(new(big.Int).Set(subtotal), providerCost)
	wantTotal.Add(wantTotal, platformCost)
	if wantTotal.Cmp(total) != 0 {
		return fmt.Errorf("buyer payment total does not reconcile")
	}
	pricingScale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(b.PricingDivisibility)), nil)
	if b.ConversionRequired {
		paymentScale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(b.PaymentDivisibility)), nil)
		numerator := new(big.Int).Mul(pricingAmount, paymentScale)
		wantSubtotal, remainder := new(big.Int), new(big.Int)
		wantSubtotal.QuoRem(numerator, rate, remainder)
		if remainder.Sign() > 0 {
			wantSubtotal.Add(wantSubtotal, big.NewInt(1))
		}
		if wantSubtotal.Cmp(subtotal) != 0 {
			return fmt.Errorf("payment subtotal does not match quoted conversion")
		}
	} else if rate.Cmp(pricingScale) != 0 || pricingAmount.Cmp(subtotal) != 0 {
		return fmt.Errorf("same-currency funding basis does not reconcile")
	}
	if b.RateSourceUpdatedUnix <= 0 || b.IssuedAtUnix <= 0 || b.ExpiresAtUnix <= b.IssuedAtUnix ||
		b.RateSourceUpdatedUnix > b.IssuedAtUnix {
		return fmt.Errorf("invalid funding-basis quote lifetime")
	}
	return nil
}

func validCanonicalSHA256Hex(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size && value == strings.ToLower(value)
}
