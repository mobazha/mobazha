package contracttype

import (
	"fmt"

	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
)

// AddToSingleTypeOrder records the first listing contract type for an order and
// reports whether a later line item still matches it.
func AddToSingleTypeOrder(
	current pb.Listing_Metadata_ContractType,
	hasCurrent bool,
	next pb.Listing_Metadata_ContractType,
) (pb.Listing_Metadata_ContractType, bool, bool) {
	if !hasCurrent {
		return next, true, true
	}
	return current, true, current == next
}

func MixedOrderTypeMessage(
	first pb.Listing_Metadata_ContractType,
	next pb.Listing_Metadata_ContractType,
	listingRef string,
) string {
	if listingRef == "" {
		return fmt.Sprintf(
			"mixed contract types are not supported in one order (first %s, next %s); split checkout by product type",
			first.String(),
			next.String(),
		)
	}
	return fmt.Sprintf(
		"mixed contract types are not supported in one order (first %s, item %q is %s); split checkout by product type",
		first.String(),
		listingRef,
		next.String(),
	)
}
