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

// ListingSupplyMode is the seller-safe supply mode shown in admin surfaces.
// It intentionally avoids exposing provider IDs, provider refs, or internal
// routing names to buyer-facing APIs.
type ListingSupplyMode string

const (
	ListingSupplyModeUnknown           ListingSupplyMode = "unknown"
	ListingSupplyModeTrackedStock      ListingSupplyMode = "tracked_stock"
	ListingSupplyModeLicenseCodes      ListingSupplyMode = "license_codes"
	ListingSupplyModeInstantDownload   ListingSupplyMode = "instant_download"
	ListingSupplyModeSupplierFulfilled ListingSupplyMode = "supplier_fulfilled"
)

// ListingSupplySummaryRequest asks for seller-visible supply summaries.
// When Slugs is empty, the seller's own listing index is paginated.
type ListingSupplySummaryRequest struct {
	Slugs  []string `json:"slugs,omitempty"`
	Limit  int      `json:"limit,omitempty"`
	Offset int      `json:"offset,omitempty"`
}

// ListingSupplySummaryResponse returns paginated seller-safe availability
// summaries suitable for admin product lists.
type ListingSupplySummaryResponse struct {
	Items  []ListingSupplySummaryItem `json:"items"`
	Limit  int                        `json:"limit"`
	Offset int                        `json:"offset"`
	Total  int                        `json:"total"`
}

// ListingSupplySummaryItem is one admin-visible listing supply summary.
type ListingSupplySummaryItem struct {
	ListingSlug          string                   `json:"listingSlug"`
	SupplyMode           ListingSupplyMode        `json:"supplyMode"`
	Status               SupplyAvailabilityStatus `json:"status"`
	AvailableQuantity    *int64                   `json:"availableQuantity,omitempty"`
	OnHandQuantity       *int64                   `json:"onHandQuantity,omitempty"`
	HeldQuantity         *int64                   `json:"heldQuantity,omitempty"`
	ManualActionRequired bool                     `json:"manualActionRequired,omitempty"`
	Reason               string                   `json:"reason,omitempty"`
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
