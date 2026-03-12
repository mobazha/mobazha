package paypal

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestServer creates an httptest.Server and a Provider configured to use it,
// with a pre-set access token to skip OAuth.
func newTestServer(t *testing.T, handler http.Handler) (*httptest.Server, *Provider) {
	t.Helper()
	ts := httptest.NewServer(handler)
	p := &Provider{
		config: Config{
			ClientID:     "test-client-id",
			ClientSecret: "test-secret",
			WebhookID:    "test-webhook-id",
			Mode:         ModeDirect,
			Sandbox:      true,
		},
		client: &apiClient{
			clientID:     "test-client-id",
			clientSecret: "test-secret",
			baseURL:      ts.URL,
			httpClient:   &http.Client{Timeout: 5 * time.Second},
			accessToken:  "test-access-token",
			tokenExpiry:  time.Now().Add(1 * time.Hour),
		},
		sigCache: newSignatureCache(),
	}
	return ts, p
}

// newWebhookTestProvider creates a Provider backed by a mock server that auto-approves
// webhook signature verification. Use for tests focused on event parsing logic.
func newWebhookTestProvider(t *testing.T) (*httptest.Server, *Provider) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/notifications/verify-webhook-signature", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(webhookVerifyResponse{VerificationStatus: "SUCCESS"})
	})
	return newTestServer(t, mux)
}

// validWebhookHeaders returns headers that satisfy the required PayPal webhook header checks.
func validWebhookHeaders() map[string]string {
	return map[string]string{
		"Paypal-Transmission-Id":   "test-transmission-id",
		"Paypal-Transmission-Sig":  "test-sig",
		"Paypal-Transmission-Time": "2026-03-11T00:00:00Z",
		"Paypal-Auth-Algo":         "SHA256withRSA",
		"Paypal-Cert-Url":          "https://api.paypal.com/cert.pem",
	}
}

func TestProvider_ProviderID(t *testing.T) {
	p := NewProvider(Config{})
	assert.Equal(t, "paypal", p.ProviderID())
}

func TestProvider_CreatePayment_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/checkout/orders", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)

		var req orderRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, "CAPTURE", req.Intent)
		assert.Len(t, req.PurchaseUnits, 1)
		assert.Equal(t, "29.99", req.PurchaseUnits[0].Amount.Value)
		assert.Equal(t, "USD", req.PurchaseUnits[0].Amount.CurrencyCode)
		assert.Equal(t, "order-123", req.PurchaseUnits[0].CustomID)

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(orderResponse{
			ID:     "ORDER-PPL-001",
			Status: "CREATED",
			Links: []link{
				{Href: "https://paypal.com/approve/ORDER-PPL-001", Rel: "approve", Method: "GET"},
			},
		})
	})

	ts, p := newTestServer(t, mux)
	defer ts.Close()

	session, err := p.CreatePayment(context.Background(), contracts.CreatePaymentParams{
		OrderID:  "order-123",
		Amount:   2999,
		Currency: "USD",
	})
	require.NoError(t, err)

	assert.Equal(t, "ORDER-PPL-001", session.SessionID)
	assert.Equal(t, contracts.CaptureManual, session.CaptureMode)
	assert.Equal(t, "CREATED", session.Status)
	assert.Contains(t, session.ApproveURL, "paypal.com/approve")
	require.NotNil(t, session.PayPal)
	assert.Equal(t, "ORDER-PPL-001", session.PayPal.OrderID)
	assert.Equal(t, "test-client-id", session.PayPal.ClientID)
}

func TestProvider_CreatePayment_PartnerMode_WithPayee(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/checkout/orders", func(w http.ResponseWriter, r *http.Request) {
		var req orderRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		require.NotNil(t, req.PurchaseUnits[0].Payee, "Partner mode should include payee")
		assert.Equal(t, "MERCHANT-123", req.PurchaseUnits[0].Payee.MerchantID)

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(orderResponse{ID: "ORDER-002", Status: "CREATED"})
	})

	ts, p := newTestServer(t, mux)
	p.config.Mode = ModePartner
	defer ts.Close()

	session, err := p.CreatePayment(context.Background(), contracts.CreatePaymentParams{
		OrderID:         "order-456",
		Amount:          1000,
		Currency:        "EUR",
		SellerAccountID: "MERCHANT-123",
	})
	require.NoError(t, err)
	assert.Equal(t, "ORDER-002", session.SessionID)
}

