//go:build !private_distribution

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"

	pkgconfig "github.com/mobazha/mobazha3.0/pkg/config"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
)

// ---------------------------------------------------------------------------
// Mock: GuestOrderService
// ---------------------------------------------------------------------------

type mockGuestOrderService struct {
	createGuestOrderFunc     func(ctx context.Context, req contracts.CreateGuestOrderRequest) (*contracts.GuestOrderResponse, error)
	getGuestOrderStatusFunc  func(ctx context.Context, token string) (*contracts.GuestOrderStatusResponse, error)
	listGuestOrdersFunc      func(ctx context.Context, filter contracts.GuestOrderFilter) ([]models.GuestOrder, int64, error)
	shipGuestOrderFunc       func(ctx context.Context, token, tracking, carrier string) error
	completeGuestOrderFunc   func(ctx context.Context, token string) error
	getGuestCheckoutCfgFunc  func(ctx context.Context) (*models.GuestCheckoutConfig, error)
	saveGuestCheckoutCfgFunc func(ctx context.Context, cfg *models.GuestCheckoutConfig) error
}

func (m *mockGuestOrderService) CreateGuestOrder(ctx context.Context, req contracts.CreateGuestOrderRequest) (*contracts.GuestOrderResponse, error) {
	if m.createGuestOrderFunc != nil {
		return m.createGuestOrderFunc(ctx, req)
	}
	return nil, errors.New("not implemented")
}

func (m *mockGuestOrderService) GetGuestOrderStatus(ctx context.Context, token string) (*contracts.GuestOrderStatusResponse, error) {
	if m.getGuestOrderStatusFunc != nil {
		return m.getGuestOrderStatusFunc(ctx, token)
	}
	return nil, errors.New("not found")
}

func (m *mockGuestOrderService) ListGuestOrders(ctx context.Context, filter contracts.GuestOrderFilter) ([]models.GuestOrder, int64, error) {
	if m.listGuestOrdersFunc != nil {
		return m.listGuestOrdersFunc(ctx, filter)
	}
	return nil, 0, nil
}

func (m *mockGuestOrderService) ShipGuestOrder(ctx context.Context, token, tracking, carrier string) error {
	if m.shipGuestOrderFunc != nil {
		return m.shipGuestOrderFunc(ctx, token, tracking, carrier)
	}
	return nil
}

func (m *mockGuestOrderService) CompleteGuestOrder(ctx context.Context, token string) error {
	if m.completeGuestOrderFunc != nil {
		return m.completeGuestOrderFunc(ctx, token)
	}
	return nil
}

func (m *mockGuestOrderService) HandlePaymentDetected(string, string, *contracts.PaymentDetectedOpts) error {
	return nil
}
func (m *mockGuestOrderService) HandleConfirmationUpdate(string, int) error { return nil }
func (m *mockGuestOrderService) HandlePoolPayment(string, string, uint64) error {
	return nil
}
func (m *mockGuestOrderService) HandleLatePayment(string, string, string, uint64, uint64) error {
	return nil
}
func (m *mockGuestOrderService) CleanupExpiredOrders(context.Context) {}
func (m *mockGuestOrderService) AutoCompleteOrders(context.Context)   {}
func (m *mockGuestOrderService) RunGuestCleanupOnce()                 {}
func (m *mockGuestOrderService) IsEnabled(context.Context) bool       { return true }

func (m *mockGuestOrderService) GetGuestCheckoutConfig(ctx context.Context) (*models.GuestCheckoutConfig, error) {
	if m.getGuestCheckoutCfgFunc != nil {
		return m.getGuestCheckoutCfgFunc(ctx)
	}
	return &models.GuestCheckoutConfig{Enabled: true, AcceptedCoins: "BTC,ETH"}, nil
}

func (m *mockGuestOrderService) SaveGuestCheckoutConfig(ctx context.Context, cfg *models.GuestCheckoutConfig) error {
	if m.saveGuestCheckoutCfgFunc != nil {
		return m.saveGuestCheckoutCfgFunc(ctx, cfg)
	}
	return nil
}

func (m *mockGuestOrderService) GetGuestCheckoutReadiness(context.Context) (*contracts.GuestCheckoutReadiness, error) {
	return &contracts.GuestCheckoutReadiness{GuestCheckoutEnabled: true}, nil
}

