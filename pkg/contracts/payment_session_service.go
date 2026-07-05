package contracts

import (
	"context"

	"github.com/mobazha/mobazha/pkg/payment"
)

// CreatePaymentSessionRequest carries the parameters needed to set up a
// unified payment session for an order.
//
// The paymentCoin field MUST be in canonical format:
//   - crypto: "crypto:{chain}:{token}" (e.g. "crypto:eth:usdc")
//   - fiat:   "fiat:{provider}:{currency}" (e.g. "fiat:stripe:USD")
//
// Legacy coin codes must be normalised at the API ingress layer before
// reaching this struct. Passing a non-canonical value is a programmer
// error and will cause CreateSession to return an error.
type CreatePaymentSessionRequest struct {
	// OrderID identifies the order this session belongs to.
	OrderID string

	// PaymentCoin is the canonical payment coin. See format rules above.
	PaymentCoin string

	// PaymentSelectionQuoteID binds Deal cross-currency provisioning to an
	// immutable server-authored quote. It is required when the selected payment
	// currency differs from the signed order pricing currency.
	PaymentSelectionQuoteID string

	// AuthorizedPaymentAmount is populated internally after the quote is loaded
	// from Core storage. API callers must not provide or calculate this value.
	AuthorizedPaymentAmount string

	// RefundAddress is the buyer's on-chain address to which funds are
	// returned on cancel/refund/dispute-release. Optional for crypto
	// orders: client-signed flows may provide it up front, while address-
	// monitored flows can auto-fill it from chain observations after
	// confirmation. Should be empty for fiat orders (provider handles refunds).
	RefundAddress string

	// BuyerPeerID is the peer ID of the buying node, used for ownership
	// checks and address derivation in certain escrow schemes.
	BuyerPeerID string

	// ── Crypto-only fields (canonical paymentCoin prefixed with "crypto:") ─────

	// PayerAddress is the buyer pubkey or chain address forwarded into escrow setup.
	PayerAddress string
	// Moderator is the dispute moderator Libp2p peer ID (required when the order uses moderated escrow).
	Moderator string

	// PayFromCustodial marks exchange/custodial-wallet payments that must
	// declare an explicit refundAddress at session creation time.
	PayFromCustodial bool

	// ── Fiat-specific fields (only set when PaymentCoin starts with "fiat:") ──

	// FiatAmountCents is the payment amount in the smallest currency unit
	// (e.g. cents for USD, pence for GBP). Must be > 0 for fiat orders.
	// The caller (API handler) converts the order total to fiat cents using
	// the exchange rate; this service does not perform currency conversion.
	FiatAmountCents int64

	// FiatDescription is shown on the provider's payment page (optional).
	FiatDescription string

	// FiatReturnURL is the URL the buyer is redirected to after approving a
	// PayPal order. Required for PayPal; ignored by Stripe.
	FiatReturnURL string

	// FiatCancelURL is the URL the buyer is redirected to if they cancel the
	// PayPal approval. Required for PayPal; ignored by Stripe.
	FiatCancelURL string
}

// CreatePaymentSelectionQuoteRequest selects one canonical payment asset for a
// Deal order. Core derives all amounts from the signed OrderOpen and its own
// exchange-rate service.
type CreatePaymentSelectionQuoteRequest struct {
	OrderID     string
	PaymentCoin string
}

// PaymentSessionService creates, reads, and refreshes unified payment
// sessions across all supported payment rails (managed escrow, UTXO, Stripe, PayPal,
// and future Squads / guest checkout).
//
// # Design philosophy
//
// This service is the "unified session projection" layer (Phase B of
// UNIFIED_PAYMENT_SESSION_ARCHITECTURE.md §12). It sits above the
// existing PaymentAppService and FiatPaymentAppService:
//
//	API Handler
//	  → PaymentSessionService
//	       → CryptoPaymentFacade → PaymentAppService + payment.Registry
//	       → FiatPaymentFacade   → FiatPaymentAppService + FiatProviderRegistry
//	       → PaymentSessionProjector (builds PaymentSession from existing data)
//
// # Phase B scope
//
// Phase B only delivers Create / Get / Refresh. Settlement actions
// (confirm, cancel, refund, complete, dispute_release) are deferred to
// SettlementActionService, which will be defined in Phase C.
//
// # Session ID convention (Phase B)
//
// Until a dedicated payment_sessions table is introduced, sessionID is
// derived as "ps_" + orderID. This is stable, requires no new storage,
// and is documented as a Phase B implementation detail.
//
// Reference: docs/payment/UNIFIED_PAYMENT_SESSION_ARCHITECTURE.md §12.2
type PaymentSessionService interface {
	// CreateSelectionQuote creates or reuses a still-valid immutable Deal
	// payment-selection quote. No payment target or Provider session is created.
	CreateSelectionQuote(ctx context.Context, req CreatePaymentSelectionQuoteRequest) (*payment.PaymentSelectionQuote, error)

	// CreateSession provisions a payment session for an order.
	//
	// For crypto orders (settlementMode=address_monitored or escrow_v1):
	//   - Delegates to the configured payment-session crypto facade.
	//   - Returns a PaymentSession with the funding address, expected amount,
	//     expiry, and the initial (zero) payment progress.
	//
	// For fiat orders (settlementMode=provider_checkout):
	//   - Delegates to FiatPaymentAppService.CreatePayment.
	//   - Maps the resulting contracts.FiatProviderSession into a PaymentSession
	//     with FundingTarget.type=provider_session.
	//
	// Callers must pass a canonical paymentCoin in req. Non-canonical
	// values are rejected with a descriptive error; normalisation is the
	// caller's (API layer's) responsibility.
	CreateSession(ctx context.Context, req CreatePaymentSessionRequest) (*payment.PaymentSession, error)

	// GetSession reads the current unified payment session for an order.
	//
	// The session is assembled by PaymentSessionProjector from:
	//   - models.Order (addresses, refund address, verification status,
	//     total received)
	//   - Serialised OrderOpen proto (payment coin, expected amount, escrow
	//     type, funding timeout)
	//   - FiatPaymentState (provider payment ID, fiat metadata)
	//   - PaymentObservations (observation count, last observed timestamp)
	//
	// No new DB table is required; projection is done on every call.
	//
	// Returns ErrNotFound if the order record does not exist.
	// If the order exists but has no payment set up yet, GetSession still
	// returns a projection with Status=awaiting_funds and an empty
	// FundingTarget.Address — callers must not treat this as an error.
	GetSession(ctx context.Context, orderID string) (*payment.PaymentSession, error)

	// RefreshSession re-evaluates the session's funding progress by
	// querying fresh on-chain observations or polling the fiat provider.
	//
	// Useful for:
	//   - Polling-based frontends that don't receive server-push updates.
	//   - Manual "I already paid" triggers from the buyer.
	//   - Post-webhook reconciliation checks.
	//
	// The returned PaymentSession reflects the state after aggregation.
	// RefreshSession must not trigger settlement actions; it is read-only
	// with respect to order state.
	RefreshSession(ctx context.Context, orderID string) (*payment.PaymentSession, error)
}