func TestProvider_CreatePayment_DirectMode_NoPayee(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/checkout/orders", func(w http.ResponseWriter, r *http.Request) {
		var req orderRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Nil(t, req.PurchaseUnits[0].Payee, "Direct mode should not include payee")

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(orderResponse{ID: "ORDER-003", Status: "CREATED"})
	})

	ts, p := newTestServer(t, mux)
	defer ts.Close()

	session, err := p.CreatePayment(context.Background(), contracts.CreatePaymentParams{
		OrderID:  "order-789",
		Amount:   500,
		Currency: "GBP",
	})
	require.NoError(t, err)
	assert.Equal(t, "ORDER-003", session.SessionID)
}

func TestProvider_CreatePayment_JPY_ZeroDecimal(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/checkout/orders", func(w http.ResponseWriter, r *http.Request) {
		var req orderRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, "5000", req.PurchaseUnits[0].Amount.Value, "JPY is zero-decimal")

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(orderResponse{ID: "ORDER-JPY", Status: "CREATED"})
	})

	ts, p := newTestServer(t, mux)
	defer ts.Close()

	session, err := p.CreatePayment(context.Background(), contracts.CreatePaymentParams{
		OrderID:  "order-jpy",
		Amount:   5000,
		Currency: "JPY",
	})
	require.NoError(t, err)
	assert.Equal(t, "ORDER-JPY", session.SessionID)
}

func TestProvider_CreatePayment_WithReturnAndCancelURL(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/checkout/orders", func(w http.ResponseWriter, r *http.Request) {
		var req orderRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		require.NotNil(t, req.ApplicationContext, "Should include application_context with URLs")
		assert.Equal(t, "https://example.com/return", req.ApplicationContext.ReturnURL)
		assert.Equal(t, "https://example.com/cancel", req.ApplicationContext.CancelURL)

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(orderResponse{ID: "ORDER-URL", Status: "CREATED"})
	})

	ts, p := newTestServer(t, mux)
	defer ts.Close()

	_, err := p.CreatePayment(context.Background(), contracts.CreatePaymentParams{
		OrderID:   "order-url",
		Amount:    100,
		Currency:  "USD",
		ReturnURL: "https://example.com/return",
		CancelURL: "https://example.com/cancel",
	})
	require.NoError(t, err)
}

func TestProvider_CreatePayment_APIError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/checkout/orders", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		w.Write([]byte(`{"name":"UNPROCESSABLE_ENTITY","message":"Invalid request"}`))
	})

	ts, p := newTestServer(t, mux)
	defer ts.Close()

	_, err := p.CreatePayment(context.Background(), contracts.CreatePaymentParams{
		OrderID:  "order-err",
		Amount:   100,
		Currency: "USD",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "paypal: create order")
}

func TestProvider_CapturePayment_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/checkout/orders/ORDER-001/capture", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		json.NewEncoder(w).Encode(orderResponse{
			ID:     "ORDER-001",
			Status: "COMPLETED",
			PurchaseUnits: []puResponse{{
				Amount: amount{CurrencyCode: "USD", Value: "29.99"},
			}},
		})
	})

	ts, p := newTestServer(t, mux)
	defer ts.Close()

	result, err := p.CapturePayment(context.Background(), "ORDER-001")
	require.NoError(t, err)
	assert.Equal(t, "ORDER-001", result.PaymentID)
	assert.Equal(t, "succeeded", result.Status)
	assert.Equal(t, int64(2999), result.Amount)
	assert.Equal(t, "USD", result.Currency)
	assert.Equal(t, "paypal", result.PaymentMethod.Type)
}

func TestProvider_CapturePayment_Declined(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/checkout/orders/ORDER-FAIL/capture", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(orderResponse{
			ID:     "ORDER-FAIL",
			Status: "DECLINED",
		})
	})

	ts, p := newTestServer(t, mux)
	defer ts.Close()

	result, err := p.CapturePayment(context.Background(), "ORDER-FAIL")
	require.NoError(t, err)
	assert.Equal(t, "failed", result.Status)
}

