package printify

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
)

const pyProviderID = "printify"

// Provider implements contracts.FulfillmentProvider,
// contracts.FulfillmentCatalogProvider, and
// contracts.FulfillmentStoreSyncProvider for the Printify API.
type Provider struct {
	client        *Client
	webhookSecret string
}

func NewProvider(token string, webhookSecret string, opts ...ClientOption) *Provider {
	return &Provider{
		client:        NewClient(token, opts...),
		webhookSecret: webhookSecret,
	}
}

// Init discovers the first Printify shop for this API token.
func (p *Provider) Init(ctx context.Context) error {
	return p.client.DiscoverShopID(ctx)
}

// ---------------------------------------------------------------------------
// contracts.FulfillmentProvider
// ---------------------------------------------------------------------------

func (p *Provider) ProviderID() string   { return pyProviderID }
func (p *Provider) ProviderType() string { return "pod" }

func (p *Provider) ValidateCredentials(ctx context.Context, creds contracts.ProviderCredentials) error {
	c := NewClient(creds.APIKey, WithBaseURL(p.client.baseURL), WithHTTPClient(p.client.httpClient))
	var shops []pyShop
	if err := c.Get(ctx, "/shops.json", &shops); err != nil {
		return fmt.Errorf("validate credentials: %w", err)
	}
	if len(shops) == 0 {
		return fmt.Errorf("validate credentials: no shops found for this token")
	}
	return nil
}

func (p *Provider) CreateFulfillmentOrder(ctx context.Context, params contracts.CreateFulfillmentParams) (*contracts.FulfillmentOrder, error) {
	lineItems := make([]pyCreateLineItem, 0, len(params.Items))
	for i, item := range params.Items {
		// Printify orders require BOTH product_id (string) and variant_id (int).
		// SyncProductID is mandatory; the numeric variant ID may live in either
		// SyncVariantID (typical, since import stores variant.ID there) or
		// CatalogVariantID (rare passthrough). Reject items that lack either
		// piece — silently sending product_id=variant_id=0 to Printify caused
		// orders to be rejected or fulfilled against the wrong product.
		if item.SyncProductID == "" {
			return nil, fmt.Errorf("printify: item %d missing SyncProductID; re-import the listing to refresh mapping", i)
		}
		variantID := 0
		if item.SyncVariantID != "" {
			if id, err := strconv.Atoi(item.SyncVariantID); err == nil {
				variantID = id
			}
		}
		if variantID == 0 && item.CatalogVariantID != "" {
			if id, err := strconv.Atoi(item.CatalogVariantID); err == nil {
				variantID = id
			}
		}
		if variantID == 0 {
			return nil, fmt.Errorf("printify: item %d has no usable numeric variant ID (sync=%q catalog=%q)",
				i, item.SyncVariantID, item.CatalogVariantID)
		}
		lineItems = append(lineItems, pyCreateLineItem{
			ProductID: item.SyncProductID,
			VariantID: variantID,
			Quantity:  item.Quantity,
		})
	}

	pyReq := pyCreateOrderRequest{
		ExternalID: params.ExternalOrderID,
		Label:      params.ExternalOrderID,
		LineItems:  lineItems,
		ShippingMethod: 1, // standard
		AddressTo: pyAddress{
			FirstName: params.Recipient.Name,
			Address1:  params.Recipient.Address1,
			Address2:  params.Recipient.Address2,
			City:      params.Recipient.City,
			Region:    params.Recipient.StateCode,
			Country:   params.Recipient.CountryCode,
			Zip:       params.Recipient.ZIP,
			Phone:     params.Recipient.Phone,
			Email:     params.Recipient.Email,
		},
	}

	var order pyOrder
	path := p.client.shopPath("/orders.json")
	if err := p.client.Post(ctx, path, pyReq, &order); err != nil {
		return nil, fmt.Errorf("create fulfillment order: %w", err)
	}

	// Confirm the order (send to production)
	var confirmed pyOrder
	confirmPath := p.client.shopPath(fmt.Sprintf("/orders/%s/send_to_production.json", order.ID))
	if err := p.client.Post(ctx, confirmPath, nil, &confirmed); err != nil {
		return convertPyOrder(&order), fmt.Errorf("send to production: %w", err)
	}

	return convertPyOrder(&confirmed), nil
}

func (p *Provider) GetFulfillmentOrder(ctx context.Context, orderID string) (*contracts.FulfillmentOrder, error) {
	var order pyOrder
	path := p.client.shopPath(fmt.Sprintf("/orders/%s.json", orderID))
	if err := p.client.Get(ctx, path, &order); err != nil {
		return nil, fmt.Errorf("get fulfillment order: %w", err)
	}
	return convertPyOrder(&order), nil
}

