package payment

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mobazha/mobazha/internal/repo"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	porderpb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
)

type paymentSelectionRates struct {
	rate      iwallet.Amount
	updatedAt time.Time
	calls     int
}

func (r *paymentSelectionRates) GetAllRates(models.CurrencyCode, bool) (map[models.CurrencyCode]iwallet.Amount, error) {
	return nil, nil
}

func (r *paymentSelectionRates) GetRate(models.CurrencyCode, models.CurrencyCode, bool) (iwallet.Amount, error) {
	r.calls++
	return r.rate, nil
}

func (r *paymentSelectionRates) LastUpdated(models.CurrencyCode) time.Time { return r.updatedAt }

func TestCreateSelectionQuote_CrossCurrencyPersistsAndReuses(t *testing.T) {
	now := time.Date(2026, time.July, 5, 8, 0, 0, 0, time.UTC)
	db, err := repo.MockDB()
	require.NoError(t, err)
	defer db.Close()

	orderID := "deal-payment-selection-order"
	open := &porderpb.OrderOpen{
		PricingCoin: "USD", Amount: "4900", DealLinkID: "deal-123", DealRevision: 2,
		TermsHash: strings.Repeat("a", 64), FeeQuoteID: "fq-123",
	}
	raw, err := (protojson.MarshalOptions{}).Marshal(open)
	require.NoError(t, err)
	require.NoError(t, db.Update(func(tx database.Tx) error {
		for _, model := range []interface{}{
			&models.Order{}, &models.PaymentObservation{}, &models.SharedPaymentIntent{}, &models.PaymentSelectionQuote{},
		} {
			if err := tx.Migrate(model); err != nil {
				return err
			}
		}
		return tx.Create(&models.Order{
			TenantMixin: models.TenantMixin{TenantID: database.StandaloneTenantID},
			ID:          models.OrderID(orderID), MyRole: string(models.RoleBuyer), Open: true,
			SerializedOrderOpen: raw,
		})
	}))

	rates := &paymentSelectionRates{rate: iwallet.NewAmount("250000"), updatedAt: now.Add(-30 * time.Second)}
	svc := NewPaymentSessionService(db)
	svc.SetExchangeRateService(rates)
	svc.now = func() time.Time { return now }

	req := contracts.CreatePaymentSelectionQuoteRequest{
		OrderID: orderID, PaymentCoin: "crypto:eip155:1:native",
	}
	first, err := svc.CreateSelectionQuote(context.Background(), req)
	require.NoError(t, err)
	require.True(t, first.ConversionRequired)
	require.Equal(t, "250000", first.ExchangeRate)
	require.Equal(t, "19600000000000000", first.PaymentSubtotal)
	require.Equal(t, "0", first.ProviderOrNetworkCost)
	require.Equal(t, "0", first.PlatformPaymentCost)
	require.Equal(t, first.PaymentSubtotal, first.BuyerPaymentTotal)
	require.Equal(t, models.PaymentSelectionQuotePilotZeroFeeV1, first.PolicyVersion)

	second, err := svc.CreateSelectionQuote(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, first.ID, second.ID)
	require.Equal(t, 1, rates.calls, "a valid quote must be reused without refreshing the rate")

	var stored models.PaymentSelectionQuote
	require.NoError(t, db.View(func(tx database.Tx) error {
		return tx.Read().Where("quote_id = ?", first.ID).First(&stored).Error
	}))
	require.Equal(t, first.BuyerPaymentTotal, stored.BuyerPaymentTotal)
}

