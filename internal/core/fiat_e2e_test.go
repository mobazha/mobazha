package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	coreorder "github.com/mobazha/mobazha3.0/internal/core/order"
	"github.com/mobazha/mobazha3.0/internal/payment/fiat/paypal"
	"github.com/mobazha/mobazha3.0/internal/payment/fiat/stripe"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	netpb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// capturingMessenger is a mock Messenger that records ReliablySendMessage calls.
type capturingMessenger struct {
	called   atomic.Int32
	mu       sync.Mutex
	lastPeer peer.ID
	lastMsg  *netpb.Message
}

func (m *capturingMessenger) ReliablySendMessage(_ database.Tx, p peer.ID, msg *netpb.Message, _ chan<- struct{}) error {
	m.called.Add(1)
	m.mu.Lock()
	m.lastPeer = p
	m.lastMsg = msg
	m.mu.Unlock()
	return nil
}

func (m *capturingMessenger) getLastPeer() peer.ID {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastPeer
}

func (m *capturingMessenger) getLastMsg() *netpb.Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastMsg
}
func (m *capturingMessenger) ProcessACK(_ database.Tx, _ *netpb.AckMessage) error { return nil }
func (m *capturingMessenger) SendACK(_ string, _ peer.ID)                         {}
func (m *capturingMessenger) Start()                                              {}
func (m *capturingMessenger) Stop()                                               {}

// Fiat E2E integration tests wire real Provider implementations (with mock HTTP
// servers) into FiatPaymentAppService to test the full webhook→parsing→order
// state transition chain, covering both Stripe and PayPal providers.

// ---------- PayPal E2E helpers ----------

func newPayPalWebhookServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/notifications/verify-webhook-signature", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"verification_status": "SUCCESS"})
	})
	return httptest.NewServer(mux)
}

func paypalHeaders() map[string]string {
	return map[string]string{
		"Paypal-Transmission-Id":   "e2e-tx-id",
		"Paypal-Transmission-Sig":  "e2e-sig",
		"Paypal-Transmission-Time": "2026-03-15T00:00:00Z",
		"Paypal-Auth-Algo":         "SHA256withRSA",
		"Paypal-Cert-Url":          "https://api.paypal.com/cert.pem",
	}
}

func registerPayPalProvider(t *testing.T, reg contracts.FiatProviderRegistry, serverURL string) {
	t.Helper()
	p := paypal.NewProvider(paypal.Config{
		ClientID:     "e2e-client",
		ClientSecret: "e2e-secret",
		WebhookID:    "e2e-webhook-id",
		Mode:         paypal.ModeDirect,
		Sandbox:      true,
	})
	p.OverrideBaseURL(serverURL)
	reg.Register(p)
}

// ---------- Stripe E2E helpers ----------

func stripeWebhookPayload(eventType string, dataObject json.RawMessage) ([]byte, string) {
	payload, _ := json.Marshal(map[string]interface{}{
		"id":          "evt_e2e_" + eventType,
		"type":        eventType,
		"api_version": "2024-04-10",
		"data":        map[string]interface{}{"object": dataObject},
	})
	return payload, stripe.SignWebhookForTest(payload, "whsec_e2e")
}

func registerStripeProvider(reg contracts.FiatProviderRegistry) {
	p := stripe.NewProvider(stripe.Config{
		SecretKey:      "sk_test_e2e",
		PublishableKey: "pk_test_e2e",
		WebhookSecret:  "whsec_e2e",
		Mode:           stripe.ModeDirect,
	})
	reg.Register(p)
}

// ==================== PayPal E2E Tests ====================

