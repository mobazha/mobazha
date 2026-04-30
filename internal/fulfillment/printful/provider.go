package printful

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

const providerID = "printful"

// Provider implements contracts.FulfillmentProvider and
// contracts.FulfillmentCatalogProvider for the Printful API.
type Provider struct {
	client        *Client
	webhookSecret string
}

// NewProvider creates a Printful fulfillment provider.
func NewProvider(token string, webhookSecret string, opts ...ClientOption) *Provider {
	return &Provider{
		client:        NewClient(token, opts...),
		webhookSecret: webhookSecret,
	}
}

// ---------------------------------------------------------------------------
// contracts.FulfillmentProvider
// ---------------------------------------------------------------------------

func (p *Provider) ProviderID() string   { return providerID }
func (p *Provider) ProviderType() string { return "pod" }

func (p *Provider) ValidateCredentials(ctx context.Context, creds contracts.ProviderCredentials) error {
	c := NewClient(creds.APIKey, WithBaseURL(p.client.baseURL), WithHTTPClient(p.client.httpClient))
	var stores []pfStore
	if err := c.Get(ctx, "/stores", &stores); err != nil {
		return fmt.Errorf("validate credentials: %w", err)
	}
	if len(stores) == 0 {
		return fmt.Errorf("validate credentials: no stores found for this token")
	}
	return nil
}

func (p *Provider) CreateFulfillmentOrder(ctx context.Context, params contracts.CreateFulfillmentParams) (*contracts.FulfillmentOrder, error) {
	reqItems := make([]pfOrderItemReq, 0, len(params.Items))
	for _, item := range params.Items {
		ri := pfOrderItemReq{
			Quantity:    item.Quantity,
			RetailPrice: item.RetailPrice,
		}
		if item.SyncVariantID != "" {
			if id, err := strconv.Atoi(item.SyncVariantID); err == nil {
				ri.SyncVariantID = id
			}
		}
		if item.CatalogVariantID != "" {
			if id, err := strconv.Atoi(item.CatalogVariantID); err == nil {
				ri.VariantID = id
			}
		}
		for _, f := range item.Files {
			ri.Files = append(ri.Files, pfOrderFile{
				Type:     f.Type,
				URL:      f.URL,
				Filename: f.Filename,
			})
		}
		reqItems = append(reqItems, ri)
	}

	pfReq := pfCreateOrderRequest{
		ExternalID: params.ExternalOrderID,
		Recipient: pfRecipient{
			Name:        params.Recipient.Name,
			Address1:    params.Recipient.Address1,
			Address2:    params.Recipient.Address2,
			City:        params.Recipient.City,
			StateCode:   params.Recipient.StateCode,
			CountryCode: params.Recipient.CountryCode,
			ZIP:         params.Recipient.ZIP,
			Phone:       params.Recipient.Phone,
			Email:       params.Recipient.Email,
		},
		Items: reqItems,
	}
	if params.RetailCosts != nil {
		pfReq.RetailCosts = &pfRetailCosts{
			Subtotal: params.RetailCosts.Subtotal,
			Shipping: params.RetailCosts.Shipping,
			Total:    params.RetailCosts.Total,
			Currency: params.RetailCosts.Currency,
		}
	}

	var order pfOrder
	// ?confirm=1 skips the "draft" state and immediately submits for
	// fulfillment (production). Without it Printful creates a draft that
	// never ships. See: https://developers.printful.com/docs/#tag/Orders-API/operation/createOrder
	if err := p.client.Post(ctx, "/orders?confirm=1", pfReq, &order); err != nil {
		return nil, fmt.Errorf("create fulfillment order: %w", err)
	}
	return convertOrder(&order), nil
}

func (p *Provider) GetFulfillmentOrder(ctx context.Context, orderID string) (*contracts.FulfillmentOrder, error) {
	var order pfOrder
	if err := p.client.Get(ctx, "/orders/"+orderID, &order); err != nil {
		return nil, fmt.Errorf("get fulfillment order: %w", err)
	}
	return convertOrder(&order), nil
}

