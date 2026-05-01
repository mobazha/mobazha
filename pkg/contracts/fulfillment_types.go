package contracts

import (
	"errors"
	"time"
)

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
// Fulfillment Location
// ---------------------------------------------------------------------------

// FulfillmentLocation represents a physical or virtual fulfillment origin.
// POD providers get a single virtual location; warehouse providers (e.g.
// CJ Dropshipping) may expose multiple locations in different countries.
type FulfillmentLocation struct {
	ID          string `json:"id"`
	ProviderID  string `json:"providerId"`
	ExternalKey string `json:"externalKey,omitempty"`
	Name        string `json:"name"`
	Type        string `json:"type"`    // "pod", "warehouse", "virtual"
	Country     string `json:"country"` // ISO 3166-1 alpha-2
	IsDefault   bool   `json:"isDefault"`
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
// Supports two modes:
//   - Catalog import: set ProductID (generic template from supplier catalog)
//   - Sync Product import: set SyncProductID (designed product from supplier dashboard)
type ImportProductParams struct {
	ProviderID    string            `json:"providerId"`
	ProductID     string            `json:"productId,omitempty"`
	SyncProductID string            `json:"syncProductId,omitempty"`
	VariantIDs    []string          `json:"variantIds"`
	RetailMarkup  float64           `json:"retailMarkup"`
	DesignFiles   map[string]string `json:"designFiles,omitempty"`
	Title         string            `json:"title,omitempty"`
	Description   string            `json:"description,omitempty"`
	Tags          []string          `json:"tags,omitempty"`
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
	Title         string    `json:"title,omitempty"`
	ThumbnailUrl  string    `json:"thumbnailUrl,omitempty"`
	ExternalID    string    `json:"externalId"`
	SyncProductID string    `json:"syncProductId"`
	Status        string    `json:"status"`
	LastSyncAt    time.Time `json:"lastSyncAt"`
	SupplierCost  string    `json:"supplierCost"`
	RetailPrice   string    `json:"retailPrice"`
}

// StoreSyncProduct represents a product in the supplier's store (created via
// their dashboard or API with custom designs). Unlike CatalogProduct which is
// a generic template, StoreSyncProduct already has designs applied.
type StoreSyncProduct struct {
	ID           string              `json:"id"`
	ExternalID   string              `json:"externalId,omitempty"`
	Name         string              `json:"name"`
	ThumbnailURL string              `json:"thumbnailUrl"`
	VariantCount int                 `json:"variantCount"`
	SyncedCount  int                 `json:"syncedCount"`
	Variants     []StoreSyncVariant  `json:"variants,omitempty"`

	// ImportedListingSlug is set when this sync product has been imported
	// into a Mobazha listing (populated from synced_product_mappings).
	ImportedListingSlug string `json:"importedListingSlug,omitempty"`
}

// StoreSyncVariant is a variant within a StoreSyncProduct.
type StoreSyncVariant struct {
	ID              string            `json:"id"`
	SyncProductID   string            `json:"syncProductId"`
	Name            string            `json:"name"`
	CatalogVariantID string           `json:"catalogVariantId"`
	RetailPrice     string            `json:"retailPrice"`
	Currency        string            `json:"currency"`
	SKU             string            `json:"sku,omitempty"`
	Size            string            `json:"size,omitempty"`
	Color           string            `json:"color,omitempty"`
	ImageURL        string            `json:"imageUrl,omitempty"`
	PreviewURL      string            `json:"previewUrl,omitempty"`
	Files           []SyncVariantFile `json:"files,omitempty"`
	InStock         bool              `json:"inStock"`
}

// SyncVariantFile is a design or preview file on a sync variant.
type SyncVariantFile struct {
	Type         string `json:"type"`
	URL          string `json:"url"`
	PreviewURL   string `json:"previewUrl,omitempty"`
	ThumbnailURL string `json:"thumbnailUrl,omitempty"`
	Filename     string `json:"filename,omitempty"`
}

// StoreSyncPage is a paginated list of store sync products.
type StoreSyncPage struct {
	Products []StoreSyncProduct `json:"products"`
	Total    int                `json:"total"`
	Offset   int                `json:"offset"`
	Limit    int                `json:"limit"`
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
//
// Provider-specific routing of these fields:
//   - Printful uses SyncVariantID alone (its sync_variant identifies both the
//     sync product and the variant) or CatalogVariantID for catalog passthrough.
//   - Printify needs BOTH a sync product ID and a variant ID. SyncProductID
//     carries the Printify product identifier; SyncVariantID (or
//     CatalogVariantID) carries the numeric variant ID.
type FulfillmentItem struct {
	SyncProductID    string            `json:"syncProductId,omitempty"`
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

// FulfillmentOrder represents the fulfillment state for a Mobazha order.
// When an order has multiple supplier groups (multi-supplier split), Groups
// contains each group's state and Status is the aggregate (worst-case) status.
type FulfillmentOrder struct {
	ID            string                `json:"id"`
	ExternalID    string                `json:"externalId"`
	Status        FulfillmentStatus     `json:"status"`
	Shipments     []FulfillmentShipment `json:"shipments,omitempty"`
	Costs         *FulfillmentCosts     `json:"costs,omitempty"`
	CreatedAt     time.Time             `json:"createdAt"`
	UpdatedAt     time.Time             `json:"updatedAt"`
	ErrorMessage  string                `json:"errorMessage,omitempty"`
	FailureReason FailureReason         `json:"failureReason,omitempty"`
	RetryCount    uint8                 `json:"retryCount,omitempty"`
	MaxRetries    uint8                 `json:"maxRetries,omitempty"`
	Groups        []FulfillmentGroup    `json:"groups,omitempty"`
}

// FulfillmentGroup represents one provider/location slice of a multi-supplier
// fulfillment. For single-supplier orders, Groups is empty and the top-level
// fields carry the state (backward compatible).
type FulfillmentGroup struct {
	GroupKey      string                `json:"groupKey"`
	ProviderID    string                `json:"providerId"`
	LocationID    string                `json:"locationId,omitempty"`
	Status        FulfillmentStatus     `json:"status"`
	Shipments     []FulfillmentShipment `json:"shipments,omitempty"`
	ErrorMessage  string                `json:"errorMessage,omitempty"`
	FailureReason FailureReason         `json:"failureReason,omitempty"`
	ItemIndices   []int                 `json:"itemIndices,omitempty"`
}

// FulfillmentStatus is the lifecycle state of a supplier order.
type FulfillmentStatus string

const (
	FulfillmentStatusDraft        FulfillmentStatus = "draft"
	FulfillmentStatusPending      FulfillmentStatus = "pending"
	FulfillmentStatusInProcess    FulfillmentStatus = "in_process"
	FulfillmentStatusShipped      FulfillmentStatus = "shipped"
	FulfillmentStatusDelivered    FulfillmentStatus = "delivered"
	FulfillmentStatusCanceled     FulfillmentStatus = "canceled"
	FulfillmentStatusFailed       FulfillmentStatus = "failed"
	FulfillmentStatusSupplierLoss FulfillmentStatus = "supplier_loss"
)

// FailureReason classifies why a fulfillment order failed.
// Only retryable_provider_error is eligible for automatic retry.
type FailureReason string

const (
	FailureReasonNone                    FailureReason = ""
	FailureReasonRetryableProviderError  FailureReason = "retryable_provider_error"
	FailureReasonValidationFailed        FailureReason = "validation_failed"
	FailureReasonMarginProtectionFailed  FailureReason = "margin_protection_failed"
	FailureReasonManualActionRequired    FailureReason = "manual_action_required"
	FailureReasonPermanentlyFailed       FailureReason = "permanently_failed"
)

// IsRetryable returns true only for transient supplier errors eligible for automatic retry.
func (r FailureReason) IsRetryable() bool {
	return r == FailureReasonRetryableProviderError
}

// FulfillmentRetryableError wraps a provider error with retryability classification.
type FulfillmentRetryableError struct {
	Err       error
	Retryable bool
}

func (e *FulfillmentRetryableError) Error() string { return e.Err.Error() }
func (e *FulfillmentRetryableError) Unwrap() error { return e.Err }

// ClassifyFulfillmentError inspects a provider error and returns the appropriate FailureReason.
func ClassifyFulfillmentError(err error) FailureReason {
	if err == nil {
		return FailureReasonNone
	}
	var re *FulfillmentRetryableError
	if errors.As(err, &re) {
		if re.Retryable {
			return FailureReasonRetryableProviderError
		}
		return FailureReasonValidationFailed
	}
	return FailureReasonRetryableProviderError
}

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

	// SyncProductID and SyncProductName are populated for product-level
	// webhooks (product_synced, stock_updated) that have no associated order.
	SyncProductID   string `json:"syncProductId,omitempty"`
	SyncProductName string `json:"syncProductName,omitempty"`
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

// ---------------------------------------------------------------------------
// Supply Chain Alerts & Auto-Action Rules (M6)
// ---------------------------------------------------------------------------

// SupplyChainAlert represents a monitoring alert surfaced to the seller.
type SupplyChainAlert struct {
	ID          string    `json:"id"`
	ProviderID  string    `json:"providerId"`
	ListingSlug string    `json:"listingSlug"`
	AlertType   string    `json:"alertType"`
	Severity    string    `json:"severity"`
	Title       string    `json:"title"`
	Message     string    `json:"message"`
	Dismissed   bool      `json:"dismissed"`
	ActionTaken string    `json:"actionTaken,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

// AutoActionRule represents a configurable trigger → action automation.
type AutoActionRule struct {
	ID         string    `json:"id"`
	ProviderID string    `json:"providerId,omitempty"`
	Trigger    string    `json:"trigger"`
	Action     string    `json:"action"`
	Threshold  float64   `json:"threshold,omitempty"`
	Enabled    *bool     `json:"enabled,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}
