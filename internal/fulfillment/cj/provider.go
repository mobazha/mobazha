package cj

import (
	"context"
	"crypto/md5"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
)

const cjProviderID = "cj"

// Provider implements contracts.FulfillmentProvider and
// contracts.FulfillmentCatalogProvider for CJ Dropshipping.
type Provider struct {
	client        *Client
	webhookSecret string
}

// NewProvider creates a CJ Dropshipping provider.
// apiKey is the CJ API key used to obtain access tokens.
func NewProvider(apiKey string, webhookSecret string, opts ...ClientOption) *Provider {
	return &Provider{
		client:        NewClient(apiKey, opts...),
		webhookSecret: webhookSecret,
	}
}

// Init obtains an access token using the API key.
func (p *Provider) Init(ctx context.Context) error {
	return p.client.ObtainAccessToken(ctx)
}

// ---------------------------------------------------------------------------
// contracts.FulfillmentProvider
// ---------------------------------------------------------------------------

func (p *Provider) ProviderID() string   { return cjProviderID }
func (p *Provider) ProviderType() string { return "dropshipping" }

func (p *Provider) ValidateCredentials(ctx context.Context, creds contracts.ProviderCredentials) error {
	// CJ requires exchanging the API key for an access token before any
	// business endpoint can be called. We obtain a token here both to validate
	// the key and to pre-warm the client for subsequent calls.
	c := NewClient(creds.APIKey, WithBaseURL(p.client.baseURL), WithHTTPClient(p.client.httpClient))
	if err := c.ObtainAccessToken(ctx); err != nil {
		return fmt.Errorf("validate credentials: %w", err)
	}
	return nil
}

func (p *Provider) CreateFulfillmentOrder(ctx context.Context, params contracts.CreateFulfillmentParams) (*contracts.FulfillmentOrder, error) {
	products := make([]cjOrderProduct, 0, len(params.Items))
	for _, item := range params.Items {
		vid := item.SyncVariantID
		if vid == "" {
			vid = item.CatalogVariantID
		}
		if vid == "" {
			return nil, fmt.Errorf("cj: item missing variant ID")
		}
		products = append(products, cjOrderProduct{
			VID:      vid,
			Quantity: item.Quantity,
		})
	}

	cjReq := cjCreateOrderRequest{
		OrderNumber:          params.ExternalOrderID,
		ShippingZip:          params.Recipient.ZIP,
		ShippingCity:         params.Recipient.City,
		ShippingCountryCode:  params.Recipient.CountryCode,
		ShippingCountry:      params.Recipient.CountryCode,
		ShippingProvince:     params.Recipient.StateCode,
		ShippingAddress:      joinAddress(params.Recipient.Address1, params.Recipient.Address2),
		ShippingCustomerName: params.Recipient.Name,
		ShippingPhone:        params.Recipient.Phone,
		Products:             products,
	}

	var orderResp cjOrder
	if err := p.client.Post(ctx, "/shopping/order/createOrderV2", cjReq, &orderResp); err != nil {
		return nil, fmt.Errorf("cj: create order: %w", err)
	}

	return convertCJOrder(&orderResp), nil
}

func (p *Provider) GetFulfillmentOrder(ctx context.Context, orderID string) (*contracts.FulfillmentOrder, error) {
	path := "/shopping/order/getOrderDetail?orderId=" + orderID
	var order cjOrder
	if err := p.client.Get(ctx, path, &order); err != nil {
		return nil, fmt.Errorf("cj: get order: %w", err)
	}
	return convertCJOrder(&order), nil
}

func (p *Provider) CancelFulfillmentOrder(_ context.Context, _ string) error {
	// CJ API does not support direct order cancellation.
	// Orders must be cancelled via CJ customer service.
	return &contracts.FulfillmentRetryableError{
		Err:       fmt.Errorf("cj: direct cancellation not supported, contact CJ support"),
		Retryable: false,
	}
}

