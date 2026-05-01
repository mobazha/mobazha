package cj

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func computeMD5(s string) string {
	h := md5.Sum([]byte(s))
	return fmt.Sprintf("%x", h)
}

func cjOK(data interface{}) string {
	d, _ := json.Marshal(data)
	return fmt.Sprintf(`{"result":true,"code":200,"message":"success","data":%s}`, string(d))
}

func cjErr(code int, msg string) string {
	return fmt.Sprintf(`{"result":false,"code":%d,"message":"%s","data":null}`, code, msg)
}

func newTestProvider(t *testing.T, handler http.HandlerFunc) *Provider {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	p := NewProvider("test-api-key", "test-webhook-secret",
		WithBaseURL(srv.URL), WithQPS(1000))
	p.client.SetAccessToken("test-access-token")
	return p
}

// ---------------------------------------------------------------------------
// ValidateCredentials
// ---------------------------------------------------------------------------

func TestValidateCredentials_Success(t *testing.T) {
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/authentication/getAccessToken")
		fmt.Fprint(w, cjOK(map[string]string{
			"accessToken":           "fresh-token",
			"accessTokenExpiryDate": "2099-01-01 00:00:00",
		}))
	})

	err := p.ValidateCredentials(context.Background(), contracts.ProviderCredentials{
		APIKey: "valid-key",
	})
	require.NoError(t, err)
}

func TestValidateCredentials_AuthError(t *testing.T) {
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, cjErr(1600100, "invalid api key"))
	})

	err := p.ValidateCredentials(context.Background(), contracts.ProviderCredentials{
		APIKey: "bad-key",
	})
	require.Error(t, err)
}

// TestDoWithRetry_RefreshOn401 verifies the client transparently refreshes its
// access token when the upstream returns 401 and retries the original request
// once. This is the long-running-node safeguard for CJ token expiry.
func TestDoWithRetry_RefreshOn401(t *testing.T) {
	var (
		businessCalls int32
		authCalls     int32
	)
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/authentication/getAccessToken":
			atomic.AddInt32(&authCalls, 1)
			fmt.Fprint(w, cjOK(map[string]string{
				"accessToken":           "fresh-token",
				"accessTokenExpiryDate": "2099-01-01 00:00:00",
			}))
		default:
			n := atomic.AddInt32(&businessCalls, 1)
			if n == 1 {
				w.WriteHeader(http.StatusUnauthorized)
				fmt.Fprint(w, `{"result":false,"code":1600101,"message":"token expired"}`)
				return
			}
			fmt.Fprint(w, cjOK([]cjCategory{{CategoryID: "1", CategoryName: "Test"}}))
		}
	})

	var categories []cjCategory
	err := p.client.Get(context.Background(), "/product/getCategory", &categories)
	require.NoError(t, err)
	require.Len(t, categories, 1)
	assert.Equal(t, int32(2), atomic.LoadInt32(&businessCalls), "should retry once after refresh")
	assert.Equal(t, int32(1), atomic.LoadInt32(&authCalls), "should call refresh exactly once")
}

