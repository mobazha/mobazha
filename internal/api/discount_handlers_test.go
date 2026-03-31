package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/models"
)

// mockDiscountService implements contracts.DiscountService for handler tests.
type mockDiscountService struct {
	createDiscountFunc     func(ctx context.Context, d *models.Discount) error
	getDiscountFunc        func(ctx context.Context, id string) (*models.Discount, error)
	listDiscountsFunc      func(ctx context.Context, filter contracts.DiscountFilter) ([]models.Discount, int64, error)
	updateDiscountFunc     func(ctx context.Context, d *models.Discount) error
	deleteDiscountFunc     func(ctx context.Context, id string) error
	addCodesFunc           func(ctx context.Context, discountID string, codes []models.DiscountCode) error
	generateCodesFunc      func(ctx context.Context, discountID string, count int, prefix string) ([]models.DiscountCode, error)
	listCodesFunc          func(ctx context.Context, discountID string) ([]models.DiscountCode, error)
	deleteCodeFunc         func(ctx context.Context, codeID string) error
	validateCodeFunc       func(ctx context.Context, code string, customerPeerID string) (*contracts.ValidateCodeResult, error)
	getApplicableFunc      func(ctx context.Context, productIDs []string) ([]models.Discount, error)
	recordRedemptionFunc   func(ctx context.Context, discountID string, codeID *string, orderID, customerPeerID, discountAmount, currency string) error
	calculateDiscountsFunc func(ctx context.Context, req contracts.CalculateDiscountsRequest) (*contracts.CalculateDiscountsResult, error)
	listRedemptionsFunc    func(ctx context.Context, discountID string, page, pageSize int) ([]models.DiscountRedemption, int64, error)
}

func (m *mockDiscountService) CreateDiscount(ctx context.Context, d *models.Discount) error {
	if m.createDiscountFunc != nil {
		return m.createDiscountFunc(ctx, d)
	}
	return nil
}

func (m *mockDiscountService) GetDiscount(ctx context.Context, id string) (*models.Discount, error) {
	if m.getDiscountFunc != nil {
		return m.getDiscountFunc(ctx, id)
	}
	return nil, errors.New("not found")
}

func (m *mockDiscountService) ListDiscounts(ctx context.Context, filter contracts.DiscountFilter) ([]models.Discount, int64, error) {
	if m.listDiscountsFunc != nil {
		return m.listDiscountsFunc(ctx, filter)
	}
	return nil, 0, nil
}

func (m *mockDiscountService) UpdateDiscount(ctx context.Context, d *models.Discount) error {
	if m.updateDiscountFunc != nil {
		return m.updateDiscountFunc(ctx, d)
	}
	return nil
}

func (m *mockDiscountService) DeleteDiscount(ctx context.Context, id string) error {
	if m.deleteDiscountFunc != nil {
		return m.deleteDiscountFunc(ctx, id)
	}
	return nil
}

func (m *mockDiscountService) AddCodes(ctx context.Context, discountID string, codes []models.DiscountCode) error {
	if m.addCodesFunc != nil {
		return m.addCodesFunc(ctx, discountID, codes)
	}
	return nil
}

func (m *mockDiscountService) GenerateCodes(ctx context.Context, discountID string, count int, prefix string) ([]models.DiscountCode, error) {
	if m.generateCodesFunc != nil {
		return m.generateCodesFunc(ctx, discountID, count, prefix)
	}
	return nil, nil
}

func (m *mockDiscountService) ListCodes(ctx context.Context, discountID string) ([]models.DiscountCode, error) {
	if m.listCodesFunc != nil {
		return m.listCodesFunc(ctx, discountID)
	}
	return nil, nil
}

func (m *mockDiscountService) DeleteCode(ctx context.Context, codeID string) error {
	if m.deleteCodeFunc != nil {
		return m.deleteCodeFunc(ctx, codeID)
	}
	return nil
}

func (m *mockDiscountService) ValidateCode(ctx context.Context, code string, customerPeerID string) (*contracts.ValidateCodeResult, error) {
	if m.validateCodeFunc != nil {
		return m.validateCodeFunc(ctx, code, customerPeerID)
	}
	return nil, errors.New("not found")
}

func (m *mockDiscountService) GetApplicableDiscounts(ctx context.Context, productIDs []string) ([]models.Discount, error) {
	if m.getApplicableFunc != nil {
		return m.getApplicableFunc(ctx, productIDs)
	}
	return nil, nil
}

