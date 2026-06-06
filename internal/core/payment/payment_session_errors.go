//go:build !private_distribution

package payment

import (
	"errors"

	pkgpayment "github.com/mobazha/mobazha3.0/pkg/payment"
)

// Sentinel errors returned by PaymentSessionService and facades. API handlers map
// them to HTTP statuses via errors.Is.
var (
	ErrFiatFacadeNotWired = errors.New(
		"payment session: fiat facade not wired",
	)

	ErrInvalidFiatAmountCents = errors.New(
		"payment session: FiatAmountCents must be greater than zero",
	)

	ErrRWAPaymentSessionUnsupported = errors.New(
		"payment session: RWA token listings are not supported by payment-session provisioning yet",
	)

	ErrPaymentCoinDisabled = errors.New(
		"payment session: requested payment coin is not enabled",
	)

	// ErrPaymentCoinMismatch is returned when CreateSession is called with a
	// paymentCoin that differs from the coin already provisioned for the order.
	// Callers must explicitly handle this as a coin-switch case rather than
	// silently receiving the existing-session view.
	ErrPaymentCoinMismatch = errors.New(
		"payment session: requested paymentCoin differs from existing session coin; " +
			"resolve the coin switch before re-provisioning",
	)

	// ErrExchangeRateUnavailable is returned when cross-currency amount
	// conversion is needed but no exchange rate service is configured.
	ErrExchangeRateUnavailable = errors.New(
		"payment session: exchange rate service unavailable for cross-currency conversion",
	)

	// ErrLegacyEVMPaymentRetired is returned when code attempts to provision
	// legacy ClientSigned EVM escrow funding (redeem-script hash path).
	ErrLegacyEVMPaymentRetired = errors.New(
		"payment session: legacy EVM contract escrow is retired; ManagedEscrow adapter required",
	)

	// ErrLegacySolanaPaymentRetired is returned when Solana setup resolves to
	// the retired ClientSigned lifecycle adapter instead of SolanaAnchorAdapter.
	ErrLegacySolanaPaymentRetired = errors.New(
		"payment session: legacy Solana client-signed escrow is retired; Anchor adapter required",
	)

	// ErrLegacyUTXOPaymentRetired is returned when UTXO setup does not resolve
	// to the address-monitored V2 path.
	ErrLegacyUTXOPaymentRetired = errors.New(
		"payment session: legacy UTXO payment path is retired; monitored adapter required",
	)

	// ErrInvalidEVMFundingAddress is returned when an EVM funding target is not
	// a 20-byte hex address (e.g. legacy redeem-script hashes).
	ErrInvalidEVMFundingAddress = errors.New(
		"payment session: invalid EVM funding address",
	)

	// ErrInvalidSolanaFundingAddress is returned when a Solana funding target is
	// not a valid base58 public key.
	ErrInvalidSolanaFundingAddress = errors.New(
		"payment session: invalid Solana funding address",
	)

	// ErrTRONPaymentRetired aliases pkg/payment.ErrTRONPaymentRetired for handler mapping.
	ErrTRONPaymentRetired = pkgpayment.ErrTRONPaymentRetired
)
