package payment

import (
	"strings"

	"github.com/ethereum/go-ethereum/common"
)

// IsValidEVMFundingAddress reports whether addr is a 20-byte hex address
// suitable for address-monitored ManagedEscrow/EOA funding. Legacy redeem-script hashes
// (32-byte / 66-char strings) are rejected.
func IsValidEVMFundingAddress(addr string) bool {
	addr = strings.TrimSpace(addr)
	return common.IsHexAddress(addr)
}
