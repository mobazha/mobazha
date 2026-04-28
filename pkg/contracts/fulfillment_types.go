package contracts

import "time"

// ---------------------------------------------------------------------------
// Credentials & Connection
// ---------------------------------------------------------------------------

// ProviderCredentials holds the API credentials for connecting to a supplier.
type ProviderCredentials struct {
	APIKey        string `json:"apiKey"`
	APISecret     string `json:"apiSecret,omitempty"`
	StoreID       string `json:"storeId,omitempty"`
	WebhookURL    string `json:"webhookUrl,omitempty"`
	WebhookSecret string `json:"webhookSecret,omitempty"`
}

// ConnectProviderParams is the request payload for connecting a fulfillment provider.
type ConnectProviderParams struct {
	ProviderID  string              `json:"providerId"`
	Credentials ProviderCredentials `json:"credentials"`
	// WebhookBaseURL is set by the handler (not from client JSON) so the
	// service can construct the full webhook URL including the secret path.
	WebhookBaseURL string `json:"-"`
}

// ProviderConnection represents a connected supplier with its current status.
type ProviderConnection struct {
	ProviderID   string    `json:"providerId"`
	ProviderType string    `json:"providerType"`
	ProviderName string    `json:"providerName"`
	Status       string    `json:"status"` // "connected", "error", "disconnected"
	StoreName    string    `json:"storeName"`
	WebhookURL   string    `json:"webhookUrl,omitempty"`
	ConnectedAt  time.Time `json:"connectedAt"`
	LastSyncAt   time.Time `json:"lastSyncAt,omitempty"`
	ErrorMessage string    `json:"errorMessage,omitempty"`
}

// ---------------------------------------------------------------------------
// Catalog & Product Browsing
// ---------------------------------------------------------------------------

// CatalogCategory represents a supplier product category.
type CatalogCategory struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	ParentID string `json:"parentId,omitempty"`
	ImageURL string `json:"imageUrl,omitempty"`
}

// CatalogQuery is the request for paginated catalog browsing.
type CatalogQuery struct {
	CategoryID string `json:"categoryId,omitempty"`
	Search     string `json:"search,omitempty"`
	Offset     int    `json:"offset"`
	Limit      int    `json:"limit"`
}

// CatalogPage is a paginated response of supplier products.
type CatalogPage struct {
	Products        []CatalogProduct `json:"products"`
	Total           int              `json:"total"`
	Offset          int              `json:"offset"`
	Limit           int              `json:"limit"`
	SearchSupported bool             `json:"searchSupported"`
}

// CatalogProduct represents a supplier product with its variants.
type CatalogProduct struct {
	ID          string           `json:"id"`
	Title       string           `json:"title"`
	Description string           `json:"description"`
	CategoryID  string           `json:"categoryId"`
	ImageURL    string           `json:"imageUrl"`
	Images      []string         `json:"images"`
	Variants    []CatalogVariant `json:"variants"`
	MinPrice    string           `json:"minPrice"`
	MaxPrice    string           `json:"maxPrice"`
	Currency    string           `json:"currency"`
	PrintAreas  []PrintArea      `json:"printAreas,omitempty"`
}

// CatalogVariant represents a specific SKU within a supplier product.
type CatalogVariant struct {
	ID         string            `json:"id"`
	Title      string            `json:"title"`
	Price      string            `json:"price"`
	Currency   string            `json:"currency"`
	SKU        string            `json:"sku,omitempty"`
	InStock    bool              `json:"inStock"`
	Attributes map[string]string `json:"attributes"`
	ImageURL   string            `json:"imageUrl,omitempty"`
}