func (m *mockDiscountService) RecordRedemption(ctx context.Context, discountID string, codeID *string, orderID, customerPeerID, discountAmount, currency string) error {
	if m.recordRedemptionFunc != nil {
		return m.recordRedemptionFunc(ctx, discountID, codeID, orderID, customerPeerID, discountAmount, currency)
	}
	return nil
}

func (m *mockDiscountService) CalculateDiscounts(ctx context.Context, req contracts.CalculateDiscountsRequest) (*contracts.CalculateDiscountsResult, error) {
	if m.calculateDiscountsFunc != nil {
		return m.calculateDiscountsFunc(ctx, req)
	}
	return &contracts.CalculateDiscountsResult{DiscountsTotal: big.NewInt(0)}, nil
}

func (m *mockDiscountService) ListRedemptions(ctx context.Context, discountID string, page, pageSize int) ([]models.DiscountRedemption, int64, error) {
	if m.listRedemptionsFunc != nil {
		return m.listRedemptionsFunc(ctx, discountID, page, pageSize)
	}
	return nil, 0, nil
}

// mockDiscountNode embeds mockNode and implements contracts.DiscountProvider.
type mockDiscountNode struct {
	mockNode
	discountSvc *mockDiscountService
}

func (n *mockDiscountNode) Discount() contracts.DiscountService {
	return n.discountSvc
}

// discountTestServer spins up an httptest.Server with a mockDiscountNode injected via context.
func discountTestServer(t *testing.T, svc *mockDiscountService) (*httptest.Server, *mockDiscountNode) {
	t.Helper()
	node := &mockDiscountNode{discountSvc: svc}

	gateway := &Gateway{config: &GatewayConfig{}}
	r := gateway.newV1Router()

	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := context.WithValue(req.Context(), nodeContextKey, contracts.NodeService(node))
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})

	ts := httptest.NewServer(r)
	t.Cleanup(ts.Close)
	return ts, node
}

func doReq(t *testing.T, ts *httptest.Server, method, path string, body []byte) (*http.Response, []byte) {
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

func assertStatus(t *testing.T, resp *http.Response, expected int) {
	t.Helper()
	if resp.StatusCode != expected {
		t.Errorf("expected status %d, got %d", expected, resp.StatusCode)
	}
}

func assertJSONPath(t *testing.T, body []byte, path string, expected interface{}) {
	t.Helper()
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("cannot unmarshal response: %s", err)
	}

	switch path {
	case "error.code":
		if errObj, ok := raw["error"].(map[string]interface{}); ok {
			if errObj["code"] != expected {
				t.Errorf("expected %s=%v, got %v", path, expected, errObj["code"])
			}
		} else {
			t.Errorf("expected error object in response")
		}
	case "data":
		if raw["data"] == nil {
			t.Errorf("expected data in response")
		}
	}
}

// =========================================================================
// CRUD Tests
// =========================================================================