func TestCreateSelectionQuote_RoundsUpAndDoesNotReuseOldDealRevision(t *testing.T) {
	now := time.Date(2026, time.July, 5, 8, 0, 0, 0, time.UTC)
	db, err := repo.MockDB()
	require.NoError(t, err)
	defer db.Close()

	orderID := "deal-payment-selection-rounding"
	open := &porderpb.OrderOpen{
		PricingCoin: "USD", Amount: "4900", DealLinkID: "deal-rounding", DealRevision: 2,
		TermsHash: strings.Repeat("c", 64), FeeQuoteID: "fq-rounding",
	}
	raw, err := (protojson.MarshalOptions{}).Marshal(open)
	require.NoError(t, err)
	require.NoError(t, db.Update(func(tx database.Tx) error {
		for _, model := range []interface{}{
			&models.Order{}, &models.PaymentObservation{}, &models.SharedPaymentIntent{}, &models.PaymentSelectionQuote{},
		} {
			if err := tx.Migrate(model); err != nil {
				return err
			}
		}
		return tx.Create(&models.Order{
			TenantMixin: models.TenantMixin{TenantID: database.StandaloneTenantID},
			ID:          models.OrderID(orderID), MyRole: string(models.RoleBuyer), Open: true,
			SerializedOrderOpen: raw,
		})
	}))

	rates := &paymentSelectionRates{rate: iwallet.NewAmount("300000"), updatedAt: now}
	svc := NewPaymentSessionService(db)
	svc.SetExchangeRateService(rates)
	svc.now = func() time.Time { return now }
	req := contracts.CreatePaymentSelectionQuoteRequest{
		OrderID: orderID, PaymentCoin: "crypto:eip155:1:native",
	}

	first, err := svc.CreateSelectionQuote(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, "16333333333333334", first.PaymentSubtotal, "fractional wei must round up")

	open.DealRevision = 3
	open.TermsHash = strings.Repeat("d", 64)
	raw, err = (protojson.MarshalOptions{}).Marshal(open)
	require.NoError(t, err)
	require.NoError(t, db.Update(func(tx database.Tx) error {
		return tx.Update("serialized_order_open", raw, map[string]interface{}{"id = ?": orderID}, &models.Order{})
	}))

	second, err := svc.CreateSelectionQuote(context.Background(), req)
	require.NoError(t, err)
	require.NotEqual(t, first.ID, second.ID)
	require.Equal(t, uint64(3), second.DealRevision)
	require.Equal(t, 2, rates.calls)
}

func TestCreateSelectionQuote_ReturnsExpiredQuoteBoundToProvisionedSession(t *testing.T) {
	now := time.Date(2026, time.July, 5, 8, 0, 0, 0, time.UTC)
	db, err := repo.MockDB()
	require.NoError(t, err)
	defer db.Close()

	orderID := "deal-bound-payment-selection"
	open := &porderpb.OrderOpen{
		PricingCoin: "USD", Amount: "4900", DealLinkID: "deal-bound", DealRevision: 1,
		TermsHash: strings.Repeat("e", 64), FeeQuoteID: "fq-bound",
	}
	raw, err := (protojson.MarshalOptions{}).Marshal(open)
	require.NoError(t, err)
	require.NoError(t, db.Update(func(tx database.Tx) error {
		for _, model := range []interface{}{
			&models.Order{}, &models.PaymentObservation{}, &models.SharedPaymentIntent{}, &models.PaymentSelectionQuote{},
		} {
			if err := tx.Migrate(model); err != nil {
				return err
			}
		}
		return tx.Create(&models.Order{
			TenantMixin: models.TenantMixin{TenantID: database.StandaloneTenantID},
			ID:          models.OrderID(orderID), MyRole: string(models.RoleBuyer), Open: true,
			SerializedOrderOpen: raw,
		})
	}))

	svc := NewPaymentSessionService(db)
	svc.now = func() time.Time { return now }
	req := contracts.CreatePaymentSelectionQuoteRequest{
		OrderID: orderID, PaymentCoin: "fiat:stripe:USD",
	}
	first, err := svc.CreateSelectionQuote(context.Background(), req)
	require.NoError(t, err)

	require.NoError(t, db.Update(func(tx database.Tx) error {
		var order models.Order
		if err := tx.Read().Where("id = ?", orderID).First(&order).Error; err != nil {
			return err
		}
		order.PaymentSelectionQuoteID = first.ID
		order.OrderOpenAcked = true
		if err := order.MergeFiatMetadata(map[string]string{
			"fiat_provider": "stripe", "fiat_currency": "USD", "fiat_session_id": "pi_bound",
		}); err != nil {
			return err
		}
		return tx.Save(&order)
	}))
	input, err := svc.projector.fetchProjectInput(orderID)
	require.NoError(t, err)
	view, err := svc.projector.Project(input)
	require.NoError(t, err)
	require.Equal(t, "fiat:stripe:USD", view.PaymentCoin)
	require.Equal(t, "pi_bound", fiatSessionIDFromView(view))
	require.Equal(t, first.ID, view.PaymentSelectionQuoteID)

	now = first.ExpiresAt.Add(time.Second)
	bound, err := svc.CreateSelectionQuote(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, first.ID, bound.ID)
	require.True(t, bound.ExpiresAt.Before(now))
}