func (m *mockGuestOrderService) GetAdminGuestOrder(_ context.Context, _ string) (*models.GuestOrder, error) {
	return nil, nil
}

// ---------------------------------------------------------------------------
// Mock node that delegates GuestOrder()
// ---------------------------------------------------------------------------

type mockGuestNode struct {
	mockNode
	guestSvc     *mockGuestOrderService
	featureStore pkgconfig.TenantFeatureStore
}

func (n *mockGuestNode) GuestOrder() contracts.GuestOrderService {
	return n.guestSvc
}

func (n *mockGuestNode) TenantFeatureStore() pkgconfig.TenantFeatureStore {
	return n.featureStore
}

// ---------------------------------------------------------------------------
// Test server helpers
// ---------------------------------------------------------------------------

func guestTestServer(t *testing.T, svc *mockGuestOrderService) *httptest.Server {
	t.Helper()
	node := &mockGuestNode{guestSvc: svc}
	return guestTestServerWithNode(t, node)
}

func guestTestServerWithNode(t *testing.T, node contracts.NodeService) *httptest.Server {
	t.Helper()
	gateway := &Gateway{
		config:            &GatewayConfig{},
		guestOrderLimiter: newRateLimiter(1000, time.Hour), // generous limit for tests
	}
	outer := chi.NewMux()
	outer.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := context.WithValue(req.Context(), nodeContextKey, contracts.NodeService(node))
			// Inject admin identity so the post-fix nodeHumaAuthMiddleware
			// short-circuits the "already authenticated" check on
			// owner-only routes (ship / settings / complete). Auth pipeline
			// coverage lives in TestNodeBridgeRequestWithOptionalAuth_* +
			// TestGateway_AuthenticationMiddleware.
			ctx = WithAuthIdentity(ctx, &AuthIdentity{
				UserID:  "test-admin",
				IsAdmin: true,
			})
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	outer.Mount("/", gateway.newV1Router(false, false))

	ts := httptest.NewServer(outer)
	t.Cleanup(ts.Close)
	return ts
}

func guestDoReq(t *testing.T, ts *httptest.Server, method, path string, body []byte) (*http.Response, []byte) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, ts.URL+path, reader)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	return resp, respBody
}

func guestAssertStatus(t *testing.T, resp *http.Response, expected int) {
	t.Helper()
	if resp.StatusCode != expected {
		t.Errorf("expected status %d, got %d", expected, resp.StatusCode)
	}
}

func guestAssertErrorCode(t *testing.T, body []byte, expectedCode string) {
	t.Helper()
	var envelope struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("cannot unmarshal error response: %s\nbody: %s", err, body)
	}
	if envelope.Error.Code != expectedCode {
		t.Errorf("expected error code %q, got %q", expectedCode, envelope.Error.Code)
	}
}

// ---------------------------------------------------------------------------
// H-01: POST /v1/guest/orders — valid request → 201
// ---------------------------------------------------------------------------

func TestPOSTGuestOrder_Valid(t *testing.T) {
	svc := &mockGuestOrderService{
		createGuestOrderFunc: func(_ context.Context, req contracts.CreateGuestOrderRequest) (*contracts.GuestOrderResponse, error) {
			return &contracts.GuestOrderResponse{
				OrderToken:     "tok_abc123",
				PaymentAddress: "bc1qtest",
				PaymentAmount:  "50000",
				PaymentCoin:    req.PaymentCoin,
				ExpiresAt:      time.Now().Add(time.Hour),
			}, nil
		},
	}
	ts := guestTestServer(t, svc)

	body, _ := json.Marshal(contracts.CreateGuestOrderRequest{
		Items: []contracts.GuestOrderItemRequest{
			{ListingSlug: "test-item", Quantity: 1},
		},
		PaymentCoin: "BTC",
	})

	resp, respBody := guestDoReq(t, ts, "POST", "/v1/guest/orders", body)
	guestAssertStatus(t, resp, http.StatusCreated)

	var envelope struct {
		Data contracts.GuestOrderResponse `json:"data"`
	}
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		t.Fatalf("cannot unmarshal: %s", err)
	}
	if envelope.Data.OrderToken != "tok_abc123" {
		t.Errorf("expected token tok_abc123, got %s", envelope.Data.OrderToken)
	}
}