func TestDiscountHandlers_Create(t *testing.T) {
	svc := &mockDiscountService{
		createDiscountFunc: func(_ context.Context, d *models.Discount) error {
			d.ID = "disc-001"
			return nil
		},
	}
	ts, _ := discountTestServer(t, svc)

	t.Run("success", func(t *testing.T) {
		body := []byte(`{"title":"Summer Sale","method":"code","valueType":"percentage","value":"10","scope":"order","appliesTo":"all","startsAt":"2026-01-01T00:00:00Z"}`)
		resp, respBody := doReq(t, ts, http.MethodPost, "/v1/discounts", body)
		assertStatus(t, resp, http.StatusCreated)

		var envelope map[string]json.RawMessage
		json.Unmarshal(respBody, &envelope)
		if envelope["data"] == nil {
			t.Error("expected data envelope")
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		resp, _ := doReq(t, ts, http.MethodPost, "/v1/discounts", []byte(`{invalid`))
		assertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("service error", func(t *testing.T) {
		svc.createDiscountFunc = func(_ context.Context, _ *models.Discount) error {
			return errors.New("title is required")
		}
		resp, _ := doReq(t, ts, http.MethodPost, "/v1/discounts", []byte(`{}`))
		assertStatus(t, resp, http.StatusBadRequest)
	})
}

func TestDiscountHandlers_List(t *testing.T) {
	svc := &mockDiscountService{
		listDiscountsFunc: func(_ context.Context, f contracts.DiscountFilter) ([]models.Discount, int64, error) {
			return []models.Discount{{ID: "d1", Title: "Test"}}, 1, nil
		},
	}
	ts, _ := discountTestServer(t, svc)

	t.Run("success", func(t *testing.T) {
		resp, respBody := doReq(t, ts, http.MethodGet, "/v1/discounts", nil)
		assertStatus(t, resp, http.StatusOK)

		var envelope map[string]json.RawMessage
		json.Unmarshal(respBody, &envelope)
		if envelope["data"] == nil {
			t.Error("expected data field")
		}
		if envelope["meta"] == nil {
			t.Error("expected meta field")
		}
	})

	t.Run("with filters", func(t *testing.T) {
		svc.listDiscountsFunc = func(_ context.Context, f contracts.DiscountFilter) ([]models.Discount, int64, error) {
			if f.Status == nil || *f.Status != models.DiscountStatusActive {
				t.Error("expected active status filter")
			}
			if f.Page != 2 {
				t.Error("expected page 2")
			}
			return nil, 0, nil
		}
		resp, _ := doReq(t, ts, http.MethodGet, "/v1/discounts?status=active&page=2", nil)
		assertStatus(t, resp, http.StatusOK)
	})
}

func TestDiscountHandlers_Get(t *testing.T) {
	svc := &mockDiscountService{
		getDiscountFunc: func(_ context.Context, id string) (*models.Discount, error) {
			if id == "disc-001" {
				return &models.Discount{ID: "disc-001", Title: "Test"}, nil
			}
			return nil, errors.New("not found")
		},
	}
	ts, _ := discountTestServer(t, svc)

	t.Run("found", func(t *testing.T) {
		resp, _ := doReq(t, ts, http.MethodGet, "/v1/discounts/disc-001", nil)
		assertStatus(t, resp, http.StatusOK)
	})

	t.Run("not found", func(t *testing.T) {
		resp, _ := doReq(t, ts, http.MethodGet, "/v1/discounts/nonexistent", nil)
		assertStatus(t, resp, http.StatusNotFound)
	})
}

func TestDiscountHandlers_Update(t *testing.T) {
	svc := &mockDiscountService{
		getDiscountFunc: func(_ context.Context, id string) (*models.Discount, error) {
			if id == "disc-001" {
				return &models.Discount{ID: "disc-001", Title: "Old", UsageCount: 5}, nil
			}
			return nil, errors.New("not found")
		},
		updateDiscountFunc: func(_ context.Context, d *models.Discount) error {
			if d.UsageCount != 5 {
				return fmt.Errorf("usageCount should be preserved, got %d", d.UsageCount)
			}
			return nil
		},
	}
	ts, _ := discountTestServer(t, svc)

	t.Run("success preserves usageCount", func(t *testing.T) {
		body := []byte(`{"title":"Updated","usageCount":999}`)
		resp, _ := doReq(t, ts, http.MethodPut, "/v1/discounts/disc-001", body)
		assertStatus(t, resp, http.StatusOK)
	})

	t.Run("not found", func(t *testing.T) {
		resp, _ := doReq(t, ts, http.MethodPut, "/v1/discounts/nonexistent", []byte(`{}`))
		assertStatus(t, resp, http.StatusNotFound)
	})

	t.Run("invalid body", func(t *testing.T) {
		resp, _ := doReq(t, ts, http.MethodPut, "/v1/discounts/disc-001", []byte(`{bad`))
		assertStatus(t, resp, http.StatusBadRequest)
	})
}

func TestDiscountHandlers_Delete(t *testing.T) {
	svc := &mockDiscountService{
		deleteDiscountFunc: func(_ context.Context, id string) error {
			if id == "disc-001" {
				return nil
			}
			return errors.New("not found")
		},
	}
	ts, _ := discountTestServer(t, svc)

	t.Run("success", func(t *testing.T) {
		resp, _ := doReq(t, ts, http.MethodDelete, "/v1/discounts/disc-001", nil)
		assertStatus(t, resp, http.StatusNoContent)
	})

	t.Run("not found", func(t *testing.T) {
		resp, _ := doReq(t, ts, http.MethodDelete, "/v1/discounts/nonexistent", nil)
		assertStatus(t, resp, http.StatusNotFound)
	})
}

// =========================================================================
// Codes Management Tests
// =========================================================================

func TestDiscountHandlers_AddCodes(t *testing.T) {
	svc := &mockDiscountService{
		addCodesFunc: func(_ context.Context, discountID string, codes []models.DiscountCode) error {
			if discountID != "disc-001" {
				return errors.New("not found")
			}
			return nil
		},
	}
	ts, _ := discountTestServer(t, svc)

	t.Run("add manual codes", func(t *testing.T) {
		body := []byte(`{"codes":[{"code":"SAVE10"}]}`)
		resp, _ := doReq(t, ts, http.MethodPost, "/v1/discounts/disc-001/codes", body)
		assertStatus(t, resp, http.StatusCreated)
	})

	t.Run("generate codes", func(t *testing.T) {
		svc.generateCodesFunc = func(_ context.Context, _ string, count int, prefix string) ([]models.DiscountCode, error) {
			codes := make([]models.DiscountCode, count)
			for i := range codes {
				codes[i] = models.DiscountCode{Code: fmt.Sprintf("%s-%04d", prefix, i)}
			}
			return codes, nil
		}
		body := []byte(`{"generate":{"count":3,"prefix":"SUMMER"}}`)
		resp, _ := doReq(t, ts, http.MethodPost, "/v1/discounts/disc-001/codes", body)
		assertStatus(t, resp, http.StatusCreated)
	})

	t.Run("empty codes without generate", func(t *testing.T) {
		body := []byte(`{"codes":[]}`)
		resp, _ := doReq(t, ts, http.MethodPost, "/v1/discounts/disc-001/codes", body)
		assertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("invalid JSON", func(t *testing.T) {
		resp, _ := doReq(t, ts, http.MethodPost, "/v1/discounts/disc-001/codes", []byte(`{bad`))
		assertStatus(t, resp, http.StatusBadRequest)
	})
}

func TestDiscountHandlers_ListCodes(t *testing.T) {
	svc := &mockDiscountService{
		listCodesFunc: func(_ context.Context, discountID string) ([]models.DiscountCode, error) {
			return []models.DiscountCode{{ID: "code-1", Code: "SAVE10"}}, nil
		},
	}
	ts, _ := discountTestServer(t, svc)

	resp, _ := doReq(t, ts, http.MethodGet, "/v1/discounts/disc-001/codes", nil)
	assertStatus(t, resp, http.StatusOK)
}

func TestDiscountHandlers_DeleteCode(t *testing.T) {
	svc := &mockDiscountService{
		deleteCodeFunc: func(_ context.Context, codeID string) error {
			if codeID == "code-1" {
				return nil
			}
			return errors.New("not found")
		},
	}
	ts, _ := discountTestServer(t, svc)

	t.Run("success", func(t *testing.T) {
		resp, _ := doReq(t, ts, http.MethodDelete, "/v1/discounts/disc-001/codes/code-1", nil)
		assertStatus(t, resp, http.StatusNoContent)
	})

	t.Run("not found", func(t *testing.T) {
		resp, _ := doReq(t, ts, http.MethodDelete, "/v1/discounts/disc-001/codes/bad", nil)
		assertStatus(t, resp, http.StatusNotFound)
	})
}

func TestDiscountHandlers_ListRedemptions(t *testing.T) {
	svc := &mockDiscountService{
		listRedemptionsFunc: func(_ context.Context, discountID string, page, pageSize int) ([]models.DiscountRedemption, int64, error) {
			return []models.DiscountRedemption{{ID: "r1", DiscountID: discountID}}, 1, nil
		},
	}
	ts, _ := discountTestServer(t, svc)

	resp, respBody := doReq(t, ts, http.MethodGet, "/v1/discounts/disc-001/redemptions?page=1&pageSize=10", nil)
	assertStatus(t, resp, http.StatusOK)

	var envelope map[string]json.RawMessage
	json.Unmarshal(respBody, &envelope)
	if envelope["meta"] == nil {
		t.Error("expected meta in response")
	}
}

// =========================================================================
// Public Endpoint Tests
// =========================================================================

func TestDiscountHandlers_Validate(t *testing.T) {
	maxAmt := "50"
	svc := &mockDiscountService{
		validateCodeFunc: func(_ context.Context, code string, _ string) (*contracts.ValidateCodeResult, error) {
			if code == "SAVE10" {
				return &contracts.ValidateCodeResult{
					Valid: true,
					Discount: &models.Discount{
						Title:             "10% Off",
						ValueType:         models.DiscountValuePercentage,
						Value:             "10",
						MaxDiscountAmount: &maxAmt,
					},
				}, nil
			}
			if code == "EXPIRED" {
				return &contracts.ValidateCodeResult{
					Valid:  false,
					Reason: "discount has expired",
				}, nil
			}
			return nil, errors.New("not found")
		},
	}
	ts, _ := discountTestServer(t, svc)

	t.Run("valid code", func(t *testing.T) {
		body := []byte(`{"code":"SAVE10"}`)
		resp, respBody := doReq(t, ts, http.MethodPost, "/v1/discounts/test-peer/validate", body)
		assertStatus(t, resp, http.StatusOK)

		var envelope map[string]json.RawMessage
		json.Unmarshal(respBody, &envelope)
		var data map[string]interface{}
		json.Unmarshal(envelope["data"], &data)
		if data["valid"] != true {
			t.Error("expected valid=true")
		}
		if data["maxDiscountAmount"] != "50" {
			t.Errorf("expected maxDiscountAmount=50, got %v", data["maxDiscountAmount"])
		}
	})

	t.Run("invalid code (expired)", func(t *testing.T) {
		body := []byte(`{"code":"EXPIRED"}`)
		resp, _ := doReq(t, ts, http.MethodPost, "/v1/discounts/test-peer/validate", body)
		assertStatus(t, resp, http.StatusUnprocessableEntity)
	})

	t.Run("code not found", func(t *testing.T) {
		body := []byte(`{"code":"BADCODE"}`)
		resp, _ := doReq(t, ts, http.MethodPost, "/v1/discounts/test-peer/validate", body)
		assertStatus(t, resp, http.StatusNotFound)
	})

	t.Run("empty code", func(t *testing.T) {
		body := []byte(`{"code":""}`)
		resp, _ := doReq(t, ts, http.MethodPost, "/v1/discounts/test-peer/validate", body)
		assertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("invalid JSON", func(t *testing.T) {
		resp, _ := doReq(t, ts, http.MethodPost, "/v1/discounts/test-peer/validate", []byte(`{bad`))
		assertStatus(t, resp, http.StatusBadRequest)
	})
}

// Public endpoint tests use direct handler invocation (httptest.NewRecorder)
// because /v1/discounts/{discountID} (auth, GET) shadows /v1/discounts/applicable (public, GET)
// in gorilla/mux route matching order.

func publicHandlerReq(t *testing.T, svc *mockDiscountService, handler http.HandlerFunc, method, target string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	node := &mockDiscountNode{discountSvc: svc}
	req := httptest.NewRequest(method, target, reader)
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), nodeContextKey, contracts.NodeService(node))
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

func TestDiscountHandlers_GetApplicable(t *testing.T) {
	gw := &Gateway{config: &GatewayConfig{}}

	svc := &mockDiscountService{
		getApplicableFunc: func(_ context.Context, productIDs []string) ([]models.Discount, error) {
			minAmt := "100"
			return []models.Discount{
				{
					Title:           "Auto 5% Off",
					ValueType:       models.DiscountValuePercentage,
					Value:           "5",
					MinPurchaseType: models.DiscountMinPurchaseAmount,
					MinAmount:       &minAmt,
				},
				{
					Title:     "Free Shipping",
					ValueType: models.DiscountValueFreeShipping,
				},
			}, nil
		},
	}

	t.Run("returns summaries", func(t *testing.T) {
		rr := publicHandlerReq(t, svc, gw.handleGetApplicableDiscounts, http.MethodGet, "/v1/discounts/applicable", nil)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
		}

		var envelope map[string]json.RawMessage
		json.Unmarshal(rr.Body.Bytes(), &envelope)
		var data []map[string]interface{}
		json.Unmarshal(envelope["data"], &data)
		if len(data) != 2 {
			t.Fatalf("expected 2 discounts, got %d", len(data))
		}
		if data[0]["minPurchaseType"] != "min_amount" {
			t.Errorf("expected min_amount, got %v", data[0]["minPurchaseType"])
		}
		if data[0]["minAmount"] != "100" {
			t.Errorf("expected minAmount=100, got %v", data[0]["minAmount"])
		}
	})

	t.Run("with listingSlug filter", func(t *testing.T) {
		svc.getApplicableFunc = func(_ context.Context, productIDs []string) ([]models.Discount, error) {
			if len(productIDs) != 1 || productIDs[0] != "my-product" {
				t.Errorf("expected productIDs=[my-product], got %v", productIDs)
			}
			return nil, nil
		}
		rr := publicHandlerReq(t, svc, gw.handleGetApplicableDiscounts, http.MethodGet, "/v1/discounts/applicable?listingSlug=my-product", nil)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})
}

func TestDiscountHandlers_Calculate(t *testing.T) {
	svc := &mockDiscountService{
		calculateDiscountsFunc: func(_ context.Context, req contracts.CalculateDiscountsRequest) (*contracts.CalculateDiscountsResult, error) {
			if req.Subtotal == "" {
				return nil, errors.New("subtotal required")
			}
			return &contracts.CalculateDiscountsResult{
				AppliedDiscounts: []contracts.AppliedDiscountInfo{
					{
						DiscountID: "d1",
						Title:      "10% Off",
						ValueType:  "percentage",
						Value:      "10",
						Amount:     "500",
					},
				},
				DiscountsTotal:   big.NewInt(-500),
				ShippingDiscount: false,
			}, nil
		},
	}
	ts, _ := discountTestServer(t, svc)

	t.Run("success", func(t *testing.T) {
		body := []byte(`{"subtotal":"5000","currency":"USD","discountCodes":["SAVE10"]}`)
		resp, respBody := doReq(t, ts, http.MethodPost, "/v1/discounts/test-peer/calculate", body)
		assertStatus(t, resp, http.StatusOK)

		var envelope map[string]json.RawMessage
		json.Unmarshal(respBody, &envelope)
		var data map[string]interface{}
		json.Unmarshal(envelope["data"], &data)
		if data["discountsTotal"] != "-500" {
			t.Errorf("expected discountsTotal=-500, got %v", data["discountsTotal"])
		}
		applied := data["appliedDiscounts"].([]interface{})
		if len(applied) != 1 {
			t.Errorf("expected 1 applied discount, got %d", len(applied))
		}
	})

	t.Run("missing subtotal", func(t *testing.T) {
		body := []byte(`{"currency":"USD"}`)
		resp, _ := doReq(t, ts, http.MethodPost, "/v1/discounts/test-peer/calculate", body)
		assertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("invalid body", func(t *testing.T) {
		resp, _ := doReq(t, ts, http.MethodPost, "/v1/discounts/test-peer/calculate", []byte(`{bad`))
		assertStatus(t, resp, http.StatusBadRequest)
	})
}

// =========================================================================
// Edge case: node without DiscountProvider returns 501
// =========================================================================

func TestDiscountHandlers_NoProvider(t *testing.T) {
	node := &mockNode{}
	gateway := &Gateway{config: &GatewayConfig{}}
	r := gateway.newV1Router()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := context.WithValue(req.Context(), nodeContextKey, contracts.NodeService(node))
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	ts := httptest.NewServer(r)
	defer ts.Close()

	routerEndpoints := []struct {
		method string
		path   string
		body   []byte
	}{
		{http.MethodPost, "/v1/discounts", []byte(`{}`)},
		{http.MethodGet, "/v1/discounts", nil},
		{http.MethodGet, "/v1/discounts/abc", nil},
		{http.MethodPut, "/v1/discounts/abc", []byte(`{}`)},
		{http.MethodDelete, "/v1/discounts/abc", nil},
		{http.MethodPost, "/v1/discounts/abc/codes", []byte(`{}`)},
		{http.MethodGet, "/v1/discounts/abc/codes", nil},
		{http.MethodDelete, "/v1/discounts/abc/codes/c1", nil},
		{http.MethodGet, "/v1/discounts/abc/redemptions", nil},
		{http.MethodPost, "/v1/discounts/test-peer/validate", []byte(`{"code":"X"}`)},
		{http.MethodPost, "/v1/discounts/test-peer/calculate", []byte(`{"subtotal":"100"}`)},
	}

	for _, ep := range routerEndpoints {
		t.Run(fmt.Sprintf("%s %s", ep.method, ep.path), func(t *testing.T) {
			var reader io.Reader
			if ep.body != nil {
				reader = bytes.NewReader(ep.body)
			}
			req, _ := http.NewRequest(ep.method, ts.URL+ep.path, reader)
			req.Header.Set("Content-Type", "application/json")
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusNotImplemented {
				t.Errorf("expected 501, got %d", resp.StatusCode)
			}
		})
	}

	// /v1/discounts/applicable (GET) tested via direct handler call
	// because /v1/discounts/{discountID} (GET) shadows it in mux ordering
	t.Run("GET /v1/discounts/applicable (direct)", func(t *testing.T) {
		gw := &Gateway{config: &GatewayConfig{}}
		req := httptest.NewRequest(http.MethodGet, "/v1/discounts/applicable", nil)
		ctx := context.WithValue(req.Context(), nodeContextKey, contracts.NodeService(node))
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()
		gw.handleGetApplicableDiscounts(rr, req)
		if rr.Code != http.StatusNotImplemented {
			t.Errorf("expected 501, got %d", rr.Code)
		}
	})
}
