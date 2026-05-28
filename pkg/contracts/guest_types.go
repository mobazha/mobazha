package contracts

import (
	"context"
	"errors"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/models"
)

// Sentinel errors for guest checkout. Service implementations wrap these
// (via fmt.Errorf("%w: ...", ...)) and HTTP handlers route status codes via
// errors.Is — substring-matching error messages is fragile because the
// human-readable suffix often shares words across roots.
var (
	// ErrGuestCheckoutDisabled — guestCheckout feature flag is disabled.
	// Maps to HTTP 403 FEATURE_DISABLED.
	ErrGuestCheckoutDisabled = errors.New("guest checkout is disabled")

	// ErrCoinUnavailable — payment coin's chain monitor is not configured
	// in this deployment. The request itself is well-formed; the operator
	// just hasn't enabled this chain. Maps to HTTP 503 SERVICE_UNAVAILABLE.
	ErrCoinUnavailable = errors.New("payment coin unavailable")

	// ErrCoinUnsupported — payment coin doesn't belong to any chain
	// family supported by guest checkout. Maps to HTTP 400.
	ErrCoinUnsupported = errors.New("payment coin not supported")

	// ErrInsufficientStock — listing inventory cannot satisfy the order.
	// Maps to HTTP 409 CONFLICT.
	ErrInsufficientStock = errors.New("insufficient stock")

	// ErrInvalidVariant — buyer's variant options don't match any SKU on the
	// listing. The listing is valid; the option combination isn't.
	// Maps to HTTP 400 BAD_REQUEST.
	ErrInvalidVariant = errors.New("invalid variant options")

	// ErrInvalidGuestRequest — request validation failure (bad amounts,
	// unknown listing slug, mixed pricing, etc). Maps to HTTP 400.
	ErrInvalidGuestRequest = errors.New("invalid guest order request")
)

// CreateGuestOrderRequest is the input for creating a Guest Order.
type CreateGuestOrderRequest struct {
	Items           []GuestOrderItemRequest `json:"items"`
	ShippingAddress interface{}             `json:"shippingAddress,omitempty"`
	ContactEmail    string                  `json:"contactEmail,omitempty"`
	PaymentCoin     string                  `json:"paymentCoin"`
}

// GuestOrderItemRequest describes a single item in a guest order.
type GuestOrderItemRequest struct {
	ListingSlug     string              `json:"listingSlug"`
	ListingHash     string              `json:"listingHash"`
	Quantity        int                 `json:"quantity"`
	Options         []map[string]string `json:"options,omitempty"`
	ShippingOption  string              `json:"shippingOption,omitempty"`
	ShippingService string              `json:"shippingService,omitempty"`
}

// GuestOrderResponse is returned after order creation.
type GuestOrderResponse struct {
	OrderToken        string                  `json:"orderToken"`
	BuyerPortalToken  string                  `json:"buyerPortalToken,omitempty"`
	PaymentAddress    string                  `json:"paymentAddress"`
	PaymentAmount     string                  `json:"paymentAmount"`
	PaymentCoin       string                  `json:"paymentCoin"`
	ReferenceKey      string                  `json:"referenceKey,omitempty"`
	ExpiresAt         time.Time               `json:"expiresAt"`
	Items             []models.GuestOrderItem `json:"items"`
	Subtotal          uint64                  `json:"subtotal"`
	ShippingCost      uint64                  `json:"shippingCost"`
	TotalPrice        uint64                  `json:"totalPrice"`
	PriceCurrency     string                  `json:"priceCurrency"`
	PriceDivisibility uint32                  `json:"priceDivisibility"`
}

// GuestOrderStatusResponse is the public status for a guest order.
type GuestOrderStatusResponse struct {
	OrderToken        string                  `json:"orderToken"`
	State             string                  `json:"state"`
	PaymentAddress    string                  `json:"paymentAddress"`
	PaymentAmount     string                  `json:"paymentAmount"`
	TotalReceived     string                  `json:"totalReceived,omitempty"`
	OverpaidAmount    string                  `json:"overpaidAmount,omitempty"`
	PaymentCoin       string                  `json:"paymentCoin"`
	ReferenceKey      string                  `json:"referenceKey,omitempty"`
	Confirmations     int                     `json:"confirmations"`
	RequiredConfs     int                     `json:"requiredConfs"`
	ChainBlockTimeSec uint32                  `json:"chainBlockTimeSec,omitempty"`
	TrackingNumber    string                  `json:"trackingNumber,omitempty"`
	ShippingCarrier   string                  `json:"shippingCarrier,omitempty"`
	SellerPeerID      string                  `json:"sellerPeerID,omitempty"`
	Items             []models.GuestOrderItem `json:"items"`
	ExpiresAt         time.Time               `json:"expiresAt"`
	CreatedAt         time.Time               `json:"createdAt"`
	UpdatedAt         time.Time               `json:"updatedAt"`
	FundedAt          *time.Time              `json:"fundedAt,omitempty"`
	ShippedAt         *time.Time              `json:"shippedAt,omitempty"`
	CompletedAt       *time.Time              `json:"completedAt,omitempty"`
	// Pool-stage UX hint (currently EXTERNAL_PAYMENT only). When PoolDetected is true,
	// a mempool transfer has been observed but is not yet mined; the order
	// is still in AWAITING_PAYMENT and may transition to PAYMENT_DETECTED
	// (if mined) or EXPIRED (if evicted / never mined). Frontends use this
	// to show "we saw your payment, waiting for confirmation" instead of a
	// blank "waiting for payment" screen.
	PoolDetected   bool       `json:"poolDetected,omitempty"`
	PoolTxHash     string     `json:"poolTxHash,omitempty"`
	PoolAmount     uint64     `json:"poolAmount,omitempty"`
	PoolDetectedAt *time.Time `json:"poolDetectedAt,omitempty"`

	// Pricing in listing currency (denormalized from GuestOrder model).
	PriceCurrency     string `json:"priceCurrency,omitempty"`
	PriceDivisibility uint32 `json:"priceDivisibility,omitempty"`
	Subtotal          uint64 `json:"subtotal,omitempty"`
	ShippingCost      uint64 `json:"shippingCost,omitempty"`
	TotalPrice        uint64 `json:"totalPrice,omitempty"`

	// On-chain transaction hash (set when payment is mined / detected).
	PaymentTxHash string `json:"txHash,omitempty"`
}

