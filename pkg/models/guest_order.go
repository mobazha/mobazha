package models

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// GuestOrderState represents the lifecycle state of a Guest Order.
type GuestOrderState int

const (
	GuestOrderAwaitingPayment GuestOrderState = 0
	GuestOrderPaymentDetected GuestOrderState = 1
	GuestOrderFunded          GuestOrderState = 2
	GuestOrderShipped         GuestOrderState = 3
	GuestOrderCompleted       GuestOrderState = 4
	GuestOrderExpired         GuestOrderState = 5
)

func (s GuestOrderState) String() string {
	switch s {
	case GuestOrderAwaitingPayment:
		return "AWAITING_PAYMENT"
	case GuestOrderPaymentDetected:
		return "PAYMENT_DETECTED"
	case GuestOrderFunded:
		return "FUNDED"
	case GuestOrderShipped:
		return "SHIPPED"
	case GuestOrderCompleted:
		return "COMPLETED"
	case GuestOrderExpired:
		return "EXPIRED"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", int(s))
	}
}

// ParseGuestOrderState converts a string (e.g. "FUNDED" or "funded") to a GuestOrderState.
// Returns -1 if the string is not recognized.
func ParseGuestOrderState(s string) (GuestOrderState, bool) {
	switch strings.ToUpper(s) {
	case "AWAITING_PAYMENT":
		return GuestOrderAwaitingPayment, true
	case "PAYMENT_DETECTED":
		return GuestOrderPaymentDetected, true
	case "FUNDED":
		return GuestOrderFunded, true
	case "SHIPPED":
		return GuestOrderShipped, true
	case "COMPLETED":
		return GuestOrderCompleted, true
	case "EXPIRED":
		return GuestOrderExpired, true
	default:
		return -1, false
	}
}

// GuestOrder represents an anonymous buyer's order on a standalone store.
// Uses node-managed HD address derivation for payment; no P2P, no escrow.
type GuestOrder struct {
	TenantMixin
	ID         int             `gorm:"primaryKey;autoIncrement:false" json:"id"`
	OrderToken string          `gorm:"uniqueIndex;size:64" json:"orderToken"`
	State      GuestOrderState `gorm:"index" json:"state"`

	// BuyerPortalTokenHash stores the SHA-256 hash of the independent bearer
	// secret used to retrieve digital entitlements. OrderToken remains the
	// order status/payment reference; it must not grant access to delivered
	// files, links, or license keys.
	BuyerPortalTokenHash      string     `gorm:"size:64" json:"-"`
	BuyerPortalTokenExpiresAt *time.Time `gorm:"index" json:"-"`
	BuyerPortalTokenVersion   int        `json:"-"`

	// Payment
	PaymentCoin    string `gorm:"index" json:"paymentCoin"`
	PaymentAddress string `gorm:"index" json:"paymentAddress"`
	PaymentAmount  string `json:"paymentAmount"`
	SweepToAddress string `json:"-"`
	ReferenceKey   string `json:"referenceKey,omitempty"`
	PaymentTxHash  string `json:"paymentTxHash,omitempty"`
	Confirmations  int    `json:"confirmations"`
	RequiredConfs  int    `json:"requiredConfs"`
	AddressIndex   uint32 `json:"-"`
	ExternalPaymentTxHeight uint64 `json:"-"`

	// Pool-stage tracking (currently only populated by ExternalPayment, where
	// mempool transfers are visible via wallet-rpc). These fields are a
	// UX hint only — the order remains in AWAITING_PAYMENT until the
	// transfer is mined and HandlePaymentDetected fires. This keeps the
	// invariant `state == PAYMENT_DETECTED ⇒ tx is on-chain` and lets
	// CleanupExpiredOrders handle pool-evicted orders without special
	// casing. PoolAmount is in atomic units of PaymentCoin.
	PoolTxHash     string     `json:"poolTxHash,omitempty"`
	PoolAmount     uint64     `json:"poolAmount,omitempty"`
	PoolDetectedAt *time.Time `json:"poolDetectedAt,omitempty"`

	// Pricing (denormalized totals in listing currency)
	Subtotal          uint64 `json:"subtotal"`
	ShippingCost      uint64 `json:"shippingCost"`
	TotalPrice        uint64 `json:"totalPrice"`
	PriceCurrency     string `json:"priceCurrency"`
	PriceDivisibility uint32 `json:"priceDivisibility"`

	// Buyer info (optional, no identity)
	// ShippingAddress holds either a JSON-encoded address struct (plaintext)
	// or an OpenPGP ASCII-armor ciphertext (PM-3a). Inspect
	// ShippingAddressEncrypted to distinguish the two cases.
	ShippingAddress          []byte `json:"-"`
	ShippingAddressEncrypted bool   `gorm:"column:shipping_address_encrypted" json:"-"`
	ContactEmail             string `json:"contactEmail,omitempty"`

	// Lifecycle
	ExpiresAt       time.Time  `gorm:"index" json:"expiresAt"`
	FundedAt        *time.Time `json:"fundedAt,omitempty"`
	ShippedAt       *time.Time `json:"shippedAt,omitempty"`
	CompletedAt     *time.Time `json:"completedAt,omitempty"`
	TrackingNumber  string     `json:"trackingNumber,omitempty"`
	ShippingCarrier string     `json:"shippingCarrier,omitempty"`
	SellerNote      string     `json:"sellerNote,omitempty"`

	// AutoCompleteAfterShipDaysOverride snapshots the seller's digital-good
	// review window for guest checkout orders. 0 keeps the legacy guest
	// auto-complete period for non-digital or pre-migration orders.
	AutoCompleteAfterShipDaysOverride uint32 `gorm:"column:auto_complete_after_ship_days_override" json:"autoCompleteAfterShipDaysOverride,omitempty"`

	CreatedAt time.Time `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updatedAt"`

	// Relations
	Items []GuestOrderItem `gorm:"foreignKey:OrderToken;references:OrderToken" json:"items"`
}

