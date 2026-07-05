package payment

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	porderpb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	paypb "github.com/mobazha/mobazha/pkg/payment"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

const (
	defaultPaymentSelectionQuoteTTL = 15 * time.Minute
	maxPaymentSelectionRateAge      = 15 * time.Minute
)

// CreateSelectionQuote creates or reuses an immutable Deal payment-selection
// quote without provisioning any external payment target.
func (s *PaymentSessionServiceImpl) CreateSelectionQuote(
	ctx context.Context,
	req contracts.CreatePaymentSelectionQuoteRequest,
) (*paypb.PaymentSelectionQuote, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("payment selection quote database is unavailable")
	}
	req.OrderID = strings.TrimSpace(req.OrderID)
	req.PaymentCoin = strings.TrimSpace(req.PaymentCoin)
	if req.OrderID == "" || req.PaymentCoin == "" {
		return nil, fmt.Errorf("%w: orderID and paymentCoin are required", ErrDealPaymentSelectionQuoteInvalid)
	}
	coin := iwallet.CoinType(req.PaymentCoin)
	if err := coin.ValidateCanonicalPaymentCoin(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDealPaymentSelectionQuoteInvalid, err)
	}
	if !iwallet.IsPaymentCoinEnabled(req.PaymentCoin) {
		return nil, fmt.Errorf("%w: %q", ErrPaymentCoinDisabled, req.PaymentCoin)
	}

	input, err := s.projector.fetchProjectInput(req.OrderID)
	if err != nil {
		return nil, fmt.Errorf("create payment selection quote: %w", err)
	}
	if input.order == nil || input.orderOpen == nil {
		return nil, fmt.Errorf("%w: signed order is unavailable", ErrDealPaymentSelectionQuoteInvalid)
	}
	ref, err := models.DealTermsSnapshotRefFromOrderOpen(input.orderOpen)
	if err != nil || ref == nil || ref.FeeQuoteID == "" {
		return nil, fmt.Errorf("%w: a complete signed Deal fee quote reference is required", ErrDealPaymentSelectionQuoteInvalid)
	}

	now := s.currentTime()
	if boundQuoteID := strings.TrimSpace(input.order.PaymentSelectionQuoteID); boundQuoteID != "" {
		view, projectErr := s.projector.Project(input)
		if projectErr != nil {
			return nil, fmt.Errorf("project bound payment selection quote: %w", projectErr)
		}
		if paymentSessionHasProvisionedTarget(view) {
			var bound models.PaymentSelectionQuote
			boundErr := s.db.View(func(tx database.Tx) error {
				return tx.Read().WithContext(ctx).Where(
					"tenant_id = ? AND quote_id = ? AND order_id = ?",
					paymentSelectionTenantID(input.order), boundQuoteID, input.order.ID.String(),
				).First(&bound).Error
			})
			if boundErr == nil {
				validationErr := validatePaymentSelectionQuoteSnapshot(
					bound, input.orderOpen, ref, req.PaymentCoin, now, false,
				)
				if validationErr == nil {
					return paymentSelectionQuoteView(bound), nil
				}
				if view.PaymentCoin == req.PaymentCoin {
					return nil, validationErr
				}
			}
			if boundErr != nil && !errors.Is(boundErr, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("load bound payment selection quote: %w", boundErr)
			}
			if errors.Is(boundErr, gorm.ErrRecordNotFound) && view.PaymentCoin == req.PaymentCoin {
				return nil, fmt.Errorf("%w: bound quote is unavailable", ErrDealPaymentSelectionQuoteInvalid)
			}
		}
	}

	var reusable models.PaymentSelectionQuote
	reuseErr := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where(
			"tenant_id = ? AND order_id = ? AND fee_quote_id = ? AND payment_coin = ? AND policy_version = ? AND expires_at > ?",
			paymentSelectionTenantID(input.order), input.order.ID.String(), ref.FeeQuoteID, req.PaymentCoin,
			models.PaymentSelectionQuotePilotZeroFeeV1, now,
		).Order("created_at DESC").First(&reusable).Error
	})
	if reuseErr == nil {
		if err := validatePaymentSelectionQuote(reusable, input.orderOpen, ref, req.PaymentCoin, now); err == nil {
			return paymentSelectionQuoteView(reusable), nil
		}
	}
	if reuseErr != nil && !errors.Is(reuseErr, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("reuse payment selection quote: %w", reuseErr)
	}

	quote, err := s.buildPaymentSelectionQuote(input.order, input.orderOpen, ref, coin, now)
	if err != nil {
		return nil, err
	}

	err = s.db.Update(func(tx database.Tx) error {
		var existing models.PaymentSelectionQuote
		reuseErr := tx.Read().Where(
			"tenant_id = ? AND order_id = ? AND fee_quote_id = ? AND payment_coin = ? AND policy_version = ? AND expires_at > ?",
			quote.TenantID, quote.OrderID, quote.FeeQuoteID, quote.PaymentCoin, quote.PolicyVersion, now,
		).Order("created_at DESC").First(&existing).Error
		if reuseErr == nil {
			if err := validatePaymentSelectionQuote(existing, input.orderOpen, ref, req.PaymentCoin, now); err == nil {
				quote = existing
				return nil
			}
		}
		if reuseErr != nil && !errors.Is(reuseErr, gorm.ErrRecordNotFound) {
			return reuseErr
		}
		return tx.Create(&quote)
	})
	if err != nil {
		return nil, fmt.Errorf("persist payment selection quote: %w", err)
	}
	return paymentSelectionQuoteView(quote), nil
}