func (p *Provider) ParseWebhook(_ context.Context, payload []byte, headers map[string]string) (*contracts.FulfillmentWebhookEvent, error) {
	var evt cjWebhookEvent
	if err := json.Unmarshal(payload, &evt); err != nil {
		return nil, fmt.Errorf("cj: unmarshal webhook: %w", err)
	}

	if p.webhookSecret != "" {
		if !p.verifyWebhookSignature(evt) {
			return nil, fmt.Errorf("cj: invalid webhook signature")
		}
	}

	webhookType := mapCJWebhookType(evt.EventType, evt.OrderStatus)

	result := &contracts.FulfillmentWebhookEvent{
		Type:       webhookType,
		EventID:    evt.EventID,
		OrderID:    evt.OrderID,
		ExternalID: evt.OrderNum,
		Timestamp:  time.Now(),
	}

	if evt.TrackNumber != "" {
		result.Data = contracts.FulfillmentShipment{
			Carrier:        evt.Carrier,
			TrackingNumber: evt.TrackNumber,
		}
	}

	return result, nil
}

func (p *Provider) EstimateShipping(ctx context.Context, params contracts.ShippingEstimateParams) ([]contracts.ShippingEstimate, error) {
	products := make([]cjFreightProduct, 0, len(params.Items))
	for _, item := range params.Items {
		vid := item.SyncVariantID
		if vid == "" {
			vid = item.CatalogVariantID
		}
		if vid == "" {
			continue
		}
		products = append(products, cjFreightProduct{
			VID:      vid,
			Quantity: item.Quantity,
		})
	}

	if len(products) == 0 {
		return nil, nil
	}

	cjReq := cjFreightRequest{
		StartCountryCode: "CN",
		EndCountryCode:   params.Recipient.CountryCode,
		Products:         products,
	}

	var rates []cjFreightResponse
	if err := p.client.Post(ctx, "/logistic/freightCalculate", cjReq, &rates); err != nil {
		return nil, fmt.Errorf("cj: estimate shipping: %w", err)
	}

	estimates := make([]contracts.ShippingEstimate, 0, len(rates))
	for i, r := range rates {
		minDays, maxDays := parseLogisticAging(r.LogisticAging)
		estimates = append(estimates, contracts.ShippingEstimate{
			ID:          fmt.Sprintf("cj-shipping-%d", i),
			Name:        r.LogisticName,
			Rate:        fmt.Sprintf("%.2f", r.LogisticPrice),
			Currency:    "USD",
			MinDelivery: minDays,
			MaxDelivery: maxDays,
		})
	}
	return estimates, nil
}

// ---------------------------------------------------------------------------
// contracts.FulfillmentCatalogProvider
// ---------------------------------------------------------------------------

func (p *Provider) ListCategories(ctx context.Context) ([]contracts.CatalogCategory, error) {
	var categories []cjCategory
	if err := p.client.Get(ctx, "/product/getCategory", &categories); err != nil {
		return nil, fmt.Errorf("cj: list categories: %w", err)
	}

	result := make([]contracts.CatalogCategory, 0, len(categories))
	for _, c := range categories {
		name := c.CategoryNameEN
		if name == "" {
			name = c.CategoryName
		}
		result = append(result, contracts.CatalogCategory{
			ID:       c.CategoryID,
			Name:     name,
			ParentID: c.ParentCategoryID,
			ImageURL: c.CategoryImage,
		})
	}
	return result, nil
}

