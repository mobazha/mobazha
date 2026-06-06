package payment

import (
	"fmt"
	"strings"
)

const (
	SettlementActionConfirm             = "confirm"
	SettlementActionCancel              = "cancel"
	SettlementActionSellerDeclineRefund = "seller_decline_refund"
	SettlementActionComplete            = "complete"
	SettlementActionDisputeRelease      = "dispute_release"
)

// SettlementActionPathHint documents supported URL path segments for settlement-actions.
const SettlementActionPathHint = `"confirm", "cancel", "seller-decline-refund", "complete", or "dispute-release"`

// ParseSettlementActionPath maps kebab-case URL segments to internal action names.
func ParseSettlementActionPath(raw string) (string, error) {
	action := strings.ToLower(strings.TrimSpace(raw))
	switch action {
	case SettlementActionConfirm, SettlementActionCancel, SettlementActionComplete:
		return action, nil
	case "seller-decline-refund":
		return SettlementActionSellerDeclineRefund, nil
	case "dispute-release":
		return SettlementActionDisputeRelease, nil
	default:
		return "", fmt.Errorf("action must be %s", SettlementActionPathHint)
	}
}

// SettlementActionPathSegment maps internal action names to kebab-case URL segments.
func SettlementActionPathSegment(action string) string {
	switch action {
	case SettlementActionDisputeRelease:
		return "dispute-release"
	case SettlementActionSellerDeclineRefund:
		return "seller-decline-refund"
	}
	return action
}