func (p *Provider) CancelFulfillmentOrder(ctx context.Context, orderID string) error {
	if err := p.client.Delete(ctx, "/orders/"+orderID, nil); err != nil {
		return fmt.Errorf("cancel fulfillment order: %w", err)
	}
	return nil
}

func (p *Provider) ParseWebhook(_ context.Context, payload []byte, headers map[string]string) (*contracts.FulfillmentWebhookEvent, error) {
	if p.webhookSecret != "" {
		sig := headers["X-Printful-Signature"]
		if sig == "" {
			sig = headers["x-printful-signature"]
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

	var wh pfWebhookPayload
	if err := jsonUnmarshal(payload, &wh); err != nil {
		return nil, fmt.Errorf("parse webhook: %w", err)
	}

	// EventID must be unique across orders AND shipments. Include the
	// Printful order ID and shipment ID (when available) to prevent
	// collisions in the idempotency table.
	eventID := fmt.Sprintf("%s_%d", wh.Type, wh.Created)
	if wh.Data.Order != nil && wh.Data.Order.ID != 0 {
		eventID = fmt.Sprintf("%s_%d_%d", wh.Type, wh.Data.Order.ID, wh.Created)
	}
	if wh.Data.Shipment != nil && wh.Data.Shipment.ID != 0 {
		eventID = fmt.Sprintf("%s_%d", eventID, wh.Data.Shipment.ID)
	}
	event := &contracts.FulfillmentWebhookEvent{
		EventID:   eventID,
		Timestamp: time.Unix(wh.Created, 0),
	}

	switch wh.Type {
	case "package_shipped":
		// Printful fires package_shipped per-package. Only treat it as
		// "shipped" when the order itself is fully fulfilled; otherwise
		// record tracking as an order update (partial shipment).
		if wh.Data.Order != nil && (wh.Data.Order.Status == "fulfilled" || wh.Data.Order.Status == "shipped") {
			event.Type = contracts.FulfillmentWebhookShipped
		} else {
			event.Type = contracts.FulfillmentWebhookOrderUpdated
		}
	case "order_updated":
		event.Type = contracts.FulfillmentWebhookOrderUpdated
	case "order_failed":
		event.Type = contracts.FulfillmentWebhookOrderFailed
	case "order_canceled":
		event.Type = contracts.FulfillmentWebhookOrderCanceled
	case "stock_updated":
		event.Type = contracts.FulfillmentWebhookStockUpdated
	case "product_synced":
		event.Type = contracts.FulfillmentWebhookProductSynced
	default:
		event.Type = contracts.FulfillmentWebhookType(wh.Type)
	}

	if wh.Data.Order != nil {
		event.OrderID = wh.Data.Order.ExternalID
		event.ExternalID = strconv.Itoa(wh.Data.Order.ID)
		fo := convertOrder(wh.Data.Order)
		// Merge data.shipment into the FulfillmentOrder if it's not
		// already present in order.shipments (Printful sends shipment
		// details at the top level for package_shipped events).
		if wh.Data.Shipment != nil && fo != nil {
			whShip := convertShipment(wh.Data.Shipment)
			if !hasShipmentID(fo.Shipments, whShip.ID) {
				fo.Shipments = append(fo.Shipments, whShip)
			}
		}
		event.Data = fo
	}

	return event, nil
}

func (p *Provider) EstimateShipping(ctx context.Context, params contracts.ShippingEstimateParams) ([]contracts.ShippingEstimate, error) {
	items := make([]pfShippingItem, 0, len(params.Items))
	for _, item := range params.Items {
		si := pfShippingItem{Quantity: item.Quantity}
		if item.CatalogVariantID != "" {
			if id, err := strconv.Atoi(item.CatalogVariantID); err == nil {
				si.VariantID = id
			}
		}
		if item.SyncVariantID != "" {
			si.ExternalVariantID = item.SyncVariantID
		}
		items = append(items, si)
	}

	pfReq := pfShippingRateRequest{
		Recipient: pfRecipient{
			Name:        params.Recipient.Name,
			Address1:    params.Recipient.Address1,
			Address2:    params.Recipient.Address2,
			City:        params.Recipient.City,
			StateCode:   params.Recipient.StateCode,
			CountryCode: params.Recipient.CountryCode,
			ZIP:         params.Recipient.ZIP,
		},
		Items: items,
	}

	var rates []pfShippingRate
	if err := p.client.Post(ctx, "/shipping/rates", pfReq, &rates); err != nil {
		return nil, fmt.Errorf("estimate shipping: %w", err)
	}

	result := make([]contracts.ShippingEstimate, 0, len(rates))
	for _, r := range rates {
		result = append(result, contracts.ShippingEstimate{
			ID:          r.ID,
			Name:        r.Name,
			Rate:        r.Rate,
			Currency:    r.Currency,
			MinDelivery: r.MinDeliveryDays,
			MaxDelivery: r.MaxDeliveryDays,
		})
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// contracts.FulfillmentCatalogProvider
// ---------------------------------------------------------------------------

func (p *Provider) ListCategories(ctx context.Context) ([]contracts.CatalogCategory, error) {
	var cats []pfCategory
	if err := p.client.Get(ctx, "/categories", &cats); err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	result := make([]contracts.CatalogCategory, 0, len(cats))
	for _, c := range cats {
		result = append(result, contracts.CatalogCategory{
			ID:       strconv.Itoa(c.ID),
			Name:     c.Title,
			ParentID: strconv.Itoa(c.ParentID),
			ImageURL: c.ImageURL,
		})
	}
	return result, nil
}

func (p *Provider) ListProducts(ctx context.Context, params contracts.CatalogQuery) (*contracts.CatalogPage, error) {
	path := "/products"
	if params.CategoryID != "" {
		path += "?category_id=" + params.CategoryID
	}
	var products []pfProduct
	if err := p.client.Get(ctx, path, &products); err != nil {
		return nil, fmt.Errorf("list products: %w", err)
	}

	offset := params.Offset
	limit := params.Limit
	if limit <= 0 {
		limit = 20
	}
	total := len(products)

	// Client-side pagination (Printful returns all products per category).
	end := offset + limit
	if end > total {
		end = total
	}
	var page []pfProduct
	if offset < total {
		page = products[offset:end]
	}

	catalogProducts := make([]contracts.CatalogProduct, 0, len(page))
	for _, p := range page {
		catalogProducts = append(catalogProducts, convertCatalogProduct(&p))
	}
	return &contracts.CatalogPage{
		Products: catalogProducts,
		Total:    total,
		Offset:   offset,
		Limit:    limit,
	}, nil
}

func (p *Provider) GetProduct(ctx context.Context, productID string) (*contracts.CatalogProduct, error) {
	// Printful returns {product: {}, variants: []}
	var resp struct {
		Product  pfProduct   `json:"product"`
		Variants []pfVariant `json:"variants"`
	}
	if err := p.client.Get(ctx, "/products/"+productID, &resp); err != nil {
		return nil, fmt.Errorf("get product: %w", err)
	}
	resp.Product.Variants = resp.Variants
	cp := convertCatalogProduct(&resp.Product)
	return &cp, nil
}

func (p *Provider) GetVariant(ctx context.Context, variantID string) (*contracts.CatalogVariant, error) {
	var resp struct {
		Variant pfVariant `json:"variant"`
	}
	if err := p.client.Get(ctx, "/products/variant/"+variantID, &resp); err != nil {
		return nil, fmt.Errorf("get variant: %w", err)
	}
	cv := convertVariant(&resp.Variant)
	return &cv, nil
}

// ---------------------------------------------------------------------------
// contracts.FulfillmentStoreSyncProvider
// ---------------------------------------------------------------------------

func (p *Provider) ListStoreSyncProducts(ctx context.Context, offset, limit int) (*contracts.StoreSyncPage, error) {
	if limit <= 0 {
		limit = 20
	}
	path := fmt.Sprintf("/store/products?offset=%d&limit=%d", offset, limit)

	// Printful wraps the list response with paging at the envelope level.
	// The Client.Get strips the envelope and gives us result directly.
	var products []pfSyncProductSummary
	if err := p.client.Get(ctx, path, &products); err != nil {
		return nil, fmt.Errorf("list store sync products: %w", err)
	}

	result := make([]contracts.StoreSyncProduct, 0, len(products))
	for _, sp := range products {
		if sp.IsIgnored {
			continue
		}
		result = append(result, contracts.StoreSyncProduct{
			ID:           strconv.Itoa(sp.ID),
			ExternalID:   sp.ExternalID,
			Name:         sp.Name,
			ThumbnailURL: sp.ThumbnailURL,
			VariantCount: sp.Variants,
			SyncedCount:  sp.Synced,
		})
	}
	return &contracts.StoreSyncPage{
		Products: result,
		Total:    len(result),
		Offset:   offset,
		Limit:    limit,
	}, nil
}

func (p *Provider) GetStoreSyncProduct(ctx context.Context, syncProductID string) (*contracts.StoreSyncProduct, error) {
	var info pfSyncProductInfo
	if err := p.client.Get(ctx, "/store/products/"+syncProductID, &info); err != nil {
		return nil, fmt.Errorf("get store sync product: %w", err)
	}
	return convertStoreSyncProduct(&info), nil
}

// ---------------------------------------------------------------------------
// Converters
// ---------------------------------------------------------------------------

func convertOrder(o *pfOrder) *contracts.FulfillmentOrder {
	fo := &contracts.FulfillmentOrder{
		ID:           strconv.Itoa(o.ID),
		ExternalID:   o.ExternalID,
		Status:       mapOrderStatus(o.Status),
		CreatedAt:    time.Unix(o.Created, 0),
		UpdatedAt:    time.Unix(o.Updated, 0),
		ErrorMessage: o.ErrorMessage,
	}
	if o.Costs != nil {
		fo.Costs = &contracts.FulfillmentCosts{
			Subtotal: o.Costs.Subtotal,
			Shipping: o.Costs.Shipping,
			Tax:      o.Costs.Tax,
			Total:    o.Costs.Total,
			Currency: o.Costs.Currency,
		}
	}
	for _, s := range o.Shipments {
		fo.Shipments = append(fo.Shipments, contracts.FulfillmentShipment{
			ID:             strconv.Itoa(s.ID),
			Carrier:        s.Carrier,
			TrackingNumber: s.TrackingNumber,
			TrackingURL:    s.TrackingURL,
			ShipDate:       s.ShipDate,
			Items:          s.Items,
		})
	}
	return fo
}

func convertShipment(s *pfShipment) contracts.FulfillmentShipment {
	return contracts.FulfillmentShipment{
		ID:             strconv.Itoa(s.ID),
		Carrier:        s.Carrier,
		TrackingNumber: s.TrackingNumber,
		TrackingURL:    s.TrackingURL,
		ShipDate:       s.ShipDate,
		Items:          s.Items,
	}
}

func hasShipmentID(shipments []contracts.FulfillmentShipment, id string) bool {
	for _, s := range shipments {
		if s.ID == id {
			return true
		}
	}
	return false
}

func mapOrderStatus(s string) contracts.FulfillmentStatus {
	switch s {
	case "draft":
		return contracts.FulfillmentStatusDraft
	case "pending", "waiting":
		return contracts.FulfillmentStatusPending
	case "inprocess":
		return contracts.FulfillmentStatusInProcess
	case "fulfilled", "shipped":
		return contracts.FulfillmentStatusShipped
	case "delivered":
		return contracts.FulfillmentStatusDelivered
	case "canceled":
		return contracts.FulfillmentStatusCanceled
	case "failed":
		return contracts.FulfillmentStatusFailed
	default:
		return contracts.FulfillmentStatusPending
	}
}

func convertCatalogProduct(p *pfProduct) contracts.CatalogProduct {
	cp := contracts.CatalogProduct{
		ID:          strconv.Itoa(p.ID),
		Title:       p.Title,
		Description: p.Description,
		ImageURL:    p.Image,
		Currency:    p.Currency,
	}
	var minVal, maxVal float64
	minSet := false
	for _, v := range p.Variants {
		cp.Variants = append(cp.Variants, convertVariant(&v))
		if pv, err := strconv.ParseFloat(v.Price, 64); err == nil {
			if !minSet || pv < minVal {
				minVal = pv
				cp.MinPrice = v.Price
				minSet = true
			}
			if pv > maxVal {
				maxVal = pv
				cp.MaxPrice = v.Price
			}
		}
	}

	for _, f := range p.Files {
		cp.PrintAreas = append(cp.PrintAreas, contracts.PrintArea{
			Type:        f.Type,
			DisplayName: f.Title,
		})
	}
	return cp
}

func convertVariant(v *pfVariant) contracts.CatalogVariant {
	attrs := map[string]string{}
	if v.Size != "" {
		attrs["size"] = v.Size
	}
	if v.Color != "" {
		attrs["color"] = v.Color
	}
	if v.ColorCode != "" {
		attrs["colorCode"] = v.ColorCode
	}
	return contracts.CatalogVariant{
		ID:         strconv.Itoa(v.ID),
		Title:      v.Name,
		Price:      v.Price,
		InStock:    v.InStock,
		Attributes: attrs,
		ImageURL:   v.Image,
	}
}

// ClassifyError wraps a Printful API error into a contracts.RetryableError
// so the supply chain service can determine whether to retry or give up.
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

// jsonUnmarshal wraps encoding/json for consistency.
var jsonUnmarshal = json.Unmarshal

func convertStoreSyncProduct(info *pfSyncProductInfo) *contracts.StoreSyncProduct {
	sp := &contracts.StoreSyncProduct{
		ID:           strconv.Itoa(info.SyncProduct.ID),
		ExternalID:   info.SyncProduct.ExternalID,
		Name:         info.SyncProduct.Name,
		ThumbnailURL: info.SyncProduct.ThumbnailURL,
		VariantCount: info.SyncProduct.Variants,
		SyncedCount:  info.SyncProduct.Synced,
	}

	for _, sv := range info.SyncVariants {
		if sv.IsIgnored {
			continue
		}
		variant := contracts.StoreSyncVariant{
			ID:               strconv.Itoa(sv.ID),
			SyncProductID:    strconv.Itoa(sv.SyncProductID),
			Name:             sv.Name,
			CatalogVariantID: strconv.Itoa(sv.VariantID),
			RetailPrice:      sv.RetailPrice,
			Currency:         sv.Currency,
			SKU:              sv.SKU,
			Size:             sv.Size,
			Color:            sv.Color,
			InStock:          sv.AvailabilityStatus == "active",
		}
		if sv.Product != nil {
			variant.ImageURL = sv.Product.Image
		}
		for _, f := range sv.Files {
			variant.Files = append(variant.Files, contracts.SyncVariantFile{
				Type:         f.Type,
				URL:          f.URL,
				PreviewURL:   f.PreviewURL,
				ThumbnailURL: f.ThumbnailURL,
				Filename:     f.Filename,
			})
			if f.PreviewURL != "" && variant.PreviewURL == "" {
				variant.PreviewURL = f.PreviewURL
			}
		}
		sp.Variants = append(sp.Variants, variant)
	}
	return sp
}

// Compile-time interface checks.
var (
	_ contracts.FulfillmentProvider          = (*Provider)(nil)
	_ contracts.FulfillmentCatalogProvider   = (*Provider)(nil)
	_ contracts.FulfillmentStoreSyncProvider = (*Provider)(nil)
)
