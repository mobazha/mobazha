package contracts

import (
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
	OrderToken     string                  `json:"orderToken"`
	PaymentAddress string                  `json:"paymentAddress"`
	PaymentAmount  string                  `json:"paymentAmount"`
	PaymentCoin    string                  `json:"paymentCoin"`
	ReferenceKey   string                  `json:"referenceKey,omitempty"`
	ExpiresAt      time.Time               `json:"expiresAt"`
	Items          []models.GuestOrderItem `json:"items"`
	Subtotal       uint64                  `json:"subtotal"`
	ShippingCost   uint64                  `json:"shippingCost"`
	TotalPrice     uint64                  `json:"totalPrice"`
	PriceCurrency  string                  `json:"priceCurrency"`
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