func TestProvider_GetPayment_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/checkout/orders/ORDER-DETAIL", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		json.NewEncoder(w).Encode(orderResponse{
			ID:         "ORDER-DETAIL",
			Status:     "COMPLETED",
			CreateTime: "2026-02-28T12:00:00Z",
			PurchaseUnits: []puResponse{{
				Amount: amount{CurrencyCode: "EUR", Value: "50.00"},
				Payee:  &payee{MerchantID: "SELLER-MID"},
			}},
		})
	})

	ts, p := newTestServer(t, mux)
	defer ts.Close()

	detail, err := p.GetPayment(context.Background(), "ORDER-DETAIL")
	require.NoError(t, err)
	assert.Equal(t, "ORDER-DETAIL", detail.PaymentID)
	assert.Equal(t, "succeeded", detail.Status)
	assert.Equal(t, int64(5000), detail.Amount)
	assert.Equal(t, "EUR", detail.Currency)
	assert.Equal(t, "SELLER-MID", detail.SellerAccountID)
	assert.Equal(t, "paypal", detail.PaymentMethod.Brand)
	assert.False(t, detail.CreatedAt.IsZero(), "CreatedAt should be parsed")
}

func TestProvider_ParseWebhook_PaymentSucceeded_ResourceLevel(t *testing.T) {
	ts, p := newWebhookTestProvider(t)
	defer ts.Close()

	payload := `{
		"id": "WH-001",
		"event_type": "CHECKOUT.ORDER.COMPLETED",
		"resource": {
			"id": "ORDER-ABC",
			"status": "COMPLETED",
			"custom_id": "my-order-id"
		}
	}`

	event, err := p.ParseWebhook(context.Background(), []byte(payload), validWebhookHeaders())
	require.NoError(t, err)

	assert.Equal(t, "WH-001", event.EventID)
	assert.Equal(t, contracts.WebhookPaymentSucceeded, event.Type)
	assert.Equal(t, "paypal", event.ProviderID)
	assert.Equal(t, "ORDER-ABC", event.PaymentID)
	assert.Equal(t, "my-order-id", event.OrderID)
}

func TestProvider_ParseWebhook_PaymentSucceeded_PurchaseUnitsFallback(t *testing.T) {
	ts, p := newWebhookTestProvider(t)
	defer ts.Close()

	payload := `{
		"id": "WH-002",
		"event_type": "PAYMENT.CAPTURE.COMPLETED",
		"resource": {
			"id": "CAP-XYZ",
			"status": "COMPLETED",
			"purchase_units": [{
				"custom_id": "fallback-order-id",
				"amount": {"currency_code": "EUR", "value": "49.99"},
				"payee": {"merchant_id": "MERCH-999"}
			}]
		}
	}`

	event, err := p.ParseWebhook(context.Background(), []byte(payload), validWebhookHeaders())
	require.NoError(t, err)

	assert.Equal(t, contracts.WebhookPaymentSucceeded, event.Type)
	assert.Equal(t, "paypal", event.ProviderID)
	assert.Equal(t, "CAP-XYZ", event.PaymentID)
	assert.Equal(t, "fallback-order-id", event.OrderID)
	assert.Equal(t, "MERCH-999", event.AccountID)
	assert.Equal(t, "fiat:EUR", event.Coin)
	assert.Equal(t, int64(4999), event.Amount)
	assert.Equal(t, "EUR", event.Currency)
	assert.Equal(t, "paypal", event.PaymentMethod.Type)
}

func TestProvider_ParseWebhook_PaymentFailed(t *testing.T) {
	ts, p := newWebhookTestProvider(t)
	defer ts.Close()

	payload := `{
		"id": "WH-FAIL",
		"event_type": "PAYMENT.CAPTURE.DENIED",
		"resource": {
			"id": "CAP-FAIL",
			"status": "DENIED",
			"custom_id": "order-fail"
		}
	}`

	event, err := p.ParseWebhook(context.Background(), []byte(payload), validWebhookHeaders())
	require.NoError(t, err)
	assert.Equal(t, contracts.WebhookPaymentFailed, event.Type)
	assert.Equal(t, "CAP-FAIL", event.PaymentID)
	assert.Equal(t, "order-fail", event.OrderID)
}

func TestProvider_ParseWebhook_DisputeCreated(t *testing.T) {
	ts, p := newWebhookTestProvider(t)
	defer ts.Close()

	payload := `{"id": "WH-DISPUTE", "event_type": "CUSTOMER.DISPUTE.CREATED", "resource": {"id": "DISP-001"}}`

	event, err := p.ParseWebhook(context.Background(), []byte(payload), validWebhookHeaders())
	require.NoError(t, err)
	assert.Equal(t, contracts.WebhookDisputeOpened, event.Type)
}

