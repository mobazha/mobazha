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

func TestWebhookVerify_ValidSignature(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/notifications/verify-webhook-signature", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)

		var req webhookVerifyRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, "test-webhook-id", req.WebhookID)
		assert.Equal(t, "tx-001", req.TransmissionID)
		assert.Equal(t, "sig-abc", req.TransmissionSig)
		assert.Equal(t, "SHA256withRSA", req.AuthAlgo)

		json.NewEncoder(w).Encode(webhookVerifyResponse{VerificationStatus: "SUCCESS"})
	})

	ts, p := newTestServer(t, mux)
	defer ts.Close()

	payload := `{"id": "WH-VALID", "event_type": "PAYMENT.CAPTURE.COMPLETED", "resource": {}}`
	headers := map[string]string{
		"Paypal-Transmission-Id":   "tx-001",
		"Paypal-Transmission-Sig":  "sig-abc",
		"Paypal-Transmission-Time": "2026-03-11T12:00:00Z",
		"Paypal-Auth-Algo":         "SHA256withRSA",
		"Paypal-Cert-Url":          "https://api.paypal.com/cert.pem",
	}

	event, err := p.ParseWebhook(context.Background(), []byte(payload), headers)
	require.NoError(t, err)
	assert.Equal(t, "WH-VALID", event.EventID)
	assert.Equal(t, contracts.WebhookPaymentSucceeded, event.Type)
}

func TestWebhookVerify_InvalidSignature(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/notifications/verify-webhook-signature", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(webhookVerifyResponse{VerificationStatus: "FAILURE"})
	})

	ts, p := newTestServer(t, mux)
	defer ts.Close()

	payload := `{"id": "WH-BAD", "event_type": "PAYMENT.CAPTURE.COMPLETED", "resource": {}}`
	headers := map[string]string{
		"Paypal-Transmission-Id":   "tx-bad",
		"Paypal-Transmission-Sig":  "forged-sig",
		"Paypal-Transmission-Time": "2026-03-11T12:00:00Z",
		"Paypal-Auth-Algo":         "SHA256withRSA",
		"Paypal-Cert-Url":          "https://api.paypal.com/cert.pem",
	}

	_, err := p.ParseWebhook(context.Background(), []byte(payload), headers)
	require.Error(t, err)
	assert.ErrorIs(t, err, contracts.ErrWebhookSignature)
}

func TestWebhookVerify_NoWebhookID(t *testing.T) {
	p := NewProvider(Config{
		ClientID:     "test",
		ClientSecret: "test",
	})

	payload := `{"id": "WH-NOID", "event_type": "PAYMENT.CAPTURE.COMPLETED", "resource": {}}`
	headers := map[string]string{
		"Paypal-Transmission-Id":  "tx-noid",
		"Paypal-Transmission-Sig": "sig-noid",
	}

	_, err := p.ParseWebhook(context.Background(), []byte(payload), headers)
	require.Error(t, err)
	assert.ErrorIs(t, err, contracts.ErrWebhookSignature)
	assert.Contains(t, err.Error(), "webhook ID not configured")
}

func TestWebhookVerify_MissingHeaders(t *testing.T) {
	p := NewProvider(Config{
		ClientID:     "test",
		ClientSecret: "test",
		WebhookID:    "WH-CONFIGURED",
	})

	payload := `{"id": "WH-MISS", "event_type": "PAYMENT.CAPTURE.COMPLETED", "resource": {}}`

	_, err := p.ParseWebhook(context.Background(), []byte(payload), map[string]string{})
	require.Error(t, err)
	assert.ErrorIs(t, err, contracts.ErrWebhookSignature)
	assert.Contains(t, err.Error(), "missing required")
}

