//go:build !private_distribution

package payment

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// ErrProvisioningNotImplemented is returned when CreateSession needs live
// provisioning but CryptoPaymentFacade is not wired (e.g. partial builds).
var ErrProvisioningNotImplemented = errors.New(
	"payment session: CreateSession: provisioning not wired for this deployment; " +
		"use the existing payment initialisation path or enable PaymentSession facades",
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
	fiat      *FiatPaymentFacade // injected via SetFiatFacade (Phase B3)
	crypto    *CryptoPaymentFacade
}

// NewPaymentSessionService constructs the service.
// Inject fiat / crypto facades via Setters after construction.
func NewPaymentSessionService(db database.Database) *PaymentSessionServiceImpl {
	return &PaymentSessionServiceImpl{
		projector: NewPaymentSessionProjector(db),
	}
}

// SetFiatFacade injects the FiatPaymentFacade so CreateSession can provision
// fiat payment sessions. Must be called during node initialisation before any
// fiat CreateSession requests are handled.
func (s *PaymentSessionServiceImpl) SetFiatFacade(f *FiatPaymentFacade) {
	s.fiat = f
}

// SetCryptoFacade injects the CryptoPaymentFacade so CreateSession can provision
// crypto payment addresses / instructions paths.
//
// Phase PS crypto closure.
func (s *PaymentSessionServiceImpl) SetCryptoFacade(c *CryptoPaymentFacade) {
	s.crypto = c
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
// # Idempotency rules
//
// When the order already has a funded address (crypto) OR a fiat session with a
// non-empty sessionID, and req.PaymentCoin matches the existing coin (or is
// empty), CreateSession returns the current projection without re-provisioning.
//
// If req.PaymentCoin differs from the existing session coin, the method returns
// ErrPaymentCoinMismatch — the caller must resolve the coin-switch explicitly
// rather than silently receiving the wrong session.
//
// # Routing
//
// When provisioning is needed, CreateSession routes by coin prefix:
//   - "fiat:*"   → FiatPaymentFacade (ErrFiatFacadeNotWired if not configured)
//   - "crypto:*" → CryptoPaymentFacade (ErrProvisioningNotImplemented if not configured)
func (s *PaymentSessionServiceImpl) CreateSession(
	ctx context.Context,
	req contracts.CreatePaymentSessionRequest,
) (*payment.PaymentSession, error) {
	if req.OrderID == "" {
		return nil, fmt.Errorf("payment session: CreateSession: orderID is required")
	}

	// Validate canonical paymentCoin — programmer error if non-canonical reaches here.
	if req.PaymentCoin != "" {
		if err := iwallet.CoinType(req.PaymentCoin).ValidateCanonicalPaymentCoin(); err != nil {
			return nil, fmt.Errorf("payment session: CreateSession: %w", err)
		}
	}

	view, err := s.GetSession(ctx, req.OrderID)
	if err != nil {
		return nil, fmt.Errorf("payment session: CreateSession: %w", err)
	}

	// Determine whether a session has already been provisioned:
	//   - Crypto: PaymentAddress is set (ManagedEscrow/UTXO address persisted after GeneratePaymentInstructions).
	//   - Fiat:   sessionID key exists in ProviderData (written after CreatePayment returns).
	//
	// NOTE: ProviderData may contain only "providerID" (from coin metadata alone)
	// without a "sessionID". We intentionally require sessionID to be present
	// before treating the fiat session as already provisioned; otherwise a
	// partially-populated view would block all subsequent CreatePayment calls.
	alreadyProvisioned := view.FundingTarget.Address != "" ||
		fiatSessionIDFromView(view) != ""

	if alreadyProvisioned && req.PaymentCoin != "" {
		// Guard: if the caller requests a different coin, reject instead of
		// silently returning a session for the wrong rail.
		if view.PaymentCoin != "" && view.PaymentCoin != req.PaymentCoin {
			return nil, fmt.Errorf("%w: existing=%q requested=%q",
				ErrPaymentCoinMismatch, view.PaymentCoin, req.PaymentCoin)
		}
	}

	if alreadyProvisioned {
		return view, nil
	}

	// Provisioning needed — route to the appropriate facade by coin prefix.
	if req.PaymentCoin != "" {
		// Fiat orders: "fiat:{provider}:{currency}"
		if strings.HasPrefix(req.PaymentCoin, "fiat:") {
			if s.fiat == nil {
				return nil, fmt.Errorf("%w: use POST /v1/fiat/{providerID}/payments", ErrFiatFacadeNotWired)
			}
			return s.fiat.CreateSession(ctx, req)
		}

		// Crypto orders (ManagedEscrow + UTXO): "crypto:{chain}:{token}"
		if strings.HasPrefix(req.PaymentCoin, "crypto:") {
			if s.crypto == nil {
				return nil, ErrProvisioningNotImplemented
			}
			return s.crypto.CreateSession(ctx, req)
		}

		return nil, fmt.Errorf(
			"payment session: CreateSession: unsupported payment coin prefix %q",
			req.PaymentCoin)
	}

	// Read-only query with no paymentCoin — return best-effort projection.
	return view, nil
}

// fiatSessionIDFromView extracts the "sessionID" key from FundingTarget.ProviderData.
// Returns "" if absent, so callers can treat absence as "not yet provisioned".
func fiatSessionIDFromView(view *payment.PaymentSession) string {
	if view == nil {
		return ""
	}
	sid, _ := view.FundingTarget.ProviderData["sessionID"].(string)
	return sid
}
