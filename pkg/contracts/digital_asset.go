package contracts

import (
	"context"
	"errors"
	"io"
	"time"
)

// Sentinel errors returned by DigitalAssetService methods. Handlers use
// errors.Is to map these to appropriate HTTP status codes.
var (
	ErrLicenseNotFound           = errors.New("license key not found")
	ErrActivationLimit           = errors.New("activation limit reached")
	ErrActivationNotFound        = errors.New("activation not found")
	ErrBuyerPortalAccess         = errors.New("buyer portal access denied")
	ErrDigitalVariantUnsupported = errors.New("variant-specific digital assets are not supported")
)

// DigitalAssetService exposes digital asset operations to the API layer.
// Implemented by internal/core/digital.DigitalAssetAppService.
type DigitalAssetService interface {
	// Buyer-facing
	GetBuyerDigitalAssets(orderID string, buyerPortalToken string, authenticatedBuyerPeerID string, allowAdmin bool, urlExpirySec int64) ([]BuyerAssetEntry, error)
	GetDigitalDeliveryStatus(orderID string, buyerPortalToken string, authenticatedPeerID string, allowAdmin bool) (*DigitalDeliveryStatus, error)
	// ServeDownload verifies a signed download URL, applies grant/expiry/quota
	// checks, records the download, and returns a streaming reader for the
	// decrypted file content. Caller must Close the returned Body.
	ServeDownload(ctx context.Context, req DownloadRequest) (*DownloadResponse, error)

	// License validation (public)
	ValidateLicense(licenseKeyPlain, appID string) (*LicenseValidationResult, error)
	ActivateLicense(licenseKeyPlain, appID, fingerprint, label, ipHash string) (*LicenseActivationResult, error)
	DeactivateLicense(licenseKeyPlain, appID, fingerprint string) error

	// Seller asset management
	// UploadFileAssetStream encrypts plaintext streamed from `src` (chunked
	// AEAD v1) and stores the ciphertext in BlobStore. Memory footprint is
	// bounded by the stream chunk size regardless of total file size.
	//
	// `expectedSize` is forwarded as advisory metadata to the BlobStore
	// (see BlobStore.PutStream) — pass the exact plaintext byte count when
	// you know it (e.g. from `Content-Length`), or -1 to let the storage
	// adapter pick its default chunked path. Reserved for future single-PUT
	// optimizations; not currently consumed by any adapter.
	UploadFileAssetStream(ctx context.Context, listingSlug, variantSKU, fileName, mimeType string, src io.Reader, expectedSize int64) (*DigitalAssetInfo, error)
	CreateLinkAsset(listingSlug, variantSKU, url string) (*DigitalAssetInfo, error)
	CreateLicenseKeyAsset(listingSlug, variantSKU, appID string) (*DigitalAssetInfo, error)
	GetAssetsByListing(listingSlug, variantSKU string) ([]DigitalAssetInfo, error)
	GetAssetByID(assetID string) (*DigitalAssetInfo, error)
	UpdateAsset(assetID string, updates AssetUpdateInput) (*DigitalAssetInfo, error)
	DeleteAsset(assetID string) error

	// Seller license key management
	ImportLicenseKeys(listingSlug, variantSKU, appID string, keys []string, licenseType string, maxActivations int, expiresAt time.Time) (int, error)
	GetLicenseKeyPoolStats(listingSlug, variantSKU string) (*LicenseKeyPoolStats, error)
	ListLicenseKeys(listingSlug, variantSKU string, limit, offset int) ([]MaskedLicenseKey, error)
	RevokeLicenseKey(keyID string) error
}

// LicenseValidationResult is the response payload for license validation.
// Metadata is intentionally excluded — this struct is returned on a public
// endpoint and must not leak seller-internal notes.
type LicenseValidationResult struct {
	Valid          bool       `json:"valid"`
	Reason         string     `json:"reason,omitempty"`
	LicenseID      string     `json:"licenseId,omitempty"`
	LicenseType    string     `json:"licenseType,omitempty"`
	ExpiresAt      *time.Time `json:"expiresAt,omitempty"`
	Activations    int64      `json:"activations,omitempty"`
	MaxActivations int        `json:"maxActivations,omitempty"`
}

// LicenseActivationResult is the response payload for license activation.
type LicenseActivationResult struct {
	ID          string    `json:"id"`
	LicenseID   string    `json:"licenseId"`
	Fingerprint string    `json:"fingerprint"`
	Label       string    `json:"label,omitempty"`
	IsActive    bool      `json:"isActive"`
	LastSeenAt  time.Time `json:"lastSeenAt"`
}

// BuyerAssetEntry is a single digital entitlement shown in the Buyer Portal.
//
// Secret fields (DownloadURL, DeliveryURL, LicenseKey) are populated ONLY
// when the grant status is accessible (active or protected). For restricted
// statuses (frozen, revoked, expired) the entry still appears but secrets
// are omitted and RestrictedReason is set.
type BuyerAssetEntry struct {
	AssetID          string              `json:"assetId"`
	AssetType        string              `json:"assetType"`
	FileName         string              `json:"fileName,omitempty"`
	FileSize         int64               `json:"fileSize,omitempty"`
	DownloadURL      string              `json:"downloadURL,omitempty"`
	DeliveryURL      string              `json:"deliveryURL,omitempty"`
	LicenseKeys      []BuyerLicenseEntry `json:"licenseKeys,omitempty"`
	Downloads        int                 `json:"downloadCount"`
	MaxDL            int                 `json:"maxDownloads"`
	ExpiresAt        *time.Time          `json:"expiresAt,omitempty"`
	Status           string              `json:"status"`
	RestrictedReason string              `json:"restrictedReason,omitempty"`
}

