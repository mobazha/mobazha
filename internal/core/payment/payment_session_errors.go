//go:build !private_distribution

package payment

import "errors"

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
)
