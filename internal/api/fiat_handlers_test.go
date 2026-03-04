package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockFiatService implements contracts.FiatService for handler tests.
type mockFiatService struct {
	enabledResult  []contracts.ProviderInfo
	enabledErr     error
	createResult   *contracts.PaymentSession
	createErr      error
	captureResult  *contracts.PaymentResult
	captureErr     error
	getResult      *contracts.PaymentDetail
	getErr         error
	webhookErr     error
	statusResult   *contracts.AccountStatus
	statusErr      error
	configResult   *contracts.ProviderConfigView
	configErr      error
	saveErr        error
	deleteErr      error
	verifyErr      error
	onboardResult  *contracts.OnboardingResult
	onboardErr     error
	onboardCBResult *contracts.AccountStatus
	onboardCBErr   error
}

func (m *mockFiatService) EnabledProviders(_ context.Context) ([]contracts.ProviderInfo, error) {
	return m.enabledResult, m.enabledErr
}

func (m *mockFiatService) CreatePayment(_ context.Context, _ string, _ contracts.CreatePaymentParams) (*contracts.PaymentSession, error) {
	return m.createResult, m.createErr
}

func (m *mockFiatService) CapturePayment(_ context.Context, _ string, _ string) (*contracts.PaymentResult, error) {
	return m.captureResult, m.captureErr
}

func (m *mockFiatService) GetPayment(_ context.Context, _ string, _ string) (*contracts.PaymentDetail, error) {
	return m.getResult, m.getErr
}

func (m *mockFiatService) HandleWebhook(_ context.Context, _ string, _ []byte, _ map[string]string) error {
	return m.webhookErr
}

func (m *mockFiatService) GetProviderStatus(_ context.Context, _ string) (*contracts.AccountStatus, error) {
	return m.statusResult, m.statusErr
}

func (m *mockFiatService) GetProviderConfig(_ string) (*contracts.ProviderConfigView, error) {
	return m.configResult, m.configErr
}

func (m *mockFiatService) SaveProviderConfig(_ string, _ contracts.ProviderConfigInput) error {
	return m.saveErr
}

func (m *mockFiatService) DeleteProviderConfig(_ string) error {
	return m.deleteErr
}

func (m *mockFiatService) VerifyProviderConfig(_ string) error {
	return m.verifyErr
}

func (m *mockFiatService) GetOnboardingURL(_ context.Context, _ string, _ contracts.OnboardingParams) (*contracts.OnboardingResult, error) {
	return m.onboardResult, m.onboardErr
}

func (m *mockFiatService) HandleOnboardingCallback(_ context.Context, _ string, _ contracts.CallbackParams) (*contracts.AccountStatus, error) {
	return m.onboardCBResult, m.onboardCBErr
}

// mockNodeWithFiat implements both contracts.NodeService (partially) and
// contracts.FiatPaymentProviderAccessor for handler test context injection.
type mockNodeWithFiat struct {
	contracts.NodeService
	fiatSvc contracts.FiatService
}

func (m *mockNodeWithFiat) Fiat() contracts.FiatService { return m.fiatSvc }

// newFiatHandlerRequest creates an http.Request with the mock node injected in context
// and optional mux vars.
func newFiatHandlerRequest(t *testing.T, method, path string, body interface{}, vars map[string]string, fiatSvc *mockFiatService) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")

	node := &mockNodeWithFiat{fiatSvc: fiatSvc}
	ctx := context.WithValue(req.Context(), nodeContextKey, node)
	req = req.WithContext(ctx)

	if len(vars) > 0 {
		req = mux.SetURLVars(req, vars)
	}
	return req
}

// --- GET /v1/fiat/providers ---

func TestHandleGETFiatProviders_Success(t *testing.T) {
	svc := &mockFiatService{
		enabledResult: []contracts.ProviderInfo{
			{ProviderID: "stripe", Status: "active", AccountID: "acct_1"},
			{ProviderID: "paypal", Status: "not_connected"},
		},
	}
	g := &Gateway{}
	w := httptest.NewRecorder()
	r := newFiatHandlerRequest(t, "GET", "/v1/fiat/providers", nil, nil, svc)

	g.handleGETFiatProviders(w, r)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data, ok := resp["data"].([]interface{})
	require.True(t, ok)
	assert.Len(t, data, 2)
}

func TestHandleGETFiatProviders_NotImplemented(t *testing.T) {
	g := &Gateway{}
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/fiat/providers", nil)
	ctx := context.WithValue(req.Context(), nodeContextKey, &mockNodeWithFiat{fiatSvc: nil})
	req = req.WithContext(ctx)

	g.handleGETFiatProviders(w, req)
	assert.Equal(t, http.StatusNotImplemented, w.Code)
}