func (p *Provider) ListProducts(ctx context.Context, params contracts.CatalogQuery) (*contracts.CatalogPage, error) {
	pageSize := params.Limit
	if pageSize <= 0 {
		pageSize = 20
	}
	pageNum := 1
	if params.Offset > 0 {
		pageNum = (params.Offset / pageSize) + 1
	}

	path := fmt.Sprintf("/product/list?pageNum=%d&pageSize=%d", pageNum, pageSize)
	if params.CategoryID != "" {
		path += "&categoryId=" + params.CategoryID
	}

	var listResp cjProductListResponse
	if err := p.client.Get(ctx, path, &listResp); err != nil {
		return nil, fmt.Errorf("cj: list products: %w", err)
	}

	products := make([]contracts.CatalogProduct, 0, len(listResp.List))
	for _, cp := range listResp.List {
		products = append(products, convertCJProduct(&cp))
	}

	return &contracts.CatalogPage{
		Products:        products,
		Total:           listResp.Total,
		Offset:          params.Offset,
		Limit:           pageSize,
		SearchSupported: false,
	}, nil
}

func (p *Provider) GetProduct(ctx context.Context, productID string) (*contracts.CatalogProduct, error) {
	path := "/product/query?pid=" + productID
	var product cjProduct
	if err := p.client.Get(ctx, path, &product); err != nil {
		return nil, fmt.Errorf("cj: get product: %w", err)
	}
	cp := convertCJProduct(&product)
	return &cp, nil
}

