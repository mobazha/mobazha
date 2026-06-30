package contracts

import (
	"context"
	"errors"
	"time"
)

// Sentinel errors returned by supply availability implementations.
// Handlers and order services should use errors.Is instead of matching text.
var (
	ErrSupplyUnavailable          = errors.New("supply unavailable")
	ErrSupplyManualActionRequired = errors.New("supply manual action required")
	ErrSupplyKindUnsupported      = errors.New("supply kind unsupported")
	ErrSupplyReservationNotFound  = errors.New("supply reservation not found")
)

// SupplyKind identifies the scarcity controller for a sellable order line.
type SupplyKind string

const (
	SupplyKindSkuQuantity      SupplyKind = "sku_quantity"
	SupplyKindLicenseKeyPool   SupplyKind = "license_key_pool"
	SupplyKindUnlimitedDigital SupplyKind = "unlimited_digital"
	SupplyKindExternalSupply   SupplyKind = "external_supply"
)

func (k SupplyKind) IsValid() bool {
	switch k {
	case SupplyKindSkuQuantity,
		SupplyKindLicenseKeyPool,
		SupplyKindUnlimitedDigital,
		SupplyKindExternalSupply:
		return true
	default:
		return false
	}
}

// SupplyAvailabilityStatus is the provider-neutral result of an advisory
// availability check. Quote results are not locks; Reserve is authoritative.
type SupplyAvailabilityStatus string

const (
	SupplyAvailabilityUnknown              SupplyAvailabilityStatus = "unknown"
	SupplyAvailabilityAvailable            SupplyAvailabilityStatus = "available"
	SupplyAvailabilityLowStock             SupplyAvailabilityStatus = "low_stock"
	SupplyAvailabilityOutOfStock           SupplyAvailabilityStatus = "out_of_stock"
	SupplyAvailabilityUnlimited            SupplyAvailabilityStatus = "unlimited"
	SupplyAvailabilityManualActionRequired SupplyAvailabilityStatus = "manual_action_required"
	SupplyAvailabilitySupplierUnavailable  SupplyAvailabilityStatus = "supplier_unavailable"
)

// SupplyReservationStatus describes the lifecycle of a hold created by
// ReserveOrder or a provider Reserve call.
type SupplyReservationStatus string

const (
	SupplyReservationNoop      SupplyReservationStatus = "noop"
	SupplyReservationReserved  SupplyReservationStatus = "reserved"
	SupplyReservationCommitted SupplyReservationStatus = "committed"
	SupplyReservationReleased  SupplyReservationStatus = "released"
	SupplyReservationFailed    SupplyReservationStatus = "failed"
)

