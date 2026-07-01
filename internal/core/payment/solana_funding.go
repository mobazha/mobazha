package payment

import (
	"strings"

	"github.com/gagliardetto/solana-go"
)

// IsValidSolanaFundingAddress reports whether addr is a valid base58 Solana
// public key suitable for address-monitored Anchor escrow funding.
func IsValidSolanaFundingAddress(addr string) bool {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return false
	}
	_, err := solana.PublicKeyFromBase58(addr)
	return err == nil
}