// ---------------------------------------------------------------------------
// H-02: POST /v1/guest/orders — empty items → 400
// ---------------------------------------------------------------------------

func TestPOSTGuestOrder_EmptyItems(t *testing.T) {
	svc := &mockGuestOrderService{}
	ts := guestTestServer(t, svc)

	body, _ := json.Marshal(contracts.CreateGuestOrderRequest{
		Items:       []contracts.GuestOrderItemRequest{},
		PaymentCoin: "BTC",
	})

	resp, respBody := guestDoReq(t, ts, "POST", "/v1/guest/orders", body)
	guestAssertStatus(t, resp, http.StatusBadRequest)
	guestAssertErrorCode(t, respBody, "BAD_REQUEST")
}

// ---------------------------------------------------------------------------
// H-03: POST /v1/guest/orders — missing paymentCoin → 400
// ---------------------------------------------------------------------------

func TestPOSTGuestOrder_MissingPaymentCoin(t *testing.T) {
	svc := &mockGuestOrderService{}
	ts := guestTestServer(t, svc)

	body, _ := json.Marshal(map[string]interface{}{
		"items": []map[string]interface{}{
			{"listingSlug": "test", "quantity": 1},
		},
	})

	resp, respBody := guestDoReq(t, ts, "POST", "/v1/guest/orders", body)
	guestAssertStatus(t, resp, http.StatusBadRequest)
	guestAssertErrorCode(t, respBody, "BAD_REQUEST")
}

// ---------------------------------------------------------------------------
// H-03b: POST /v1/guest/orders — ambiguous paymentCoin (non-canonical ticker) → 400
// ---------------------------------------------------------------------------

func TestPOSTGuestOrder_AmbiguousPaymentCoin(t *testing.T) {
	svc := &mockGuestOrderService{}
	ts := guestTestServer(t, svc)

	body, _ := json.Marshal(contracts.CreateGuestOrderRequest{
		Items: []contracts.GuestOrderItemRequest{
			{ListingSlug: "test-item", Quantity: 1},
		},
		PaymentCoin: "USDC",
	})

	resp, respBody := guestDoReq(t, ts, "POST", "/v1/guest/orders", body)
	guestAssertStatus(t, resp, http.StatusBadRequest)
	guestAssertErrorCode(t, respBody, "BAD_REQUEST")
}

// ---------------------------------------------------------------------------
// H-04: GET /v1/guest/orders/{token} — found → 200
// ---------------------------------------------------------------------------

func TestGETGuestOrderStatus_Found(t *testing.T) {
	svc := &mockGuestOrderService{
		getGuestOrderStatusFunc: func(_ context.Context, token string) (*contracts.GuestOrderStatusResponse, error) {
			return &contracts.GuestOrderStatusResponse{
				OrderToken:     token,
				State:          "AWAITING_PAYMENT",
				PaymentAddress: "bc1qtest",
				PaymentAmount:  "50000",
				PaymentCoin:    "BTC",
				CreatedAt:      time.Now(),
			}, nil
		},
	}
	ts := guestTestServer(t, svc)

	resp, respBody := guestDoReq(t, ts, "GET", "/v1/guest/orders/tok_abc123", nil)
	guestAssertStatus(t, resp, http.StatusOK)

	var envelope struct {
		Data contracts.GuestOrderStatusResponse `json:"data"`
	}
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		t.Fatalf("cannot unmarshal: %s", err)
	}
	if envelope.Data.State != "AWAITING_PAYMENT" {
		t.Errorf("expected state AWAITING_PAYMENT, got %s", envelope.Data.State)
	}
}

// ---------------------------------------------------------------------------
// H-05: GET /v1/guest/orders/{token} — not found → 404
// ---------------------------------------------------------------------------

func TestGETGuestOrderStatus_NotFound(t *testing.T) {
	svc := &mockGuestOrderService{
		getGuestOrderStatusFunc: func(context.Context, string) (*contracts.GuestOrderStatusResponse, error) {
			return nil, gorm.ErrRecordNotFound
		},
	}
	ts := guestTestServer(t, svc)

	resp, respBody := guestDoReq(t, ts, "GET", "/v1/guest/orders/tok_nonexistent", nil)
	guestAssertStatus(t, resp, http.StatusNotFound)
	guestAssertErrorCode(t, respBody, "NOT_FOUND")
}