func (s *PaymentSessionServiceImpl) buildPaymentSelectionQuote(
	order *models.Order,
	orderOpen *porderpb.OrderOpen,
	ref *models.DealTermsSnapshotRef,
	coin iwallet.CoinType,
	now time.Time,
) (models.PaymentSelectionQuote, error) {
	pricingCode, err := dealPricingCurrencyCode(orderOpen.GetPricingCoin())
	if err != nil {
		return models.PaymentSelectionQuote{}, fmt.Errorf("%w: %v", ErrDealPaymentSelectionQuoteInvalid, err)
	}
	paymentCode, err := coin.PricingCurrencyCode()
	if err != nil {
		return models.PaymentSelectionQuote{}, fmt.Errorf("%w: resolve payment currency: %v", ErrDealPaymentSelectionQuoteInvalid, err)
	}
	pricingCurrency, err := models.CurrencyDefinitions.Lookup(pricingCode)
	if err != nil {
		return models.PaymentSelectionQuote{}, fmt.Errorf("%w: unknown pricing currency %q", ErrDealPaymentSelectionQuoteInvalid, pricingCode)
	}
	paymentCurrency, err := models.CurrencyDefinitions.Lookup(paymentCode)
	if err != nil {
		return models.PaymentSelectionQuote{}, fmt.Errorf("%w: unknown payment currency %q", ErrDealPaymentSelectionQuoteInvalid, paymentCode)
	}
	pricingAmount, ok := new(big.Int).SetString(strings.TrimSpace(orderOpen.GetAmount()), 10)
	if !ok || pricingAmount.Sign() <= 0 {
		return models.PaymentSelectionQuote{}, fmt.Errorf("%w: signed pricing amount must be a positive integer", ErrDealPaymentSelectionQuoteInvalid)
	}

	conversionRequired := !strings.EqualFold(pricingCode, paymentCode)
	paymentSubtotal := new(big.Int).Set(pricingAmount)
	pricingScale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(pricingCurrency.Divisibility)), nil)
	exchangeRate := pricingScale.String()
	rateUpdatedAt := now
	if conversionRequired {
		if s.exchange == nil {
			return models.PaymentSelectionQuote{}, ErrExchangeRateUnavailable
		}
		rate, rateErr := s.exchange.GetRate(models.CurrencyCode(paymentCode), models.CurrencyCode(pricingCode), true)
		if rateErr != nil {
			return models.PaymentSelectionQuote{}, fmt.Errorf("%w: %v", ErrExchangeRateUnavailable, rateErr)
		}
		rateInt := big.Int(rate)
		if rateInt.Sign() <= 0 {
			return models.PaymentSelectionQuote{}, fmt.Errorf("%w: exchange rate must be positive", ErrExchangeRateUnavailable)
		}
		exchangeRate = rateInt.String()
		scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(paymentCurrency.Divisibility)), nil)
		numerator := new(big.Int).Mul(pricingAmount, scale)
		remainder := new(big.Int)
		paymentSubtotal.QuoRem(numerator, &rateInt, remainder)
		// A payment target must never under-collect the signed Deal amount.
		// Round a positive fractional smallest unit up, not down.
		if remainder.Sign() > 0 {
			paymentSubtotal.Add(paymentSubtotal, big.NewInt(1))
		}
		if paymentSubtotal.Sign() <= 0 {
			return models.PaymentSelectionQuote{}, fmt.Errorf("%w: converted payment amount is zero", ErrDealPaymentSelectionQuoteInvalid)
		}
		if updatedAt := s.exchange.LastUpdated(models.CurrencyCode(paymentCode)); !updatedAt.IsZero() {
			rateUpdatedAt = updatedAt.UTC()
			if now.Sub(rateUpdatedAt) >= maxPaymentSelectionRateAge {
				return models.PaymentSelectionQuote{}, fmt.Errorf("%w: exchange-rate snapshot is stale", ErrExchangeRateUnavailable)
			}
		}
	}
	if coin.IsFiatPayment() {
		if !paymentSubtotal.IsInt64() {
			return models.PaymentSelectionQuote{}, fmt.Errorf("%w: quoted fiat amount exceeds supported range", ErrDealPaymentSelectionQuoteInvalid)
		}
	} else if !paymentSubtotal.IsUint64() {
		return models.PaymentSelectionQuote{}, fmt.Errorf("%w: quoted crypto amount exceeds supported range", ErrDealPaymentSelectionQuoteInvalid)
	}

	ttl := s.quoteTTL
	if ttl <= 0 {
		ttl = defaultPaymentSelectionQuoteTTL
	}
	expiresAt := now.Add(ttl)
	if conversionRequired {
		rateExpiry := rateUpdatedAt.Add(maxPaymentSelectionRateAge)
		if rateExpiry.Before(expiresAt) {
			expiresAt = rateExpiry
		}
	}
	tenantID := paymentSelectionTenantID(order)
	amount := paymentSubtotal.String()
	return models.PaymentSelectionQuote{
		TenantID: tenantID, QuoteID: uuid.NewString(), OrderID: order.ID.String(),
		FeeQuoteID: ref.FeeQuoteID, DealLinkID: ref.DealLinkID, DealRevision: ref.Revision, TermsHash: ref.TermsHash,
		SchemaVersion: 1, PolicyVersion: models.PaymentSelectionQuotePilotZeroFeeV1,
		PricingCurrency: pricingCode, PricingAmount: pricingAmount.String(), PricingDivisibility: pricingCurrency.Divisibility,
		PaymentCoin: string(coin), PaymentCurrency: paymentCode, PaymentDivisibility: paymentCurrency.Divisibility,
		ConversionRequired: conversionRequired, ExchangeRate: exchangeRate, ExchangeRateBase: paymentCode,
		ExchangeRateQuote: pricingCode, ExchangeRateQuoteDivisibility: pricingCurrency.Divisibility,
		RateSourceUpdatedAt: rateUpdatedAt, PaymentSubtotal: amount, ProviderOrNetworkCost: "0",
		PlatformPaymentCost: "0", BuyerPaymentTotal: amount, ExpiresAt: expiresAt, CreatedAt: now,
	}, nil
}

