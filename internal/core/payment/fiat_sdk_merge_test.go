package payment

import (
	"testing"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/models"
	porderpb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	paypb "github.com/mobazha/mobazha3.0/pkg/payment"
)

func TestMergeFiatProviderSessionIntoView_Stripe(t *testing.T) {
	view := &paypb.PaymentSession{
		FundingTarget: paypb.FundingTargetView{},
	}
	mergeFiatProviderSessionIntoView(view, "stripe", &contracts.FiatProviderSession{
		SessionID:   "pi_snap",
		CaptureMode: contracts.CaptureAutomatic,
		Status:      "open",
		ExpiresAt:   time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC),
		Stripe: &contracts.StripeSessionData{
			ClientSecret:       "secret",
			PublishableKey:     "pk",
			ConnectedAccountID: "acct",
		},
	})
	got := view.FundingTarget.ProviderData
	if got["providerID"] != "stripe" || got["sessionID"] != "pi_snap" {
		t.Fatalf("provider/session merge: %+v", got)
	}
	if got["clientSecret"] != "secret" || got["publishableKey"] != "pk" || got["connectedAccountId"] != "acct" {
		t.Fatalf("stripe keys merge: %+v", got)
	}
}

func TestMergeFiatProviderSessionIntoView_PayPal(t *testing.T) {
	view := &paypb.PaymentSession{FundingTarget: paypb.FundingTargetView{}}
	mergeFiatProviderSessionIntoView(view, "paypal", &contracts.FiatProviderSession{
		SessionID: "ORDER-123",
		PayPal: &contracts.PayPalSessionData{
			OrderID:  "PAYPAL-ORDER",
			ClientID: "cid",
		},
	})
	got := view.FundingTarget.ProviderData
	if got["orderID"] != "PAYPAL-ORDER" || got["clientID"] != "cid" {
		t.Fatalf("paypal keys merge: %+v", got)
	}
}

func TestBuildFiatRecoveryMetadata_RoundTripToProviderData(t *testing.T) {
	session := &contracts.FiatProviderSession{
		SessionID:   "sess_123",
		CaptureMode: contracts.CaptureAutomatic,
		Status:      "requires_payment_method",
		ExpiresAt:   time.Date(2030, 2, 3, 4, 5, 6, 0, time.UTC),
		ApproveURL:  "https://example.test/approve",
		Stripe: &contracts.StripeSessionData{
			ClientSecret:       "cs_test",
			PublishableKey:     "pk_test",
			ConnectedAccountID: "acct_test",
		},
	}

	meta := buildFiatRecoveryMetadata(session)
	providerData := map[string]interface{}{
		"providerID": "stripe",
		"sessionID":  session.SessionID,
	}
	mergeFiatRecoveryMetadata(providerData, meta)

	if providerData["clientSecret"] != "cs_test" ||
		providerData["publishableKey"] != "pk_test" ||
		providerData["connectedAccountId"] != "acct_test" ||
		providerData["approveURL"] != "https://example.test/approve" {
		t.Fatalf("recovery metadata merge: %+v", providerData)
	}
}

func TestDerivePaymentInfo_FiatFallsBackToOrderOpenPricingCoin(t *testing.T) {
	p := &PaymentSessionProjector{}
	order := &models.Order{
		OrderPaymentState: models.OrderPaymentState{
			FiatPaymentState: models.FiatPaymentState{
				FiatMetadata: []byte(`{"fiat_provider":"paypal","fiat_session_id":"ORDER-1"}`),
			},
		},
	}
	open := &porderpb.OrderOpen{PricingCoin: "usd"}

	coin, mode := p.derivePaymentInfo(order, open, nil)
	if coin != "fiat:paypal:USD" {
		t.Fatalf("payment coin = %q", coin)
	}
	if mode != paypb.ProductModeCancelable {
		t.Fatalf("mode=%v", mode)
	}
}

func TestDeriveFiatFundingTarget_RestoresRecoveryMetadata(t *testing.T) {
	p := &PaymentSessionProjector{}
	order := &models.Order{}
	session := &contracts.FiatProviderSession{
		SessionID:   "sess_saved",
		CaptureMode: contracts.CaptureManual,
		Status:      "pending",
		ApproveURL:  "https://paypal.test/approve",
		PayPal: &contracts.PayPalSessionData{
			OrderID:  "PP-ORDER",
			ClientID: "paypal-client",
		},
	}
	if err := order.MergeFiatMetadata(buildFiatRecoveryMetadata(session)); err != nil {
		t.Fatal(err)
	}
	if err := order.MergeFiatMetadata(map[string]string{
		"fiat_provider":   "paypal",
		"fiat_session_id": "sess_saved",
		"fiat_currency":   "USD",
	}); err != nil {
		t.Fatal(err)
	}

	target := p.deriveFiatFundingTarget(order, "fiat:paypal:USD", "100")
	if target.ProviderData["captureMode"] != string(contracts.CaptureManual) ||
		target.ProviderData["providerStatus"] != "pending" ||
		target.ProviderData["approveURL"] != "https://paypal.test/approve" ||
		target.ProviderData["orderID"] != "PP-ORDER" ||
		target.ProviderData["clientID"] != "paypal-client" {
		t.Fatalf("providerData = %+v", target.ProviderData)
	}
}

func TestDeriveFiatFundingTarget_DoesNotRestoreCheckoutMetadataAfterCapture(t *testing.T) {
	p := &PaymentSessionProjector{}
	order := &models.Order{}
	session := &contracts.FiatProviderSession{
		SessionID:  "ORDER-123",
		ApproveURL: "https://paypal.test/approve",
		PayPal: &contracts.PayPalSessionData{
			OrderID:  "ORDER-123",
			ClientID: "paypal-client",
		},
	}
	if err := order.MergeFiatMetadata(buildFiatRecoveryMetadata(session)); err != nil {
		t.Fatal(err)
	}
	if err := order.MergeFiatMetadata(map[string]string{
		"fiat_provider":   "paypal",
		"fiat_session_id": "ORDER-123",
		"fiat_currency":   "USD",
	}); err != nil {
		t.Fatal(err)
	}
	order.PaymentTransactionID = "CAPTURE-456"

	target := p.deriveFiatFundingTarget(order, "fiat:paypal:USD", "100")
	if target.ProviderData["sessionID"] != "CAPTURE-456" {
		t.Fatalf("providerData = %+v", target.ProviderData)
	}
	if _, ok := target.ProviderData["approveURL"]; ok {
		t.Fatalf("unexpected stale approveURL in providerData: %+v", target.ProviderData)
	}
	if _, ok := target.ProviderData["orderID"]; ok {
		t.Fatalf("unexpected stale orderID in providerData: %+v", target.ProviderData)
	}
}