func TestE2E_PayPal_PaymentSucceeded_OrderTransition(t *testing.T) {
	ts := newPayPalWebhookServer(t)
	defer ts.Close()

	reg := newMockFiatRegistry()
	registerPayPalProvider(t, reg, ts.URL)
	svc, _ := newFiatTestService(t, reg)

	orderRepo := newMockOrderRepo()
	orderRepo.addOrder(&models.Order{
		ID:    "pp-order-001",
		State: models.OrderState_AWAITING_PAYMENT,
	})
	svc.SetOrderRepo(orderRepo)

	var handled *contracts.WebhookEvent
	svc.SetWebhookHandler(func(_ context.Context, evt *contracts.WebhookEvent) error {
		handled = evt
		return nil
	})

	payload := `{
		"id": "WH-E2E-001",
		"event_type": "CHECKOUT.ORDER.COMPLETED",
		"resource": {
			"id": "PP-ORDER-E2E",
			"status": "COMPLETED",
			"custom_id": "pp-order-001",
			"purchase_units": [{
				"custom_id": "pp-order-001",
				"amount": {"currency_code": "USD", "value": "49.99"},
				"payee": {"merchant_id": "SELLER-E2E"}
			}]
		}
	}`

	err := svc.HandleWebhook(context.Background(), "paypal", []byte(payload), paypalHeaders())
	require.NoError(t, err)
	require.NotNil(t, handled, "webhook handler should have been called")

	assert.Equal(t, contracts.WebhookPaymentSucceeded, handled.Type)
	assert.Equal(t, "paypal", handled.ProviderID)
	assert.Equal(t, "PP-ORDER-E2E", handled.PaymentID)
	assert.Equal(t, "pp-order-001", handled.OrderID)
	assert.Equal(t, "SELLER-E2E", handled.AccountID)
	assert.Equal(t, int64(4999), handled.Amount)
	assert.Equal(t, "USD", handled.Currency)
}

func TestE2E_PayPal_CaptureRefunded_OrderTransition(t *testing.T) {
	ts := newPayPalWebhookServer(t)
	defer ts.Close()

	reg := newMockFiatRegistry()
	registerPayPalProvider(t, reg, ts.URL)
	svc, _ := newFiatTestService(t, reg)

	orderRepo := newMockOrderRepo()
	orderRepo.addOrder(&models.Order{
		ID:    "pp-refund-order",
		State: models.OrderState_SHIPPED,
		OrderPaymentState: models.OrderPaymentState{
			FiatPaymentState: models.FiatPaymentState{
				PaymentTransactionID: "CAP-E2E",
			},
		},
	})
	svc.SetOrderRepo(orderRepo)

	payload := `{
		"id": "WH-E2E-REFUND",
		"event_type": "PAYMENT.CAPTURE.REFUNDED",
		"resource": {
			"id": "PP-REFUND-E2E",
			"status": "COMPLETED",
			"amount": {"currency_code": "EUR", "value": "25.00"},
			"links": [
				{"href": "https://api.paypal.com/v2/payments/captures/CAP-E2E", "rel": "up"}
			]
		}
	}`

	err := svc.HandleWebhook(context.Background(), "paypal", []byte(payload), paypalHeaders())
	require.NoError(t, err)

	require.Len(t, orderRepo.savedOrders, 1)
	assert.Equal(t, models.OrderState_REFUNDED, orderRepo.savedOrders[0].State)
}

func TestE2E_PayPal_DisputeCreated_StoresMetadata(t *testing.T) {
	ts := newPayPalWebhookServer(t)
	defer ts.Close()

	reg := newMockFiatRegistry()
	registerPayPalProvider(t, reg, ts.URL)
	svc, _ := newFiatTestService(t, reg)

	orderRepo := newMockOrderRepo()
	orderRepo.addOrder(&models.Order{
		ID:    "pp-dispute-order",
		State: models.OrderState_SHIPPED,
	})
	svc.SetOrderRepo(orderRepo)

	payload := `{
		"id": "WH-E2E-DISPUTE",
		"event_type": "CUSTOMER.DISPUTE.CREATED",
		"resource": {
			"dispute_id": "PP-D-E2E-001",
			"reason": "MERCHANDISE_OR_SERVICE_NOT_RECEIVED",
			"status": "OPEN",
			"disputed_transactions": [{
				"seller_transaction_id": "CAP-DISPUTE-E2E",
				"custom": "pp-dispute-order"
			}],
			"dispute_amount": {"currency_code": "USD", "value": "30.00"}
		}
	}`

	err := svc.HandleWebhook(context.Background(), "paypal", []byte(payload), paypalHeaders())
	require.NoError(t, err)

	meta, ok := orderRepo.mergedMeta["pp-dispute-order"]
	require.True(t, ok, "dispute metadata should be stored")
	assert.Equal(t, "opened", meta["fiat_dispute_status"])
	assert.Equal(t, "PP-D-E2E-001", meta["fiat_dispute_id"])
	assert.Equal(t, "MERCHANDISE_OR_SERVICE_NOT_RECEIVED", meta["fiat_dispute_reason"])
}

