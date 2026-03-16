package models

import (
	"time"

	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
)

// OrderProtectionPolicy defines buyer-protection timeout parameters for a
// specific contract type. All durations are expressed in calendar days.
type OrderProtectionPolicy struct {
	// AutoCompleteAfterShipDays: days after fulfillment before auto-complete.
	AutoCompleteAfterShipDays int
	// MaxFulfillDays: days after payment before auto-refund if unfulfilled.
	MaxFulfillDays int
	// AfterSaleWindowDays: dispute window after auto-complete.
	AfterSaleWindowDays int
	// ExtendProtectionDays: buyer may extend protection (physical goods only; 0 = not allowed).
	ExtendProtectionDays int
	// DisputeNegotiationDays: seller-buyer negotiation period once dispute opened.
	DisputeNegotiationDays int
	// DisputeResolutionDays: platform arbitration period.
	DisputeResolutionDays int
}

// AutoCompleteDuration returns AutoCompleteAfterShipDays as time.Duration.
func (p OrderProtectionPolicy) AutoCompleteDuration() time.Duration {
	return time.Duration(p.AutoCompleteAfterShipDays) * 24 * time.Hour
}

// MaxFulfillDuration returns MaxFulfillDays as time.Duration.
func (p OrderProtectionPolicy) MaxFulfillDuration() time.Duration {
	return time.Duration(p.MaxFulfillDays) * 24 * time.Hour
}

// AfterSaleWindowDuration returns AfterSaleWindowDays as time.Duration.
func (p OrderProtectionPolicy) AfterSaleWindowDuration() time.Duration {
	return time.Duration(p.AfterSaleWindowDays) * 24 * time.Hour
}

var defaultProtectionPolicies = map[pb.Listing_Metadata_ContractType]OrderProtectionPolicy{
	pb.Listing_Metadata_PHYSICAL_GOOD: {
		AutoCompleteAfterShipDays: 14,
		MaxFulfillDays:            7,
		AfterSaleWindowDays:       7,
		ExtendProtectionDays:      14,
		DisputeNegotiationDays:    7,
		DisputeResolutionDays:     7,
	},
	pb.Listing_Metadata_DIGITAL_GOOD: {
		AutoCompleteAfterShipDays: 3,
		MaxFulfillDays:            3,
		AfterSaleWindowDays:       7,
		ExtendProtectionDays:      0,
		DisputeNegotiationDays:    7,
		DisputeResolutionDays:     7,
	},
	pb.Listing_Metadata_SERVICE: {
		AutoCompleteAfterShipDays: 7,
		MaxFulfillDays:            3,
		AfterSaleWindowDays:       7,
		ExtendProtectionDays:      0,
		DisputeNegotiationDays:    7,
		DisputeResolutionDays:     7,
	},
}

// DefaultProtectionPolicy returns the buyer-protection policy for the given
// contract type. Unknown types fall back to the PHYSICAL_GOOD defaults.
func DefaultProtectionPolicy(ct pb.Listing_Metadata_ContractType) OrderProtectionPolicy {
	if p, ok := defaultProtectionPolicies[ct]; ok {
		return p
	}
	return defaultProtectionPolicies[pb.Listing_Metadata_PHYSICAL_GOOD]
}
