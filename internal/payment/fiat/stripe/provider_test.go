package stripe

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	gostripe "github.com/stripe/stripe-go/v82"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testAPIVersion = gostripe.APIVersion

// newTestProvider creates an httptest.Server and a Provider configured to use it.
func newTestProvider(t *testing.T, handler http.Handler) (*httptest.Server, *Provider) {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	p := NewProvider(Config{
		SecretKey:      "sk_test_xxx",
		PublishableKey: "pk_test_xxx",
		WebhookSecret:  "whsec_test",
		Mode:           ModeDirect,
		BackendURL:     ts.URL,
	})
	return ts, p
}

// signWebhookPayload signs a payload using the webhook secret for test purposes.
func signWebhookPayload(payload []byte, secret string) string {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts))
	mac.Write([]byte("."))
	mac.Write(payload)
	sig := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("t=%s,v1=%s", ts, sig)
}

func TestProvider_ProviderID(t *testing.T) {
	p := NewProvider(Config{})
	assert.Equal(t, "stripe", p.ProviderID())
}

func TestProvider_CreatePayment_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/payment_intents", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)

		body, _ := io.ReadAll(r.Body)
		vals := string(body)
		assert.Contains(t, vals, "amount=2500")
		assert.Contains(t, vals, "currency=usd")
		assert.Contains(t, vals, "metadata[order_id]=order_123")

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":            "pi_test_123",
			"object":        "payment_intent",
			"amount":        2500,
			"currency":      "usd",
			"status":        "requires_payment_method",
			"client_secret": "pi_test_123_secret_xxx",
		})
	})

	_, p := newTestProvider(t, mux)

	session, err := p.CreatePayment(context.Background(), contracts.CreatePaymentParams{
		OrderID:  "order_123",
		Amount:   2500,
		Currency: "usd",
	})
	require.NoError(t, err)
	assert.Equal(t, "pi_test_123", session.SessionID)
	assert.Equal(t, contracts.CaptureAutomatic, session.CaptureMode)
	assert.Equal(t, "requires_payment_method", session.Status)
	require.NotNil(t, session.Stripe)
	assert.Equal(t, "pi_test_123_secret_xxx", session.Stripe.ClientSecret)
	assert.Equal(t, "pk_test_xxx", session.Stripe.PublishableKey)
}

func TestProvider_CreatePayment_ConnectedMode(t *testing.T) {
	var gotStripeAccount string
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/payment_intents", func(w http.ResponseWriter, r *http.Request) {
		gotStripeAccount = r.Header.Get("Stripe-Account")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":            "pi_connected",
			"object":        "payment_intent",
			"amount":        1000,
			"currency":      "usd",
			"status":        "requires_payment_method",
			"client_secret": "cs_xxx",
		})
	})

	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	p := NewProvider(Config{
		SecretKey:      "sk_test_platform",
		PublishableKey: "pk_test_platform",
		WebhookSecret:  "whsec_test",
		Mode:           ModeConnected,
		BackendURL:     ts.URL,
	})

	session, err := p.CreatePayment(context.Background(), contracts.CreatePaymentParams{
		OrderID:         "order_conn",
		Amount:          1000,
		Currency:        "usd",
		SellerAccountID: "acct_seller_123",
	})
	require.NoError(t, err)
	assert.Equal(t, "acct_seller_123", gotStripeAccount)
	require.NotNil(t, session.Stripe)
	assert.Equal(t, "acct_seller_123", session.Stripe.ConnectedAccountID)
	assert.Equal(t, "pk_test_platform", session.Stripe.PublishableKey)
}

func TestProvider_CreatePayment_Error(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/payment_intents", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"type":    "invalid_request_error",
				"message": "Amount must be at least 50 cents",
			},
		})
	})

	_, p := newTestProvider(t, mux)
	_, err := p.CreatePayment(context.Background(), contracts.CreatePaymentParams{
		OrderID: "order_err", Amount: 10, Currency: "usd",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "stripe")
}

