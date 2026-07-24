package payment

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mobazha/mobazha/internal/repo"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	porderpb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	pkpayment "github.com/mobazha/mobazha/pkg/payment"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestPaymentSessionServiceImpl_CreateSession_RejectsNonCanonicalPaymentCoin(t *testing.T) {
	svc := NewPaymentSessionService(nil)

	_, err := svc.CreateSession(context.Background(), contracts.CreatePaymentSessionRequest{
		OrderID:     "any-order-id",
		PaymentCoin: "USDC",
	})
	if err == nil {
		t.Fatal("expected error for ambiguous non-canonical coin")
	}
	msg := strings.ToLower(err.Error())
	if !strings.Contains(msg, "canonical") && !strings.Contains(msg, "payment coin") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestPaymentSessionServiceImpl_CreateSession_RejectsProductDisabledZEC(t *testing.T) {
	svc := NewPaymentSessionService(nil)

	_, err := svc.CreateSession(context.Background(), contracts.CreatePaymentSessionRequest{
		OrderID:     "any-order-id",
		PaymentCoin: "crypto:zcash:mainnet:native",
	})
	if err == nil {
		t.Fatal("expected error for product-disabled ZEC")
	}
	if !errors.Is(err, ErrPaymentCoinDisabled) {
		t.Fatalf("error = %v, want ErrPaymentCoinDisabled", err)
	}
	msg := strings.ToLower(err.Error())
	if !strings.Contains(msg, "not enabled") || !strings.Contains(msg, "zcash") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestPaymentSessionServiceImpl_CreateSession_RejectsStandardCrossCurrencyFiatWithoutQuote(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	defer db.Close()

	readyAt := time.Now().UTC()
	orderID := "standard-cross-fiat-no-quote"
	open := &porderpb.OrderOpen{PricingCoin: "USD", Amount: "4900"}
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
			TenantMixin:         models.TenantMixin{TenantID: database.StandaloneTenantID},
			ID:                  models.OrderID(orderID),
			MyRole:              string(models.RoleBuyer),
			Open:                true,
			OrderOpenAcked:      true,
			PaymentReadyAt:      &readyAt,
			SerializedOrderOpen: raw,
		})
	}))

	svc := NewPaymentSessionService(db)
	// Intentionally leave FiatFacade unset. A missing quote must fail closed
	// before any provider session is created (which would surface as
	// ErrFiatFacadeNotWired if conversion validation were skipped).
	_, err = svc.CreateSession(context.Background(), contracts.CreatePaymentSessionRequest{
		OrderID:         orderID,
		PaymentCoin:     "fiat:stripe:EUR",
		FiatAmountCents: 1,
	})
	require.ErrorIs(t, err, ErrDealPaymentConversionQuoteRequired)
}

func TestCanReprovisionForCoinSwitch_AllowsUnfundedFiatCheckoutToCrypto(t *testing.T) {
	view := &pkpayment.PaymentSession{
		PaymentCoin:    "fiat:stripe:USD",
		SettlementMode: pkpayment.SettlementModeProviderCheckout,
		PaymentProgress: pkpayment.PaymentProgressView{
			ObservedAmount: "0.00",
			FundingState:   pkpayment.FundingStateProviderProcessing,
		},
	}

	if !canReprovisionForCoinSwitch(view, "crypto:solana:mainnet:native", false) {
		t.Fatal("expected unfunded provider checkout to allow crypto reprovision")
	}
}

func TestCanReprovisionForCoinSwitch_RejectsFundedFiatCheckout(t *testing.T) {
	view := &pkpayment.PaymentSession{
		PaymentCoin:    "fiat:stripe:USD",
		SettlementMode: pkpayment.SettlementModeProviderCheckout,
		PaymentProgress: pkpayment.PaymentProgressView{
			ObservedAmount: "29.00",
			FundingState:   pkpayment.FundingStateFullyFunded,
		},
	}

	if canReprovisionForCoinSwitch(view, "crypto:solana:mainnet:native", false) {
		t.Fatal("funded provider checkout must not allow crypto reprovision")
	}
}

func TestCanReprovisionForCoinSwitch_RejectsFrozenAttempt(t *testing.T) {
	view := &pkpayment.PaymentSession{
		PaymentCoin:    "crypto:eip155:1:native",
		SettlementMode: pkpayment.SettlementModeAddressMonitored,
		PaymentProgress: pkpayment.PaymentProgressView{
			ObservedAmount: "0",
			FundingState:   pkpayment.FundingStateAwaitingFunds,
		},
	}

	if canReprovisionForCoinSwitch(view, "crypto:solana:mainnet:native", true) {
		t.Fatal("frozen payment attempt must not allow crypto reprovision")
	}
}
