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
	// ReminderBeforeDays: send countdown reminders at these remaining-day marks.
	ReminderBeforeDays []int
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
		ReminderBeforeDays:        []int{3, 1},
	},
	pb.Listing_Metadata_DIGITAL_GOOD: {
		AutoCompleteAfterShipDays: 3,
		MaxFulfillDays:            3,
		AfterSaleWindowDays:       7,
		ExtendProtectionDays:      0,
		DisputeNegotiationDays:    7,
		DisputeResolutionDays:     7,
		ReminderBeforeDays:        []int{1},
	},
	pb.Listing_Metadata_SERVICE: {
		AutoCompleteAfterShipDays: 7,
		MaxFulfillDays:            3,
		AfterSaleWindowDays:       7,
		ExtendProtectionDays:      0,
		DisputeNegotiationDays:    7,
		DisputeResolutionDays:     7,
		ReminderBeforeDays:        []int{1},
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

// Protection stage constants returned by ComputeProtection.
const (
	ProtectionStageEscrowed         = "ESCROWED"
	ProtectionStageProtectionPeriod = "PROTECTION_PERIOD"
	ProtectionStageCompleted        = "COMPLETED"
	ProtectionStageDisputed         = "DISPUTED"
	ProtectionStageAfterSaleWindow  = "AFTER_SALE_WINDOW"
)

// OrderProtectionInfo is a derived (non-persisted) view of buyer-protection
// status, computed at API response time from order state + timestamps + policy.
type OrderProtectionInfo struct {
	Stage              string     `json:"stage"`
	DaysRemaining      int        `json:"daysRemaining"`
	AutoCompleteAt     *time.Time `json:"autoCompleteAt,omitempty"`
	Extendable         bool       `json:"extendable"`
	Extended           bool       `json:"extended"`
	AfterSaleWindowDays int       `json:"afterSaleWindowDays"`
}

// ComputeProtection derives the buyer-protection status for the order.
// Returns nil for states where protection is not applicable (PENDING,
// AWAITING_PAYMENT, CANCELED, DECLINED, REFUNDED, etc.).
func (o *Order) ComputeProtection(now time.Time) *OrderProtectionInfo {
	policy := DefaultProtectionPolicy(o.ContractType())

	switch o.State {
	case OrderState_AWAITING_FULFILLMENT:
		return &OrderProtectionInfo{
			Stage:               ProtectionStageEscrowed,
			DaysRemaining:       0,
			Extendable:          false,
			Extended:            false,
			AfterSaleWindowDays: policy.AfterSaleWindowDays,
		}

	case OrderState_FULFILLED:
		if o.FulfilledAt == nil {
			return &OrderProtectionInfo{
				Stage:               ProtectionStageProtectionPeriod,
				DaysRemaining:       policy.AutoCompleteAfterShipDays,
				Extendable:          policy.ExtendProtectionDays > 0,
				Extended:            false,
				AfterSaleWindowDays: policy.AfterSaleWindowDays,
			}
		}
		deadline := o.FulfilledAt.Add(policy.AutoCompleteDuration())
		daysLeft := daysUntil(now, deadline)
		return &OrderProtectionInfo{
			Stage:               ProtectionStageProtectionPeriod,
			DaysRemaining:       daysLeft,
			AutoCompleteAt:      &deadline,
			Extendable:          policy.ExtendProtectionDays > 0,
			Extended:            false,
			AfterSaleWindowDays: policy.AfterSaleWindowDays,
		}

	case OrderState_COMPLETED, OrderState_PAYMENT_FINALIZED:
		if o.CompletedAt != nil {
			afterSaleEnd := o.CompletedAt.Add(policy.AfterSaleWindowDuration())
			if now.Before(afterSaleEnd) {
				return &OrderProtectionInfo{
					Stage:               ProtectionStageAfterSaleWindow,
					DaysRemaining:       daysUntil(now, afterSaleEnd),
					AfterSaleWindowDays: policy.AfterSaleWindowDays,
				}
			}
		}
		return &OrderProtectionInfo{
			Stage:               ProtectionStageCompleted,
			DaysRemaining:       0,
			AfterSaleWindowDays: policy.AfterSaleWindowDays,
		}

	case OrderState_DISPUTED, OrderState_DECIDED:
		info := &OrderProtectionInfo{
			Stage:               ProtectionStageDisputed,
			DaysRemaining:       0,
			AfterSaleWindowDays: policy.AfterSaleWindowDays,
		}
		if o.FulfilledAt != nil {
			deadline := o.FulfilledAt.Add(policy.AutoCompleteDuration())
			daysLeft := daysUntil(now, deadline)
			info.DaysRemaining = daysLeft
			info.AutoCompleteAt = &deadline
		}
		return info

	default:
		return nil
	}
}

// daysUntil returns the number of whole days remaining until deadline.
// Returns 0 if the deadline has already passed.
func daysUntil(now, deadline time.Time) int {
	remaining := deadline.Sub(now)
	if remaining <= 0 {
		return 0
	}
	days := int(remaining.Hours() / 24)
	if remaining > time.Duration(days)*24*time.Hour {
		days++
	}
	return days
}
