package printify

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
)

func testServer(handler http.HandlerFunc) (*httptest.Server, *Provider) {
	ts := httptest.NewServer(handler)
	p := NewProvider("test-token", "test-secret",
		WithBaseURL(ts.URL),
		WithHTTPClient(ts.Client()),
	)
	p.client.shopID = "12345"
	return ts, p
}

// ---------------------------------------------------------------------------
// ValidateCredentials
// ---------------------------------------------------------------------------

func TestValidateCredentials_Success(t *testing.T) {
	ts, p := testServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/shops.json" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer good-token" {
			t.Error("expected custom token in Authorization header")
		}
		json.NewEncoder(w).Encode([]pyShop{{ID: 1, Title: "My Shop"}})
	})
	defer ts.Close()

	err := p.ValidateCredentials(context.Background(), contracts.ProviderCredentials{APIKey: "good-token"})
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestValidateCredentials_NoShops(t *testing.T) {
	ts, p := testServer(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode([]pyShop{})
	})
	defer ts.Close()

	err := p.ValidateCredentials(context.Background(), contracts.ProviderCredentials{APIKey: "empty-token"})
	if err == nil {
		t.Fatal("expected error for empty shops")
	}
}

// ---------------------------------------------------------------------------
// CreateFulfillmentOrder
// ---------------------------------------------------------------------------