// SupplyLine identifies the sellable supply bucket for one checkout/order line.
// A product may have multiple delivery assets; this line points only to the
// provider that controls scarcity for the line.
type SupplyLine struct {
	LineID       string            `json:"lineID,omitempty"`
	ListingSlug  string            `json:"listingSlug"`
	VariantHash  string            `json:"variantHash,omitempty"`
	VariantSKU   string            `json:"variantSKU,omitempty"`
	Quantity     int               `json:"quantity"`
	SupplyKind   SupplyKind        `json:"supplyKind"`
	StockTracked bool              `json:"stockTracked,omitempty"`
	StockLimit   int64             `json:"stockLimit,omitempty"`
	ProviderID   string            `json:"providerID,omitempty"`
	ProviderRef  string            `json:"providerRef,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// DigitalOrderLineItem is the channel-neutral input used to resolve digital
// goods into supply lines.
type DigitalOrderLineItem struct {
	ListingSlug string
	VariantSKU  string
	Quantity    uint32
}

// DigitalSupplyLineResolver maps digital order metadata to the provider-neutral
// supply contract shared by standard orders, guest checkout, and quote flows.
type DigitalSupplyLineResolver interface {
	SupplyAvailabilityLinesForOrderItems([]DigitalOrderLineItem) ([]SupplyLine, error)
}

// AvailabilityRequest asks a single provider for advisory availability.
type AvailabilityRequest struct {
	Line        SupplyLine `json:"line"`
	BuyerPeerID string     `json:"buyerPeerID,omitempty"`
	CheckedAt   time.Time  `json:"checkedAt,omitempty"`
}

// AvailabilityResult is an advisory response for buyer/admin display and
// preflight checks. It must be re-checked during Reserve.
type AvailabilityResult struct {
	LineID               string                   `json:"lineID,omitempty"`
	SupplyKind           SupplyKind               `json:"supplyKind"`
	Status               SupplyAvailabilityStatus `json:"status"`
	Available            bool                     `json:"available"`
	Unlimited            bool                     `json:"unlimited,omitempty"`
	AvailableQuantity    int64                    `json:"availableQuantity,omitempty"`
	ManualActionRequired bool                     `json:"manualActionRequired,omitempty"`
	Reason               string                   `json:"reason,omitempty"`
	ProviderID           string                   `json:"providerID,omitempty"`
	ProviderRef          string                   `json:"providerRef,omitempty"`
	CheckedAt            time.Time                `json:"checkedAt,omitempty"`
}

// ReserveSupplyRequest asks a provider to create an authoritative hold.
type ReserveSupplyRequest struct {
	OrderRef    string     `json:"orderRef"`
	OrderType   string     `json:"orderType"`
	Line        SupplyLine `json:"line"`
	BuyerPeerID string     `json:"buyerPeerID,omitempty"`
	ExpiresAt   time.Time  `json:"expiresAt"`
}

// SupplyReservation is the provider-neutral representation of a supply hold.
type SupplyReservation struct {
	ID             string                  `json:"id,omitempty"`
	OrderRef       string                  `json:"orderRef"`
	OrderType      string                  `json:"orderType"`
	LineID         string                  `json:"lineID,omitempty"`
	SupplyKind     SupplyKind              `json:"supplyKind"`
	ListingSlug    string                  `json:"listingSlug"`
	VariantHash    string                  `json:"variantHash,omitempty"`
	VariantSKU     string                  `json:"variantSKU,omitempty"`
	Quantity       int                     `json:"quantity"`
	ReservationRef string                  `json:"reservationRef,omitempty"`
	Status         SupplyReservationStatus `json:"status"`
	ExpiresAt      time.Time               `json:"expiresAt,omitempty"`
	CommittedAt    time.Time               `json:"committedAt,omitempty"`
	ReleasedAt     time.Time               `json:"releasedAt,omitempty"`
	Reason         string                  `json:"reason,omitempty"`
}

type CommitSupplyRequest struct {
	OrderRef       string   `json:"orderRef"`
	OrderType      string   `json:"orderType"`
	ReservationIDs []string `json:"reservationIDs,omitempty"`
}

type ReleaseSupplyRequest struct {
	OrderRef       string   `json:"orderRef"`
	OrderType      string   `json:"orderType"`
	ReservationIDs []string `json:"reservationIDs,omitempty"`
	Reason         string   `json:"reason,omitempty"`
}

type SupplyQuoteRequest struct {
	OrderRef    string       `json:"orderRef,omitempty"`
	OrderType   string       `json:"orderType,omitempty"`
	BuyerPeerID string       `json:"buyerPeerID,omitempty"`
	Lines       []SupplyLine `json:"lines"`
}

type SupplyQuoteResult struct {
	Results              []AvailabilityResult `json:"results"`
	CanSell              bool                 `json:"canSell"`
	ManualActionRequired bool                 `json:"manualActionRequired,omitempty"`
	Reason               string               `json:"reason,omitempty"`
}

type ReserveOrderSupplyRequest struct {
	OrderRef    string       `json:"orderRef"`
	OrderType   string       `json:"orderType"`
	BuyerPeerID string       `json:"buyerPeerID,omitempty"`
	Lines       []SupplyLine `json:"lines"`
	ExpiresAt   time.Time    `json:"expiresAt"`
}

type ReserveOrderSupplyResult struct {
	Reservations         []SupplyReservation `json:"reservations"`
	ManualActionRequired bool                `json:"manualActionRequired,omitempty"`
	Reason               string              `json:"reason,omitempty"`
}

// SupplyProvider controls availability and reservations for one SupplyKind.
// It must not take over fulfillment or digital entitlement delivery.
type SupplyProvider interface {
	Kind() SupplyKind
	GetAvailability(ctx context.Context, req AvailabilityRequest) (*AvailabilityResult, error)
	Reserve(ctx context.Context, req ReserveSupplyRequest) (*SupplyReservation, error)
	Commit(ctx context.Context, req CommitSupplyRequest) error
	Release(ctx context.Context, req ReleaseSupplyRequest) error
}

// PartitionReservableSupplyLines splits lines that can receive an authoritative
// hold from external supply lines that must stay manual-action until the
// provider supports real external holds.
func PartitionReservableSupplyLines(lines []SupplyLine) (reservable []SupplyLine, manualAction []SupplyLine) {
	if len(lines) == 0 {
		return nil, nil
	}
	reservable = make([]SupplyLine, 0, len(lines))
	manualAction = make([]SupplyLine, 0)
	for _, line := range lines {
		if line.SupplyKind == SupplyKindExternalSupply {
			manualAction = append(manualAction, line)
			continue
		}
		reservable = append(reservable, line)
	}
	return reservable, manualAction
}

// SupplyAvailabilityService is the order-facing aggregate boundary. Quote is
// advisory; ReserveOrder is the authoritative availability check and hold.
type SupplyAvailabilityService interface {
	Quote(ctx context.Context, req SupplyQuoteRequest) (*SupplyQuoteResult, error)
	ReserveOrder(ctx context.Context, req ReserveOrderSupplyRequest) (*ReserveOrderSupplyResult, error)
	CommitOrder(ctx context.Context, orderRef string, orderType string) error
	ReleaseOrder(ctx context.Context, orderRef string, orderType string, reason string) error
}
