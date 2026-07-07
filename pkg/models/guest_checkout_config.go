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
	// encrypt buyer shipping addresses client-side. Empty pauses physical
	// guest checkout. Only a passphrase-encrypted private-key backup may be
	// stored; plaintext private key material is never accepted.
	PGPPublicKey string `json:"pgpPublicKey,omitempty" gorm:"type:text"`
	// PGPKeyFingerprint identifies the active address-encryption key. It is
	// buyer-visible so checkout can bind ciphertext to a stable key version.
	PGPKeyFingerprint string `json:"pgpKeyFingerprint,omitempty" gorm:"size:64"`
	PGPKeyVersion     int    `json:"pgpKeyVersion,omitempty"`
	// PGPEncryptedPrivateKey is an authenticated-seller backup of the private
	// key encrypted with the merchant's recovery passphrase. It is never
	// serialized by public settings endpoints.
	PGPEncryptedPrivateKey string `json:"-" gorm:"type:text"`
	// AddressEncryptionRequired makes physical guest checkout fail closed.
	// Sovereign composition enables it when the merchant creates an address
	// protection key; the generic open-core configuration remains opt-in.
	AddressEncryptionRequired bool `json:"addressEncryptionRequired" gorm:"not null;default:false"`

	// AvailableCoins is a computed, non-persisted field populated by
	// GetGuestCheckoutConfig at query time. It reflects the subset of
	// AcceptedCoins that are buyer-visible on the running node because the
	// guest checkout closure path is ready. Buyer-facing UIs should use this
	// field; the admin settings editor should continue using AcceptedCoins so
	// the stored config is preserved.
	AvailableCoins string `json:"availableCoins" gorm:"-"`
}

func (GuestCheckoutConfig) TableName() string {
	return "guest_checkout_configs"
}