// ---------------------------------------------------------------------------
// H-06: POST /v1/guest/orders — service error (e.g. checkout disabled) → 500
// ---------------------------------------------------------------------------

func TestPOSTGuestOrder_ServiceError(t *testing.T) {
	svc := &mockGuestOrderService{
		createGuestOrderFunc: func(context.Context, contracts.CreateGuestOrderRequest) (*contracts.GuestOrderResponse, error) {
			return nil, errors.New("guest checkout not enabled for this store")
		},
	}
	ts := guestTestServer(t, svc)

	body, _ := json.Marshal(contracts.CreateGuestOrderRequest{
		Items:       []contracts.GuestOrderItemRequest{{ListingSlug: "item", Quantity: 1}},
		PaymentCoin: "BTC",
	})

	resp, _ := guestDoReq(t, ts, "POST", "/v1/guest/orders", body)
	guestAssertStatus(t, resp, http.StatusInternalServerError)
}

// ---------------------------------------------------------------------------
// H-07: GET /v1/guest/orders (list) — pagination params forwarded correctly
// (Auth enforcement is verified by middleware-level tests; here we test
//  handler behavior when the request passes auth.)
// ---------------------------------------------------------------------------

func TestGETGuestOrders_Pagination(t *testing.T) {
	var capturedFilter contracts.GuestOrderFilter
	svc := &mockGuestOrderService{
		listGuestOrdersFunc: func(_ context.Context, filter contracts.GuestOrderFilter) ([]models.GuestOrder, int64, error) {
			capturedFilter = filter
			return nil, 0, nil
		},
	}
	ts := guestTestServer(t, svc)

	resp, _ := guestDoReq(t, ts, "GET", "/v1/guest/orders?page=2&pageSize=5&state=FUNDED", nil)
	guestAssertStatus(t, resp, http.StatusOK)

	if capturedFilter.Page != 2 {
		t.Errorf("expected page=2, got %d", capturedFilter.Page)
	}
	if capturedFilter.PageSize != 5 {
		t.Errorf("expected pageSize=5, got %d", capturedFilter.PageSize)
	}
	if capturedFilter.State == nil || *capturedFilter.State != models.GuestOrderFunded {
		t.Errorf("expected state=FUNDED, got %v", capturedFilter.State)
	}
}

// ---------------------------------------------------------------------------
// H-08: PUT /v1/guest/orders/{token}/ship — valid → 204
// ---------------------------------------------------------------------------

func TestShipGuestOrder_Valid(t *testing.T) {
	var capturedToken, capturedTracking, capturedCarrier string
	svc := &mockGuestOrderService{
		shipGuestOrderFunc: func(_ context.Context, token, tracking, carrier string) error {
			capturedToken = token
			capturedTracking = tracking
			capturedCarrier = carrier
			return nil
		},
	}

	node := &mockGuestNode{guestSvc: svc}
	gateway := &Gateway{
		config:            &GatewayConfig{},
		guestOrderLimiter: newRateLimiter(1000, time.Hour),
	}
	outer := chi.NewMux()
	outer.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := context.WithValue(req.Context(), nodeContextKey, contracts.NodeService(node))
			// Inject admin identity so the post-fix nodeHumaAuthMiddleware
			// short-circuits the "already authenticated" check on
			// owner-only routes (ship / settings / complete). Auth pipeline
			// coverage lives in TestNodeBridgeRequestWithOptionalAuth_* +
			// TestGateway_AuthenticationMiddleware.
			ctx = WithAuthIdentity(ctx, &AuthIdentity{
				UserID:  "test-admin",
				IsAdmin: true,
			})
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	outer.Mount("/", gateway.newV1Router(false, false))

	body, _ := json.Marshal(map[string]string{
		"trackingNumber": "TRACK123",
		"carrier":        "UPS",
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/v1/guest/orders/tok_test/ship", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	outer.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d, body: %s", rr.Code, rr.Body.String())
	}
	if capturedToken != "tok_test" {
		t.Errorf("expected token tok_test, got %s", capturedToken)
	}
	if capturedTracking != "TRACK123" || capturedCarrier != "UPS" {
		t.Errorf("tracking/carrier mismatch: %s/%s", capturedTracking, capturedCarrier)
	}
}

// ---------------------------------------------------------------------------
// H-09: GET /v1/settings/guest-checkout → 200
// ---------------------------------------------------------------------------

