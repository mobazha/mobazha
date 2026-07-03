package guest

import (
	"strings"

	"github.com/mobazha/mobazha/pkg/models"
)

// ContractTypeFromItems returns the persisted order-level contract type.
// An empty result means at least one line item did not persist contractType.
func ContractTypeFromItems(items []models.GuestOrderItem) string {
	if len(items) == 0 {
		return ""
	}
	allDigital := true
	firstNonDigital := ""
	for _, it := range items {
		contractType := strings.TrimSpace(it.ContractType)
		if contractType == "" {
			return ""
		}
		if contractType != "DIGITAL_GOOD" {
			allDigital = false
			if firstNonDigital == "" {
				firstNonDigital = contractType
			}
		}
	}
	if allDigital {
		return "DIGITAL_GOOD"
	}
	return firstNonDigital
}