func TestProvider_CapturePayment_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/payment_intents/pi_cap", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":       "pi_cap",
			"object":   "payment_intent",
			"amount":   5000,
			"currency": "eur",
			"status":   "succeeded",
			"payment_method": map[string]interface{}{
				"id":     "pm_123",
				"object": "payment_method",
				"type":   "card",
				"card": map[string]interface{}{
					"brand": "mastercard",
					"last4": "5678",
				},
			},
		})
	})

	_, p := newTestProvider(t, mux)
	result, err := p.CapturePayment(context.Background(), "pi_cap")
	require.NoError(t, err)
	assert.Equal(t, "pi_cap", result.PaymentID)
	assert.Equal(t, "succeeded", result.Status)
	assert.Equal(t, int64(5000), result.Amount)
	assert.Equal(t, "eur", result.Currency)
	assert.Equal(t, "mastercard", result.PaymentMethod.Brand)
	assert.Equal(t, "5678", result.PaymentMethod.Last4)
}

func TestProvider_GetPayment_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/payment_intents/pi_get", func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.RawQuery, "expand")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":       "pi_get",
			"object":   "payment_intent",
			"amount":   3000,
			"currency": "gbp",
			"status":   "succeeded",
			"created":  1700000000,
			"payment_method": map[string]interface{}{
				"id":     "pm_456",
				"object": "payment_method",
				"type":   "card",
				"card":   map[string]interface{}{"brand": "visa", "last4": "1234"},
			},
		})
	})

	_, p := newTestProvider(t, mux)
	detail, err := p.GetPayment(context.Background(), "pi_get")
	require.NoError(t, err)
	assert.Equal(t, "pi_get", detail.PaymentID)
	assert.Equal(t, int64(3000), detail.Amount)
	assert.Equal(t, "gbp", detail.Currency)
	assert.Equal(t, "visa", detail.PaymentMethod.Brand)
	assert.Equal(t, "1234", detail.PaymentMethod.Last4)
}

func TestProvider_GetPayment_WithReceipt(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/payment_intents/pi_receipt", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":       "pi_receipt",
			"object":   "payment_intent",
			"amount":   999,
			"currency": "usd",
			"status":   "succeeded",
			"created":  1700000000,
			"latest_charge": map[string]interface{}{
				"id":          "ch_receipt",
				"object":      "charge",
				"receipt_url": "https://pay.stripe.com/receipts/xxx",
			},
		})
	})

	_, p := newTestProvider(t, mux)
	detail, err := p.GetPayment(context.Background(), "pi_receipt")
	require.NoError(t, err)
	assert.Equal(t, "https://pay.stripe.com/receipts/xxx", detail.ReceiptURL)
}

func TestProvider_ParseWebhook_MissingSignature(t *testing.T) {
	p := NewProvider(Config{WebhookSecret: "whsec_test"})
	_, err := p.ParseWebhook(context.Background(), []byte("{}"), map[string]string{})
	assert.Error(t, err)
	assert.ErrorIs(t, err, contracts.ErrWebhookSignature)
}

func TestProvider_ParseWebhook_InvalidSignature(t *testing.T) {
	p := NewProvider(Config{WebhookSecret: "whsec_test"})
	_, err := p.ParseWebhook(context.Background(), []byte("{}"), map[string]string{
		"Stripe-Signature": "t=123,v1=bad",
	})
	assert.Error(t, err)
	assert.ErrorIs(t, err, contracts.ErrWebhookSignature)
}

func TestProvider_ParseWebhook_PaymentSucceeded(t *testing.T) {
	secret := "whsec_test_parse"
	piJSON, _ := json.Marshal(map[string]interface{}{
		"id":       "pi_webhook",
		"object":   "payment_intent",
		"amount":   2999,
		"currency": "usd",
		"metadata": map[string]string{"order_id": "order_wh"},
	})
	eventPayload, _ := json.Marshal(map[string]interface{}{
		"id":          "evt_test_1",
		"type":        "payment_intent.succeeded",
		"api_version": testAPIVersion,
		"data":        map[string]interface{}{"object": json.RawMessage(piJSON)},
	})
	sig := signWebhookPayload(eventPayload, secret)

	p := NewProvider(Config{WebhookSecret: secret})
	event, err := p.ParseWebhook(context.Background(), eventPayload, map[string]string{
		"Stripe-Signature": sig,
	})
	require.NoError(t, err)
	assert.Equal(t, "evt_test_1", event.EventID)
	assert.Equal(t, contracts.WebhookPaymentSucceeded, event.Type)
	assert.Equal(t, "pi_webhook", event.PaymentID)
	assert.Equal(t, "order_wh", event.OrderID)
	assert.Equal(t, "fiat:USD", event.Coin)
	assert.Equal(t, int64(2999), event.Amount)
	assert.Equal(t, "USD", event.Currency)
}

