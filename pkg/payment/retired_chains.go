package payment

import (
	"errors"

	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// ErrTRONPaymentRetired is returned when checkout or payment setup targets TRON.
// TRON client-signed escrow is retired; no replacement path is registered yet.
var ErrTRONPaymentRetired = errors.New(
	"payment: TRON escrow is retired and not available for new orders",
)

// IsRetiredPaymentChain reports chains that no longer accept new payment setup.
func IsRetiredPaymentChain(chain iwallet.ChainType) bool {
	return chain == iwallet.ChainTRON
}