func TestE2E_PayPal_DisputeResolved_Lost_TransitionsToRefunded(t *testing.T) {
	ts := newPayPalWebhookServer(t)
	defer ts.Close()

	reg := newMockFiatRegistry()
	registerPayPalProvider(t, reg, ts.URL)
	svc, _ := newFiatTestService(t, reg)

	orderRepo := newMockOrderRepo()
	orderRepo.addOrder(&models.Order{
		ID:    "pp-dispute-lost",
		State: models.OrderState_SHIPPED,
	})
	svc.SetOrderRepo(orderRepo)

	payload := `{
		"id": "WH-E2E-DR-LOST",
		"event_type": "CUSTOMER.DISPUTE.RESOLVED",
		"resource": {
			"dispute_id": "PP-D-E2E-LOST",
			"reason": "UNAUTHORIZED",
			"dispute_outcome": {"outcome_code": "RESOLVED_BUYER_FAVOUR"},
			"disputed_transactions": [{
				"seller_transaction_id": "CAP-DR-LOST",
				"custom": "pp-dispute-lost"
			}],
			"dispute_amount": {"currency_code": "GBP", "value": "75.00"}
		}
	}`

	err := svc.HandleWebhook(context.Background(), "paypal", []byte(payload), paypalHeaders())
	require.NoError(t, err)

	meta := orderRepo.mergedMeta["pp-dispute-lost"]
	assert.Equal(t, "resolved", meta["fiat_dispute_status"])
	assert.Equal(t, "lost", meta["fiat_dispute_outcome"])

	require.Len(t, orderRepo.savedOrders, 1)
	assert.Equal(t, models.OrderState_REFUNDED, orderRepo.savedOrders[0].State)
}

func TestE2E_PayPal_DisputeResolved_Won_TransitionsToResolved(t *testing.T) {
	ts := newPayPalWebhookServer(t)
	defer ts.Close()

	reg := newMockFiatRegistry()
	registerPayPalProvider(t, reg, ts.URL)
	svc, _ := newFiatTestService(t, reg)

	orderRepo := newMockOrderRepo()
	orderRepo.addOrder(&models.Order{
		ID:    "pp-dispute-won",
		State: models.OrderState_SHIPPED,
	})
	svc.SetOrderRepo(orderRepo)

	payload := `{
		"id": "WH-E2E-DR-WON",
		"event_type": "CUSTOMER.DISPUTE.RESOLVED",
		"resource": {
			"dispute_id": "PP-D-E2E-WON",
			"reason": "MERCHANDISE_OR_SERVICE_NOT_AS_DESCRIBED",
			"dispute_outcome": {"outcome_code": "RESOLVED_SELLER_FAVOUR"},
			"disputed_transactions": [{
				"seller_transaction_id": "CAP-DR-WON",
				"custom": "pp-dispute-won"
			}],
			"dispute_amount": {"currency_code": "USD", "value": "50.00"}
		}
	}`

	err := svc.HandleWebhook(context.Background(), "paypal", []byte(payload), paypalHeaders())
	require.NoError(t, err)

	meta := orderRepo.mergedMeta["pp-dispute-won"]
	assert.Equal(t, "resolved", meta["fiat_dispute_status"])
	assert.Equal(t, "won", meta["fiat_dispute_outcome"])

	require.Len(t, orderRepo.savedOrders, 1)
	assert.Equal(t, models.OrderState_RESOLVED, orderRepo.savedOrders[0].State,
		"dispute won → RESOLVED (seller keeps funds)")
}

func TestE2E_PayPal_PaymentFailed_NoStateChange(t *testing.T) {
	ts := newPayPalWebhookServer(t)
	defer ts.Close()

	reg := newMockFiatRegistry()
	registerPayPalProvider(t, reg, ts.URL)
	svc, _ := newFiatTestService(t, reg)

	orderRepo := newMockOrderRepo()
	orderRepo.addOrder(&models.Order{
		ID:    "pp-fail-order",
		State: models.OrderState_AWAITING_PAYMENT,
	})
	svc.SetOrderRepo(orderRepo)

	payload := `{
		"id": "WH-E2E-FAIL",
		"event_type": "PAYMENT.CAPTURE.DENIED",
		"resource": {
			"id": "CAP-DENIED-E2E",
			"status": "DENIED",
			"custom_id": "pp-fail-order"
		}
	}`

	err := svc.HandleWebhook(context.Background(), "paypal", []byte(payload), paypalHeaders())
	require.NoError(t, err)

	assert.Equal(t, models.OrderState_AWAITING_PAYMENT, orderRepo.orders["pp-fail-order"].State,
		"PaymentFailed should not change order state")
}