func (p *Provider) CancelFulfillmentOrder(ctx context.Context, orderID string) error {
	path := p.client.shopPath(fmt.Sprintf("/orders/%s/cancel.json", orderID))
	if err := p.client.Post(ctx, path, nil, nil); err != nil {
		return fmt.Errorf("cancel fulfillment order: %w", err)
	}
	return nil
}

func (p *Provider) ParseWebhook(_ context.Context, payload []byte, headers map[string]string) (*contracts.FulfillmentWebhookEvent, error) {
	if p.webhookSecret != "" {
		sig := headers["X-Pfy-Signature"]
		if sig == "" {
			sig = headers["x-pfy-signature"]
		}
		if sig == "" {
			return nil, fmt.Errorf("webhook signature missing")
		}
		mac := hmac.New(sha256.New, []byte(p.webhookSecret))
		mac.Write(payload)
		expected := hex.EncodeToString(mac.Sum(nil))
		if !hmac.Equal([]byte(sig), []byte(expected)) {
			return nil, fmt.Errorf("webhook signature mismatch")
		}
	}

	var wh pyWebhookEvent
	if err := jsonUnmarshal(payload, &wh); err != nil {
		return nil, fmt.Errorf("parse webhook: %w", err)
	}

	eventID := fmt.Sprintf("%s_%s_%s", wh.Type, wh.Resource.ID, wh.ID)
	event := &contracts.FulfillmentWebhookEvent{
		EventID:   eventID,
		Timestamp: wh.CreatedAt,
	}

	switch wh.Type {
	case "order:created":
		event.Type = contracts.FulfillmentWebhookOrderUpdated
	case "order:updated":
		event.Type = contracts.FulfillmentWebhookOrderUpdated
	case "order:sent-to-production":
		event.Type = contracts.FulfillmentWebhookOrderUpdated
	case "order:shipment:created":
		event.Type = contracts.FulfillmentWebhookShipped
	case "order:shipment:delivered":
		event.Type = contracts.FulfillmentWebhookShipped
	case "product:deleted":
		event.Type = contracts.FulfillmentWebhookProductSynced
	case "product:publish:started":
		event.Type = contracts.FulfillmentWebhookProductSynced
	default:
		event.Type = contracts.FulfillmentWebhookType(wh.Type)
	}

	if wh.Resource.Type == "order" {
		event.ExternalID = wh.Resource.ID
		// Try to extract external_id from resource data
		if extID, ok := wh.Resource.Data["external_id"].(string); ok {
			event.OrderID = extID
		}
	}

	if wh.Resource.Type == "product" {
		event.SyncProductID = wh.Resource.ID
		if title, ok := wh.Resource.Data["title"].(string); ok {
			event.SyncProductName = title
		}
	}

	return event, nil
}

func (p *Provider) EstimateShipping(ctx context.Context, params contracts.ShippingEstimateParams) ([]contracts.ShippingEstimate, error) {
	// Printify V1 doesn't have a direct shipping rate API like Printful.
	// Use the blueprint shipping info approach instead.
	// For now, return an empty set — shipping costs are calculated at order time.
	return nil, nil
}

// ---------------------------------------------------------------------------
// contracts.FulfillmentCatalogProvider
// ---------------------------------------------------------------------------

func (p *Provider) ListCategories(_ context.Context) ([]contracts.CatalogCategory, error) {
	// Printify doesn't have a category API; blueprints are the top-level catalog.
	// Return a single virtual category.
	return []contracts.CatalogCategory{
		{ID: "all", Name: "All Products"},
	}, nil
}

func (p *Provider) ListProducts(ctx context.Context, params contracts.CatalogQuery) (*contracts.CatalogPage, error) {
	path := "/catalog/blueprints.json"

	var blueprints []pyBlueprint
	if err := p.client.Get(ctx, path, &blueprints); err != nil {
		return nil, fmt.Errorf("list catalog products: %w", err)
	}

	offset := params.Offset
	limit := params.Limit
	if limit <= 0 {
		limit = 20
	}
	total := len(blueprints)

	end := offset + limit
	if end > total {
		end = total
	}
	var page []pyBlueprint
	if offset < total {
		page = blueprints[offset:end]
	}

	products := make([]contracts.CatalogProduct, 0, len(page))
	for _, bp := range page {
		products = append(products, contracts.CatalogProduct{
			ID:          strconv.Itoa(bp.ID),
			Title:       bp.Title,
			Description: bp.Description,
		})
	}

	return &contracts.CatalogPage{
		Products: products,
		Total:    total,
		Offset:   offset,
		Limit:    limit,
	}, nil
}