func TestWebhookVerify_Cache(t *testing.T) {
	callCount := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/notifications/verify-webhook-signature", func(w http.ResponseWriter, r *http.Request) {
		callCount++
		json.NewEncoder(w).Encode(webhookVerifyResponse{VerificationStatus: "SUCCESS"})
	})

	ts, p := newTestServer(t, mux)
	defer ts.Close()

	payload := `{"id": "WH-CACHE", "event_type": "PAYMENT.CAPTURE.COMPLETED", "resource": {}}`
	headers := map[string]string{
		"Paypal-Transmission-Id":   "tx-cached",
		"Paypal-Transmission-Sig":  "sig-cached",
		"Paypal-Transmission-Time": "2026-03-11T12:00:00Z",
		"Paypal-Auth-Algo":         "SHA256withRSA",
		"Paypal-Cert-Url":          "https://api.paypal.com/cert.pem",
	}

	_, err := p.ParseWebhook(context.Background(), []byte(payload), headers)
	require.NoError(t, err)
	assert.Equal(t, 1, callCount, "first call should hit PayPal API")

	_, err = p.ParseWebhook(context.Background(), []byte(payload), headers)
	require.NoError(t, err)
	assert.Equal(t, 1, callCount, "second call with same transmission ID should use cache")
}

func TestWebhookVerify_APIDown(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/notifications/verify-webhook-signature", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "service unavailable"}`))
	})

	ts, p := newTestServer(t, mux)
	defer ts.Close()

	payload := `{"id": "WH-DOWN", "event_type": "PAYMENT.CAPTURE.COMPLETED", "resource": {}}`
	headers := map[string]string{
		"Paypal-Transmission-Id":   "tx-down",
		"Paypal-Transmission-Sig":  "sig-down",
		"Paypal-Transmission-Time": "2026-03-11T12:00:00Z",
		"Paypal-Auth-Algo":         "SHA256withRSA",
		"Paypal-Cert-Url":          "https://api.paypal.com/cert.pem",
	}

	_, err := p.ParseWebhook(context.Background(), []byte(payload), headers)
	require.Error(t, err)
	assert.ErrorIs(t, err, contracts.ErrWebhookSignature)
}

func TestSignatureCache_BasicOperations(t *testing.T) {
	c := newSignatureCache()

	assert.False(t, c.isVerified("tx-1"))

	c.markVerified("tx-1")
	assert.True(t, c.isVerified("tx-1"))
	assert.False(t, c.isVerified("tx-2"))
}

func TestSignatureCache_Expiry(t *testing.T) {
	c := newSignatureCache()

	c.mu.Lock()
	c.items["tx-old"] = time.Now().Add(-6 * time.Minute)
	c.mu.Unlock()

	assert.False(t, c.isVerified("tx-old"), "expired entry should not be considered verified")
}

func TestSignatureCache_Cleanup(t *testing.T) {
	c := newSignatureCache()

	for i := 0; i < signatureCacheMaxSize+5; i++ {
		c.mu.Lock()
		c.items[string(rune('A'+i))] = time.Now().Add(-6 * time.Minute)
		c.mu.Unlock()
	}

	c.markVerified("fresh-entry")

	c.mu.RLock()
	defer c.mu.RUnlock()
	assert.LessOrEqual(t, len(c.items), signatureCacheMaxSize,
		"cleanup should remove expired entries when cache exceeds max size")
	_, hasFresh := c.items["fresh-entry"]
	assert.True(t, hasFresh, "fresh entry should survive cleanup")
}

func TestWebhookVerify_HeaderCaseInsensitive(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/notifications/verify-webhook-signature", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(webhookVerifyResponse{VerificationStatus: "SUCCESS"})
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	p := &Provider{
		config: Config{
			ClientID:     "test",
			ClientSecret: "test",
			WebhookID:    "WH-CASE",
		},
		client: &apiClient{
			clientID:     "test",
			clientSecret: "test",
			baseURL:      ts.URL,
			httpClient:   &http.Client{Timeout: 5 * time.Second},
			accessToken:  "test-token",
			tokenExpiry:  time.Now().Add(1 * time.Hour),
		},
		sigCache: newSignatureCache(),
	}

	payload := `{"id": "WH-CASE", "event_type": "PAYMENT.CAPTURE.COMPLETED", "resource": {}}`
	headers := map[string]string{
		"paypal-transmission-id":   "tx-case",
		"paypal-transmission-sig":  "sig-case",
		"paypal-transmission-time": "2026-03-11T12:00:00Z",
		"paypal-auth-algo":         "SHA256withRSA",
		"paypal-cert-url":          "https://api.paypal.com/cert.pem",
	}

	event, err := p.ParseWebhook(context.Background(), []byte(payload), headers)
	require.NoError(t, err)
	assert.Equal(t, "WH-CASE", event.EventID)
}