func TestE2E_PayPal_SaleRefunded_OrderTransition(t *testing.T) {
	ts := newPayPalWebhookServer(t)
	defer ts.Close()

	reg := newMockFiatRegistry()
	registerPayPalProvider(t, reg, ts.URL)
	svc, _ := newFiatTestService(t, reg)

	orderRepo := newMockOrderRepo()
	orderRepo.addOrder(&models.Order{
		ID:    "pp-sale-refund-order",
		State: models.OrderState_SHIPPED,
		OrderPaymentState: models.OrderPaymentState{
			FiatPaymentState: models.FiatPaymentState{
				PaymentTransactionID: "CAP-SALE-E2E",
			},
		},
	})
	svc.SetOrderRepo(orderRepo)

	payload := `{
		"id": "WH-E2E-SALE-REFUND",
		"event_type": "PAYMENT.SALE.REFUNDED",
		"resource": {
			"id": "SALE-REF-E2E",
			"status": "COMPLETED",
			"amount": {"currency_code": "USD", "value": "19.99"},
			"links": [
				{"href": "https://api.paypal.com/v2/payments/captures/CAP-SALE-E2E", "rel": "up"}
			]
		}
	}`

	err := svc.HandleWebhook(context.Background(), "paypal", []byte(payload), paypalHeaders())
	require.NoError(t, err)

	require.Len(t, orderRepo.savedOrders, 1)
	assert.Equal(t, models.OrderState_REFUNDED, orderRepo.savedOrders[0].State)
}

// ==================== Stripe E2E Tests ====================

func TestE2E_Stripe_PaymentSucceeded_OrderTransition(t *testing.T) {
	reg := newMockFiatRegistry()
	registerStripeProvider(reg)
	svc, _ := newFiatTestService(t, reg)

	orderRepo := newMockOrderRepo()
	orderRepo.addOrder(&models.Order{
		ID:    "stripe-order-001",
		State: models.OrderState_AWAITING_PAYMENT,
	})
	svc.SetOrderRepo(orderRepo)

	var handled *contracts.WebhookEvent
	svc.SetWebhookHandler(func(_ context.Context, evt *contracts.WebhookEvent) error {
		handled = evt
		return nil
	})

	piJSON, _ := json.Marshal(map[string]interface{}{
		"id":       "pi_e2e_001",
		"object":   "payment_intent",
		"status":   "succeeded",
		"amount":   3999,
		"currency": "usd",
		"metadata": map[string]string{"order_id": "stripe-order-001"},
		"charges": map[string]interface{}{
			"data": []map[string]interface{}{
				{"payment_method_details": map[string]interface{}{
					"type": "card",
					"card": map[string]string{"brand": "visa", "last4": "4242"},
				}},
			},
		},
	})
	payload, sig := stripeWebhookPayload("payment_intent.succeeded", piJSON)

	err := svc.HandleWebhook(context.Background(), "stripe", payload, map[string]string{
		"Stripe-Signature": sig,
	})
	require.NoError(t, err)
	require.NotNil(t, handled)

	assert.Equal(t, contracts.WebhookPaymentSucceeded, handled.Type)
	assert.Equal(t, "stripe", handled.ProviderID)
	assert.Equal(t, "pi_e2e_001", handled.PaymentID)
	assert.Equal(t, "stripe-order-001", handled.OrderID)
	assert.Equal(t, int64(3999), handled.Amount)
	assert.Equal(t, "USD", handled.Currency)
}

func TestE2E_Stripe_ChargeRefunded_OrderTransition(t *testing.T) {
	reg := newMockFiatRegistry()
	registerStripeProvider(reg)
	svc, _ := newFiatTestService(t, reg)

	orderRepo := newMockOrderRepo()
	orderRepo.addOrder(&models.Order{
		ID:    "stripe-refund-order",
		State: models.OrderState_SHIPPED,
	})
	svc.SetOrderRepo(orderRepo)

	chargeJSON, _ := json.Marshal(map[string]interface{}{
		"id":              "ch_e2e_refund",
		"object":          "charge",
		"amount_refunded": 2500,
		"currency":        "eur",
		"payment_intent": map[string]interface{}{
			"id":       "pi_e2e_refund",
			"metadata": map[string]string{"order_id": "stripe-refund-order"},
		},
		"refunds": map[string]interface{}{
			"data": []map[string]interface{}{
				{"id": "re_e2e_001", "amount": 2500, "currency": "eur"},
			},
		},
	})
	payload, sig := stripeWebhookPayload("charge.refunded", chargeJSON)

	err := svc.HandleWebhook(context.Background(), "stripe", payload, map[string]string{
		"Stripe-Signature": sig,
	})
	require.NoError(t, err)

	require.Len(t, orderRepo.savedOrders, 1)
	assert.Equal(t, models.OrderState_REFUNDED, orderRepo.savedOrders[0].State)
}

