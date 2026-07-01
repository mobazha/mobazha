package distribution

import (
	"github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

type ListingPolicy interface {
	ValidateListingPricingCurrency(code string) error
	ValidateListingFormat(format mbzpb.Listing_Metadata_Format, contractType mbzpb.Listing_Metadata_ContractType) error
}

type GuestPaymentPolicy interface {
	SupportsGuestPaymentCoin(coin iwallet.CoinType) bool
	ValidateGuestPaymentCoin(coin iwallet.CoinType) error
	AdvertisedPaymentCoins() []iwallet.CoinType
	ValidateCrossCurrencyCheckout(pricingCurrency, paymentSymbol string) error
}

const (
	MCPToolCatalogFull       = "full"
	MCPToolCatalogRestricted = "restricted"
	CoreAPISurfaceFull       = "full"
	CoreAPISurfaceRestricted = "restricted"
)

// ProductSurfacePolicy owns distribution-level API decisions that cannot be
// inferred from a chain or payment capability alone.
type ProductSurfacePolicy interface {
	ExternalExchangeRatesEnabled() bool
	MCPToolCatalog() string
	CoreAPISurface() string
}

// SovereignNodePolicy contains local-first product decisions while Core
// retains the listing and order state machines that enforce them.
type SovereignNodePolicy interface {
	ListingPolicy
	GuestPaymentPolicy
	ProductSurfacePolicy
	AIHTTPPolicy
	EnabledBackgroundJobs() []string
}