// TestDoWithRetry_NoInfiniteLoopOnPersistent401 verifies that if the upstream
// keeps returning 401 even after a token refresh, the client surfaces AuthError
// without looping forever.
func TestDoWithRetry_NoInfiniteLoopOnPersistent401(t *testing.T) {
	var businessCalls int32
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/authentication/getAccessToken" {
			fmt.Fprint(w, cjOK(map[string]string{
				"accessToken":           "fresh-token",
				"accessTokenExpiryDate": "2099-01-01 00:00:00",
			}))
			return
		}
		atomic.AddInt32(&businessCalls, 1)
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"result":false,"code":1600100,"message":"invalid token"}`)
	})

	err := p.client.Get(context.Background(), "/product/getCategory", nil)
	require.Error(t, err)
	_, ok := err.(*AuthError)
	assert.True(t, ok, "expected AuthError, got %T", err)
	assert.Equal(t, int32(2), atomic.LoadInt32(&businessCalls), "should attempt at most twice (initial + 1 retry)")
}

// ---------------------------------------------------------------------------
// CreateFulfillmentOrder
// ---------------------------------------------------------------------------

func TestCreateFulfillmentOrder(t *testing.T) {
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/shopping/order/createOrderV2")
		assert.Equal(t, http.MethodPost, r.Method)
		fmt.Fprint(w, cjOK(cjOrder{
			OrderID:     "CJ-12345",
			OrderNum:    "TEST-001",
			OrderStatus: "CREATED",
		}))
	})

	order, err := p.CreateFulfillmentOrder(context.Background(), contracts.CreateFulfillmentParams{
		ExternalOrderID: "TEST-001",
		Recipient: contracts.FulfillmentRecipient{
			Name:        "Test Buyer",
			Address1:    "123 Main St",
			City:        "New York",
			StateCode:   "NY",
			CountryCode: "US",
			ZIP:         "10001",
		},
		Items: []contracts.FulfillmentItem{
			{SyncVariantID: "variant-1", Quantity: 2},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "CJ-12345", order.ID)
	assert.Equal(t, contracts.FulfillmentStatusPending, order.Status)
}

func TestCreateFulfillmentOrder_MissingVariant(t *testing.T) {
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach server")
	})

	_, err := p.CreateFulfillmentOrder(context.Background(), contracts.CreateFulfillmentParams{
		Items: []contracts.FulfillmentItem{{Quantity: 1}},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing variant ID")
}

// ---------------------------------------------------------------------------
// GetFulfillmentOrder
// ---------------------------------------------------------------------------

func TestGetFulfillmentOrder(t *testing.T) {
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/shopping/order/getOrderDetail")
		fmt.Fprint(w, cjOK(cjOrder{
			OrderID:       "CJ-12345",
			OrderStatus:   "SHIPPED",
			TrackNumber:   "TRK123456",
			LogisticName:  "YunExpress",
			ProductAmount: 15.50,
			ShippingPrice: 3.20,
			OrderAmount:   18.70,
		}))
	})

	order, err := p.GetFulfillmentOrder(context.Background(), "CJ-12345")
	require.NoError(t, err)
	assert.Equal(t, contracts.FulfillmentStatusShipped, order.Status)
	require.Len(t, order.Shipments, 1)
	assert.Equal(t, "TRK123456", order.Shipments[0].TrackingNumber)
	assert.Equal(t, "YunExpress", order.Shipments[0].Carrier)
	assert.Equal(t, "18.70", order.Costs.Total)
}

// ---------------------------------------------------------------------------
// CancelFulfillmentOrder
// ---------------------------------------------------------------------------

func TestCancelFulfillmentOrder_NotSupported(t *testing.T) {
	p := NewProvider("key", "secret")
	err := p.CancelFulfillmentOrder(context.Background(), "any-id")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")
}

// ---------------------------------------------------------------------------
// ParseWebhook
// ---------------------------------------------------------------------------

func TestParseWebhook_OrderShipped(t *testing.T) {
	secret := "mysecret"
	p := NewProvider("key", secret)

	signInput := "CJ-12345" + "SHIPPED" + "1714567890" + secret
	signHash := computeMD5(signInput)

	evt := cjWebhookEvent{
		EventType:   "ORDER_SHIPPED",
		EventID:     "evt-001",
		OrderID:     "CJ-12345",
		OrderNum:    "TEST-001",
		OrderStatus: "SHIPPED",
		TrackNumber: "TRK-789",
		Carrier:     "YunExpress",
		Timestamp:   "1714567890",
		Sign:        signHash,
	}

	payload, _ := json.Marshal(evt)
	result, err := p.ParseWebhook(context.Background(), payload, nil)
	require.NoError(t, err)
	assert.Equal(t, contracts.FulfillmentWebhookShipped, result.Type)
	assert.Equal(t, "evt-001", result.EventID)
	assert.Equal(t, "CJ-12345", result.OrderID)
}

func TestParseWebhook_InvalidSignature(t *testing.T) {
	p := NewProvider("key", "real-secret")

	evt := cjWebhookEvent{
		EventType:   "ORDER_SHIPPED",
		EventID:     "evt-002",
		OrderID:     "CJ-999",
		OrderStatus: "SHIPPED",
		Timestamp:   "1714567890",
		Sign:        "invalid-signature",
	}
	payload, _ := json.Marshal(evt)

	_, err := p.ParseWebhook(context.Background(), payload, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid webhook signature")
}

func TestParseWebhook_NoSecret(t *testing.T) {
	p := NewProvider("key", "")

	evt := cjWebhookEvent{
		EventType:   "ORDER_CANCELLED",
		EventID:     "evt-003",
		OrderID:     "CJ-111",
		OrderStatus: "CANCELLED",
	}
	payload, _ := json.Marshal(evt)

	result, err := p.ParseWebhook(context.Background(), payload, nil)
	require.NoError(t, err)
	assert.Equal(t, contracts.FulfillmentWebhookOrderCanceled, result.Type)
}

// ---------------------------------------------------------------------------
// EstimateShipping
// ---------------------------------------------------------------------------

func TestEstimateShipping(t *testing.T) {
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/logistic/freightCalculate")
		fmt.Fprint(w, cjOK([]cjFreightResponse{
			{LogisticName: "YunExpress", LogisticPrice: 4.50, LogisticAging: "7-15 business days"},
			{LogisticName: "ePacket", LogisticPrice: 2.80, LogisticAging: "15-30 business days"},
		}))
	})

	estimates, err := p.EstimateShipping(context.Background(), contracts.ShippingEstimateParams{
		Recipient: contracts.FulfillmentRecipient{CountryCode: "US"},
		Items:     []contracts.FulfillmentItem{{SyncVariantID: "v1", Quantity: 1}},
	})
	require.NoError(t, err)
	require.Len(t, estimates, 2)
	assert.Equal(t, "YunExpress", estimates[0].Name)
	assert.Equal(t, "4.50", estimates[0].Rate)
	assert.Equal(t, 7, estimates[0].MinDelivery)
	assert.Equal(t, 15, estimates[0].MaxDelivery)
}

// ---------------------------------------------------------------------------
// ListCategories
// ---------------------------------------------------------------------------

func TestListCategories(t *testing.T) {
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, cjOK([]cjCategory{
			{CategoryID: "1", CategoryNameEN: "Clothing", CategoryImage: "https://img.cj/1.jpg"},
			{CategoryID: "2", CategoryNameEN: "Electronics"},
		}))
	})

	cats, err := p.ListCategories(context.Background())
	require.NoError(t, err)
	require.Len(t, cats, 2)
	assert.Equal(t, "Clothing", cats[0].Name)
}

// ---------------------------------------------------------------------------
// ListProducts
// ---------------------------------------------------------------------------

func TestListProducts(t *testing.T) {
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/product/list")
		fmt.Fprint(w, cjOK(cjProductListResponse{
			Total:    100,
			PageNum:  1,
			PageSize: 20,
			List: []cjProduct{
				{PID: "p1", ProductName: "T-Shirt", SellPrice: 9.99,
					Variants: []cjVariant{
						{VID: "v1", VariantName: "S / Black", VariantSellPrice: 9.99},
						{VID: "v2", VariantName: "M / Black", VariantSellPrice: 10.99},
					}},
			},
		}))
	})

	page, err := p.ListProducts(context.Background(), contracts.CatalogQuery{Limit: 20})
	require.NoError(t, err)
	assert.Equal(t, 100, page.Total)
	require.Len(t, page.Products, 1)
	assert.Equal(t, "T-Shirt", page.Products[0].Title)
	require.Len(t, page.Products[0].Variants, 2)
	assert.Equal(t, "9.99", page.Products[0].MinPrice)
	assert.Equal(t, "10.99", page.Products[0].MaxPrice)
}

// ---------------------------------------------------------------------------
// Status Mapping
// ---------------------------------------------------------------------------

func TestMapCJOrderStatus(t *testing.T) {
	cases := []struct {
		input  string
		expect contracts.FulfillmentStatus
	}{
		{"CREATED", contracts.FulfillmentStatusPending},
		{"WAIT_CONFIRM", contracts.FulfillmentStatusPending},
		{"IN_CART", contracts.FulfillmentStatusInProcess},
		{"ORDERED", contracts.FulfillmentStatusInProcess},
		{"SHIPPED", contracts.FulfillmentStatusShipped},
		{"DELIVERING", contracts.FulfillmentStatusShipped},
		{"DELIVERED", contracts.FulfillmentStatusDelivered},
		{"CANCELLED", contracts.FulfillmentStatusCanceled},
		{"FAILED", contracts.FulfillmentStatusFailed},
		{"OUT_OF_STOCK", contracts.FulfillmentStatusFailed},
		{"UNKNOWN", contracts.FulfillmentStatusPending},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			assert.Equal(t, tc.expect, mapCJOrderStatus(tc.input))
		})
	}
}

// ---------------------------------------------------------------------------
// Rate Limiting
// ---------------------------------------------------------------------------

func TestRateLimiting(t *testing.T) {
	var count int32
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&count, 1)
		fmt.Fprint(w, cjOK([]cjCategory{}))
	})
	// Override to strict 2 QPS for test speed
	p.client.qps = 2

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	for i := 0; i < 3; i++ {
		err := p.client.Get(ctx, "/product/getCategory", nil)
		require.NoError(t, err)
	}
	assert.Equal(t, int32(3), atomic.LoadInt32(&count))
}

// ---------------------------------------------------------------------------
// ClassifyError
// ---------------------------------------------------------------------------

func TestClassifyError(t *testing.T) {
	assert.Nil(t, ClassifyError(nil))

	re := ClassifyError(&APIError{StatusCode: 500, Message: "internal"})
	require.NotNil(t, re)
	assert.True(t, re.Retryable)

	re = ClassifyError(&APIError{StatusCode: 400, Message: "bad request"})
	require.NotNil(t, re)
	assert.False(t, re.Retryable)

	re = ClassifyError(&AuthError{Message: "expired"})
	require.NotNil(t, re)
	assert.False(t, re.Retryable)

	re = ClassifyError(&RateLimitError{RetryAfter: time.Second})
	require.NotNil(t, re)
	assert.True(t, re.Retryable)
}

// ---------------------------------------------------------------------------
// parseLogisticAging
// ---------------------------------------------------------------------------

func TestParseLogisticAging(t *testing.T) {
	cases := []struct {
		input      string
		expectMin  int
		expectMax  int
	}{
		{"7-15 business days", 7, 15},
		{"15-30 working days", 15, 30},
		{"10-20 days", 10, 20},
		{"5-10", 5, 10},
		{"7", 7, 7},
		{"unknown", 7, 21},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			min, max := parseLogisticAging(tc.input)
			assert.Equal(t, tc.expectMin, min)
			assert.Equal(t, tc.expectMax, max)
		})
	}
}
