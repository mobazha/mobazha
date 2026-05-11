package models

import "time"

// ---------------------------------------------------------------------------
// AssetType constants
// ---------------------------------------------------------------------------

const (
	AssetTypeFile       = "file"
	AssetTypeLicenseKey = "license_key"
	AssetTypeLink       = "link"
	AssetTypeWebhook    = "webhook"
	AssetTypeMembership = "membership"
)

// ---------------------------------------------------------------------------
// DigitalAsset — pre-configured digital deliverable attached to a listing
// ---------------------------------------------------------------------------

// DigitalAsset represents a pre-configured digital deliverable attached to a listing.
// One listing variant (listing_slug + variant_sku) can have multiple DigitalAssets.
//
// ID strategy: string UUID v7 (app-layer assigned).
// Index on (tenant_id, listing_slug, variant_sku) created via migration SQL (non-UNIQUE).
type DigitalAsset struct {
	TenantMixin
	ID          string `gorm:"primaryKey"`
	ListingSlug string `gorm:"column:listing_slug;type:varchar(255);not null"`
	VariantSKU  string `gorm:"column:variant_sku;type:varchar(255);not null;default:''"`
	AssetType   string `gorm:"column:asset_type;type:varchar(32);not null"`

	// For AssetType="file": encrypted file stored in BlobStore.
	// Encryption key derived via HKDF at runtime — never stored.
	FileHash string `gorm:"column:file_hash;type:varchar(255)"`
	FileName string `gorm:"column:file_name;type:varchar(255)"`
	FileSize int64  `gorm:"column:file_size;default:0"`
	MimeType string `gorm:"column:mime_type;type:varchar(128)"`

	// KeyVersion selects the master key version for HKDF derivation.
	// Incremented on key rotation; decrypt uses the version stored here.
	KeyVersion int `gorm:"column:key_version;default:1"`

	// DeliveryData holds encrypted JSON config per AssetType (link URL, webhook config, etc.).
	DeliveryData []byte `gorm:"column:delivery_data;type:blob"`

	SortOrder    int `gorm:"column:sort_order;default:0"`
	MaxDownloads int `gorm:"column:max_downloads;default:0"`
	ExpiryHours  int `gorm:"column:expiry_hours;default:0"`

	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (DigitalAsset) TableName() string { return "digital_assets" }

// ---------------------------------------------------------------------------
// LicenseKeyStatus constants
// ---------------------------------------------------------------------------

const (
	LicenseKeyStatusAvailable = "available"
	LicenseKeyStatusDispensed = "dispensed"
	LicenseKeyStatusSuspended = "suspended"
	LicenseKeyStatusRevoked   = "revoked"
)

// ---------------------------------------------------------------------------
// DigitalLicenseKey — pooled license key for automatic dispensing
// ---------------------------------------------------------------------------

// DigitalLicenseKey stores a single license key in an encrypted pool.
// Atomic allocation uses a conditional UPDATE + RowsAffected check (SQLite-compatible).
//
// LicenseHash = SHA-256(plainKey) for equality lookups without decrypting
// the entire pool.
type DigitalLicenseKey struct {
	TenantMixin
	ID          string `gorm:"primaryKey"`
	ListingSlug string `gorm:"column:listing_slug;type:varchar(255);not null"`
	VariantSKU  string `gorm:"column:variant_sku;type:varchar(255);not null;default:''"`
	LicenseKey  []byte `gorm:"column:license_key;type:blob"`
	KeyVersion  int    `gorm:"column:key_version;default:1"`
	LicenseHash string `gorm:"column:license_hash;type:varchar(128);not null"`

	// AppID is a seller-defined application identifier for multi-product stores.
	// Defaults to ListingSlug on import; seller can override in listing settings.
	AppID string `gorm:"column:app_id;type:varchar(255);not null"`

	Status      string    `gorm:"column:status;type:varchar(16);not null;default:'available'"`
	OrderID     string    `gorm:"column:order_id;type:varchar(255)"`
	BuyerPeerID string    `gorm:"column:buyer_peer_id;type:varchar(255)"`
	DispensedAt time.Time `gorm:"column:dispensed_at"`
	RevokedAt   time.Time `gorm:"column:revoked_at"`
	ExpiresAt   time.Time `gorm:"column:expires_at"`

	MaxActivations int    `gorm:"column:max_activations;default:0"`
	LicenseType    string `gorm:"column:license_type;type:varchar(32);default:'perpetual'"`
	Metadata       []byte `gorm:"column:metadata;type:blob"`
}

func (DigitalLicenseKey) TableName() string { return "digital_license_keys" }

// ---------------------------------------------------------------------------
// LicenseActivation — device activation record for a dispensed license key
// ---------------------------------------------------------------------------

// LicenseActivation tracks device activations for a dispensed license key.
// Idempotency: same (license_id, fingerprint) can only have one active record.
// Partial unique index on (tenant_id, license_id, fingerprint) WHERE is_active = true.
//
// DeactivatedAt uses *time.Time (pointer) so GORM writes SQL NULL for active records.
// IsActive bool provides a cross-dialect partial index condition (SQLite + PostgreSQL).
type LicenseActivation struct {
	TenantMixin
	ID            string     `gorm:"primaryKey"`
	LicenseID     string     `gorm:"column:license_id;type:varchar(255);not null"`
	Fingerprint   string     `gorm:"column:fingerprint;type:varchar(255);not null"`
	Label         string     `gorm:"column:label;type:varchar(255)"`
	IPHash        string     `gorm:"column:ip_hash;type:varchar(128)"`
	IsActive      bool       `gorm:"column:is_active;default:true"`
	ActivatedAt   time.Time  `gorm:"column:activated_at;autoCreateTime"`
	LastSeenAt    time.Time  `gorm:"column:last_seen_at"`
	DeactivatedAt *time.Time `gorm:"column:deactivated_at"`
}

func (LicenseActivation) TableName() string { return "license_activations" }

// ---------------------------------------------------------------------------
// DownloadGrant status constants
// ---------------------------------------------------------------------------

const (
	GrantStatusActive    = "active"
	GrantStatusProtected = "protected" // escrow still held; buyer CAN access content
	GrantStatusFrozen    = "frozen"    // dispute; buyer CANNOT access content
	GrantStatusRevoked   = "revoked"   // seller revoked; buyer CANNOT access content
	GrantStatusExpired   = "expired"   // time-limited grant expired
)

// IsGrantAccessible returns true if the grant status allows the buyer to view
// secret fields (download URLs, license keys, delivery URLs) and download
// files. Active and Protected are the accessible states; Frozen, Revoked,
// and Expired are restricted.
func IsGrantAccessible(status string) bool {
	return status == GrantStatusActive || status == GrantStatusProtected
}

// IsGrantAccessibleWithExpiry combines status check with time-based expiry.
// A grant whose ExpiresAt is non-zero and in the past is treated as restricted
// even if the status field hasn't been flipped yet.
func IsGrantAccessibleWithExpiry(status string, expiresAt time.Time) bool {
	if !IsGrantAccessible(status) {
		return false
	}
	if !expiresAt.IsZero() && time.Now().After(expiresAt) {
		return false
	}
	return true
}

// ---------------------------------------------------------------------------
// DownloadGrant — revocable entitlement for a buyer to access a digital asset
// ---------------------------------------------------------------------------

// DownloadGrant is a revocable entitlement for a buyer to download/access a digital asset.
// Created on OrderConfirmation; frozen on DisputeOpen; revoked on Refund/DisputeClose (buyer wins).
//
// Status state machine:
//   create → active:    CANCELABLE / FIAT / zero-amount DIRECT
//   create → protected: MODERATED (funds still in 2-of-3 escrow)
//   active → frozen:    DisputeOpen / AfterSaleDisputeOpened
//   protected → frozen: DisputeOpen (MODERATED order)
//   frozen → revoked:   Refund / DisputeClose buyer wins
//   frozen → active:    DisputeClose seller wins (was active)
//   frozen → protected: DisputeClose seller wins (was MODERATED)
type DownloadGrant struct {
	// TenantID is redefined here (rather than via TenantMixin) so it can be
	// part of the idempotency composite uniqueIndex below.
	TenantID string `gorm:"column:tenant_id;primaryKey;default:'';uniqueIndex:idx_grant_tenant_order_asset,priority:1" json:"-"`
	ID       string `gorm:"primaryKey"`
	// Idempotency: at most one grant per (tenant_id, order_id, asset_id).
	// Replayed OrderConfirmation events therefore cannot create duplicate
	// entitlements. The index includes tenant_id so the same orderID can
	// exist for different tenants (buyer + seller copies in SaaS).
	AssetID     string `gorm:"column:asset_id;type:varchar(255);not null;uniqueIndex:idx_grant_tenant_order_asset,priority:3"`
	OrderID     string `gorm:"column:order_id;type:varchar(255);not null;uniqueIndex:idx_grant_tenant_order_asset,priority:2"`
	BuyerPeerID string `gorm:"column:buyer_peer_id;type:varchar(255);not null"`
	Status      string `gorm:"column:status;type:varchar(16);not null;default:'active'"`

	// Nonce is globally unique; download URL tokens are looked up by nonce.
	Nonce   string `gorm:"column:nonce;uniqueIndex;type:varchar(128);not null"`
	Version int    `gorm:"column:version;default:1"`

	// EntitlementSnapshot freezes the DigitalAsset definition at OrderConfirmation time.
	// Prevents "content drift" — seller updating file/link after buyer paid.
	AssetSnapshot []byte `gorm:"column:asset_snapshot;type:blob"`

	// PreviousStatus stores the status before freeze, for correct restoration on dispute close.
	PreviousStatus string `gorm:"column:previous_status;type:varchar(16)"`

	MaxDownloads  int       `gorm:"column:max_downloads;default:0"`
	DownloadCount int       `gorm:"column:download_count;default:0"`
	ExpiresAt     time.Time `gorm:"column:expires_at"`
	RevokedAt     time.Time `gorm:"column:revoked_at"`
	RevokeReason  string    `gorm:"column:revoke_reason;type:varchar(32)"`
	CreatedAt     time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt     time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (DownloadGrant) TableName() string { return "download_grants" }

// ---------------------------------------------------------------------------
// DigitalDownloadLog — download audit trail
// ---------------------------------------------------------------------------

// DigitalDownloadLog records each download attempt for audit and analytics.
type DigitalDownloadLog struct {
	TenantMixin
	ID           string    `gorm:"primaryKey"`
	GrantID      string    `gorm:"column:grant_id;type:varchar(255);not null"`
	AssetID      string    `gorm:"column:asset_id;type:varchar(255);not null"`
	OrderID      string    `gorm:"column:order_id;type:varchar(255);not null"`
	BuyerPeerID  string    `gorm:"column:buyer_peer_id;type:varchar(255)"`
	IPHash       string    `gorm:"column:ip_hash;type:varchar(128)"`
	UserAgent    string    `gorm:"column:user_agent;type:text"`
	Success      bool      `gorm:"column:success;default:true"`
	BlockReason  string    `gorm:"column:block_reason;type:varchar(32)"`
	DownloadedAt time.Time `gorm:"column:downloaded_at;autoCreateTime"`
}

func (DigitalDownloadLog) TableName() string { return "digital_download_logs" }

// ---------------------------------------------------------------------------
// AssetSnapshot — JSON structure stored in DownloadGrant.AssetSnapshot
// ---------------------------------------------------------------------------

// AssetSnapshot captures the DigitalAsset state at OrderConfirmation time.
// Serialized as JSON into DownloadGrant.AssetSnapshot.
type AssetSnapshot struct {
	AssetType    string `json:"assetType"`
	FileHash     string `json:"fileHash,omitempty"`
	FileName     string `json:"fileName,omitempty"`
	FileSize     int64  `json:"fileSize,omitempty"`
	KeyVersion   int    `json:"keyVersion,omitempty"`
	DeliveryData []byte `json:"deliveryData,omitempty"`
}