func (s *PaymentSessionServiceImpl) resolveDealPaymentSelectionQuote(
	ctx context.Context,
	order *models.Order,
	orderOpen *porderpb.OrderOpen,
	req contracts.CreatePaymentSessionRequest,
) (*models.PaymentSelectionQuote, error) {
	ref, err := models.DealTermsSnapshotRefFromOrderOpen(orderOpen)
	if err != nil || ref == nil || strings.TrimSpace(req.PaymentSelectionQuoteID) == "" {
		return nil, nil
	}
	var quote models.PaymentSelectionQuote
	err = s.db.View(func(tx database.Tx) error {
		return tx.Read().WithContext(ctx).Where(
			"tenant_id = ? AND quote_id = ? AND order_id = ?",
			paymentSelectionTenantID(order), strings.TrimSpace(req.PaymentSelectionQuoteID), order.ID.String(),
		).First(&quote).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("%w: quote not found", ErrDealPaymentSelectionQuoteInvalid)
		}
		return nil, fmt.Errorf("load payment selection quote: %w", err)
	}
	if err := validatePaymentSelectionQuoteSnapshot(
		quote, orderOpen, ref, req.PaymentCoin, s.currentTime(), false,
	); err != nil {
		return nil, err
	}
	return &quote, nil
}

func validatePaymentSelectionQuote(
	quote models.PaymentSelectionQuote,
	orderOpen *porderpb.OrderOpen,
	ref *models.DealTermsSnapshotRef,
	paymentCoin string,
	now time.Time,
) error {
	return validatePaymentSelectionQuoteSnapshot(quote, orderOpen, ref, paymentCoin, now, true)
}

