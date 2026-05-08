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

	// Pricing (denormalized totals in listing currency)
	Subtotal          uint64 `json:"subtotal"`
	ShippingCost      uint64 `json:"shippingCost"`
	TotalPrice        uint64 `json:"totalPrice"`
	PriceCurrency     string `json:"priceCurrency"`
	PriceDivisibility uint32 `json:"priceDivisibility"`

	// Buyer info (optional, no identity)
	ShippingAddress []byte `json:"-"`
	ContactEmail    string `json:"contactEmail,omitempty"`

	// Lifecycle
	ExpiresAt       time.Time  `gorm:"index" json:"expiresAt"`
	FundedAt        *time.Time `json:"fundedAt,omitempty"`
	ShippedAt       *time.Time `json:"shippedAt,omitempty"`
	CompletedAt     *time.Time `json:"completedAt,omitempty"`
	TrackingNumber  string     `json:"trackingNumber,omitempty"`
	ShippingCarrier string     `json:"shippingCarrier,omitempty"`
	SellerNote      string     `json:"sellerNote,omitempty"`

	CreatedAt time.Time `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updatedAt"`

	// Relations
	Items []GuestOrderItem `gorm:"foreignKey:OrderToken;references:OrderToken" json:"items"`
}

// TableName overrides the default GORM table name.
func (GuestOrder) TableName() string { return "guest_orders" }

// SetShippingAddress marshals the address struct to JSON bytes.
func (o *GuestOrder) SetShippingAddress(addr interface{}) error {
	data, err := json.Marshal(addr)
	if err != nil {
		return fmt.Errorf("marshal shipping address: %w", err)
	}
	o.ShippingAddress = data
	return nil
}

// GetShippingAddress unmarshals the stored JSON into the target.
func (o *GuestOrder) GetShippingAddress(target interface{}) error {
	if o.ShippingAddress == nil {
		return nil
	}
	return json.Unmarshal(o.ShippingAddress, target)
}

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

	ListingHash   string `json:"listingHash"`
	ListingTitle  string `json:"listingTitle"`
	ListingSlug   string `gorm:"index:idx_guest_item_variant" json:"listingSlug"`
	Quantity      int    `json:"quantity"`
	VariantOptions []byte `json:"-"`
	// VariantHash is a stable hash of the buyer's variant options
	// (empty for listings without SKUs). Used in conjunction with
	// ListingSlug to scope inventory reservations per variant.
	VariantHash   string `gorm:"index:idx_guest_item_variant" json:"variantHash,omitempty"`
	UnitPrice     uint64 `json:"unitPrice"`
	ItemTotal     uint64 `json:"itemTotal"`
	PriceCurrency string `json:"priceCurrency"`
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
