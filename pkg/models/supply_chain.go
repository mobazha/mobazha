package models

import "time"

// ---------------------------------------------------------------------------
// FulfillmentProviderConfig — per-tenant supplier credentials
// ---------------------------------------------------------------------------

// FulfillmentProviderConfig stores encrypted API credentials for a fulfillment provider.
// Each tenant can connect one instance per provider (e.g. one Printful account).
//
// ID strategy: string UUID v7 (avoids TenantMixin + uint autoIncrement:false pitfalls).
// uniqueIndex on (tenant_id, provider_id) created via migration SQL.
type FulfillmentProviderConfig struct {
	TenantMixin
	ID           string `gorm:"primaryKey"`
	ProviderID   string `gorm:"column:provider_id;type:varchar(32);not null"`
	ProviderType string `gorm:"column:provider_type;type:varchar(16);not null"`

	// Encrypted JSON blob of ProviderCredentials (AES-256-GCM via pkg/encryption).
	Credentials []byte `gorm:"column:credentials;type:blob"`

	// WebhookSecret is a per-connection crypto/rand hex token embedded in the webhook URL.
	// Globally unique so SaaS can route incoming webhooks to the correct tenant.
	WebhookSecret string `gorm:"column:webhook_secret;uniqueIndex"`

	StoreID   string `gorm:"column:store_id;type:varchar(255)"`
	StoreName string `gorm:"column:store_name;type:varchar(255)"`

	// Status: "connected", "error", "disconnected"
	Status string `gorm:"column:status;type:varchar(16);not null;default:'disconnected'"`

	ConnectedAt time.Time `gorm:"column:connected_at"`
	LastSyncAt  time.Time `gorm:"column:last_sync_at"`
}

func (FulfillmentProviderConfig) TableName() string { return "fulfillment_provider_configs" }

// MaskCredentials replaces the encrypted credentials with nil for API responses.
func (c *FulfillmentProviderConfig) MaskCredentials() {
	c.Credentials = nil
}

// ---------------------------------------------------------------------------
// SyncedProductMapping — supplier product ↔ Mobazha listing link
// ---------------------------------------------------------------------------

// SyncedProductMapping tracks the relationship between a supplier product and a
// Mobazha listing created via ImportProduct.
//
// uniqueIndex on (tenant_id, listing_slug) created via migration SQL.
type SyncedProductMapping struct {
	TenantMixin
	ID            string    `gorm:"primaryKey"`
	ProviderID    string    `gorm:"column:provider_id;type:varchar(32);not null"`
	ListingSlug   string    `gorm:"column:listing_slug;type:varchar(255);not null"`
	ExternalID    string    `gorm:"column:external_id;type:varchar(255)"`
	SyncProductID string    `gorm:"column:sync_product_id;type:varchar(255)"`
	SupplierCost  string    `gorm:"column:supplier_cost;type:varchar(64)"`
	RetailPrice   string    `gorm:"column:retail_price;type:varchar(64)"`
	Status        string    `gorm:"column:status;type:varchar(16);not null;default:'synced'"`
	LastSyncAt    time.Time `gorm:"column:last_sync_at"`

	// Metadata stores provider-specific data (variant mappings, design files, etc.)
	Metadata []byte `gorm:"column:metadata;type:blob"`
}

func (SyncedProductMapping) TableName() string { return "synced_product_mappings" }

// ---------------------------------------------------------------------------
// FulfillmentOrderMapping — Mobazha order ↔ supplier fulfillment order link
// ---------------------------------------------------------------------------

// FulfillmentOrderMapping tracks the relationship between a Mobazha order and one
// or more supplier fulfillment orders. For v1, each Mobazha order maps to at most
// one supplier order (multi-supplier split deferred to FF-3).
//
// uniqueIndex on (tenant_id, mobazha_order_id) created via migration SQL.
type FulfillmentOrderMapping struct {
	TenantMixin
	ID                 string    `gorm:"primaryKey"`
	MobazhaOrderID     string    `gorm:"column:mobazha_order_id;type:varchar(255);not null"`
	ProviderID         string    `gorm:"column:provider_id;type:varchar(32);not null"`
	FulfillmentOrderID string    `gorm:"column:fulfillment_order_id;type:varchar(255)"`
	Status             string    `gorm:"column:status;type:varchar(32);not null;default:'pending'"`
	TrackingNumber     string    `gorm:"column:tracking_number;type:varchar(255)"`
	TrackingURL        string    `gorm:"column:tracking_url;type:text"`
	Carrier            string    `gorm:"column:carrier;type:varchar(64)"`
	SupplierCost       string    `gorm:"column:supplier_cost;type:varchar(64)"`
	ErrorMessage       string    `gorm:"column:error_message;type:text"`

	// ItemIndices: JSON array of item indices for multi-supplier split (FF-3).
	ItemIndices string `gorm:"column:item_indices;type:text"`

	// Retry fields for failed supplier API calls.
	RetryCount  int       `gorm:"column:retry_count;default:0"`
	NextRetryAt time.Time `gorm:"column:next_retry_at"`

	// LastWebhookEventID for idempotency (quick-check before ProcessedFulfillmentEvent).
	LastWebhookEventID string `gorm:"column:last_webhook_event_id;type:varchar(255)"`

	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (FulfillmentOrderMapping) TableName() string { return "fulfillment_order_mappings" }

// ---------------------------------------------------------------------------
// ProcessedFulfillmentEvent — webhook idempotency deduplication
// ---------------------------------------------------------------------------

// ProcessedFulfillmentEvent prevents duplicate processing of supplier webhook events.
// Records are cleaned up after a retention period via a periodic job.
//
// uniqueIndex on (tenant_id, provider_id, event_id) created via migration SQL.
type ProcessedFulfillmentEvent struct {
	TenantMixin
	ID          string    `gorm:"primaryKey"`
	ProviderID  string    `gorm:"column:provider_id;type:varchar(32);not null"`
	EventID     string    `gorm:"column:event_id;type:varchar(255);not null"`
	EventType   string    `gorm:"column:event_type;type:varchar(64)"`
	OrderID     string    `gorm:"column:order_id;type:varchar(255)"`
	ProcessedAt time.Time `gorm:"column:processed_at;autoCreateTime"`
}

func (ProcessedFulfillmentEvent) TableName() string { return "processed_fulfillment_events" }