func validatePaymentSelectionQuoteSnapshot(
	quote models.PaymentSelectionQuote,
	orderOpen *porderpb.OrderOpen,
	ref *models.DealTermsSnapshotRef,
	paymentCoin string,
	now time.Time,
	requireUnexpired bool,
) error {
	pricingCode, err := dealPricingCurrencyCode(orderOpen.GetPricingCoin())
	if err != nil {
		return fmt.Errorf("%w: %v", ErrDealPaymentSelectionQuoteInvalid, err)
	}
	paymentCode, err := iwallet.CoinType(paymentCoin).PricingCurrencyCode()
	if err != nil {
		return fmt.Errorf("%w: invalid payment currency", ErrDealPaymentSelectionQuoteInvalid)
	}
	pricingCurrency, err := models.CurrencyDefinitions.Lookup(pricingCode)
	if err != nil {
		return fmt.Errorf("%w: invalid pricing currency", ErrDealPaymentSelectionQuoteInvalid)
	}
	paymentCurrency, err := models.CurrencyDefinitions.Lookup(paymentCode)
	if err != nil {
		return fmt.Errorf("%w: invalid payment currency", ErrDealPaymentSelectionQuoteInvalid)
	}
	if requireUnexpired && !quote.ExpiresAt.After(now) {
		return fmt.Errorf("%w: quote expired", ErrDealPaymentSelectionQuoteInvalid)
	}
	if quote.FeeQuoteID != ref.FeeQuoteID || quote.DealLinkID != ref.DealLinkID ||
		quote.DealRevision != ref.Revision || quote.TermsHash != ref.TermsHash ||
		quote.PricingCurrency != pricingCode || quote.PricingAmount != orderOpen.GetAmount() ||
		quote.PricingDivisibility != pricingCurrency.Divisibility || quote.PaymentCoin != paymentCoin ||
		quote.PaymentCurrency != paymentCode || quote.PaymentDivisibility != paymentCurrency.Divisibility ||
		quote.ConversionRequired != !strings.EqualFold(pricingCode, paymentCode) ||
		quote.ExchangeRateBase != paymentCode || quote.ExchangeRateQuote != pricingCode ||
		quote.ExchangeRateQuoteDivisibility != pricingCurrency.Divisibility || quote.SchemaVersion != 1 ||
		quote.PolicyVersion != models.PaymentSelectionQuotePilotZeroFeeV1 {
		return fmt.Errorf("%w: quote does not match the signed Deal order and selected asset", ErrDealPaymentSelectionQuoteInvalid)
	}
	rate, ok := new(big.Int).SetString(quote.ExchangeRate, 10)
	if !ok || rate.Sign() <= 0 {
		return fmt.Errorf("%w: invalid exchange rate", ErrDealPaymentSelectionQuoteInvalid)
	}
	if quote.PaymentSubtotal == "" || quote.ProviderOrNetworkCost == "" ||
		quote.PlatformPaymentCost == "" || quote.BuyerPaymentTotal == "" {
		return fmt.Errorf("%w: quote monetary fields are incomplete", ErrDealPaymentSelectionQuoteInvalid)
	}
	if quote.ProviderOrNetworkCost != "0" || quote.PlatformPaymentCost != "0" {
		return fmt.Errorf("%w: zero-fee policy contains a non-zero payment cost", ErrDealPaymentSelectionQuoteInvalid)
	}
	wantTotal, ok := new(big.Int).SetString(quote.PaymentSubtotal, 10)
	if !ok || wantTotal.Sign() <= 0 {
		return fmt.Errorf("%w: invalid payment subtotal", ErrDealPaymentSelectionQuoteInvalid)
	}
	for _, fee := range []string{quote.ProviderOrNetworkCost, quote.PlatformPaymentCost} {
		feeInt, feeOK := new(big.Int).SetString(fee, 10)
		if !feeOK || feeInt.Sign() < 0 {
			return fmt.Errorf("%w: invalid payment cost", ErrDealPaymentSelectionQuoteInvalid)
		}
		wantTotal.Add(wantTotal, feeInt)
	}
	if wantTotal.String() != quote.BuyerPaymentTotal {
		return fmt.Errorf("%w: buyer payment total does not equal subtotal plus costs", ErrDealPaymentSelectionQuoteInvalid)
	}
	return nil
}

