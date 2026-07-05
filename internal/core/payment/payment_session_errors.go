package payment

import (
	"errors"

	pkgpayment "github.com/mobazha/mobazha/pkg/payment"
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

	ErrOrderExtensionReservation = errors.New(
		"payment session: order extension reservation failed",
	)

	ErrOrderExtensionSettlement = errors.New(
		"payment session: order extension settlement policy rejected the funding rail",
	)

	ErrOrderExtensionCollateral = errors.New(
		"payment session: order extension collateral admission failed",
	)

	ErrPaymentCoinDisabled = errors.New(
		"payment session: requested payment coin is not enabled",
	)

	// ErrDealPaymentQuoteRequired is returned when a Deal-backed signed order
	// reaches payment provisioning without an immutable Hosting fee quote.
	ErrDealPaymentQuoteRequired = errors.New(
		"payment session: Deal order requires an immutable fee quote",
	)

	// ErrDealPaymentConversionQuoteRequired prevents a Deal-backed order from
	// using an ephemeral exchange rate. Cross-currency payment requires a
	// separately persisted quote binding source amount, rate, target amount,
	// payment asset, and expiry before a funding target can be provisioned.
	ErrDealPaymentConversionQuoteRequired = errors.New(
		"payment session: Deal cross-currency payment requires an immutable conversion quote",
	)

	// ErrDealPaymentSelectionQuoteInvalid identifies an absent, expired or
	// mismatched immutable payment-selection quote.
	ErrDealPaymentSelectionQuoteInvalid = errors.New(
		"payment session: Deal payment-selection quote is invalid",
	)

	// ErrDealPaymentAmountIntegrity identifies a mismatch between signed Deal
	// terms and the amount or asset exposed by an actionable PaymentSession.
	ErrDealPaymentAmountIntegrity = errors.New(
		"payment session: Deal amount integrity validation failed",
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
		"payment session: legacy EVM contract escrow is retired; managed EVM adapter required",
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