func TestValidatePaymentSelectionQuote_RejectsExpiryAndTampering(t *testing.T) {
	now := time.Now().UTC()
	open := &porderpb.OrderOpen{
		PricingCoin: "USD", Amount: "4900", DealLinkID: "deal-123", DealRevision: 2,
		TermsHash: strings.Repeat("b", 64), FeeQuoteID: "fq-123",
	}
	ref, err := models.DealTermsSnapshotRefFromOrderOpen(open)
	require.NoError(t, err)
	base := models.PaymentSelectionQuote{
		FeeQuoteID: "fq-123", DealLinkID: "deal-123", DealRevision: 2,
		TermsHash: open.TermsHash, PolicyVersion: models.PaymentSelectionQuotePilotZeroFeeV1,
		SchemaVersion: 1, PricingCurrency: "USD", PricingAmount: "4900", PricingDivisibility: 2,
		PaymentCoin: "crypto:eip155:1:native", PaymentCurrency: "ETH", PaymentDivisibility: 18,
		ConversionRequired: true, ExchangeRate: "250000", ExchangeRateBase: "ETH",
		ExchangeRateQuote: "USD", ExchangeRateQuoteDivisibility: 2,
		PaymentSubtotal: "19600000000000000", ProviderOrNetworkCost: "0",
		PlatformPaymentCost: "0", BuyerPaymentTotal: "19600000000000000", ExpiresAt: now.Add(time.Minute),
	}
	require.NoError(t, validatePaymentSelectionQuote(base, open, ref, base.PaymentCoin, now))

	expired := base
	expired.ExpiresAt = now
	require.ErrorIs(t, validatePaymentSelectionQuote(expired, open, ref, expired.PaymentCoin, now), ErrDealPaymentSelectionQuoteInvalid)

	tampered := base
	tampered.BuyerPaymentTotal = "1"
	require.ErrorIs(t, validatePaymentSelectionQuote(tampered, open, ref, tampered.PaymentCoin, now), ErrDealPaymentSelectionQuoteInvalid)

	wrongAsset := base
	require.ErrorIs(t, validatePaymentSelectionQuote(wrongAsset, open, ref, "fiat:stripe:USD", now), ErrDealPaymentSelectionQuoteInvalid)
}

func TestBuildPaymentSetupParamsFromOrder_UsesAuthorizedQuoteAmount(t *testing.T) {
	coin := iwallet.CoinType("crypto:eip155:1:native")
	order := &models.Order{ID: models.OrderID("quoted-order")}
	open := &porderpb.OrderOpen{PricingCoin: "USD", Amount: "4900"}

	params, err := buildPaymentSetupParamsFromOrder(
		order, open, coin, "", "", "", "19600000000000000", nil,
	)
	require.NoError(t, err)
	require.Equal(t, uint64(19600000000000000), params.Amount)
}