func (s *PaymentSessionServiceImpl) bindPaymentSelectionQuote(order *models.Order, quote *models.PaymentSelectionQuote) error {
	if order == nil || quote == nil {
		return nil
	}
	return s.db.Update(func(tx database.Tx) error {
		return tx.Update(
			"payment_selection_quote_id",
			quote.QuoteID,
			map[string]interface{}{
				"id = ?":        order.ID.String(),
				"tenant_id = ?": paymentSelectionTenantID(order),
			},
			&models.Order{},
		)
	})
}

func (s *PaymentSessionServiceImpl) currentTime() time.Time {
	if s != nil && s.now != nil {
		return s.now().UTC()
	}
	return time.Now().UTC()
}

func paymentSelectionTenantID(order *models.Order) string {
	if order != nil && order.TenantID != "" {
		return order.TenantID
	}
	return database.StandaloneTenantID
}

func paymentSelectionQuoteView(quote models.PaymentSelectionQuote) *paypb.PaymentSelectionQuote {
	return &paypb.PaymentSelectionQuote{
		ID: quote.QuoteID, OrderID: quote.OrderID, FeeQuoteID: quote.FeeQuoteID,
		DealLinkID: quote.DealLinkID, DealRevision: quote.DealRevision, TermsHash: quote.TermsHash,
		SchemaVersion: quote.SchemaVersion, PolicyVersion: quote.PolicyVersion,
		PricingCurrency: quote.PricingCurrency, PricingAmount: quote.PricingAmount,
		PricingDivisibility: quote.PricingDivisibility, PaymentCoin: quote.PaymentCoin,
		PaymentCurrency: quote.PaymentCurrency, PaymentDivisibility: quote.PaymentDivisibility,
		ConversionRequired: quote.ConversionRequired, ExchangeRate: quote.ExchangeRate,
		ExchangeRateBase: quote.ExchangeRateBase, ExchangeRateQuote: quote.ExchangeRateQuote,
		ExchangeRateQuoteDivisibility: quote.ExchangeRateQuoteDivisibility,
		RateSourceUpdatedAt:           quote.RateSourceUpdatedAt, PaymentSubtotal: quote.PaymentSubtotal,
		ProviderOrNetworkCost: quote.ProviderOrNetworkCost, PlatformPaymentCost: quote.PlatformPaymentCost,
		BuyerPaymentTotal: quote.BuyerPaymentTotal, ExpiresAt: quote.ExpiresAt, CreatedAt: quote.CreatedAt,
	}
}