// GuestOrderFilter for listing guest orders.
type GuestOrderFilter struct {
	State    *models.GuestOrderState
	Page     int
	PageSize int
}

// GuestUTXOChainReadiness is per-chain guest UTXO closure status for operators.
type GuestUTXOChainReadiness struct {
	Chain                  string `json:"chain"`
	HealthySourceCount     int    `json:"healthySourceCount"`
	WalletLoaded           bool   `json:"walletLoaded"`
	ReceivingAccountActive bool   `json:"receivingAccountActive"`
	BuyerVisible           bool   `json:"buyerVisible"`
	Reason                 string `json:"reason,omitempty"`
}

// GuestEVMChainReadiness is per-chain guest EVM ManagedEscrow closure status for operators.
type GuestEVMChainReadiness struct {
	Chain                  string `json:"chain"`
	Coin                   string `json:"coin"`
	FundingReady           bool   `json:"fundingReady"`
	ObservationReady       bool   `json:"observationReady"`
	SettlementReady        bool   `json:"settlementReady"`
	RelayReady             bool   `json:"relayReady"`
	RelayGasHealthy        bool   `json:"relayGasHealthy"`
	RelayGasReason         string `json:"relayGasReason,omitempty"`
	ManagedEscrowMonitorActive      bool   `json:"managed_escrowMonitorActive"`
	ReceivingAccountActive bool   `json:"receivingAccountActive"`
	BuyerVisible           bool   `json:"buyerVisible"`
	Reason                 string `json:"reason,omitempty"`
}

// GuestCheckoutReadiness summarizes guest checkout runtime health.
type GuestCheckoutReadiness struct {
	GuestCheckoutEnabled bool                      `json:"guestCheckoutEnabled"`
	WatchedAddressCount  int                       `json:"watchedAddressCount"`
	SweepTasksPending    int                       `json:"sweepTasksPending"`
	SweepTasksSubmitted  int                       `json:"sweepTasksSubmitted"`
	SweepTasksFailed     int                       `json:"sweepTasksFailed"`
	Chains               []GuestUTXOChainReadiness `json:"chains"`
	EVMChains            []GuestEVMChainReadiness  `json:"evmChains,omitempty"`
}

// --- UnifiedOrderView types ---

// OrderSummary is a normalized summary suitable for the seller's unified order list.
type OrderSummary struct {
	ID                 string       `json:"id"`
	Type               string       `json:"type"`
	State              string       `json:"state"`
	BuyerName          string       `json:"buyerName"`
	Items              []ItemBrief  `json:"items"`
	Total              PriceSummary `json:"total"`
	PaymentCoin        string       `json:"paymentCoin"`
	SettlementAction   string       `json:"settlementAction,omitempty"`
	SettlementActionID string       `json:"settlementActionId,omitempty"`
	SettlementState    string       `json:"settlementState,omitempty"`
	SettlementTxHash   string       `json:"settlementTxHash,omitempty"`
	TrackingNumber     string       `json:"trackingNumber,omitempty"`
	SweepStatus        string       `json:"sweepStatus,omitempty"`
	CreatedAt          time.Time    `json:"createdAt"`
	UpdatedAt          time.Time    `json:"updatedAt"`
}

// ItemBrief is a minimal item summary for the unified list.
type ItemBrief struct {
	Title    string `json:"title"`
	Quantity int    `json:"quantity"`
}

// PriceSummary carries a price with currency metadata.
type PriceSummary struct {
	Amount       string `json:"amount"`
	CurrencyCode string `json:"currencyCode"`
	Divisibility uint32 `json:"divisibility"`
}

// OrderListFilter controls the unified order list query.
type OrderListFilter struct {
	View     string `json:"view"` // "all" | "standard" | "guest"
	State    string `json:"state,omitempty"`
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
}

// OrderListMeta carries pagination metadata for the unified list.
type OrderListMeta struct {
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
}

// UnifiedOrderViewService provides a merged view of standard + guest orders.
type UnifiedOrderViewService interface {
	ListOrders(ctx context.Context, filter OrderListFilter) ([]OrderSummary, *OrderListMeta, error)
}

// UnifiedOrderViewProvider is satisfied by NodeService implementations
// that expose the unified order view.
type UnifiedOrderViewProvider interface {
	UnifiedOrders() UnifiedOrderViewService
}