// TableName overrides the default GORM table name.
func (GuestOrder) TableName() string { return "guest_orders" }

// SetShippingAddress stores the buyer's shipping address.
//
// If addr is a string starting with "-----BEGIN PGP MESSAGE-----", it is
// treated as an OpenPGP ASCII-armor ciphertext (PM-3a encrypted mode) and
// stored verbatim with ShippingAddressEncrypted = true.
//
// Any other value is JSON-marshalled and stored as plaintext
// (ShippingAddressEncrypted = false).
func (o *GuestOrder) SetShippingAddress(addr interface{}) error {
	if s, ok := addr.(string); ok {
		if strings.HasPrefix(s, "-----BEGIN PGP MESSAGE-----") {
			o.ShippingAddress = []byte(s)
			o.ShippingAddressEncrypted = true
			return nil
		}
		return fmt.Errorf("raw string address not allowed; submit a PGP-armored ciphertext or an address struct")
	}
	data, err := json.Marshal(addr)
	if err != nil {
		return fmt.Errorf("marshal shipping address: %w", err)
	}
	o.ShippingAddress = data
	o.ShippingAddressEncrypted = false
	return nil
}

// GetShippingAddress unmarshals the stored plaintext JSON into the target.
// Returns ErrShippingAddressEncrypted if the address is PGP-encrypted;
// callers that need the ciphertext should read ShippingAddress directly.
func (o *GuestOrder) GetShippingAddress(target interface{}) error {
	if o.ShippingAddress == nil {
		return nil
	}
	if o.ShippingAddressEncrypted {
		return ErrShippingAddressEncrypted
	}
	return json.Unmarshal(o.ShippingAddress, target)
}

// ErrShippingAddressEncrypted is returned when GetShippingAddress is called
// on an order whose address is stored as PGP ciphertext. The caller must
// read ShippingAddress directly and decrypt it in the Admin browser.
var ErrShippingAddressEncrypted = fmt.Errorf("shipping address is PGP-encrypted; decrypt in Admin browser")

// IsTerminal returns true if the order is in a final state.
func (o *GuestOrder) IsTerminal() bool {
	return o.State == GuestOrderCompleted || o.State == GuestOrderExpired
}

// validTransitions defines the allowed state transitions for a Guest Order.
var validTransitions = map[GuestOrderState][]GuestOrderState{
	GuestOrderAwaitingPayment: {GuestOrderPaymentDetected, GuestOrderExpired},
	GuestOrderPaymentDetected: {GuestOrderFunded, GuestOrderExpired},
	GuestOrderFunded:          {GuestOrderShipped, GuestOrderCompleted},
	GuestOrderShipped:         {GuestOrderCompleted},
	// GuestOrderCompleted and GuestOrderExpired are terminal — no outgoing transitions.
}

// ValidTransition checks whether a transition from the current state to the target is allowed.
func ValidTransition(from, to GuestOrderState) bool {
	allowed, ok := validTransitions[from]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}

// GuestOrderItem represents a single line item within a Guest Order.
type GuestOrderItem struct {
	TenantMixin
	ID         int    `gorm:"primaryKey;autoIncrement:false" json:"id"`
	OrderToken string `gorm:"index;size:64" json:"orderToken"`

	ListingHash    string `json:"listingHash"`
	ListingTitle   string `json:"listingTitle"`
	ListingSlug    string `gorm:"index:idx_guest_item_variant" json:"listingSlug"`
	Quantity       int    `json:"quantity"`
	VariantOptions []byte `json:"-"`
	// VariantHash is a stable hash of the buyer's variant options
	// (empty for listings without SKUs). Used in conjunction with
	// ListingSlug to scope inventory reservations per variant.
	VariantHash       string `gorm:"index:idx_guest_item_variant" json:"variantHash,omitempty"`
	UnitPrice         uint64 `json:"unitPrice"`
	ItemTotal         uint64 `json:"itemTotal"`
	PriceCurrency     string `json:"priceCurrency"`
	PriceDivisibility uint32 `json:"priceDivisibility"`

	ShippingOption  string `json:"shippingOption,omitempty"`
	ShippingService string `json:"shippingService,omitempty"`
	ShippingPrice   uint64 `json:"shippingPrice"`
}

// TableName overrides the default GORM table name.
func (GuestOrderItem) TableName() string { return "guest_order_items" }

// SetVariantOptions marshals variant options to JSON bytes.
func (i *GuestOrderItem) SetVariantOptions(opts []map[string]string) error {
	data, err := json.Marshal(opts)
	if err != nil {
		return fmt.Errorf("marshal variant options: %w", err)
	}
	i.VariantOptions = data
	return nil
}

// GetVariantOptions unmarshals the stored variant options.
func (i *GuestOrderItem) GetVariantOptions() ([]map[string]string, error) {
	if i.VariantOptions == nil {
		return nil, nil
	}
	var opts []map[string]string
	if err := json.Unmarshal(i.VariantOptions, &opts); err != nil {
		return nil, err
	}
	return opts, nil
}
