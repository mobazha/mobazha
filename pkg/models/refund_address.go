package models

import (
	"errors"
	"fmt"

	"github.com/mobazha/mobazha/pkg/paymentaddress"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

// ErrRefundAddressRequired is returned when an order requires a buyer-declared
// refund address but the caller passed an empty string. The Monitor-Driven
// Payment model (docs/escrow/MONITOR_DRIVEN_PAYMENT.md §P0-3) makes RefundAddress
// a hard requirement for CEX direct-pay scenarios where the on-chain sender is an
// exchange omnibus address that must NEVER receive a refund.
var ErrRefundAddressRequired = errors.New("refund address is required for crypto orders")

// ErrRefundAddressInvalid is returned when a buyer-supplied refund address
// fails chain-specific format validation (hex / base58 / utxo).
var ErrRefundAddressInvalid = errors.New("refund address has invalid format")

// ValidateRefundAddress applies payment-address format checks while preserving
// refund-specific error sentinels for API and order-flow callers.
func ValidateRefundAddress(coin iwallet.CoinType, addr string) error {
	if err := paymentaddress.Validate(coin, addr); err != nil {
		if errors.Is(err, paymentaddress.ErrRequired) {
			return ErrRefundAddressRequired
		}
		return fmt.Errorf("%w: %v", ErrRefundAddressInvalid, err)
	}
	return nil
}

// ValidateRefundReceivingAddresses validates buyer default refund destinations
// while preserving refund-specific error sentinels.
func ValidateRefundReceivingAddresses(addrs map[string]string) error {
	if err := paymentaddress.ValidatePaymentCoinAddressMap(addrs); err != nil {
		if errors.Is(err, paymentaddress.ErrRequired) {
			return ErrRefundAddressRequired
		}
		return fmt.Errorf("%w: %v", ErrRefundAddressInvalid, err)
	}
	return nil
}