func (p *Provider) GetVariant(ctx context.Context, variantID string) (*contracts.CatalogVariant, error) {
	return nil, contracts.ErrFulfillmentNotImplemented
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func convertCJOrder(o *cjOrder) *contracts.FulfillmentOrder {
	result := &contracts.FulfillmentOrder{
		ID:         o.OrderID,
		ExternalID: o.OrderNum,
		Status:     mapCJOrderStatus(o.OrderStatus),
		CreatedAt:  parseTime(o.CreateDate),
		UpdatedAt:  time.Now(),
	}

	if o.TrackNumber != "" {
		result.Shipments = []contracts.FulfillmentShipment{{
			Carrier:        o.LogisticName,
			TrackingNumber: o.TrackNumber,
		}}
	}

	total := o.OrderAmount
	if total == 0 {
		total = o.ProductAmount + o.ShippingPrice
	}
	result.Costs = &contracts.FulfillmentCosts{
		Subtotal: fmt.Sprintf("%.2f", o.ProductAmount),
		Shipping: fmt.Sprintf("%.2f", o.ShippingPrice),
		Total:    fmt.Sprintf("%.2f", total),
		Currency: "USD",
	}

	return result
}

func convertCJProduct(cp *cjProduct) contracts.CatalogProduct {
	variants := make([]contracts.CatalogVariant, 0, len(cp.Variants))
	minPrice := cp.SellPrice
	maxPrice := cp.SellPrice

	for _, v := range cp.Variants {
		if v.VariantSellPrice < minPrice {
			minPrice = v.VariantSellPrice
		}
		if v.VariantSellPrice > maxPrice {
			maxPrice = v.VariantSellPrice
		}

		attrs := make(map[string]string)
		if v.VariantProperty != "" {
			attrs["property"] = v.VariantProperty
		}

		name := v.VariantName
		if name == "" {
			name = v.VariantNameCN
		}
		variants = append(variants, contracts.CatalogVariant{
			ID:         v.VID,
			Title:      name,
			Price:      fmt.Sprintf("%.2f", v.VariantSellPrice),
			Currency:   "USD",
			SKU:        v.VariantSKU,
			InStock:    true,
			Attributes: attrs,
			ImageURL:   v.VariantImage,
		})
	}

	title := cp.ProductName
	if title == "" {
		title = cp.ProductNameCN
	}

	return contracts.CatalogProduct{
		ID:          cp.PID,
		Title:       title,
		Description: cp.Description,
		CategoryID:  cp.CategoryID,
		ImageURL:    cp.ProductImage,
		Variants:    variants,
		MinPrice:    fmt.Sprintf("%.2f", minPrice),
		MaxPrice:    fmt.Sprintf("%.2f", maxPrice),
		Currency:    "USD",
	}
}

func mapCJOrderStatus(status string) contracts.FulfillmentStatus {
	switch strings.ToUpper(status) {
	case "CREATED", "WAIT_CONFIRM":
		return contracts.FulfillmentStatusPending
	case "IN_CART", "ORDERED", "IN_PROCESS":
		return contracts.FulfillmentStatusInProcess
	case "SHIPPED", "DELIVERING":
		return contracts.FulfillmentStatusShipped
	case "DELIVERED":
		return contracts.FulfillmentStatusDelivered
	case "CANCELLED", "CANCELED":
		return contracts.FulfillmentStatusCanceled
	case "FAILED", "OUT_OF_STOCK":
		return contracts.FulfillmentStatusFailed
	default:
		return contracts.FulfillmentStatusPending
	}
}

func mapCJWebhookType(eventType, orderStatus string) contracts.FulfillmentWebhookType {
	switch {
	case eventType == "ORDER_SHIPPED" || strings.ToUpper(orderStatus) == "SHIPPED":
		return contracts.FulfillmentWebhookShipped
	case eventType == "ORDER_CANCELLED" || strings.ToUpper(orderStatus) == "CANCELLED":
		return contracts.FulfillmentWebhookOrderCanceled
	case eventType == "ORDER_FAILED" || strings.ToUpper(orderStatus) == "FAILED":
		return contracts.FulfillmentWebhookOrderFailed
	case eventType == "STOCK_UPDATED":
		return contracts.FulfillmentWebhookStockUpdated
	default:
		return contracts.FulfillmentWebhookOrderUpdated
	}
}

// verifyWebhookSignature validates CJ's MD5-based webhook signature.
// sign = MD5(orderId + orderStatus + timestamp + webhookSecret)
func (p *Provider) verifyWebhookSignature(evt cjWebhookEvent) bool {
	raw := evt.OrderID + evt.OrderStatus + evt.Timestamp + p.webhookSecret
	h := md5.Sum([]byte(raw))
	expected := hex.EncodeToString(h[:])
	return subtle.ConstantTimeCompare([]byte(expected), []byte(evt.Sign)) == 1
}

func joinAddress(addr1, addr2 string) string {
	if addr2 == "" {
		return addr1
	}
	return addr1 + ", " + addr2
}

func parseTime(s string) time.Time {
	t, _ := time.Parse("2006-01-02 15:04:05", s)
	return t
}

// parseLogisticAging parses CJ's delivery time format "7-15 business days"
// into min/max day integers.
func parseLogisticAging(aging string) (int, int) {
	aging = strings.TrimSpace(aging)
	aging = strings.ToLower(aging)
	aging = strings.ReplaceAll(aging, "business days", "")
	aging = strings.ReplaceAll(aging, "working days", "")
	aging = strings.ReplaceAll(aging, "days", "")
	aging = strings.TrimSpace(aging)

	parts := strings.SplitN(aging, "-", 2)
	if len(parts) != 2 {
		if d, err := strconv.Atoi(strings.TrimSpace(aging)); err == nil {
			return d, d
		}
		return 7, 21
	}

	minD, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	maxD, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err1 != nil || err2 != nil {
		return 7, 21
	}
	return minD, maxD
}

// ClassifyError maps CJ errors to retryability classification.
// Follows the same signature as printful.ClassifyError / printify.ClassifyError.
func ClassifyError(err error) *contracts.FulfillmentRetryableError {
	if err == nil {
		return nil
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return &contracts.FulfillmentRetryableError{Err: err, Retryable: apiErr.IsRetryable()}
	}
	var authErr *AuthError
	if errors.As(err, &authErr) {
		return &contracts.FulfillmentRetryableError{Err: err, Retryable: false}
	}
	var rateErr *RateLimitError
	if errors.As(err, &rateErr) {
		return &contracts.FulfillmentRetryableError{Err: err, Retryable: true}
	}
	return &contracts.FulfillmentRetryableError{Err: err, Retryable: true}
}
