package models

// GuestCheckoutConfig stores per-tenant guest checkout business settings.
// Singleton per tenant (ID is always 1).
//
// Activation requires BOTH:
//   - Platform feature flag "guest_checkout" = enabled (FeatureOverride table)
//   - This config's Enabled = true (seller opt-in)
//
// The feature flag is the platform-level gate; this Enabled field is the
// seller-level opt-in within an enabled platform. Removing this duplication
// would require all sellers to enable guest checkout when the platform does.
type GuestCheckoutConfig struct {
	TenantMixin
	ID             int    `json:"-" gorm:"primaryKey;autoIncrement:false"`
	Enabled        bool   `json:"enabled"`
	AcceptedCoins  string `json:"acceptedCoins"`  // comma-separated coin codes, e.g. "BTC,ETH,SOL"
	MaxOrderAmount string `json:"maxOrderAmount"` // smallest-unit string; "0" = unlimited
	PaymentTimeout int    `json:"paymentTimeout"` // minutes; 0 = use default (1440 = 24h)

	// PGPPublicKey is the seller's OpenPGP ASCII armor public key used to
	// encrypt buyer shipping addresses client-side (PM-3a). Empty means
	// encryption is unavailable; buyers will see a plaintext-warning before
	// submitting their address. The private key is NEVER stored here — it
	// lives only in the Admin's browser and is used for in-browser decryption.
	PGPPublicKey string `json:"pgpPublicKey,omitempty" gorm:"size:8192"`
}

func (GuestCheckoutConfig) TableName() string {
	return "guest_checkout_configs"
}