// PrintArea describes a printable region on a POD product.
type PrintArea struct {
	Type        string `json:"type"`
	DisplayName string `json:"displayName"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	DPI         int    `json:"dpi"`
}

// ---------------------------------------------------------------------------
// Product Import & Sync
// ---------------------------------------------------------------------------

// ImportProductParams is the request for importing a supplier product as a Mobazha listing.
type ImportProductParams struct {
	ProviderID   string            `json:"providerId"`
	ProductID    string            `json:"productId"`
	VariantIDs   []string          `json:"variantIds"`
	RetailMarkup float64           `json:"retailMarkup"`
	DesignFiles  map[string]string `json:"designFiles,omitempty"`
	Title        string            `json:"title,omitempty"`
	Description  string            `json:"description,omitempty"`
	Tags         []string          `json:"tags,omitempty"`
}

// ImportResult is the response after successfully importing a supplier product.
type ImportResult struct {
	ListingSlug   string `json:"listingSlug"`
	SyncProductID string `json:"syncProductId"`
	VariantsCount int    `json:"variantsCount"`
	RetailPrice   string `json:"retailPrice"`
	SupplierCost  string `json:"supplierCost"`
}

// SyncProductParams is the request for bidirectional product sync.
type SyncProductParams struct {
	SyncProductID string `json:"syncProductId"`
}

// SyncedProduct represents a supplier product linked to a Mobazha listing.
type SyncedProduct struct {
	ID            string    `json:"id"`
	ProviderID    string    `json:"providerId"`
	ListingSlug   string    `json:"listingSlug"`
	ExternalID    string    `json:"externalId"`
	SyncProductID string    `json:"syncProductId"`
	Status        string    `json:"status"`
	LastSyncAt    time.Time `json:"lastSyncAt"`
	SupplierCost  string    `json:"supplierCost"`
	RetailPrice   string    `json:"retailPrice"`
}

// SyncStatus represents the current sync state of a product.
type SyncStatus struct {
	SyncProductID string    `json:"syncProductId"`
	Status        string    `json:"status"` // "synced", "pending", "error"
	LastSyncAt    time.Time `json:"lastSyncAt"`
	ErrorMessage  string    `json:"errorMessage,omitempty"`
}

// ---------------------------------------------------------------------------
// Fulfillment Order
// ---------------------------------------------------------------------------

// CreateFulfillmentParams is the request to create a supplier fulfillment order.
type CreateFulfillmentParams struct {
	ExternalOrderID string               `json:"externalOrderId"`
	Recipient       FulfillmentRecipient `json:"recipient"`
	Items           []FulfillmentItem    `json:"items"`
	RetailCosts     *RetailCosts         `json:"retailCosts,omitempty"`
}

// FulfillmentRecipient is the shipping address for a fulfillment order.
type FulfillmentRecipient struct {
	Name        string `json:"name"`
	Address1    string `json:"address1"`
	Address2    string `json:"address2,omitempty"`
	City        string `json:"city"`
	StateCode   string `json:"stateCode"`
	CountryCode string `json:"countryCode"`
	ZIP         string `json:"zip"`
	Phone       string `json:"phone,omitempty"`
	Email       string `json:"email,omitempty"`
}

// FulfillmentItem is a line item in a fulfillment order.
type FulfillmentItem struct {
	SyncVariantID    string            `json:"syncVariantId,omitempty"`
	CatalogVariantID string            `json:"catalogVariantId,omitempty"`
	Quantity         int               `json:"quantity"`
	Files            []FulfillmentFile `json:"files,omitempty"`
	RetailPrice      string            `json:"retailPrice,omitempty"`
}

// FulfillmentFile is a design file attached to a POD line item.
type FulfillmentFile struct {
	Type     string `json:"type"`
	URL      string `json:"url"`
	Filename string `json:"filename,omitempty"`
}

// FulfillmentOrder represents a supplier order and its current state.
type FulfillmentOrder struct {
	ID           string                `json:"id"`
	ExternalID   string                `json:"externalId"`
	Status       FulfillmentStatus     `json:"status"`
	Shipments    []FulfillmentShipment `json:"shipments,omitempty"`
	Costs        *FulfillmentCosts     `json:"costs,omitempty"`
	CreatedAt    time.Time             `json:"createdAt"`
	UpdatedAt    time.Time             `json:"updatedAt"`
	ErrorMessage string                `json:"errorMessage,omitempty"`
}

// FulfillmentStatus is the lifecycle state of a supplier order.
type FulfillmentStatus string

const (
	FulfillmentStatusDraft      FulfillmentStatus = "draft"
	FulfillmentStatusPending    FulfillmentStatus = "pending"
	FulfillmentStatusInProcess  FulfillmentStatus = "in_process"
	FulfillmentStatusShipped    FulfillmentStatus = "shipped"
	FulfillmentStatusDelivered  FulfillmentStatus = "delivered"
	FulfillmentStatusCanceled   FulfillmentStatus = "canceled"
	FulfillmentStatusFailed     FulfillmentStatus = "failed"
)

// FulfillmentShipment holds tracking info for a shipped package.
type FulfillmentShipment struct {
	ID             string `json:"id"`
	Carrier        string `json:"carrier"`
	TrackingNumber string `json:"trackingNumber"`
	TrackingURL    string `json:"trackingUrl"`
	ShipDate       string `json:"shipDate"`
	Items          []int  `json:"items"`
}

// FulfillmentCosts is the cost breakdown from the supplier.
type FulfillmentCosts struct {
	Subtotal string `json:"subtotal"`
	Shipping string `json:"shipping"`
	Tax      string `json:"tax"`
	Total    string `json:"total"`
	Currency string `json:"currency"`
}

// RetailCosts is the seller-facing cost display (for Printful receipts).
type RetailCosts struct {
	Subtotal string `json:"subtotal"`
	Shipping string `json:"shipping"`
	Total    string `json:"total"`
	Currency string `json:"currency"`
}

// ---------------------------------------------------------------------------
// Shipping Estimation
// ---------------------------------------------------------------------------

// ShippingEstimateParams is the request for estimating shipping costs.
type ShippingEstimateParams struct {
	Recipient FulfillmentRecipient `json:"recipient"`
	Items     []FulfillmentItem    `json:"items"`
}

// ShippingEstimate is a single shipping option with cost and ETA.
type ShippingEstimate struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Rate         string `json:"rate"`
	Currency     string `json:"currency"`
	MinDelivery  int    `json:"minDelivery"`
	MaxDelivery  int    `json:"maxDelivery"`
}

// ---------------------------------------------------------------------------
// Webhook Events
// ---------------------------------------------------------------------------

// FulfillmentWebhookEvent is the standardized webhook payload from any supplier.
type FulfillmentWebhookEvent struct {
	Type       FulfillmentWebhookType `json:"type"`
	EventID    string                 `json:"eventId"`
	OrderID    string                 `json:"orderId"`
	ExternalID string                 `json:"externalId"`
	Data       interface{}            `json:"data"`
	Timestamp  time.Time              `json:"timestamp"`
}

// FulfillmentWebhookType classifies the type of supplier webhook event.
type FulfillmentWebhookType string

const (
	FulfillmentWebhookShipped       FulfillmentWebhookType = "package_shipped"
	FulfillmentWebhookOrderUpdated  FulfillmentWebhookType = "order_updated"
	FulfillmentWebhookOrderFailed   FulfillmentWebhookType = "order_failed"
	FulfillmentWebhookOrderCanceled FulfillmentWebhookType = "order_canceled"
	FulfillmentWebhookStockUpdated  FulfillmentWebhookType = "stock_updated"
	FulfillmentWebhookProductSynced FulfillmentWebhookType = "product_synced"
)

// ---------------------------------------------------------------------------
// Mockup (POD-specific)
// ---------------------------------------------------------------------------

// MockupParams is the request for generating a POD product mockup.
type MockupParams struct {
	ProductID string            `json:"productId"`
	VariantID string            `json:"variantId"`
	Files     []FulfillmentFile `json:"files"`
}

// MockupResult is the generated mockup images.
type MockupResult struct {
	TaskID string   `json:"taskId"`
	Status string   `json:"status"` // "pending", "completed", "failed"
	Images []string `json:"images,omitempty"`
}