func TestE2E_Stripe_DisputeCreated_StoresMetadata(t *testing.T) {
	reg := newMockFiatRegistry()
	registerStripeProvider(reg)
	svc, _ := newFiatTestService(t, reg)

	orderRepo := newMockOrderRepo()
	orderRepo.addOrder(&models.Order{
		ID:    "stripe-dispute-order",
		State: models.OrderState_SHIPPED,
	})
	svc.SetOrderRepo(orderRepo)

	disputeJSON, _ := json.Marshal(map[string]interface{}{
		"id":     "dp_e2e_001",
		"object": "dispute",
		"reason": "product_not_received",
		"status": "needs_response",
		"payment_intent": map[string]interface{}{
			"id":       "pi_e2e_dispute",
			"metadata": map[string]string{"order_id": "stripe-dispute-order"},
		},
	})
	payload, sig := stripeWebhookPayload("charge.dispute.created", disputeJSON)

	err := svc.HandleWebhook(context.Background(), "stripe", payload, map[string]string{
		"Stripe-Signature": sig,
	})
	require.NoError(t, err)

	meta, ok := orderRepo.mergedMeta["stripe-dispute-order"]
	require.True(t, ok)
	assert.Equal(t, "opened", meta["fiat_dispute_status"])
	assert.Equal(t, "dp_e2e_001", meta["fiat_dispute_id"])
}

func TestE2E_Stripe_DisputeClosed_Lost_TransitionsToRefunded(t *testing.T) {
	reg := newMockFiatRegistry()
	registerStripeProvider(reg)
	svc, _ := newFiatTestService(t, reg)

	orderRepo := newMockOrderRepo()
	orderRepo.addOrder(&models.Order{
		ID:    "stripe-dispute-lost",
		State: models.OrderState_SHIPPED,
	})
	svc.SetOrderRepo(orderRepo)

	disputeJSON, _ := json.Marshal(map[string]interface{}{
		"id":     "dp_e2e_lost",
		"object": "dispute",
		"reason": "fraudulent",
		"status": "lost",
		"payment_intent": map[string]interface{}{
			"id":       "pi_e2e_dlost",
			"metadata": map[string]string{"order_id": "stripe-dispute-lost"},
		},
	})
	payload, sig := stripeWebhookPayload("charge.dispute.closed", disputeJSON)

	err := svc.HandleWebhook(context.Background(), "stripe", payload, map[string]string{
		"Stripe-Signature": sig,
	})
	require.NoError(t, err)

	meta := orderRepo.mergedMeta["stripe-dispute-lost"]
	assert.Equal(t, "resolved", meta["fiat_dispute_status"])
	assert.Equal(t, "lost", meta["fiat_dispute_outcome"])

	require.Len(t, orderRepo.savedOrders, 1)
	assert.Equal(t, models.OrderState_REFUNDED, orderRepo.savedOrders[0].State)
}

func TestE2E_Stripe_PaymentCanceled_NoStateChange(t *testing.T) {
	reg := newMockFiatRegistry()
	registerStripeProvider(reg)
	svc, _ := newFiatTestService(t, reg)

	orderRepo := newMockOrderRepo()
	orderRepo.addOrder(&models.Order{
		ID:    "stripe-cancel-order",
		State: models.OrderState_AWAITING_PAYMENT,
	})
	svc.SetOrderRepo(orderRepo)

	piJSON, _ := json.Marshal(map[string]interface{}{
		"id":       "pi_e2e_cancel",
		"object":   "payment_intent",
		"status":   "canceled",
		"metadata": map[string]string{"order_id": "stripe-cancel-order"},
	})
	payload, sig := stripeWebhookPayload("payment_intent.canceled", piJSON)

	err := svc.HandleWebhook(context.Background(), "stripe", payload, map[string]string{
		"Stripe-Signature": sig,
	})
	require.NoError(t, err)

	assert.Equal(t, models.OrderState_AWAITING_PAYMENT, orderRepo.orders["stripe-cancel-order"].State)
}