func (p *Provider) GetProduct(ctx context.Context, productID string) (*contracts.CatalogProduct, error) {
	// Fetch blueprint details + print providers + variants
	var bp pyBlueprint
	if err := p.client.Get(ctx, "/catalog/blueprints/"+productID+".json", &bp); err != nil {
		return nil, fmt.Errorf("get blueprint: %w", err)
	}

	var providers []pyPrintProvider
	if err := p.client.Get(ctx, fmt.Sprintf("/catalog/blueprints/%s/print_providers.json", productID), &providers); err != nil {
		return nil, fmt.Errorf("get print providers: %w", err)
	}

	cp := contracts.CatalogProduct{
		ID:          strconv.Itoa(bp.ID),
		Title:       bp.Title,
		Description: bp.Description,
	}

	// Fetch variants from the first print provider
	if len(providers) > 0 {
		providerID := strconv.Itoa(providers[0].ID)
		var variants []pyPrintProviderVariant
		path := fmt.Sprintf("/catalog/blueprints/%s/print_providers/%s/variants.json", productID, providerID)
		if err := p.client.Get(ctx, path, &variants); err == nil {
			for _, v := range variants {
				price := fmt.Sprintf("%.2f", float64(v.Price)/100.0)
				cp.Variants = append(cp.Variants, contracts.CatalogVariant{
					ID:    strconv.Itoa(v.ID),
					Price: price,
				})
			}
		}
		if len(cp.Variants) > 0 {
			cp.MinPrice = cp.Variants[0].Price
			cp.MaxPrice = cp.Variants[len(cp.Variants)-1].Price
		}
	}

	return &cp, nil
}

func (p *Provider) GetVariant(_ context.Context, _ string) (*contracts.CatalogVariant, error) {
	return nil, fmt.Errorf("printify: GetVariant not supported (use blueprint/provider/variant path)")
}

// ---------------------------------------------------------------------------
// contracts.FulfillmentStoreSyncProvider
// ---------------------------------------------------------------------------

func (p *Provider) ListStoreSyncProducts(ctx context.Context, offset, limit int) (*contracts.StoreSyncPage, error) {
	if limit <= 0 {
		limit = 20
	}
	page := (offset / limit) + 1
	path := p.client.shopPath(fmt.Sprintf("/products.json?page=%d&limit=%d", page, limit))

	var resp struct {
		CurrentPage int         `json:"current_page"`
		Data        []pyProduct `json:"data"`
		Total       int         `json:"total"`
	}
	if err := p.client.Get(ctx, path, &resp); err != nil {
		return nil, fmt.Errorf("list store sync products: %w", err)
	}

	result := make([]contracts.StoreSyncProduct, 0, len(resp.Data))
	for _, prod := range resp.Data {
		var thumbnailURL string
		if len(prod.Images) > 0 {
			thumbnailURL = prod.Images[0].Src
		}

		enabledCount := 0
		for _, v := range prod.Variants {
			if v.IsEnabled {
				enabledCount++
			}
		}

		result = append(result, contracts.StoreSyncProduct{
			ID:           prod.ID,
			Name:         prod.Title,
			ThumbnailURL: thumbnailURL,
			VariantCount: len(prod.Variants),
			SyncedCount:  enabledCount,
		})
	}

	return &contracts.StoreSyncPage{
		Products: result,
		Total:    resp.Total,
		Offset:   offset,
		Limit:    limit,
	}, nil
}

func (p *Provider) GetStoreSyncProduct(ctx context.Context, syncProductID string) (*contracts.StoreSyncProduct, error) {
	var prod pyProduct
	path := p.client.shopPath(fmt.Sprintf("/products/%s.json", syncProductID))
	if err := p.client.Get(ctx, path, &prod); err != nil {
		return nil, fmt.Errorf("get store sync product: %w", err)
	}
	return convertStoreSyncProduct(&prod), nil
}

// ---------------------------------------------------------------------------
// Converters
// ---------------------------------------------------------------------------

