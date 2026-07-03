package printful

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mobazha/mobazha/pkg/contracts"
)

// newTestServer returns an httptest.Server with a handler that dispatches on
// method+path. Callers provide a map of "METHOD /path" → handler func.
func newTestServer(routes map[string]http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Method + " " + r.URL.Path
		if h, ok := routes[key]; ok {
			h(w, r)
			return
		}
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(apiResponse{Code: 404})
	}))
}

func writeJSON(w http.ResponseWriter, code int, result interface{}) {
	b, _ := json.Marshal(result)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(apiResponse{Code: code, Result: json.RawMessage(b)})
}

// ---------------------------------------------------------------------------
// ValidateCredentials
// ---------------------------------------------------------------------------

func TestValidateCredentials_Success(t *testing.T) {
	ts := newTestServer(map[string]http.HandlerFunc{
		"GET /stores": func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer test-key" {
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(apiResponse{Code: 401})
				return
			}
			writeJSON(w, 200, []pfStore{{ID: 1, Name: "Test Store"}})
		},
	})
	defer ts.Close()

	p := NewProvider("dummy", "", WithBaseURL(ts.URL))
	err := p.ValidateCredentials(context.Background(), contracts.ProviderCredentials{
		APIKey: "test-key",
	})
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestValidateCredentials_Unauthorized(t *testing.T) {
	ts := newTestServer(map[string]http.HandlerFunc{
		"GET /stores": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"code":401,"error":{"message":"Invalid token"}}`))
		},
	})
	defer ts.Close()

	p := NewProvider("dummy", "", WithBaseURL(ts.URL))
	err := p.ValidateCredentials(context.Background(), contracts.ProviderCredentials{
		APIKey: "bad-key",
	})
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
	var authErr *AuthError
	if !errors.As(err, &authErr) {
		t.Fatalf("expected *AuthError, got %T: %v", err, err)
	}
}

// ---------------------------------------------------------------------------
// ListCategories
// ---------------------------------------------------------------------------

func TestListCategories(t *testing.T) {
	ts := newTestServer(map[string]http.HandlerFunc{
		"GET /categories": func(w http.ResponseWriter, _ *http.Request) {
			cats := []pfCategory{
				{ID: 1, Title: "Men's T-Shirts", ImageURL: "https://img/tshirt.jpg"},
				{ID: 2, Title: "Hoodies", ParentID: 1},
			}
			writeJSON(w, 200, cats)
		},
	})
	defer ts.Close()

	p := NewProvider("token", "", WithBaseURL(ts.URL))
	cats, err := p.ListCategories(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cats) != 2 {
		t.Fatalf("expected 2 categories, got %d", len(cats))
	}
	if cats[0].Name != "Men's T-Shirts" {
		t.Errorf("expected 'Men's T-Shirts', got %q", cats[0].Name)
	}
}

// ---------------------------------------------------------------------------
// GetProduct
// ---------------------------------------------------------------------------

func TestGetProduct(t *testing.T) {
	ts := newTestServer(map[string]http.HandlerFunc{
		"GET /products/71": func(w http.ResponseWriter, _ *http.Request) {
			resp := struct {
				Product  pfProduct   `json:"product"`
				Variants []pfVariant `json:"variants"`
			}{
				Product: pfProduct{
					ID:    71,
					Title: "Unisex Softstyle T-Shirt",
					Image: "https://img/tshirt.jpg",
				},
				Variants: []pfVariant{
					{ID: 4011, Name: "S / Black", Size: "S", Color: "Black", Price: "8.25", InStock: true},
					{ID: 4012, Name: "M / Black", Size: "M", Color: "Black", Price: "8.25", InStock: true},
				},
			}
			writeJSON(w, 200, resp)
		},
	})
	defer ts.Close()

	p := NewProvider("token", "", WithBaseURL(ts.URL))
	product, err := p.GetProduct(context.Background(), "71")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if product.ID != "71" {
		t.Errorf("expected ID '71', got %q", product.ID)
	}
	if len(product.Variants) != 2 {
		t.Fatalf("expected 2 variants, got %d", len(product.Variants))
	}
	if product.Variants[0].Attributes["size"] != "S" {
		t.Errorf("expected size 'S', got %q", product.Variants[0].Attributes["size"])
	}
}

// ---------------------------------------------------------------------------
// EstimateShipping
// ---------------------------------------------------------------------------

func TestEstimateShipping(t *testing.T) {
	ts := newTestServer(map[string]http.HandlerFunc{
		"POST /shipping/rates": func(w http.ResponseWriter, _ *http.Request) {
			rates := []pfShippingRate{
				{ID: "STANDARD", Name: "Flat Rate", Rate: "3.99", Currency: "USD", MinDeliveryDays: 5, MaxDeliveryDays: 8},
				{ID: "EXPRESS", Name: "Express", Rate: "9.99", Currency: "USD", MinDeliveryDays: 2, MaxDeliveryDays: 3},
			}
			writeJSON(w, 200, rates)
		},
	})
	defer ts.Close()

	p := NewProvider("token", "", WithBaseURL(ts.URL))
	rates, err := p.EstimateShipping(context.Background(), contracts.ShippingEstimateParams{
		Recipient: contracts.FulfillmentRecipient{
			CountryCode: "US",
			StateCode:   "CA",
			City:        "LA",
			ZIP:         "90001",
		},
		Items: []contracts.FulfillmentItem{
			{CatalogVariantID: "4011", Quantity: 1},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rates) != 2 {
		t.Fatalf("expected 2 rates, got %d", len(rates))
	}
	if rates[0].Rate != "3.99" {
		t.Errorf("expected rate '3.99', got %q", rates[0].Rate)
	}
}

// ---------------------------------------------------------------------------
// CreateFulfillmentOrder
// ---------------------------------------------------------------------------

func TestCreateFulfillmentOrder(t *testing.T) {
	var gotConfirm string
	ts := newTestServer(map[string]http.HandlerFunc{
		"POST /orders": func(w http.ResponseWriter, r *http.Request) {
			gotConfirm = r.URL.Query().Get("confirm")
			order := pfOrder{
				ID:         12345,
				ExternalID: "mbz-order-001",
				Status:     "pending",
				Created:    1714000000,
				Updated:    1714000000,
			}
			writeJSON(w, 200, order)
		},
	})
	defer ts.Close()

	p := NewProvider("token", "", WithBaseURL(ts.URL))
	fo, err := p.CreateFulfillmentOrder(context.Background(), contracts.CreateFulfillmentParams{
		ExternalOrderID: "mbz-order-001",
		Recipient: contracts.FulfillmentRecipient{
			Name:        "Alice",
			Address1:    "123 Main St",
			City:        "LA",
			StateCode:   "CA",
			CountryCode: "US",
			ZIP:         "90001",
		},
		Items: []contracts.FulfillmentItem{
			{CatalogVariantID: "4011", Quantity: 1},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fo.ID != "12345" {
		t.Errorf("expected ID '12345', got %q", fo.ID)
	}
	if fo.ExternalID != "mbz-order-001" {
		t.Errorf("expected ExternalID 'mbz-order-001', got %q", fo.ExternalID)
	}
	if gotConfirm != "1" {
		t.Errorf("expected ?confirm=1 query param, got %q", gotConfirm)
	}
}

// ---------------------------------------------------------------------------
// ParseWebhook
// ---------------------------------------------------------------------------

func TestParseWebhook_ValidSignature(t *testing.T) {
	secret := "webhook-secret-123"
	payload := []byte(`{"type":"package_shipped","created":1714000000,"store":1,"data":{"order":{"id":12345,"external_id":"mbz-001","status":"shipped"}}}`)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	sig := hex.EncodeToString(mac.Sum(nil))

	p := NewProvider("token", secret)
	event, err := p.ParseWebhook(context.Background(), payload, map[string]string{
		"X-Printful-Signature": sig,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event.Type != contracts.FulfillmentWebhookShipped {
		t.Errorf("expected type 'package_shipped', got %q", event.Type)
	}
	if event.OrderID != "mbz-001" {
		t.Errorf("expected orderID 'mbz-001', got %q", event.OrderID)
	}
}

func TestParseWebhook_InvalidSignature(t *testing.T) {
	p := NewProvider("token", "correct-secret")
	payload := []byte(`{"type":"order_updated","created":1714000000}`)
	_, err := p.ParseWebhook(context.Background(), payload, map[string]string{
		"X-Printful-Signature": "bad-signature",
	})
	if err == nil {
		t.Fatal("expected signature mismatch error")
	}
}

func TestParseWebhook_MissingSignature(t *testing.T) {
	p := NewProvider("token", "my-secret")
	payload := []byte(`{"type":"order_updated","created":1714000000}`)
	_, err := p.ParseWebhook(context.Background(), payload, map[string]string{})
	if err == nil {
		t.Fatal("expected missing signature error when secret is configured")
	}
}

func TestParseWebhook_PartialShipmentDowngrade(t *testing.T) {
	// When order status is "partial" (not fully fulfilled), package_shipped
	// should be downgraded to OrderUpdated to avoid premature auto-confirm.
	payload := []byte(`{
		"type": "package_shipped",
		"created": 1714000000,
		"store": 1,
		"data": {
			"order": {"id": 12345, "external_id": "mbz-001", "status": "partial"},
			"shipment": {"id": 9001, "carrier": "USPS", "tracking_number": "TRACK123", "tracking_url": "https://usps.com/TRACK123"}
		}
	}`)

	p := NewProvider("token", "")
	event, err := p.ParseWebhook(context.Background(), payload, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event.Type != contracts.FulfillmentWebhookOrderUpdated {
		t.Errorf("partial shipment should be OrderUpdated, got %q", event.Type)
	}
	if !strings.Contains(event.EventID, "9001") {
		t.Errorf("EventID should include shipment ID 9001, got %q", event.EventID)
	}
	fo, ok := event.Data.(*contracts.FulfillmentOrder)
	if !ok || fo == nil {
		t.Fatal("event.Data should be *FulfillmentOrder")
	}
	if len(fo.Shipments) == 0 {
		t.Fatal("expected at least one shipment merged from data.shipment")
	}
	found := false
	for _, s := range fo.Shipments {
		if s.TrackingNumber == "TRACK123" {
			found = true
			break
		}
	}
	if !found {
		t.Error("data.shipment tracking not merged into FulfillmentOrder.Shipments")
	}
}

func TestParseWebhook_FullyFulfilledShipped(t *testing.T) {
	// When order status is "fulfilled", package_shipped should remain as Shipped.
	payload := []byte(`{
		"type": "package_shipped",
		"created": 1714000001,
		"store": 1,
		"data": {
			"order": {"id": 12345, "external_id": "mbz-002", "status": "fulfilled",
				"shipments": [{"id": 9001, "carrier": "FedEx", "tracking_number": "FDX1"}]},
			"shipment": {"id": 9001, "carrier": "FedEx", "tracking_number": "FDX1"}
		}
	}`)

	p := NewProvider("token", "")
	event, err := p.ParseWebhook(context.Background(), payload, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event.Type != contracts.FulfillmentWebhookShipped {
		t.Errorf("fulfilled order should map to Shipped, got %q", event.Type)
	}
	fo, ok := event.Data.(*contracts.FulfillmentOrder)
	if !ok || fo == nil {
		t.Fatal("event.Data should be *FulfillmentOrder")
	}
	// data.shipment is same as order.shipments[0], should not duplicate
	if len(fo.Shipments) != 1 {
		t.Errorf("expected 1 shipment (no duplicate), got %d", len(fo.Shipments))
	}
}

func TestParseWebhook_ProductSynced(t *testing.T) {
	payload := []byte(`{
		"type": "product_synced",
		"created": 1714000002,
		"store": 1,
		"data": {
			"sync_product": {
				"id": 42,
				"external_id": "ext-42",
				"name": "My Custom T-Shirt",
				"variants": 3,
				"synced": 3,
				"thumbnail_url": "https://example.com/thumb.jpg",
				"is_ignored": false
			}
		}
	}`)

	p := NewProvider("token", "")
	event, err := p.ParseWebhook(context.Background(), payload, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event.Type != contracts.FulfillmentWebhookProductSynced {
		t.Errorf("expected product_synced, got %q", event.Type)
	}
	if event.SyncProductID != "42" {
		t.Errorf("expected SyncProductID '42', got %q", event.SyncProductID)
	}
	if event.SyncProductName != "My Custom T-Shirt" {
		t.Errorf("expected SyncProductName 'My Custom T-Shirt', got %q", event.SyncProductName)
	}
	if event.OrderID != "" {
		t.Errorf("product_synced should have no OrderID, got %q", event.OrderID)
	}
	if event.ExternalID != "" {
		t.Errorf("product_synced should have no ExternalID, got %q", event.ExternalID)
	}
	wantEventIDPrefix := "product_synced_sp42_"
	if !strings.HasPrefix(event.EventID, wantEventIDPrefix) {
		t.Errorf("EventID should start with %q, got %q", wantEventIDPrefix, event.EventID)
	}
}

// ---------------------------------------------------------------------------
// Rate Limiting (429)
// ---------------------------------------------------------------------------

func TestRateLimitError(t *testing.T) {
	ts := newTestServer(map[string]http.HandlerFunc{
		"GET /stores": func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Retry-After", "5")
			w.WriteHeader(http.StatusTooManyRequests)
		},
	})
	defer ts.Close()

	p := NewProvider("token", "", WithBaseURL(ts.URL))
	err := p.ValidateCredentials(context.Background(), contracts.ProviderCredentials{APIKey: "key"})
	if err == nil {
		t.Fatal("expected rate limit error")
	}
	var rlErr *RateLimitError
	if !errors.As(err, &rlErr) {
		t.Fatalf("expected *RateLimitError, got %T: %v", err, err)
	}
	if rlErr.RetryAfter != "5" {
		t.Errorf("expected RetryAfter '5', got %q", rlErr.RetryAfter)
	}
}

// ---------------------------------------------------------------------------
// Server Error (5xx)
// ---------------------------------------------------------------------------

func TestServerError(t *testing.T) {
	ts := newTestServer(map[string]http.HandlerFunc{
		"GET /categories": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"code":500,"error":{"message":"Internal server error"}}`))
		},
	})
	defer ts.Close()

	p := NewProvider("token", "", WithBaseURL(ts.URL))
	_, err := p.ListCategories(context.Background())
	if err == nil {
		t.Fatal("expected server error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != 500 {
		t.Errorf("expected status 500, got %d", apiErr.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// CancelFulfillmentOrder
// ---------------------------------------------------------------------------

func TestCancelFulfillmentOrder(t *testing.T) {
	ts := newTestServer(map[string]http.HandlerFunc{
		"DELETE /orders/12345": func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, 200, nil)
		},
	})
	defer ts.Close()

	p := NewProvider("token", "", WithBaseURL(ts.URL))
	err := p.CancelFulfillmentOrder(context.Background(), "12345")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Interface compliance
// ---------------------------------------------------------------------------

func TestConvertCatalogProduct_PriceRange(t *testing.T) {
	p := &pfProduct{
		ID:    100,
		Title: "Test Product",
		Variants: []pfVariant{
			{ID: 1, Name: "S", Price: "9.99", InStock: true},
			{ID: 2, Name: "M", Price: "10.00", InStock: true},
			{ID: 3, Name: "L", Price: "25.50", InStock: true},
		},
	}
	cp := convertCatalogProduct(p)
	if cp.MinPrice != "9.99" {
		t.Errorf("expected MinPrice '9.99', got %q", cp.MinPrice)
	}
	if cp.MaxPrice != "25.50" {
		t.Errorf("expected MaxPrice '25.50', got %q", cp.MaxPrice)
	}
}

func TestInterfaceCompliance(t *testing.T) {
	var _ contracts.FulfillmentProvider = (*Provider)(nil)
	var _ contracts.FulfillmentCatalogProvider = (*Provider)(nil)
	var _ contracts.FulfillmentStoreSyncProvider = (*Provider)(nil)
}

// ---------------------------------------------------------------------------
// ListStoreSyncProducts
// ---------------------------------------------------------------------------

func TestListStoreSyncProducts_Success(t *testing.T) {
	ts := newTestServer(map[string]http.HandlerFunc{
		"GET /store/products": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, 200, []pfSyncProductSummary{
				{ID: 100, Name: "Custom T-shirt", ThumbnailURL: "https://img.example.com/tshirt.png", Variants: 5, Synced: 5},
				{ID: 101, Name: "Ignored Product", IsIgnored: true, Variants: 2, Synced: 0},
				{ID: 102, Name: "Custom Hoodie", ThumbnailURL: "https://img.example.com/hoodie.png", Variants: 3, Synced: 3},
			})
		},
	})
	defer ts.Close()

	p := NewProvider("token", "", WithBaseURL(ts.URL))
	page, err := p.ListStoreSyncProducts(context.Background(), 0, 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(page.Products) != 2 {
		t.Fatalf("expected 2 products (ignored filtered), got %d", len(page.Products))
	}
	if page.Products[0].ID != "100" || page.Products[0].Name != "Custom T-shirt" {
		t.Errorf("unexpected first product: %+v", page.Products[0])
	}
	if page.Products[1].ID != "102" || page.Products[1].Name != "Custom Hoodie" {
		t.Errorf("unexpected second product: %+v", page.Products[1])
	}
}

func TestListStoreSyncProducts_Empty(t *testing.T) {
	ts := newTestServer(map[string]http.HandlerFunc{
		"GET /store/products": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, 200, []pfSyncProductSummary{})
		},
	})
	defer ts.Close()

	p := NewProvider("token", "", WithBaseURL(ts.URL))
	page, err := p.ListStoreSyncProducts(context.Background(), 0, 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(page.Products) != 0 {
		t.Errorf("expected 0 products, got %d", len(page.Products))
	}
}

