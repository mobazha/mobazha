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
	Credentials []byte `gorm:"column:credentials"`

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
// FulfillmentLocation — physical or virtual fulfillment location
// ---------------------------------------------------------------------------

// FulfillmentLocation represents a physical warehouse or virtual POD location
// from which products are fulfilled. Each connected provider gets at least one
// default location; multi-warehouse providers (e.g. CJ Dropshipping) may have
// several.
//
// uniqueIndex on (tenant_id, provider_id, external_key) created via migration SQL.
type FulfillmentLocation struct {
	TenantMixin
	ID          string `gorm:"primaryKey"`
	ProviderID  string `gorm:"column:provider_id;type:varchar(32);not null"`
	ExternalKey string `gorm:"column:external_key;type:varchar(255);not null;default:'default'"`
	Name        string `gorm:"column:name;type:varchar(255);not null"`
	Type        string `gorm:"column:type;type:varchar(16);not null;default:'virtual'"` // "pod", "warehouse", "virtual"
	Country     string `gorm:"column:country;type:varchar(2)"`                          // ISO 3166-1 alpha-2
	IsDefault   bool   `gorm:"column:is_default;default:true"`
	CreatedAt   time.Time
}

func (FulfillmentLocation) TableName() string { return "fulfillment_locations" }

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
	LocationID    string    `gorm:"column:location_id;type:varchar(64)"`
	ListingSlug   string    `gorm:"column:listing_slug;type:varchar(255);not null"`
	ExternalID    string    `gorm:"column:external_id;type:varchar(255)"`
	SyncProductID string    `gorm:"column:sync_product_id;type:varchar(255)"`
	SupplierCost  string    `gorm:"column:supplier_cost;type:varchar(64)"`
	RetailPrice   string    `gorm:"column:retail_price;type:varchar(64)"`
	Status        string    `gorm:"column:status;type:varchar(16);not null;default:'synced'"`
	LastSyncAt    time.Time `gorm:"column:last_sync_at"`

	// PreviousListingStatus records the listing status before automation hid it.
	// Used by stock_back / show_listing rules to only restore listings that were
	// previously published — prevents auto-publishing seller drafts.
	PreviousListingStatus string `gorm:"column:previous_listing_status;type:varchar(16)"`

	// Metadata stores provider-specific data (variant mappings, design files, etc.)
	Metadata []byte `gorm:"column:metadata"`
}

func (SyncedProductMapping) TableName() string { return "synced_product_mappings" }

// ---------------------------------------------------------------------------
// FulfillmentOrderMapping — Mobazha order ↔ supplier fulfillment order link
// ---------------------------------------------------------------------------