// --- POST /v1/fiat/{providerID}/payments ---

func TestHandlePOSTFiatPayment_Success(t *testing.T) {
	svc := &mockFiatService{
		createResult: &contracts.PaymentSession{
			SessionID:   "sess_test",
			CaptureMode: contracts.CaptureAutomatic,
			ExpiresAt:   time.Now().Add(30 * time.Minute),
			Status:      "requires_payment_method",
			Stripe:      &contracts.StripeSessionData{ClientSecret: "cs_xxx", PublishableKey: "pk_xxx"},
		},
	}
	g := &Gateway{}
	w := httptest.NewRecorder()
	body := map[string]interface{}{
		"orderID": "order_123", "amount": 2500, "currency": "usd",
	}
	r := newFiatHandlerRequest(t, "POST", "/v1/fiat/stripe/payments", body,
		map[string]string{"providerID": "stripe"}, svc)

	g.handlePOSTFiatPayment(w, r)
	assert.Equal(t, http.StatusCreated, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, "sess_test", data["sessionID"])
}

func TestHandlePOSTFiatPayment_MissingBody(t *testing.T) {
	svc := &mockFiatService{}
	g := &Gateway{}
	w := httptest.NewRecorder()
	body := map[string]interface{}{"orderID": "", "amount": 0, "currency": ""}
	r := newFiatHandlerRequest(t, "POST", "/v1/fiat/stripe/payments", body,
		map[string]string{"providerID": "stripe"}, svc)

	g.handlePOSTFiatPayment(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandlePOSTFiatPayment_MissingProviderID(t *testing.T) {
	svc := &mockFiatService{}
	g := &Gateway{}
	w := httptest.NewRecorder()
	body := map[string]interface{}{"orderID": "o1", "amount": 100, "currency": "usd"}
	r := newFiatHandlerRequest(t, "POST", "/v1/fiat//payments", body,
		map[string]string{"providerID": ""}, svc)

	g.handlePOSTFiatPayment(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- POST /v1/fiat/{providerID}/payments/{sessionID}/capture ---

func TestHandlePOSTFiatCapture_Success(t *testing.T) {
	svc := &mockFiatService{
		captureResult: &contracts.PaymentResult{
			PaymentID: "pi_cap", Status: "succeeded", Amount: 5000, Currency: "usd",
		},
	}
	g := &Gateway{}
	w := httptest.NewRecorder()
	r := newFiatHandlerRequest(t, "POST", "/v1/fiat/stripe/payments/sess_1/capture", nil,
		map[string]string{"providerID": "stripe", "sessionID": "sess_1"}, svc)

	g.handlePOSTFiatCapture(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

// --- GET /v1/fiat/{providerID}/payments/{paymentID} ---

func TestHandleGETFiatPayment_Success(t *testing.T) {
	svc := &mockFiatService{
		getResult: &contracts.PaymentDetail{
			PaymentID: "pi_detail", Status: "succeeded", Amount: 3000, Currency: "eur",
		},
	}
	g := &Gateway{}
	w := httptest.NewRecorder()
	r := newFiatHandlerRequest(t, "GET", "/v1/fiat/stripe/payments/pi_detail", nil,
		map[string]string{"providerID": "stripe", "paymentID": "pi_detail"}, svc)

	g.handleGETFiatPayment(w, r)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, "pi_detail", data["paymentID"])
}

// --- POST /v1/fiat/{providerID}/webhooks ---

func TestHandlePOSTFiatWebhook_Success(t *testing.T) {
	svc := &mockFiatService{webhookErr: nil}
	g := &Gateway{}
	w := httptest.NewRecorder()
	r := newFiatHandlerRequest(t, "POST", "/v1/fiat/stripe/webhooks",
		map[string]string{"test": "payload"},
		map[string]string{"providerID": "stripe"}, svc)

	g.handlePOSTFiatWebhook(w, r)
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestHandlePOSTFiatWebhook_SignatureError(t *testing.T) {
	svc := &mockFiatService{webhookErr: contracts.ErrWebhookSignature}
	g := &Gateway{}
	w := httptest.NewRecorder()
	r := newFiatHandlerRequest(t, "POST", "/v1/fiat/stripe/webhooks",
		map[string]string{"bad": "sig"},
		map[string]string{"providerID": "stripe"}, svc)

	g.handlePOSTFiatWebhook(w, r)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// --- GET /v1/fiat/{providerID}/config ---

func TestHandleGETFiatConfig_Success(t *testing.T) {
	svc := &mockFiatService{
		configResult: &contracts.ProviderConfigView{
			ProviderID: "stripe", AccountID: "acct_test", SecretKey: "sk_****",
			IsActive: true,
		},
	}
	g := &Gateway{}
	w := httptest.NewRecorder()
	r := newFiatHandlerRequest(t, "GET", "/v1/fiat/stripe/config", nil,
		map[string]string{"providerID": "stripe"}, svc)

	g.handleGETFiatProviderConfig(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleGETFiatConfig_NotFound(t *testing.T) {
	svc := &mockFiatService{configErr: contracts.ErrProviderNotFound}
	g := &Gateway{}
	w := httptest.NewRecorder()
	r := newFiatHandlerRequest(t, "GET", "/v1/fiat/paypal/config", nil,
		map[string]string{"providerID": "paypal"}, svc)

	g.handleGETFiatProviderConfig(w, r)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// --- PUT /v1/fiat/{providerID}/config ---

func TestHandlePUTFiatConfig_Success(t *testing.T) {
	svc := &mockFiatService{
		configResult: &contracts.ProviderConfigView{
			ProviderID: "stripe", SecretKey: "sk_****", IsActive: true,
		},
	}
	g := &Gateway{}
	w := httptest.NewRecorder()
	body := contracts.ProviderConfigInput{
		AccountID: "acct_new", PublicKey: "pk_new", SecretKey: "sk_new", WebhookSecret: "wh_new",
	}
	r := newFiatHandlerRequest(t, "PUT", "/v1/fiat/stripe/config", body,
		map[string]string{"providerID": "stripe"}, svc)

	g.handlePUTFiatProviderConfig(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandlePUTFiatConfig_MissingSecretKey(t *testing.T) {
	svc := &mockFiatService{}
	g := &Gateway{}
	w := httptest.NewRecorder()
	body := contracts.ProviderConfigInput{AccountID: "acct_new"}
	r := newFiatHandlerRequest(t, "PUT", "/v1/fiat/stripe/config", body,
		map[string]string{"providerID": "stripe"}, svc)

	g.handlePUTFiatProviderConfig(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- DELETE /v1/fiat/{providerID}/config ---

func TestHandleDELETEFiatConfig_Success(t *testing.T) {
	svc := &mockFiatService{}
	g := &Gateway{}
	w := httptest.NewRecorder()
	r := newFiatHandlerRequest(t, "DELETE", "/v1/fiat/stripe/config", nil,
		map[string]string{"providerID": "stripe"}, svc)

	g.handleDELETEFiatProviderConfig(w, r)
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestHandleDELETEFiatConfig_Error(t *testing.T) {
	svc := &mockFiatService{deleteErr: errors.New("delete failed")}
	g := &Gateway{}
	w := httptest.NewRecorder()
	r := newFiatHandlerRequest(t, "DELETE", "/v1/fiat/stripe/config", nil,
		map[string]string{"providerID": "stripe"}, svc)

	g.handleDELETEFiatProviderConfig(w, r)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- GET /v1/fiat/{providerID}/status ---

func TestHandleGETFiatProviderStatus_Success(t *testing.T) {
	svc := &mockFiatService{
		statusResult: &contracts.AccountStatus{
			AccountID: "acct_1", IsActive: true, Status: "active",
			ChargesEnabled: true, PayoutsEnabled: true,
		},
	}
	g := &Gateway{}
	w := httptest.NewRecorder()
	r := newFiatHandlerRequest(t, "GET", "/v1/fiat/stripe/status", nil,
		map[string]string{"providerID": "stripe"}, svc)

	g.handleGETFiatProviderStatus(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

// --- POST /v1/fiat/{providerID}/verify ---

func TestHandlePOSTFiatProviderVerify_Success(t *testing.T) {
	svc := &mockFiatService{}
	g := &Gateway{}
	w := httptest.NewRecorder()
	r := newFiatHandlerRequest(t, "POST", "/v1/fiat/stripe/verify", nil,
		map[string]string{"providerID": "stripe"}, svc)

	g.handlePOSTFiatProviderVerify(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandlePOSTFiatProviderVerify_Failed(t *testing.T) {
	svc := &mockFiatService{verifyErr: errors.New("connection refused")}
	g := &Gateway{}
	w := httptest.NewRecorder()
	r := newFiatHandlerRequest(t, "POST", "/v1/fiat/stripe/verify", nil,
		map[string]string{"providerID": "stripe"}, svc)

	g.handlePOSTFiatProviderVerify(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