func convertPyOrder(o *pyOrder) *contracts.FulfillmentOrder {
	fo := &contracts.FulfillmentOrder{
		ID:         o.ID,
		ExternalID: o.ID,
		Status:     mapPyOrderStatus(o.Status),
	}

	if o.CreatedAt != "" {
		if t, err := time.Parse(time.RFC3339, o.CreatedAt); err == nil {
			fo.CreatedAt = t
		}
	}
	if o.UpdatedAt != "" {
		if t, err := time.Parse(time.RFC3339, o.UpdatedAt); err == nil {
			fo.UpdatedAt = t
		}
	}

	if o.Metadata != nil {
		fo.ExternalID = o.Metadata.ShopOrderID
	}

	// Costs
	fo.Costs = &contracts.FulfillmentCosts{
		Subtotal: centsToString(o.TotalPrice),
		Shipping: centsToString(o.TotalShipping),
		Tax:      centsToString(o.TotalTax),
		Total:    centsToString(o.TotalPrice + o.TotalShipping + o.TotalTax),
		Currency: "USD",
	}

	for _, s := range o.Shipments {
		fo.Shipments = append(fo.Shipments, contracts.FulfillmentShipment{
			ID:             s.Number,
			Carrier:        s.Carrier,
			TrackingNumber: s.Number,
			TrackingURL:    s.URL,
		})
	}

	return fo
}

func mapPyOrderStatus(s string) contracts.FulfillmentStatus {
	switch s {
	case "pending", "on-hold":
		return contracts.FulfillmentStatusPending
	case "in-production", "sending-to-production":
		return contracts.FulfillmentStatusInProcess
	case "fulfilled", "partially-fulfilled":
		return contracts.FulfillmentStatusShipped
	case "canceled", "payment-canceled":
		return contracts.FulfillmentStatusCanceled
	case "payment-not-received":
		return contracts.FulfillmentStatusFailed
	default:
		return contracts.FulfillmentStatusPending
	}
}

func convertStoreSyncProduct(prod *pyProduct) *contracts.StoreSyncProduct {
	sp := &contracts.StoreSyncProduct{
		ID:   prod.ID,
		Name: prod.Title,
	}

	if len(prod.Images) > 0 {
		sp.ThumbnailURL = prod.Images[0].Src
	}

	enabledCount := 0
	for _, v := range prod.Variants {
		if !v.IsEnabled {
			continue
		}
		enabledCount++
		// StoreSyncVariant.RetailPrice is consumed by the importer and price
		// drift / margin guard logic as the *supplier-side cost* baseline. For
		// Printify that is the production cost (v.Cost in cents) — v.Price is
		// the retail price the Printify shop owner sets and would inflate the
		// cost basis, breaking margin guard and triggering bogus drift alerts.
		supplierCostCents := v.Cost
		if supplierCostCents == 0 {
			// Some legacy / unconfigured variants only expose Price. Fall back
			// to it so import doesn't fail outright, but log so the seller can
			// fix the Printify product.
			supplierCostCents = v.Price
		}
		variant := contracts.StoreSyncVariant{
			ID:               strconv.Itoa(v.ID),
			SyncProductID:    prod.ID,
			Name:             v.Title,
			CatalogVariantID: strconv.Itoa(v.ID),
			RetailPrice:      centsToString(supplierCostCents),
			Currency:         "USD",
			SKU:              v.SKU,
			InStock:          v.IsAvailable,
		}

		// Find image for this variant
		for _, img := range prod.Images {
			for _, vid := range img.VariantIDs {
				if vid == v.ID {
					variant.ImageURL = img.Src
					variant.PreviewURL = img.Src
					break
				}
			}
			if variant.ImageURL != "" {
				break
			}
		}

		sp.Variants = append(sp.Variants, variant)
	}

	// VariantCount must match len(sp.Variants); SyncedCount is the same here
	// since we only emit enabled variants. Mismatch caused the UI to show
	// "10 variants synced" while only 4 were importable.
	sp.VariantCount = enabledCount
	sp.SyncedCount = enabledCount

	return sp
}

func centsToString(cents int) string {
	return fmt.Sprintf("%.2f", float64(cents)/100.0)
}

// ClassifyError wraps a Printify API error for retry determination.
func ClassifyError(err error) *contracts.FulfillmentRetryableError {
	if err == nil {
		return nil
	}
	var rateLimitErr *RateLimitError
	if errors.As(err, &rateLimitErr) {
		return &contracts.FulfillmentRetryableError{Err: err, Retryable: true}
	}
	var authErr *AuthError
	if errors.As(err, &authErr) {
		return &contracts.FulfillmentRetryableError{Err: err, Retryable: false}
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return &contracts.FulfillmentRetryableError{Err: err, Retryable: apiErr.IsRetryable()}
	}
	return &contracts.FulfillmentRetryableError{Err: err, Retryable: true}
}

var jsonUnmarshal = json.Unmarshal

// Compile-time interface checks.
var (
	_ contracts.FulfillmentProvider          = (*Provider)(nil)
	_ contracts.FulfillmentCatalogProvider   = (*Provider)(nil)
	_ contracts.FulfillmentStoreSyncProvider = (*Provider)(nil)
)