func TestGETGuestCheckoutSettings(t *testing.T) {
	svc := &mockGuestOrderService{
		getGuestCheckoutCfgFunc: func(context.Context) (*models.GuestCheckoutConfig, error) {
			return &models.GuestCheckoutConfig{
				Enabled:       true,
				AcceptedCoins: "BTC,ETH,SOL",
			}, nil
		},
	}

	node := &mockGuestNode{guestSvc: svc}
	gateway := &Gateway{
		config:            &GatewayConfig{},
		guestOrderLimiter: newRateLimiter(1000, time.Hour),
	}
	outer := chi.NewMux()
	outer.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := context.WithValue(req.Context(), nodeContextKey, contracts.NodeService(node))
			// Inject admin identity so the post-fix nodeHumaAuthMiddleware
			// short-circuits the "already authenticated" check on
			// owner-only routes (ship / settings / complete). Auth pipeline
			// coverage lives in TestNodeBridgeRequestWithOptionalAuth_* +
			// TestGateway_AuthenticationMiddleware.
			ctx = WithAuthIdentity(ctx, &AuthIdentity{
				UserID:  "test-admin",
				IsAdmin: true,
			})
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	outer.Mount("/", gateway.newV1Router(false, false))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/settings/guest-checkout", nil)
	outer.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d, body: %s", rr.Code, rr.Body.String())
	}

	var envelope struct {
		Data models.GuestCheckoutConfig `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal: %s", err)
	}
	if !envelope.Data.Enabled {
		t.Error("expected enabled=true")
	}
	if envelope.Data.AcceptedCoins != "BTC,ETH,SOL" {
		t.Errorf("expected BTC,ETH,SOL, got %s", envelope.Data.AcceptedCoins)
	}
}

// ---------------------------------------------------------------------------
// H-10: PUT /v1/settings/guest-checkout — save config → 200
// ---------------------------------------------------------------------------

func TestPUTGuestCheckoutSettings(t *testing.T) {
	var saved *models.GuestCheckoutConfig
	svc := &mockGuestOrderService{
		saveGuestCheckoutCfgFunc: func(_ context.Context, cfg *models.GuestCheckoutConfig) error {
			saved = cfg
			return nil
		},
	}

	node := &mockGuestNode{guestSvc: svc}
	gateway := &Gateway{
		config:            &GatewayConfig{},
		guestOrderLimiter: newRateLimiter(1000, time.Hour),
	}
	outer := chi.NewMux()
	outer.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := context.WithValue(req.Context(), nodeContextKey, contracts.NodeService(node))
			// Inject admin identity so the post-fix nodeHumaAuthMiddleware
			// short-circuits the "already authenticated" check on
			// owner-only routes (ship / settings / complete). Auth pipeline
			// coverage lives in TestNodeBridgeRequestWithOptionalAuth_* +
			// TestGateway_AuthenticationMiddleware.
			ctx = WithAuthIdentity(ctx, &AuthIdentity{
				UserID:  "test-admin",
				IsAdmin: true,
			})
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	outer.Mount("/", gateway.newV1Router(false, false))

	body, _ := json.Marshal(models.GuestCheckoutConfig{
		Enabled:        true,
		AcceptedCoins:  "BTC",
		PaymentTimeout: 30,
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/v1/settings/guest-checkout", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	outer.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d, body: %s", rr.Code, rr.Body.String())
	}
	if saved == nil {
		t.Fatal("config was not saved")
	}
	if !saved.Enabled {
		t.Error("expected saved.Enabled=true")
	}
	if saved.AcceptedCoins != "BTC" {
		t.Errorf("expected BTC, got %s", saved.AcceptedCoins)
	}
}

// ---------------------------------------------------------------------------
// H-extra: POST /v1/guest/orders — malformed JSON body → 400 (Huma body parse)
// ---------------------------------------------------------------------------

func TestPOSTGuestOrder_InvalidJSON(t *testing.T) {
	svc := &mockGuestOrderService{}
	ts := guestTestServer(t, svc)

	resp, _ := guestDoReq(t, ts, "POST", "/v1/guest/orders", []byte(`{invalid`))
	guestAssertStatus(t, resp, http.StatusBadRequest)
}

// ---------------------------------------------------------------------------
// H-extra: PUT /v1/guest/orders/{token}/complete — valid → 204
// ---------------------------------------------------------------------------

func TestCompleteGuestOrder_Valid(t *testing.T) {
	var capturedToken string
	svc := &mockGuestOrderService{
		completeGuestOrderFunc: func(_ context.Context, token string) error {
			capturedToken = token
			return nil
		},
	}

	node := &mockGuestNode{guestSvc: svc}
	gateway := &Gateway{
		config:            &GatewayConfig{},
		guestOrderLimiter: newRateLimiter(1000, time.Hour),
	}
	outer := chi.NewMux()
	outer.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := context.WithValue(req.Context(), nodeContextKey, contracts.NodeService(node))
			// Inject admin identity so the post-fix nodeHumaAuthMiddleware
			// short-circuits the "already authenticated" check on
			// owner-only routes (ship / settings / complete). Auth pipeline
			// coverage lives in TestNodeBridgeRequestWithOptionalAuth_* +
			// TestGateway_AuthenticationMiddleware.
			ctx = WithAuthIdentity(ctx, &AuthIdentity{
				UserID:  "test-admin",
				IsAdmin: true,
			})
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	outer.Mount("/", gateway.newV1Router(false, false))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/v1/guest/orders/tok_xyz/complete", nil)
	outer.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d, body: %s", rr.Code, rr.Body.String())
	}
	if capturedToken != "tok_xyz" {
		t.Errorf("expected tok_xyz, got %s", capturedToken)
	}
}

// ---------------------------------------------------------------------------
// H-extra: GuestOrder() returns nil (feature disabled) → 501
// ---------------------------------------------------------------------------

func TestGuestOrder_NotImplemented(t *testing.T) {
	node := &mockNode{} // mockNode.GuestOrder() returns nil
	gateway := &Gateway{
		config:            &GatewayConfig{},
		guestOrderLimiter: newRateLimiter(1000, time.Hour),
	}
	outer := chi.NewMux()
	outer.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := context.WithValue(req.Context(), nodeContextKey, contracts.NodeService(node))
			// Inject admin identity so the post-fix nodeHumaAuthMiddleware
			// short-circuits the "already authenticated" check on
			// owner-only routes (ship / settings / complete). Auth pipeline
			// coverage lives in TestNodeBridgeRequestWithOptionalAuth_* +
			// TestGateway_AuthenticationMiddleware.
			ctx = WithAuthIdentity(ctx, &AuthIdentity{
				UserID:  "test-admin",
				IsAdmin: true,
			})
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	outer.Mount("/", gateway.newV1Router(false, false))

	ts := httptest.NewServer(outer)
	defer ts.Close()

	body, _ := json.Marshal(contracts.CreateGuestOrderRequest{
		Items:       []contracts.GuestOrderItemRequest{{ListingSlug: "item", Quantity: 1}},
		PaymentCoin: "BTC",
	})

	resp, respBody := guestDoReq(t, ts, "POST", "/v1/guest/orders", body)
	guestAssertStatus(t, resp, http.StatusNotImplemented)
	guestAssertErrorCode(t, respBody, "NOT_IMPLEMENTED")

	resp2, _ := guestDoReq(t, ts, "GET", "/v1/guest/orders/tok_any", nil)
	guestAssertStatus(t, resp2, http.StatusNotImplemented)
}

// ---------------------------------------------------------------------------
// H-11: PUT /v1/settings/guest-checkout via real HTTP — frontend DTO format
// Validates the Huma bridge correctly forwards the body through a real TCP
// connection (closer to production SaaS path than httptest.NewRequest).
// ---------------------------------------------------------------------------

func TestPUTGuestCheckoutSettings_RealHTTP_FrontendDTO(t *testing.T) {
	var saved *models.GuestCheckoutConfig
	svc := &mockGuestOrderService{
		saveGuestCheckoutCfgFunc: func(_ context.Context, cfg *models.GuestCheckoutConfig) error {
			saved = cfg
			return nil
		},
	}
	ts := guestTestServer(t, svc)

	// Simulate exactly what the frontend sends (GuestCheckoutSettingsDTO format)
	payload := `{"enabled":true,"acceptedCoins":"ETH","paymentTimeout":120}`
	resp, respBody := guestDoReq(t, ts, "PUT", "/v1/settings/guest-checkout", []byte(payload))
	guestAssertStatus(t, resp, http.StatusOK)

	if saved == nil {
		t.Fatalf("config was not saved; response body: %s", string(respBody))
	}
	if !saved.Enabled {
		t.Error("expected saved.Enabled=true")
	}
	if saved.AcceptedCoins != "ETH" {
		t.Errorf("expected ETH, got %s", saved.AcceptedCoins)
	}
	if saved.PaymentTimeout != 120 {
		t.Errorf("expected 120, got %d", saved.PaymentTimeout)
	}
}

func TestPUTGuestCheckoutSettings_RealHTTP_UIStateDTO(t *testing.T) {
	var saved *models.GuestCheckoutConfig
	svc := &mockGuestOrderService{
		saveGuestCheckoutCfgFunc: func(_ context.Context, cfg *models.GuestCheckoutConfig) error {
			saved = cfg
			return nil
		},
	}
	ts := guestTestServer(t, svc)

	payload := `{"enabled":true,"acceptedCoins":["ETH"," BTC ",""],"paymentTimeoutMinutes":120}`
	resp, respBody := guestDoReq(t, ts, "PUT", "/v1/settings/guest-checkout", []byte(payload))
	guestAssertStatus(t, resp, http.StatusOK)

	if saved == nil {
		t.Fatalf("config was not saved; response body: %s", string(respBody))
	}
	if !saved.Enabled {
		t.Error("expected saved.Enabled=true")
	}
	if saved.AcceptedCoins != "ETH,BTC" {
		t.Errorf("expected normalized ETH,BTC, got %s", saved.AcceptedCoins)
	}
	if saved.PaymentTimeout != 120 {
		t.Errorf("expected 120, got %d", saved.PaymentTimeout)
	}
}

func TestPUTGuestCheckoutSettings_IgnoresPGPPublicKey(t *testing.T) {
	var saved *models.GuestCheckoutConfig
	svc := &mockGuestOrderService{
		saveGuestCheckoutCfgFunc: func(_ context.Context, cfg *models.GuestCheckoutConfig) error {
			saved = cfg
			return nil
		},
	}
	ts := guestTestServer(t, svc)

	payload := `{"enabled":true,"acceptedCoins":"ETH","paymentTimeout":120,"pgpPublicKey":"-----BEGIN PGP PUBLIC KEY BLOCK-----\nignored\n-----END PGP PUBLIC KEY BLOCK-----"}`
	resp, respBody := guestDoReq(t, ts, "PUT", "/v1/settings/guest-checkout", []byte(payload))
	guestAssertStatus(t, resp, http.StatusOK)

	if saved == nil {
		t.Fatalf("config was not saved; response body: %s", string(respBody))
	}
	if saved.PGPPublicKey != "" {
		t.Fatalf("settings save path must not mutate PGP key, got %q", saved.PGPPublicKey)
	}
}

func TestPUTGuestCheckoutSettings_SyncsTenantFeatureFlag(t *testing.T) {
	var saved *models.GuestCheckoutConfig
	store := newMemTenantStore()
	svc := &mockGuestOrderService{
		saveGuestCheckoutCfgFunc: func(_ context.Context, cfg *models.GuestCheckoutConfig) error {
			saved = cfg
			return nil
		},
	}
	node := &mockGuestNode{guestSvc: svc, featureStore: store}
	ts := guestTestServerWithNode(t, node)

	payload := `{"enabled":true,"acceptedCoins":"ETH,LTC","paymentTimeout":120}`
	resp, respBody := guestDoReq(t, ts, "PUT", "/v1/settings/guest-checkout", []byte(payload))
	guestAssertStatus(t, resp, http.StatusOK)

	if saved == nil {
		t.Fatalf("config was not saved; response body: %s", string(respBody))
	}
	if store.lastSet.tenant != database.StandaloneTenantID {
		t.Errorf("tenant mismatch: got %q, want %q", store.lastSet.tenant, database.StandaloneTenantID)
	}
	if store.lastSet.key != pkgconfig.FeatureGuestCheckoutEnabled.Key {
		t.Errorf("key mismatch: got %q", store.lastSet.key)
	}
	if !store.lastSet.value {
		t.Error("expected guestCheckout tenant feature to be enabled")
	}
	if store.lastSet.actor == "" {
		t.Error("actor should not be empty")
	}
}