func TestProvider_ParseWebhook_PaymentFailed(t *testing.T) {
	secret := "whsec_test_fail"
	piJSON, _ := json.Marshal(map[string]interface{}{
		"id":       "pi_failed",
		"object":   "payment_intent",
		"metadata": map[string]string{"order_id": "order_fail"},
	})
	eventPayload, _ := json.Marshal(map[string]interface{}{
		"id":          "evt_fail",
		"type":        "payment_intent.payment_failed",
		"api_version": testAPIVersion,
		"data":        map[string]interface{}{"object": json.RawMessage(piJSON)},
	})
	sig := signWebhookPayload(eventPayload, secret)

	p := NewProvider(Config{WebhookSecret: secret})
	event, err := p.ParseWebhook(context.Background(), eventPayload, map[string]string{
		"Stripe-Signature": sig,
	})
	require.NoError(t, err)
	assert.Equal(t, contracts.WebhookPaymentFailed, event.Type)
	assert.Equal(t, "pi_failed", event.PaymentID)
}

func TestProvider_ParseWebhook_Dispute(t *testing.T) {
	secret := "whsec_test_dispute"
	eventPayload, _ := json.Marshal(map[string]interface{}{
		"id":          "evt_dispute",
		"type":        "charge.dispute.created",
		"api_version": testAPIVersion,
		"data":        map[string]interface{}{"object": map[string]interface{}{}},
	})
	sig := signWebhookPayload(eventPayload, secret)

	p := NewProvider(Config{WebhookSecret: secret})
	event, err := p.ParseWebhook(context.Background(), eventPayload, map[string]string{
		"Stripe-Signature": sig,
	})
	require.NoError(t, err)
	assert.Equal(t, contracts.WebhookDisputeOpened, event.Type)
}

func TestProvider_ParseWebhook_AccountUpdated(t *testing.T) {
	secret := "whsec_test_acct"
	eventPayload, _ := json.Marshal(map[string]interface{}{
		"id":          "evt_acct",
		"type":        "account.updated",
		"account":     "acct_updated_123",
		"api_version": testAPIVersion,
		"data":        map[string]interface{}{"object": map[string]interface{}{"id": "acct_updated_123"}},
	})
	sig := signWebhookPayload(eventPayload, secret)

	p := NewProvider(Config{WebhookSecret: secret})
	event, err := p.ParseWebhook(context.Background(), eventPayload, map[string]string{
		"Stripe-Signature": sig,
	})
	require.NoError(t, err)
	assert.Equal(t, contracts.WebhookAccountUpdated, event.Type)
	assert.Equal(t, "acct_updated_123", event.AccountID)
}

func TestProvider_ParseWebhook_CaseInsensitiveHeader(t *testing.T) {
	secret := "whsec_ci"
	eventPayload, _ := json.Marshal(map[string]interface{}{
		"id":          "evt_ci",
		"type":        "charge.refunded",
		"api_version": testAPIVersion,
		"data":        map[string]interface{}{"object": map[string]interface{}{}},
	})
	sig := signWebhookPayload(eventPayload, secret)

	p := NewProvider(Config{WebhookSecret: secret})
	event, err := p.ParseWebhook(context.Background(), eventPayload, map[string]string{
		"stripe-signature": sig,
	})
	require.NoError(t, err)
	assert.Equal(t, contracts.WebhookRefundCreated, event.Type)
}

func TestProvider_GetOnboardingURL_AutoCreateAccount(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/accounts", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":   "acct_auto_created",
			"type": "standard",
		})
	})
	mux.HandleFunc("/v1/account_links", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		body, _ := io.ReadAll(r.Body)
		assert.Contains(t, string(body), "account=acct_auto_created")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"url":        "https://connect.stripe.com/setup/s/onboard-auto",
			"expires_at": time.Now().Add(5 * time.Minute).Unix(),
		})
	})

	_, p := newTestProvider(t, mux)
	result, err := p.GetOnboardingURL(context.Background(), contracts.OnboardingParams{
		SellerID:   "seller-123",
		ReturnURL:  "https://app.com/return",
		RefreshURL: "https://app.com/refresh",
	})
	require.NoError(t, err)
	assert.Equal(t, "https://connect.stripe.com/setup/s/onboard-auto", result.URL)
	assert.Equal(t, "acct_auto_created", result.AccountID)
}

