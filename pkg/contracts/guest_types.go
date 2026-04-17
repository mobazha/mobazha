package contracts

import (
	"context"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/models"
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
	PaymentAddress    string                  `json:"paymentAddress"`
	PaymentAmount     string                  `json:"paymentAmount"`
	PaymentCoin       string                  `json:"paymentCoin"`
	ReferenceKey      string                  `json:"referenceKey,omitempty"`
	ExpiresAt         time.Time               `json:"expiresAt"`
	Items             []models.GuestOrderItem `json:"items"`
	Subtotal          uint64                  `json:"subtotal"`
	PriceCurrency     string                  `json:"priceCurrency"`
	PriceDivisibility uint32                  `json:"priceDivisibility"`
}

// GuestOrderStatusResponse is the public status for a guest order.
type GuestOrderStatusResponse struct {
	OrderToken     string                  `json:"orderToken"`
	State          string                  `json:"state"`
	PaymentAddress string                  `json:"paymentAddress"`
	PaymentAmount  string                  `json:"paymentAmount"`
	PaymentCoin    string                  `json:"paymentCoin"`
	ReferenceKey   string                  `json:"referenceKey,omitempty"`
	Confirmations  int                     `json:"confirmations"`
	RequiredConfs  int                     `json:"requiredConfs"`
	TrackingNumber string                  `json:"trackingNumber,omitempty"`
	Items          []models.GuestOrderItem `json:"items"`
	ExpiresAt      time.Time               `json:"expiresAt"`
	CreatedAt      time.Time               `json:"createdAt"`
}

// GuestOrderFilter for listing guest orders.
type GuestOrderFilter struct {
	State    *models.GuestOrderState
	Page     int
	PageSize int
}

// --- UnifiedOrderView types ---

// OrderSummary is a normalized summary suitable for the seller's unified order list.
type OrderSummary struct {
	ID             string       `json:"id"`
	Type           string       `json:"type"`
	State          string       `json:"state"`
	BuyerName      string       `json:"buyerName"`
	Items          []ItemBrief  `json:"items"`
	Total          PriceSummary `json:"total"`
	PaymentCoin    string       `json:"paymentCoin"`
	TrackingNumber string       `json:"trackingNumber,omitempty"`
	SweepStatus    string       `json:"sweepStatus,omitempty"`
	CreatedAt      time.Time    `json:"createdAt"`
	UpdatedAt      time.Time    `json:"updatedAt"`
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