// ==================== Webhook → Relay → Buyer E2E ====================

// TestE2E_Stripe_WebhookToBuyerRelay exercises the full payment notification
// chain: Stripe webhook → FiatPaymentAppService.HandleWebhook → webhook handler
// (mirrors options.go wiring) → buildFiatPaymentData → RelayPaymentToBuyer →
// P2P message sent to buyer via messenger.
func TestE2E_Stripe_WebhookToBuyerRelay(t *testing.T) {
	const (
		orderID   = "stripe-relay-order-001"
		paymentID = "pi_e2e_relay_001"
	)
	buyerPeerID, err := peer.Decode("QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG")
	require.NoError(t, err)

	fiatReg := newMockFiatRegistry()
	registerStripeProvider(fiatReg)
	fiatSvc, _ := newFiatTestService(t, fiatReg)

	orderRepo := newMockOrderRepo()
	orderRepo.addOrder(&models.Order{
		ID:    orderID,
		State: models.OrderState_AWAITING_PAYMENT,
		Open:  true,
	})
	fiatSvc.SetOrderRepo(orderRepo)

	mockMsgr := &capturingMessenger{}
	sellerOrderSvc := coreorder.NewTestOrderAppService(t, coreorder.OrderAppServiceConfig{
		Messenger: mockMsgr,
		Signer:    newMockSigner(),
	})
	coreorder.SeedOrderWithBuyer(t, sellerOrderSvc, orderID, buyerPeerID.String(), models.OrderState_AWAITING_PAYMENT)

	fiatSvc.SetWebhookHandler(func(ctx context.Context, event *contracts.WebhookEvent) error {
		pd, err := buildFiatPaymentData(event)
		if err != nil {
			return err
		}
		return sellerOrderSvc.RelayPaymentToBuyer(ctx, event.OrderID, pd)
	})

	piJSON, _ := json.Marshal(map[string]interface{}{
		"id":       paymentID,
		"object":   "payment_intent",
		"status":   "succeeded",
		"amount":   3999,
		"currency": "usd",
		"metadata": map[string]string{"order_id": orderID},
		"charges": map[string]interface{}{
			"data": []map[string]interface{}{
				{"payment_method_details": map[string]interface{}{
					"type": "card",
					"card": map[string]string{"brand": "visa", "last4": "4242"},
				}},
			},
		},
	})
	payload, sig := stripeWebhookPayload("payment_intent.succeeded", piJSON)

	err = fiatSvc.HandleWebhook(context.Background(), "stripe", payload, map[string]string{
		"Stripe-Signature": sig,
	})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return mockMsgr.called.Load() >= 1
	}, 2*time.Second, 50*time.Millisecond, "messenger should send payment relay to buyer")

	assert.Equal(t, buyerPeerID, mockMsgr.getLastPeer())
	assert.NotNil(t, mockMsgr.getLastMsg())
}

// TestE2E_PayPal_WebhookToBuyerRelay exercises the same chain for PayPal:
// PayPal webhook → HandleWebhook → buildFiatPaymentData → RelayPaymentToBuyer → P2P message.
func TestE2E_PayPal_WebhookToBuyerRelay(t *testing.T) {
	const orderID = "paypal-relay-order-001"
	buyerPeerID, err := peer.Decode("QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG")
	require.NoError(t, err)

	ts := newPayPalWebhookServer(t)
	defer ts.Close()

	fiatReg := newMockFiatRegistry()
	registerPayPalProvider(t, fiatReg, ts.URL)
	fiatSvc, _ := newFiatTestService(t, fiatReg)

	orderRepo := newMockOrderRepo()
	orderRepo.addOrder(&models.Order{
		ID:    orderID,
		State: models.OrderState_AWAITING_PAYMENT,
		Open:  true,
	})
	fiatSvc.SetOrderRepo(orderRepo)

	mockMsgr := &capturingMessenger{}
	sellerOrderSvc := coreorder.NewTestOrderAppService(t, coreorder.OrderAppServiceConfig{
		Messenger: mockMsgr,
		Signer:    newMockSigner(),
	})
	coreorder.SeedOrderWithBuyer(t, sellerOrderSvc, orderID, buyerPeerID.String(), models.OrderState_AWAITING_PAYMENT)

	fiatSvc.SetWebhookHandler(func(ctx context.Context, event *contracts.WebhookEvent) error {
		pd, err := buildFiatPaymentData(event)
		if err != nil {
			return err
		}
		return sellerOrderSvc.RelayPaymentToBuyer(ctx, event.OrderID, pd)
	})

	payload := fmt.Sprintf(`{
		"id": "WH-E2E-RELAY-PP",
		"event_type": "CHECKOUT.ORDER.COMPLETED",
		"resource": {
			"id": "PP-RELAY-ORDER-E2E",
			"status": "COMPLETED",
			"custom_id": "%s",
			"purchase_units": [{
				"custom_id": "%s",
				"amount": {"currency_code": "USD", "value": "49.99"},
				"payee": {"merchant_id": "SELLER-RELAY-E2E"}
			}]
		}
	}`, orderID, orderID)

	err = fiatSvc.HandleWebhook(context.Background(), "paypal", []byte(payload), paypalHeaders())
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return mockMsgr.called.Load() >= 1
	}, 2*time.Second, 50*time.Millisecond, "messenger should send PayPal payment relay to buyer")

	assert.Equal(t, buyerPeerID, mockMsgr.getLastPeer())
	assert.NotNil(t, mockMsgr.getLastMsg())
}

