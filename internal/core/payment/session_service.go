//go:build !private_distribution

package payment

import (
	"context"
	"errors"
	"fmt"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// ErrProvisioningNotImplemented is returned by CreateSession when the caller
// requests provisioning (i.e. paymentCoin is specified and the order has no
// existing payment setup) but the required facade (CryptoPaymentFacade or
// FiatPaymentFacade) has not yet been wired in the current Phase B step.
//
// Callers should fall back to the existing payment initialisation path
// (e.g. GET /v1/wallet/spend or POST /v1/fiat/payments) until Phase B
// Step 2 and Step 3 are implemented.
//
// TECHDEBT(TD-PSS-01): remove once CryptoPaymentFacade and FiatPaymentFacade
// are implemented (Phase B Step 2 + Step 3).
var ErrProvisioningNotImplemented = errors.New(
	"payment session: CreateSession: provisioning not yet implemented; " +
		"use the existing payment initialisation path (TECHDEBT TD-PSS-01)",
)

// PaymentSessionServiceImpl implements contracts.PaymentSessionService.
//
// # Phase B scope
//
// CreateSession, GetSession, and RefreshSession delegate to:
//   - CryptoPaymentFacade for address-monitored / wallet-push orders
//   - FiatPaymentFacade for provider_checkout orders
//   - PaymentSessionProjector for read-side projection
//
// The facades are thin wrappers around existing PaymentAppService and
// FiatPaymentAppService — no existing handler is modified.
//
// SettlementActionService (confirm / cancel / refund / complete /
// dispute_release) is deferred to Phase C.
//
// Reference: PAYMENT_SESSION_SERVICE_SPEC.md §5 + §12
type PaymentSessionServiceImpl struct {
	projector *PaymentSessionProjector
	// crypto    *CryptoPaymentFacade — injected in Phase B Step 2 (ManagedEscrow first)
	// fiat      *FiatPaymentFacade   — injected in Phase B Step 3
}

// NewPaymentSessionService constructs the service.
// Additional facade dependencies will be added in subsequent Phase B steps.
func NewPaymentSessionService(db database.Database) *PaymentSessionServiceImpl {
	return &PaymentSessionServiceImpl{
		projector: NewPaymentSessionProjector(db),
	}
}

// Ensure PaymentSessionServiceImpl satisfies the contracts interface at
// compile time. This guard catches missing method implementations early.
var _ contracts.PaymentSessionService = (*PaymentSessionServiceImpl)(nil)

// GetSession reads the current unified payment session for an order.
//
// It builds a projection from existing order, payment, and fiat metadata.
// No new DB table is required.
func (s *PaymentSessionServiceImpl) GetSession(
	ctx context.Context,
	orderID string,
) (*payment.PaymentSession, error) {
	input, err := s.projector.fetchProjectInput(orderID)
	if err != nil {
		return nil, err
	}
	return s.projector.Project(input)
}

// RefreshSession re-evaluates the session's funding progress.
//
// Phase B: delegates to GetSession (re-reads from DB).
// Phase C: will additionally poll fiat providers and re-aggregate
// payment observations.
func (s *PaymentSessionServiceImpl) RefreshSession(
	ctx context.Context,
	orderID string,
) (*payment.PaymentSession, error) {
	// Phase B: re-projection is equivalent to a fresh read.
	// Phase C will add: fiat provider poll + observation aggregation trigger.
	return s.GetSession(ctx, orderID)
}

// CreateSession provisions a payment session for an order.
//
// # Current Phase B Step 1 behaviour
//
// When the order already has a payment set up (crypto funding address exists
// or a fiat provider session was previously created), CreateSession behaves
// as an idempotent re-read and returns the current projection — this is safe
// for duplicate call / retry scenarios.
//
// When provisioning is genuinely needed (no address, no fiat session) AND the
// caller supplies a paymentCoin, this method returns ErrProvisioningNotImplemented
// rather than silently returning an empty / half-baked view. The caller must
// use the existing payment initialisation path until Phase B Step 2/3.
//
// # Phase B Step 2 (ManagedEscrow / UTXO) + Step 3 (fiat) — not yet wired
//
// Once CryptoPaymentFacade and FiatPaymentFacade are injected, CreateSession
// will delegate to them based on req.PaymentCoin prefix:
//   - "fiat:*"   → FiatPaymentFacade.CreateSession
//   - "crypto:*" → CryptoPaymentFacade.Provision (ManagedEscrow address derivation / UTXO)
//
// TECHDEBT(TD-PSS-01): full provisioning deferred to Phase B Step 2 + Step 3.
// Cleanup condition: CryptoPaymentFacade and FiatPaymentFacade implemented.
func (s *PaymentSessionServiceImpl) CreateSession(
	ctx context.Context,
	req contracts.CreatePaymentSessionRequest,
) (*payment.PaymentSession, error) {
	if req.OrderID == "" {
		return nil, fmt.Errorf("payment session: CreateSession: orderID is required")
	}

	// Validate canonical paymentCoin per interface contract
	// (pkg/contracts/payment_session_service.go §CreatePaymentSessionRequest).
	// Non-canonical values are a programmer error — the API ingress layer must
	// normalise before calling CreateSession.
	if req.PaymentCoin != "" {
		if err := iwallet.CoinType(req.PaymentCoin).ValidateCanonicalPaymentCoin(); err != nil {
			return nil, fmt.Errorf("payment session: CreateSession: %w", err)
		}
	}

	view, err := s.GetSession(ctx, req.OrderID)
	if err != nil {
		return nil, fmt.Errorf("payment session: CreateSession: %w", err)
	}

	// Idempotent re-read: the order already has a payment set up.
	// FundingTarget.Address non-empty → crypto address provisioned.
	// FundingTarget.ProviderData non-nil → fiat session exists.
	alreadyProvisioned := view.FundingTarget.Address != "" ||
		len(view.FundingTarget.ProviderData) > 0

	if alreadyProvisioned {
		return view, nil
	}

	// Provisioning is needed but the facades are not yet wired.
	// Returning a half-baked view here would create a false-positive: the
	// caller would believe a session was created when nothing was actually
	// provisioned.  Return an explicit error so the caller falls back to
	// the existing payment initialisation path.
	//
	// Exception: if the caller passes no paymentCoin it is requesting a
	// read-only projection (e.g. to check current state), which is safe
	// to return even when the order is not yet provisioned.
	if req.PaymentCoin != "" {
		return nil, ErrProvisioningNotImplemented
	}

	// Read-only query with no paymentCoin — return best-effort projection.
	return view, nil
}