func TestProvider_ParseWebhook_DisputeResolved(t *testing.T) {
	ts, p := newWebhookTestProvider(t)
	defer ts.Close()

	payload := `{"id": "WH-DR", "event_type": "CUSTOMER.DISPUTE.RESOLVED", "resource": {"id": "DISP-002"}}`

	event, err := p.ParseWebhook(context.Background(), []byte(payload), validWebhookHeaders())
	require.NoError(t, err)
	assert.Equal(t, contracts.WebhookDisputeResolved, event.Type)
}

func TestProvider_ParseWebhook_Refund(t *testing.T) {
	ts, p := newWebhookTestProvider(t)
	defer ts.Close()

	payload := `{"id": "WH-REFUND", "event_type": "PAYMENT.CAPTURE.REFUNDED", "resource": {"id": "REFUND-001"}}`

	event, err := p.ParseWebhook(context.Background(), []byte(payload), validWebhookHeaders())
	require.NoError(t, err)
	assert.Equal(t, contracts.WebhookRefundCreated, event.Type)
}

func TestProvider_ParseWebhook_AccountUpdated(t *testing.T) {
	ts, p := newWebhookTestProvider(t)
	defer ts.Close()

	payload := `{"id": "WH-ONBOARD", "event_type": "MERCHANT.ONBOARDING.COMPLETED", "resource": {"merchant_id": "M-001"}}`

	event, err := p.ParseWebhook(context.Background(), []byte(payload), validWebhookHeaders())
	require.NoError(t, err)
	assert.Equal(t, contracts.WebhookAccountUpdated, event.Type)
}

func TestProvider_ParseWebhook_UnknownType(t *testing.T) {
	ts, p := newWebhookTestProvider(t)
	defer ts.Close()

	payload := `{"id": "WH-UNKNOWN", "event_type": "SOME.UNKNOWN.EVENT", "resource": {}}`

	event, err := p.ParseWebhook(context.Background(), []byte(payload), validWebhookHeaders())
	require.NoError(t, err)
	assert.Equal(t, contracts.WebhookEventType("SOME.UNKNOWN.EVENT"), event.Type)
}

func TestProvider_ParseWebhook_InvalidJSON(t *testing.T) {
	ts, p := newWebhookTestProvider(t)
	defer ts.Close()

	// Invalid JSON fails during webhook signature verification (marshaling the raw payload)
	// or during event parsing. Either way, an error is expected.
	_, err := p.ParseWebhook(context.Background(), []byte("not-json"), validWebhookHeaders())
	require.Error(t, err)
}

func TestProvider_ParseWebhook_SignatureVerification_MissingHeaders(t *testing.T) {
	ts, p := newWebhookTestProvider(t)
	defer ts.Close()

	payload := `{"id": "WH-SIG", "event_type": "PAYMENT.CAPTURE.COMPLETED", "resource": {}}`

	_, err := p.ParseWebhook(context.Background(), []byte(payload), map[string]string{})
	require.Error(t, err)
	assert.ErrorIs(t, err, contracts.ErrWebhookSignature)
}

func TestProvider_GetOnboardingURL_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/customer/partner-referrals", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(partnerReferralResponse{
			Links: []link{
				{Href: "https://paypal.com/action/referral-123", Rel: "action_url"},
			},
		})
	})

	ts, p := newTestServer(t, mux)
	defer ts.Close()

	result, err := p.GetOnboardingURL(context.Background(), contracts.OnboardingParams{
		SellerID:  "seller-001",
		ReturnURL: "https://example.com/return",
	})
	require.NoError(t, err)
	assert.Equal(t, "https://paypal.com/action/referral-123", result.URL)
}

func TestProvider_HandleOnboardingCallback_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/customer/partners/PARTNER-ID/merchant-integrations/MERCH-001", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(merchantIntegration{
			MerchantID:            "MERCH-001",
			PaymentsReceivable:    true,
			PrimaryEmailConfirmed: true,
		})
	})

	ts, p := newTestServer(t, mux)
	p.config.PartnerID = "PARTNER-ID"
	defer ts.Close()

	account, err := p.HandleOnboardingCallback(context.Background(), contracts.CallbackParams{
		MerchantIDPP: "MERCH-001",
	})
	require.NoError(t, err)
	assert.Equal(t, "paypal", account.ProviderID)
	assert.Equal(t, "MERCH-001", account.AccountID)
	assert.Equal(t, "active", account.Status)
}