func TestCreateFulfillmentOrder(t *testing.T) {
	ts, p := testServer(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/shops/12345/orders.json" && r.Method == "POST":
			json.NewEncoder(w).Encode(pyOrder{
				ID:     "ord_abc",
				Status: "pending",
			})
		case r.URL.Path == "/shops/12345/orders/ord_abc/send_to_production.json":
			json.NewEncoder(w).Encode(pyOrder{
				ID:     "ord_abc",
				Status: "sending-to-production",
			})
		default:
			http.NotFound(w, r)
		}
	})
	defer ts.Close()

	fo, err := p.CreateFulfillmentOrder(context.Background(), contracts.CreateFulfillmentParams{
		ExternalOrderID: "mbz-123",
		Recipient: contracts.FulfillmentRecipient{
			Name:        "John Doe",
			Address1:    "123 Main St",
			City:        "LA",
			StateCode:   "CA",
			CountryCode: "US",
			ZIP:         "90001",
		},
		Items: []contracts.FulfillmentItem{
			// Printify orders need both product_id (string) and variant_id (int).
			// SyncProductID = Printify sync product ID; SyncVariantID = numeric variant ID.
			{SyncProductID: "prod1", SyncVariantID: "12345", Quantity: 1},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fo.ID != "ord_abc" {
		t.Errorf("expected ord_abc, got %s", fo.ID)
	}
	if fo.Status != contracts.FulfillmentStatusInProcess {
		t.Errorf("expected in_process, got %s", fo.Status)
	}
}

func TestCreateFulfillmentOrder_RejectsMissingSyncProductID(t *testing.T) {
	ts, p := testServer(func(w http.ResponseWriter, _ *http.Request) {
		// Should never reach the Printify API.
		t.Error("Printify API was called but the request should have been rejected")
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer ts.Close()

	_, err := p.CreateFulfillmentOrder(context.Background(), contracts.CreateFulfillmentParams{
		ExternalOrderID: "mbz-bad",
		Items: []contracts.FulfillmentItem{
			// Only the variant ID, missing the product ID — must fail closed
			// rather than silently send product_id=variant_id which Printify
			// rejects (the previous bug let bogus orders be created).
			{SyncVariantID: "12345", Quantity: 1},
		},
	})
	if err == nil {
		t.Fatal("expected error when SyncProductID is missing, got nil")
	}
}

func TestCreateFulfillmentOrder_RejectsBadVariantID(t *testing.T) {
	ts, p := testServer(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("Printify API was called but the request should have been rejected")
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer ts.Close()

	_, err := p.CreateFulfillmentOrder(context.Background(), contracts.CreateFulfillmentParams{
		ExternalOrderID: "mbz-bad",
		Items: []contracts.FulfillmentItem{
			{SyncProductID: "prod1", SyncVariantID: "not-a-number", Quantity: 1},
		},
	})
	if err == nil {
		t.Fatal("expected error when variant ID is non-numeric, got nil")
	}
}

// ---------------------------------------------------------------------------
// GetFulfillmentOrder
// ---------------------------------------------------------------------------

func TestGetFulfillmentOrder(t *testing.T) {
	ts, p := testServer(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(pyOrder{
			ID:     "ord_xyz",
			Status: "fulfilled",
			Shipments: []pyShipment{
				{Carrier: "USPS", Number: "TRACK123", URL: "https://track.example.com/123"},
			},
		})
	})
	defer ts.Close()

	fo, err := p.GetFulfillmentOrder(context.Background(), "ord_xyz")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fo.Status != contracts.FulfillmentStatusShipped {
		t.Errorf("expected shipped, got %s", fo.Status)
	}
	if len(fo.Shipments) != 1 {
		t.Fatalf("expected 1 shipment, got %d", len(fo.Shipments))
	}
	if fo.Shipments[0].TrackingNumber != "TRACK123" {
		t.Errorf("expected TRACK123, got %s", fo.Shipments[0].TrackingNumber)
	}
}

// ---------------------------------------------------------------------------
// ParseWebhook
// ---------------------------------------------------------------------------

func signPayload(secret string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

func TestParseWebhook_OrderShipment(t *testing.T) {
	ts, p := testServer(nil)
	defer ts.Close()

	wh := pyWebhookEvent{
		ID:        "evt_1",
		Type:      "order:shipment:created",
		CreatedAt: time.Now(),
		Resource: pyWebhookResource{
			ID:   "ord_123",
			Type: "order",
			Data: map[string]interface{}{
				"external_id": "mbz-order-456",
			},
		},
	}
	payload, _ := json.Marshal(wh)
	sig := signPayload("test-secret", payload)

	event, err := p.ParseWebhook(context.Background(), payload, map[string]string{
		"X-Pfy-Signature": sig,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event.Type != contracts.FulfillmentWebhookShipped {
		t.Errorf("expected shipped, got %s", event.Type)
	}
	if event.ExternalID != "ord_123" {
		t.Errorf("expected ord_123, got %s", event.ExternalID)
	}
	if event.OrderID != "mbz-order-456" {
		t.Errorf("expected mbz-order-456, got %s", event.OrderID)
	}
}

func TestParseWebhook_ProductDeleted(t *testing.T) {
	ts, p := testServer(nil)
	defer ts.Close()

	wh := pyWebhookEvent{
		ID:        "evt_2",
		Type:      "product:deleted",
		CreatedAt: time.Now(),
		Resource: pyWebhookResource{
			ID:   "prod_789",
			Type: "product",
			Data: map[string]interface{}{
				"title": "Cool T-Shirt",
			},
		},
	}
	payload, _ := json.Marshal(wh)
	sig := signPayload("test-secret", payload)

	event, err := p.ParseWebhook(context.Background(), payload, map[string]string{
		"X-Pfy-Signature": sig,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event.Type != contracts.FulfillmentWebhookProductSynced {
		t.Errorf("expected product_synced, got %s", event.Type)
	}
	if event.SyncProductID != "prod_789" {
		t.Errorf("expected prod_789, got %s", event.SyncProductID)
	}
	if event.SyncProductName != "Cool T-Shirt" {
		t.Errorf("expected Cool T-Shirt, got %s", event.SyncProductName)
	}
}

func TestParseWebhook_InvalidSignature(t *testing.T) {
	ts, p := testServer(nil)
	defer ts.Close()

	payload := []byte(`{"type":"order:updated"}`)
	_, err := p.ParseWebhook(context.Background(), payload, map[string]string{
		"X-Pfy-Signature": "bad-signature",
	})
	if err == nil {
		t.Fatal("expected error for invalid signature")
	}
}

func TestParseWebhook_MissingSignature(t *testing.T) {
	ts, p := testServer(nil)
	defer ts.Close()

	payload := []byte(`{"type":"order:updated"}`)
	_, err := p.ParseWebhook(context.Background(), payload, map[string]string{})
	if err == nil {
		t.Fatal("expected error for missing signature")
	}
}

// ---------------------------------------------------------------------------
// ListStoreSyncProducts
// ---------------------------------------------------------------------------

func TestListStoreSyncProducts(t *testing.T) {
	ts, p := testServer(func(w http.ResponseWriter, r *http.Request) {
		resp := struct {
			CurrentPage int         `json:"current_page"`
			Data        []pyProduct `json:"data"`
			Total       int         `json:"total"`
		}{
			CurrentPage: 1,
			Total:       2,
			Data: []pyProduct{
				{
					ID:    "prod_1",
					Title: "Product One",
					Variants: []pyProductVariant{
						{ID: 1, IsEnabled: true, Price: 2500},
						{ID: 2, IsEnabled: false, Price: 3000},
					},
					Images: []pyImage{{Src: "https://img.example.com/1.jpg"}},
				},
				{
					ID:    "prod_2",
					Title: "Product Two",
					Variants: []pyProductVariant{
						{ID: 3, IsEnabled: true, Price: 1500},
					},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	})
	defer ts.Close()

	page, err := p.ListStoreSyncProducts(context.Background(), 0, 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(page.Products) != 2 {
		t.Fatalf("expected 2 products, got %d", len(page.Products))
	}
	if page.Products[0].Name != "Product One" {
		t.Errorf("expected Product One, got %s", page.Products[0].Name)
	}
	if page.Products[0].SyncedCount != 1 {
		t.Errorf("expected 1 synced, got %d", page.Products[0].SyncedCount)
	}
	if page.Products[0].ThumbnailURL != "https://img.example.com/1.jpg" {
		t.Errorf("expected thumbnail, got %s", page.Products[0].ThumbnailURL)
	}
}

// ---------------------------------------------------------------------------
// GetStoreSyncProduct
// ---------------------------------------------------------------------------

func TestGetStoreSyncProduct(t *testing.T) {
	ts, p := testServer(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(pyProduct{
			ID:    "prod_detail",
			Title: "Detailed Product",
			Variants: []pyProductVariant{
				{ID: 10, Title: "S / Black", Price: 2000, Cost: 1000, SKU: "SKU-S-BLK", IsEnabled: true, IsAvailable: true},
				{ID: 11, Title: "M / White", Price: 2000, Cost: 1000, SKU: "SKU-M-WHT", IsEnabled: true, IsAvailable: false},
			},
			Images: []pyImage{
				{Src: "https://img.example.com/s-black.jpg", VariantIDs: []int{10}},
				{Src: "https://img.example.com/m-white.jpg", VariantIDs: []int{11}},
			},
		})
	})
	defer ts.Close()

	sp, err := p.GetStoreSyncProduct(context.Background(), "prod_detail")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sp.Name != "Detailed Product" {
		t.Errorf("expected Detailed Product, got %s", sp.Name)
	}
	if len(sp.Variants) != 2 {
		t.Fatalf("expected 2 variants, got %d", len(sp.Variants))
	}
	// StoreSyncVariant.RetailPrice carries the *supplier-side cost* baseline
	// (importer + price-drift logic consume it as cost). Printify's v.Cost
	// is what we get charged, NOT v.Price (which is the Printify shop's
	// retail price). Variant 0 has Cost=1000 cents, so $10.00.
	if sp.Variants[0].RetailPrice != "10.00" {
		t.Errorf("expected supplier cost 10.00, got %s", sp.Variants[0].RetailPrice)
	}
	if sp.Variants[0].CatalogVariantID != "10" {
		t.Errorf("expected CatalogVariantID 10, got %s", sp.Variants[0].CatalogVariantID)
	}
	if sp.Variants[0].InStock != true {
		t.Error("expected variant 0 in stock")
	}
	if sp.Variants[1].InStock != false {
		t.Error("expected variant 1 out of stock")
	}
	if sp.Variants[0].ImageURL != "https://img.example.com/s-black.jpg" {
		t.Errorf("expected variant image, got %s", sp.Variants[0].ImageURL)
	}
}

// ---------------------------------------------------------------------------
// Status Mapping
// ---------------------------------------------------------------------------

func TestMapPyOrderStatus(t *testing.T) {
	tests := []struct {
		input    string
		expected contracts.FulfillmentStatus
	}{
		{"pending", contracts.FulfillmentStatusPending},
		{"on-hold", contracts.FulfillmentStatusPending},
		{"in-production", contracts.FulfillmentStatusInProcess},
		{"sending-to-production", contracts.FulfillmentStatusInProcess},
		{"fulfilled", contracts.FulfillmentStatusShipped},
		{"partially-fulfilled", contracts.FulfillmentStatusShipped},
		{"canceled", contracts.FulfillmentStatusCanceled},
		{"payment-canceled", contracts.FulfillmentStatusCanceled},
		{"payment-not-received", contracts.FulfillmentStatusFailed},
		{"unknown-status", contracts.FulfillmentStatusPending},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := mapPyOrderStatus(tt.input); got != tt.expected {
				t.Errorf("mapPyOrderStatus(%q) = %s, want %s", tt.input, got, tt.expected)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Rate Limiting
// ---------------------------------------------------------------------------

func TestRateLimiting(t *testing.T) {
	var requestCount int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		json.NewEncoder(w).Encode([]pyShop{{ID: 1, Title: "Shop"}})
	}))
	defer ts.Close()

	c := NewClient("tok", WithBaseURL(ts.URL), WithHTTPClient(ts.Client()), WithRateLimit(5))

	ctx := context.Background()
	for i := 0; i < 5; i++ {
		var shops []pyShop
		if err := c.Get(ctx, "/shops.json", &shops); err != nil {
			t.Fatalf("request %d failed: %v", i, err)
		}
	}

	count := atomic.LoadInt32(&requestCount)
	if count != 5 {
		t.Errorf("expected 5 requests, got %d", count)
	}
}

// ---------------------------------------------------------------------------
// Error Types
// ---------------------------------------------------------------------------

func TestClassifyError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		retryable bool
	}{
		{"nil", nil, false},
		{"rate limit", &RateLimitError{RetryAfter: time.Minute}, true},
		{"auth error", &AuthError{Message: "bad token"}, false},
		{"server error", &APIError{StatusCode: 500, Message: "internal"}, true},
		{"client error", &APIError{StatusCode: 400, Message: "bad request"}, false},
		{"unknown", fmt.Errorf("something"), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyError(tt.err)
			if tt.err == nil {
				if result != nil {
					t.Error("expected nil for nil error")
				}
				return
			}
			if result.Retryable != tt.retryable {
				t.Errorf("retryable = %v, want %v", result.Retryable, tt.retryable)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ProviderID / ProviderType
// ---------------------------------------------------------------------------

func TestProviderIdentity(t *testing.T) {
	p := NewProvider("tok", "secret")
	if p.ProviderID() != "printify" {
		t.Errorf("expected printify, got %s", p.ProviderID())
	}
	if p.ProviderType() != "pod" {
		t.Errorf("expected pod, got %s", p.ProviderType())
	}
}

// ---------------------------------------------------------------------------
// CentsToString
// ---------------------------------------------------------------------------

func TestCentsToString(t *testing.T) {
	tests := []struct {
		cents    int
		expected string
	}{
		{0, "0.00"},
		{100, "1.00"},
		{2599, "25.99"},
		{1, "0.01"},
	}
	for _, tt := range tests {
		if got := centsToString(tt.cents); got != tt.expected {
			t.Errorf("centsToString(%d) = %s, want %s", tt.cents, got, tt.expected)
		}
	}
}