// BuyerLicenseEntry represents a single license key allocated to a buyer.
type BuyerLicenseEntry struct {
	LicenseKey     string `json:"licenseKey"`
	LicenseType    string `json:"licenseType,omitempty"`
	Activations    int64  `json:"activations"`
	MaxActivations int    `json:"maxActivations,omitempty"`
}

const (
	DigitalDeliveryStatusNotDigital     = "not_digital"
	DigitalDeliveryStatusReady          = "ready"
	DigitalDeliveryStatusDelivered      = "delivered"
	DigitalDeliveryStatusManualRequired = "manual_required"
	DigitalDeliveryStatusPending        = "pending"
	DigitalDeliveryStatusRestricted     = "restricted"
)

// DigitalDeliveryStatus is the order-level contract used by seller and buyer
// order pages to decide whether digital delivery is automatic or requires a
// manual fallback link.
type DigitalDeliveryStatus struct {
	OrderID                string   `json:"orderID"`
	IsDigitalOrder         bool     `json:"isDigitalOrder"`
	Status                 string   `json:"status"`
	AssetCount             int      `json:"assetCount"`
	GrantCount             int      `json:"grantCount"`
	AccessibleGrantCount   int      `json:"accessibleGrantCount"`
	DeliveryURL            string   `json:"deliveryURL,omitempty"`
	ManualFallbackAllowed  bool     `json:"manualFallbackAllowed"`
	Reason                 string   `json:"reason,omitempty"`
	ListingSlugs           []string `json:"listingSlugs,omitempty"`
	PreconfiguredAssetHint bool     `json:"preconfiguredAssetHint"`
}

// DigitalAssetInfo is the API-facing representation of a digital asset.
// For link-type assets, URL is populated with the decrypted plaintext URL
// on seller-authenticated endpoints only.
type DigitalAssetInfo struct {
	ID           string    `json:"id"`
	ListingSlug  string    `json:"listingSlug"`
	VariantSKU   string    `json:"variantSku,omitempty"`
	AssetType    string    `json:"assetType"`
	FileName     string    `json:"fileName,omitempty"`
	FileSize     int64     `json:"fileSize,omitempty"`
	MimeType     string    `json:"mimeType,omitempty"`
	URL          string    `json:"url,omitempty"`
	SortOrder    int       `json:"sortOrder"`
	MaxDownloads int       `json:"maxDownloads"`
	ExpiryHours  int       `json:"expiryHours"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// AssetUpdateInput holds the mutable fields for UpdateAsset.
type AssetUpdateInput struct {
	MaxDownloads *int    `json:"maxDownloads,omitempty"`
	ExpiryHours  *int    `json:"expiryHours,omitempty"`
	SortOrder    *int    `json:"sortOrder,omitempty"`
	URL          *string `json:"url,omitempty"`
}

// LicenseKeyPoolStats summarises the key pool for a listing.
type LicenseKeyPoolStats struct {
	Available int64 `json:"available"`
	Dispensed int64 `json:"dispensed"`
	Revoked   int64 `json:"revoked"`
	Total     int64 `json:"total"`
}

// MaskedLicenseKey is a license key entry with the actual key masked.
type MaskedLicenseKey struct {
	ID             string     `json:"id"`
	Status         string     `json:"status"`
	MaskedKey      string     `json:"maskedKey"`
	LicenseType    string     `json:"licenseType"`
	MaxActivations int        `json:"maxActivations"`
	OrderID        string     `json:"orderId,omitempty"`
	DispensedAt    *time.Time `json:"dispensedAt,omitempty"`
	ExpiresAt      *time.Time `json:"expiresAt,omitempty"`
}

// DownloadRequest carries fields parsed from a signed download URL plus the
// buyer's request fingerprint (used for audit log only).
//
// KeyVersion is intentionally omitted: it lives in the grant's AssetSnapshot
// and is not in the URL, so ServeDownload derives it server-side. This keeps
// URLs short and prevents tampering — a buyer who flips the version to a
// non-existent key would simply produce an invalid signature.
type DownloadRequest struct {
	OrderID      string
	GrantNonce   string // grant.Nonce
	AssetID      string
	ExpiryUnix   int64
	GrantVersion int
	Signature    []byte // raw bytes (already hex-decoded by handler)
	BuyerIPHash  string
	UserAgent    string
}

// DownloadResponse carries the streaming file body plus headers needed by
// the HTTP handler. Caller MUST Close Body even on error paths.
type DownloadResponse struct {
	FileName string
	MimeType string
	FileSize int64
	Body     io.ReadCloser
}

// DigitalAssetProvider exposes the per-node digital asset subsystem.
// Handlers obtain this via type assertion on NodeService.
type DigitalAssetProvider interface {
	DigitalAssets() DigitalAssetService
}