func TestProvider_HandleOnboardingCallback_Pending(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/customer/partners/PARTNER-ID/merchant-integrations/MERCH-002", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(merchantIntegration{
			MerchantID:            "MERCH-002",
			PaymentsReceivable:    false,
			PrimaryEmailConfirmed: false,
		})
	})

	ts, p := newTestServer(t, mux)
	p.config.PartnerID = "PARTNER-ID"
	defer ts.Close()

	account, err := p.HandleOnboardingCallback(context.Background(), contracts.CallbackParams{
		MerchantIDPP: "MERCH-002",
	})
	require.NoError(t, err)
	assert.Equal(t, "pending", account.Status)
}

func TestProvider_HandleOnboardingCallback_MissingMerchantID(t *testing.T) {
	p := NewProvider(Config{})
	_, err := p.HandleOnboardingCallback(context.Background(), contracts.CallbackParams{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "merchant_id_in_paypal is required")
}

func TestProvider_GetAccountStatus_DirectMode(t *testing.T) {
	p := NewProvider(Config{Mode: ModeDirect})

	status, err := p.GetAccountStatus(context.Background(), "MERCH-DIRECT")
	require.NoError(t, err)
	assert.Equal(t, "MERCH-DIRECT", status.AccountID)
	assert.True(t, status.IsActive)
	assert.Equal(t, "active", status.Status)
}

func TestProvider_GetAccountStatus_PartnerMode(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/customer/partners/PARTNER-ID/merchant-integrations/MERCH-003", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(merchantIntegration{
			MerchantID:            "MERCH-003",
			PaymentsReceivable:    true,
			PrimaryEmailConfirmed: true,
		})
	})

	ts, p := newTestServer(t, mux)
	p.config.PartnerID = "PARTNER-ID"
	defer ts.Close()

	status, err := p.GetAccountStatus(context.Background(), "MERCH-003")
	require.NoError(t, err)
	assert.True(t, status.IsActive)
	assert.True(t, status.ChargesEnabled)
	assert.True(t, status.PayoutsEnabled)
	assert.Equal(t, "active", status.Status)
}

// --- Helper function tests ---

func TestFormatAmount(t *testing.T) {
	tests := []struct {
		cents    int64
		currency string
		expected string
	}{
		{2999, "USD", "29.99"},
		{100, "EUR", "1.00"},
		{5000, "JPY", "5000"},
		{0, "USD", "0.00"},
		{1, "GBP", "0.01"},
		{10000, "KRW", "10000"},
	}

	for _, tt := range tests {
		got := formatAmount(tt.cents, tt.currency)
		assert.Equal(t, tt.expected, got, "formatAmount(%d, %s)", tt.cents, tt.currency)
	}
}

func TestParseAmount(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"29.99", 2999},
		{"1.00", 100},
		{"0.01", 1},
		{"100.50", 10050},
		{"0.10", 10},
		{"999.99", 99999},
	}

	for _, tt := range tests {
		got, err := parseAmount(tt.input)
		require.NoError(t, err)
		assert.Equal(t, tt.expected, got, "parseAmount(%s)", tt.input)
	}
}

func TestParseAmount_Error(t *testing.T) {
	_, err := parseAmount("not-a-number")
	require.Error(t, err)
}

func TestMapPayPalStatus(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"COMPLETED", "succeeded"},
		{"DECLINED", "failed"},
		{"VOIDED", "failed"},
		{"CREATED", "pending"},
		{"SAVED", "pending"},
		{"APPROVED", "pending"},
		{"PAYER_ACTION_REQUIRED", "pending"},
		{"UNKNOWN_STATUS", "UNKNOWN_STATUS"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, mapPayPalStatus(tt.input), "mapPayPalStatus(%s)", tt.input)
	}
}

func TestGetHeader_CaseInsensitive(t *testing.T) {
	headers := map[string]string{
		"Paypal-Transmission-Id": "abc123",
	}
	assert.Equal(t, "abc123", getHeader(headers, "Paypal-Transmission-Id"))
	assert.Equal(t, "abc123", getHeader(headers, "paypal-transmission-id"))
	assert.Equal(t, "", getHeader(headers, "Missing-Header"))
}
