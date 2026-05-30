package wallet_interface

import "strings"

// IsPaymentCoinEnabled reports whether a canonical or legacy payment coin is
// enabled for product checkout flows.
func IsPaymentCoinEnabled(raw string) bool {
	coin := strings.TrimSpace(raw)
	if coin == "" {
		return true
	}
	if normalized, ok := TryNormalizePaymentCoin(coin); ok {
		coin = string(normalized)
	}
	return !strings.HasPrefix(strings.ToLower(coin), "crypto:zcash:")
}