func TestProvider_GetOnboardingURL_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/account_links", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		body, _ := io.ReadAll(r.Body)
		assert.Contains(t, string(body), "account=acct_onboard")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"object":     "account_link",
			"url":        "https://connect.stripe.com/setup/xxx",
			"expires_at": time.Now().Add(5 * time.Minute).Unix(),
		})
	})

	_, p := newTestProvider(t, mux)
	result, err := p.GetOnboardingURL(context.Background(), contracts.OnboardingParams{
		AccountID:  "acct_onboard",
		ReturnURL:  "https://app.com/return",
		RefreshURL: "https://app.com/refresh",
	})
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(result.URL, "https://connect.stripe.com/"))
	assert.Equal(t, "acct_onboard", result.AccountID)
}

func TestProvider_HandleOnboardingCallback_MissingAccountID(t *testing.T) {
	p := NewProvider(Config{SecretKey: "sk_test"})
	_, err := p.HandleOnboardingCallback(context.Background(), contracts.CallbackParams{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "AccountID")
}

func TestProvider_HandleOnboardingCallback_Active(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/accounts/acct_active", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":              "acct_active",
			"object":          "account",
			"email":           "seller@example.com",
			"charges_enabled": true,
			"payouts_enabled": true,
		})
	})

	_, p := newTestProvider(t, mux)
	acct, err := p.HandleOnboardingCallback(context.Background(), contracts.CallbackParams{
		AccountID: "acct_active",
	})
	require.NoError(t, err)
	assert.Equal(t, "acct_active", acct.AccountID)
	assert.Equal(t, "seller@example.com", acct.Email)
	assert.Equal(t, "active", acct.Status)
}

func TestProvider_HandleOnboardingCallback_Pending(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/accounts/acct_pending", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":              "acct_pending",
			"object":          "account",
			"charges_enabled": false,
			"payouts_enabled": false,
			"requirements": map[string]interface{}{
				"currently_due": []string{"external_account"},
				"errors":        []interface{}{},
			},
		})
	})

	_, p := newTestProvider(t, mux)
	acct, err := p.HandleOnboardingCallback(context.Background(), contracts.CallbackParams{
		AccountID: "acct_pending",
	})
	require.NoError(t, err)
	assert.Equal(t, "pending", acct.Status)
}

func TestProvider_GetAccountStatus_Active(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/accounts/acct_stat_active", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":              "acct_stat_active",
			"object":          "account",
			"charges_enabled": true,
			"payouts_enabled": true,
		})
	})

	_, p := newTestProvider(t, mux)
	status, err := p.GetAccountStatus(context.Background(), "acct_stat_active")
	require.NoError(t, err)
	assert.True(t, status.IsActive)
	assert.Equal(t, "active", status.Status)
	assert.True(t, status.ChargesEnabled)
	assert.True(t, status.PayoutsEnabled)
}

func TestProvider_GetAccountStatus_Restricted(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/accounts/acct_restricted", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":              "acct_restricted",
			"object":          "account",
			"charges_enabled": false,
			"payouts_enabled": false,
			"requirements": map[string]interface{}{
				"currently_due": []string{},
				"errors": []interface{}{
					map[string]interface{}{"reason": "identity_verification_required"},
				},
			},
		})
	})

	_, p := newTestProvider(t, mux)
	status, err := p.GetAccountStatus(context.Background(), "acct_restricted")
	require.NoError(t, err)
	assert.False(t, status.IsActive)
	assert.Equal(t, "restricted", status.Status)
	assert.Contains(t, status.Requirements, "identity_verification_required")
}

func TestProvider_MapStripeStatus(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{"succeeded maps to succeeded", "succeeded", "succeeded"},
		{"canceled maps to failed", "canceled", "failed"},
		{"requires_payment_method maps to pending", "requires_payment_method", "pending"},
		{"requires_confirmation maps to pending", "requires_confirmation", "pending"},
		{"requires_action maps to pending", "requires_action", "pending"},
		{"processing maps to pending", "processing", "pending"},
		{"unknown passes through", "custom_status", "custom_status"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expect, mapStripeStatus(gostripe.PaymentIntentStatus(tt.input)))
		})
	}
}
