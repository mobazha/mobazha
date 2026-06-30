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

// PrivateDistributionPolicy contains product decisions while Core retains the listing and
// order state machines that enforce them.
type PrivateDistributionPolicy interface {
	ListingPolicy
	GuestPaymentPolicy
	EnabledBackgroundJobs() []string
}
