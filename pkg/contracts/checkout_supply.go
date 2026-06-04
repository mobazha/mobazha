package contracts

// CheckoutSupplyItemRequest describes one checkout line for advisory supply preflight.
type CheckoutSupplyItemRequest struct {
	ListingSlug string `json:"listingSlug"`
	// ListingHash is echoed from the client for correlation only. Advisory quote
	// resolution uses ListingSlug (and Options) on the seller node; hash mismatch
	// is not validated at quote time.
	ListingHash     string              `json:"listingHash"`
	Quantity        int                 `json:"quantity"`
	Options         []map[string]string `json:"options,omitempty"`
	ShippingOption  string              `json:"shippingOption,omitempty"`
	ShippingService string              `json:"shippingService,omitempty"`
}

// QuoteCheckoutSupplyRequest is the authenticated standard-checkout supply preflight input.
type QuoteCheckoutSupplyRequest struct {
	Items []CheckoutSupplyItemRequest `json:"items"`
}

// CheckoutSupplyQuoteResponse is a buyer-safe advisory supply preflight response.
// It intentionally omits provider identifiers and provider references.
type CheckoutSupplyQuoteResponse struct {
	Items                []CheckoutSupplyQuoteItem `json:"items"`
	CanSell              bool                      `json:"canSell"`
	ManualActionRequired bool                      `json:"manualActionRequired,omitempty"`
	Reason               string                    `json:"reason,omitempty"`
}

// CheckoutSupplyQuoteItem is one buyer-visible line in a checkout supply quote.
type CheckoutSupplyQuoteItem struct {
	ListingSlug          string                   `json:"listingSlug"`
	Quantity             int                      `json:"quantity"`
	SupplyKind           SupplyKind               `json:"-"` // buyer-safe API: internal routing only
	Status               SupplyAvailabilityStatus `json:"status"`
	Available            bool                     `json:"available"`
	Unlimited            bool                     `json:"unlimited,omitempty"`
	AvailableQuantity    int64                    `json:"availableQuantity,omitempty"`
	ManualActionRequired bool                     `json:"manualActionRequired,omitempty"`
	Reason               string                   `json:"reason,omitempty"`
}