// FulfillmentOrderMapping tracks the relationship between a Mobazha order and
// supplier fulfillment orders. A single Mobazha order may produce multiple
// mappings when items come from different providers or locations — each group
// is keyed by FulfillmentGroupKey ("providerID:locationID").
//
// uniqueIndex on (tenant_id, mobazha_order_id, fulfillment_group_key) in migration SQL.
type FulfillmentOrderMapping struct {
	TenantMixin
	ID                  string `gorm:"primaryKey"`
	MobazhaOrderID      string `gorm:"column:mobazha_order_id;type:varchar(255);not null"`
	ProviderID          string `gorm:"column:provider_id;type:varchar(32);not null"`
	LocationID          string `gorm:"column:location_id;type:varchar(64)"`
	FulfillmentGroupKey string `gorm:"column:fulfillment_group_key;type:varchar(255);not null;default:'default'"`
	FulfillmentOrderID  string `gorm:"column:fulfillment_order_id;type:varchar(255)"`
	Status             string    `gorm:"column:status;type:varchar(32);not null;default:'pending'"`
	TrackingNumber     string    `gorm:"column:tracking_number;type:varchar(255)"`
	TrackingURL        string    `gorm:"column:tracking_url;type:text"`
	Carrier            string    `gorm:"column:carrier;type:varchar(64)"`
	SupplierCost       string    `gorm:"column:supplier_cost;type:varchar(64)"`
	ErrorMessage       string    `gorm:"column:error_message;type:text"`

	// FailureReason classifies why the fulfillment failed (FF-2).
	// Only "retryable_provider_error" is eligible for automatic retry by the worker.
	// Other reasons (validation_failed, margin_protection_failed, etc.) require manual intervention.
	FailureReason string `gorm:"column:failure_reason;type:varchar(64)"`

	// DisputeHeld is set when a dispute is opened on the Mobazha order.
	// Retry and reconcile workers skip rows where DisputeHeld is true.
	DisputeHeld bool `gorm:"column:dispute_held;default:false"`

	// ItemIndices: JSON array of item indices for multi-supplier split (FF-3).
	ItemIndices string `gorm:"column:item_indices;type:text"`

	// Retry fields for failed supplier API calls.
	RetryCount  int       `gorm:"column:retry_count;default:0"`
	NextRetryAt time.Time `gorm:"column:next_retry_at"`

	// RetryLockedUntil is used for atomic worker lease. A worker sets this to
	// now+5min before processing; other goroutines skip rows with a future value.
	// On release, this is reset to time.Time{} (zero value, written as
	// '0001-01-01 00:00:00' — not NULL — so claim queries also test for `<= now`).
	RetryLockedUntil time.Time `gorm:"column:retry_locked_until"`

	// LastWebhookEventID for idempotency (quick-check before ProcessedFulfillmentEvent).
	LastWebhookEventID string `gorm:"column:last_webhook_event_id;type:varchar(255)"`

	// OrderAdvancementStatus tracks whether the Mobazha order state has been
	// advanced (AutoConfirmAndShip) after this mapping reached `shipped`.
	// Decoupled from `Status` because if the supplier-side status update
	// succeeded but the subsequent autoConfirm/ship call failed (chain hiccup,
	// order app service error, etc.) the mapping must still be revisited by
	// the reconcile worker. Values: "" (not yet shipped), "pending" (shipped
	// in mapping, order advance not yet attempted/succeeded), "done"
	// (advance succeeded), "permanent_fail" (advance permanently failed).
	OrderAdvancementStatus string `gorm:"column:order_advancement_status;type:varchar(16);default:''"`

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
// The unique index (tenant_id, provider_id, event_id) ensures tenant isolation:
// the same Printful event for different SaaS tenants won't collide.
// tenant_id is included via the explicit index tag on TenantID below.
//
// Status lifecycle: "processing" → "processed". A row in "processing" state means
// another goroutine is currently handling this event. A unique constraint violation
// on insert blocks concurrent duplicates atomically.
type ProcessedFulfillmentEvent struct {
	TenantID    string    `gorm:"column:tenant_id;primaryKey;default:'';uniqueIndex:idx_pfe_tenant_provider_event" json:"-"`
	ID          string    `gorm:"primaryKey"`
	ProviderID  string    `gorm:"column:provider_id;type:varchar(32);not null;uniqueIndex:idx_pfe_tenant_provider_event"`
	EventID     string    `gorm:"column:event_id;type:varchar(255);not null;uniqueIndex:idx_pfe_tenant_provider_event"`
	EventType   string    `gorm:"column:event_type;type:varchar(64)"`
	OrderID     string    `gorm:"column:order_id;type:varchar(255)"`
	Status      string    `gorm:"column:status;type:varchar(16);not null;default:'processing'"`
	ProcessedAt time.Time `gorm:"column:processed_at;autoCreateTime"`
}

func (ProcessedFulfillmentEvent) TableName() string { return "processed_fulfillment_events" }

// ---------------------------------------------------------------------------
// SupplyChainAlert — inventory/price/rule-triggered alerts
// ---------------------------------------------------------------------------

// AlertType classifies the nature of a supply chain alert.
type AlertType string

const (
	AlertTypeStockOut            AlertType = "stock_out"
	AlertTypeStockBack           AlertType = "stock_back"
	AlertTypePriceDrift          AlertType = "price_drift"
	AlertTypeRuleAction          AlertType = "rule_action"
	AlertTypeProductChanged      AlertType = "product_changed"
	AlertTypeProductDiscontinued AlertType = "product_discontinued"
)

// AlertSeverity indicates priority.
type AlertSeverity string

const (
	AlertSeverityInfo     AlertSeverity = "info"
	AlertSeverityWarning  AlertSeverity = "warning"
	AlertSeverityCritical AlertSeverity = "critical"
)

// SupplyChainAlert records a supply chain event requiring seller attention.
type SupplyChainAlert struct {
	TenantMixin
	ID          string        `gorm:"primaryKey"`
	ProviderID  string        `gorm:"column:provider_id;type:varchar(32);not null"`
	ListingSlug string        `gorm:"column:listing_slug;type:varchar(255)"`
	AlertType   AlertType     `gorm:"column:alert_type;type:varchar(32);not null"`
	Severity    AlertSeverity `gorm:"column:severity;type:varchar(16);not null;default:'warning'"`
	Title       string        `gorm:"column:title;type:varchar(255);not null"`
	Message     string        `gorm:"column:message;type:text"`
	Metadata    []byte        `gorm:"column:metadata"`
	Dismissed   bool          `gorm:"column:dismissed;default:false"`
	ActionTaken string        `gorm:"column:action_taken;type:varchar(64)"`
	CreatedAt   time.Time     `gorm:"column:created_at;autoCreateTime"`
}

func (SupplyChainAlert) TableName() string { return "supply_chain_alerts" }

// ---------------------------------------------------------------------------
// AutoActionRule — configurable trigger → action rules
// ---------------------------------------------------------------------------

// RuleTrigger defines what triggers an auto-action.
type RuleTrigger string

const (
	RuleTriggerStockOut           RuleTrigger = "stock_out"
	RuleTriggerStockBack          RuleTrigger = "stock_back"
	RuleTriggerPriceDrift         RuleTrigger = "price_drift"
	RuleTriggerProductCostChanged RuleTrigger = "product_cost_changed"
	RuleTriggerProductDiscontinued RuleTrigger = "product_discontinued"
)

// RuleAction defines what happens when a rule fires.
type RuleAction string

const (
	RuleActionHideListing  RuleAction = "hide_listing"
	RuleActionShowListing  RuleAction = "show_listing"
	RuleActionPauseListing RuleAction = "pause_listing"
	RuleActionNotifyOnly   RuleAction = "notify_only"
	RuleActionAutoDelist   RuleAction = "auto_delist"
	// Note: an "auto_update_price" action was considered but removed — the
	// monitor goroutine cannot safely sign + republish a listing, so the
	// action would have been a misleading name for "notify". Sellers who
	// want a heads-up on supplier cost changes use trigger
	// `product_cost_changed` with action `notify_only`.
)

// AutoActionRule stores seller-configurable automation rules.
type AutoActionRule struct {
	TenantMixin
	ID         string      `gorm:"primaryKey"`
	ProviderID string      `gorm:"column:provider_id;type:varchar(32)"`
	Trigger    RuleTrigger `gorm:"column:trigger;type:varchar(32);not null"`
	Action     RuleAction  `gorm:"column:action;type:varchar(32);not null"`
	Threshold  float64     `gorm:"column:threshold;default:0"`
	Enabled    bool        `gorm:"column:enabled;default:true"`
	CreatedAt  time.Time   `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt  time.Time   `gorm:"column:updated_at;autoUpdateTime"`
}

func (AutoActionRule) TableName() string { return "auto_action_rules" }