// TestE2E_Stripe_WebhookIdempotency_BuyerCalledOnce verifies that duplicate
// Stripe webhooks only relay to the buyer once (FiatPaymentAppService dedup).
func TestE2E_Stripe_WebhookIdempotency_BuyerCalledOnce(t *testing.T) {
	const orderID = "stripe-idem-relay-001"
	buyerPeerID, _ := peer.Decode("QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG")

	fiatReg := newMockFiatRegistry()
	registerStripeProvider(fiatReg)
	fiatSvc, _ := newFiatTestService(t, fiatReg)

	orderRepo := newMockOrderRepo()
	orderRepo.addOrder(&models.Order{
		ID:    orderID,
		State: models.OrderState_AWAITING_PAYMENT,
		Open:  true,
	})
	fiatSvc.SetOrderRepo(orderRepo)

	mockMsgr := &capturingMessenger{}
	sellerOrderSvc := coreorder.NewTestOrderAppService(t, coreorder.OrderAppServiceConfig{
		Messenger: mockMsgr,
		Signer:    newMockSigner(),
	})
	coreorder.SeedOrderWithBuyer(t, sellerOrderSvc, orderID, buyerPeerID.String(), models.OrderState_AWAITING_PAYMENT)

	fiatSvc.SetWebhookHandler(func(ctx context.Context, event *contracts.WebhookEvent) error {
		pd, err := buildFiatPaymentData(event)
		if err != nil {
			return err
		}
		return sellerOrderSvc.RelayPaymentToBuyer(ctx, event.OrderID, pd)
	})

	piJSON, _ := json.Marshal(map[string]interface{}{
		"id": "pi_idem_001", "object": "payment_intent", "status": "succeeded",
		"amount": 1000, "currency": "usd",
		"metadata": map[string]string{"order_id": orderID},
		"charges":  map[string]interface{}{"data": []interface{}{}},
	})
	payload, sig := stripeWebhookPayload("payment_intent.succeeded", piJSON)
	headers := map[string]string{"Stripe-Signature": sig}

	require.NoError(t, fiatSvc.HandleWebhook(context.Background(), "stripe", payload, headers))
	require.NoError(t, fiatSvc.HandleWebhook(context.Background(), "stripe", payload, headers))

	require.Eventually(t, func() bool {
		return mockMsgr.called.Load() >= 1
	}, 2*time.Second, 50*time.Millisecond)

	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, int32(1), mockMsgr.called.Load(),
		"messenger should send only ONE relay despite duplicate webhooks")
}

// ==================== PlatformProviderOpts Tests ====================

func TestRegisterPlatformProvider_PayPalWithPartnerID(t *testing.T) {
	reg := newMockFiatRegistry()
	svc, _ := newFiatTestService(t, reg)

	svc.RegisterPlatformProvider("paypal", "pp-secret", "pp-client-id", "pp-webhook-id",
		&contracts.PlatformProviderOpts{PayPalPartnerID: "PARTNER-E2E"})

	p, err := reg.ForProvider("paypal")
	require.NoError(t, err)
	assert.Equal(t, "paypal", p.ProviderID())
}

func TestRegisterPlatformProvider_StripeNilOpts(t *testing.T) {
	reg := newMockFiatRegistry()
	svc, _ := newFiatTestService(t, reg)

	svc.RegisterPlatformProvider("stripe", "sk_test", "pk_test", "whsec_test", nil)

	p, err := reg.ForProvider("stripe")
	require.NoError(t, err)
	assert.Equal(t, "stripe", p.ProviderID())
}

