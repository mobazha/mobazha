package payment

import (
	"fmt"
	"strings"
)

const (
	SettlementActionConfirm        = "confirm"
	SettlementActionCancel         = "cancel"
	SettlementActionComplete       = "complete"
	SettlementActionDisputeRelease = "dispute_release"
)

// SettlementActionPathHint documents supported URL path segments for settlement-actions.
const SettlementActionPathHint = `"confirm", "cancel", "complete", or "dispute-release"`

// ParseSettlementActionPath maps kebab-case URL segments to internal action names.
func ParseSettlementActionPath(raw string) (string, error) {
	action := strings.ToLower(strings.TrimSpace(raw))
	switch action {
	case SettlementActionConfirm, SettlementActionCancel, SettlementActionComplete:
		return action, nil
	case "dispute-release":
		return SettlementActionDisputeRelease, nil
	default:
		return "", fmt.Errorf("action must be %s", SettlementActionPathHint)
	}
}

// SettlementActionPathSegment maps internal action names to kebab-case URL segments.
func SettlementActionPathSegment(action string) string {
	if action == SettlementActionDisputeRelease {
		return "dispute-release"
	}
	return action
}
