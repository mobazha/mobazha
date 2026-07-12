package payment

import "strings"

// SameCryptoAddress compares funding addresses using the canonical semantics
// of the selected rail. EVM addresses are case-insensitive, Solana public keys
// are exact base58 strings, and UTXO rails retain their network-aware aliases.
func SameCryptoAddress(assetID, left, right string) bool {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if left == "" || right == "" {
		return false
	}
	switch {
	case strings.HasPrefix(strings.TrimSpace(assetID), "crypto:eip155:"):
		return strings.EqualFold(left, right)
	case strings.HasPrefix(strings.TrimSpace(assetID), "crypto:solana:"):
		return left == right
	default:
		return SameUTXOAddress(left, right)
	}
}