func TestRegisterPlatformProvider_PayPalEmptyPartnerID(t *testing.T) {
	reg := newMockFiatRegistry()
	svc, _ := newFiatTestService(t, reg)

	svc.RegisterPlatformProvider("paypal", "pp-secret", "pp-client", "pp-wh",
		&contracts.PlatformProviderOpts{PayPalPartnerID: ""})

	p, err := reg.ForProvider("paypal")
	require.NoError(t, err)
	assert.Equal(t, "paypal", p.ProviderID())
}

// ==================== Cross-provider idempotency ====================

func TestE2E_DuplicateWebhook_SecondCallIsIdempotent(t *testing.T) {
	ts := newPayPalWebhookServer(t)
	defer ts.Close()

	reg := newMockFiatRegistry()
	registerPayPalProvider(t, reg, ts.URL)
	svc, _ := newFiatTestService(t, reg)

	orderRepo := newMockOrderRepo()
	orderRepo.addOrder(&models.Order{
		ID:    "pp-idem-order",
		State: models.OrderState_AWAITING_PAYMENT,
	})
	svc.SetOrderRepo(orderRepo)

	callCount := 0
	svc.SetWebhookHandler(func(_ context.Context, _ *contracts.WebhookEvent) error {
		callCount++
		return nil
	})

	payload := `{
		"id": "WH-IDEM-001",
		"event_type": "CHECKOUT.ORDER.COMPLETED",
		"resource": {
			"id": "ORDER-IDEM",
			"status": "COMPLETED",
			"custom_id": "pp-idem-order"
		}
	}`

	err1 := svc.HandleWebhook(context.Background(), "paypal", []byte(payload), paypalHeaders())
	require.NoError(t, err1)

	err2 := svc.HandleWebhook(context.Background(), "paypal", []byte(payload), paypalHeaders())
	require.NoError(t, err2)

	assert.Equal(t, 1, callCount, "webhook handler should only be called once (idempotent)")
}

// ==================== Multi-provider registry ====================

func TestE2E_MultiProvider_StripeAndPayPalCoexist(t *testing.T) {
	ts := newPayPalWebhookServer(t)
	defer ts.Close()

	reg := newMockFiatRegistry()
	registerStripeProvider(reg)
	registerPayPalProvider(t, reg, ts.URL)

	svc, _ := newFiatTestService(t, reg)
	orderRepo := newMockOrderRepo()
	orderRepo.addOrder(&models.Order{ID: "multi-stripe", State: models.OrderState_AWAITING_PAYMENT})
	orderRepo.addOrder(&models.Order{ID: "multi-paypal", State: models.OrderState_AWAITING_PAYMENT})
	svc.SetOrderRepo(orderRepo)

	events := make([]*contracts.WebhookEvent, 0, 2)
	svc.SetWebhookHandler(func(_ context.Context, evt *contracts.WebhookEvent) error {
		events = append(events, evt)
		return nil
	})

	piJSON, _ := json.Marshal(map[string]interface{}{
		"id": "pi_multi", "object": "payment_intent", "status": "succeeded",
		"amount": 1000, "currency": "usd",
		"metadata": map[string]string{"order_id": "multi-stripe"},
		"charges":  map[string]interface{}{"data": []interface{}{}},
	})
	stripePayload, stripeSig := stripeWebhookPayload("payment_intent.succeeded", piJSON)
	err := svc.HandleWebhook(context.Background(), "stripe", stripePayload, map[string]string{
		"Stripe-Signature": stripeSig,
	})
	require.NoError(t, err)

	paypalPayload := `{
		"id": "WH-MULTI-PP",
		"event_type": "CHECKOUT.ORDER.COMPLETED",
		"resource": {"id": "PP-MULTI", "status": "COMPLETED", "custom_id": "multi-paypal"}
	}`
	err = svc.HandleWebhook(context.Background(), "paypal", []byte(paypalPayload), paypalHeaders())
	require.NoError(t, err)

	require.Len(t, events, 2)
	assert.Equal(t, "stripe", events[0].ProviderID)
	assert.Equal(t, "multi-stripe", events[0].OrderID)
	assert.Equal(t, "paypal", events[1].ProviderID)
	assert.Equal(t, "multi-paypal", events[1].OrderID)
}