// ---------------------------------------------------------------------------
// GetStoreSyncProduct
// ---------------------------------------------------------------------------

func TestGetStoreSyncProduct_Success(t *testing.T) {
	ts := newTestServer(map[string]http.HandlerFunc{
		"GET /store/products/100": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, 200, pfSyncProductInfo{
				SyncProduct: pfSyncProductSummary{
					ID:           100,
					Name:         "Custom T-shirt",
					ThumbnailURL: "https://img.example.com/tshirt.png",
					Variants:     2,
					Synced:       2,
				},
				SyncVariants: []pfSyncVariant{
					{
						ID:            201,
						SyncProductID: 100,
						Name:          "Custom T-shirt / S / White",
						VariantID:     3001,
						RetailPrice:   "29.99",
						Currency:      "USD",
						Size:          "S",
						Color:         "White",
						AvailabilityStatus: "active",
						Product: &pfItemProduct{
							VariantID: 3001,
							ProductID: 301,
							Image:     "https://img.example.com/variant-s.png",
							Name:      "Bella + Canvas 3001",
						},
						Files: []pfSyncFile{
							{
								Type:       "default",
								URL:        "https://files.example.com/design.png",
								PreviewURL: "https://files.example.com/preview.png",
								Filename:   "design.png",
							},
						},
					},
					{
						ID:            202,
						SyncProductID: 100,
						Name:          "Custom T-shirt / M / White",
						VariantID:     3002,
						RetailPrice:   "29.99",
						Currency:      "USD",
						Size:          "M",
						Color:         "White",
						AvailabilityStatus: "active",
						IsIgnored:         true,
					},
				},
			})
		},
	})
	defer ts.Close()

	p := NewProvider("token", "", WithBaseURL(ts.URL))
	product, err := p.GetStoreSyncProduct(context.Background(), "100")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if product.ID != "100" || product.Name != "Custom T-shirt" {
		t.Errorf("unexpected product: %+v", product)
	}
	if len(product.Variants) != 1 {
		t.Fatalf("expected 1 variant (ignored filtered), got %d", len(product.Variants))
	}
	v := product.Variants[0]
	if v.ID != "201" || v.CatalogVariantID != "3001" {
		t.Errorf("unexpected variant: %+v", v)
	}
	if v.RetailPrice != "29.99" || v.Currency != "USD" {
		t.Errorf("unexpected pricing: %+v", v)
	}
	if v.ImageURL != "https://img.example.com/variant-s.png" {
		t.Errorf("expected image from product, got %q", v.ImageURL)
	}
	if v.PreviewURL != "https://files.example.com/preview.png" {
		t.Errorf("expected preview from file, got %q", v.PreviewURL)
	}
	if len(v.Files) != 1 || v.Files[0].Type != "default" {
		t.Errorf("unexpected files: %+v", v.Files)
	}
	if !v.InStock {
		t.Error("expected InStock=true for 'active' availability")
	}
}

func TestGetStoreSyncProduct_NotFound(t *testing.T) {
	ts := newTestServer(map[string]http.HandlerFunc{})
	defer ts.Close()

	p := NewProvider("token", "", WithBaseURL(ts.URL))
	_, err := p.GetStoreSyncProduct(context.Background(), "999")
	if err == nil {
		t.Fatal("expected error for not found product")
	}
}
