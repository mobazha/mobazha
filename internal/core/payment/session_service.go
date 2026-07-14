package payment

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	porderpb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha/pkg/payment"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

// ErrProvisioningNotImplemented is returned when CreateSession needs live
// provisioning but CryptoPaymentFacade is not wired (e.g. partial builds).
var ErrProvisioningNotImplemented = errors.New(
	"payment session: CreateSession: provisioning not wired for this deployment; " +
		"enable PaymentSession facades",
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
	db        database.Database
	projector *PaymentSessionProjector
	fiat      *FiatPaymentFacade // injected via SetFiatFacade (Phase B3)
	crypto    *CryptoPaymentFacade
	exchange  contracts.ExchangeRateService
	policies  []SessionProvisioningPolicy
	now       func() time.Time
	quoteTTL  time.Duration
}

// NewPaymentSessionService constructs the service.
// Inject fiat / crypto facades via Setters after construction.
func NewPaymentSessionService(db database.Database) *PaymentSessionServiceImpl {
	return &PaymentSessionServiceImpl{
		db:        db,
		projector: NewPaymentSessionProjector(db),
		now:       time.Now,
		quoteTTL:  defaultPaymentSelectionQuoteTTL,
	}
}

// SetExchangeRateService injects the authoritative rate source used to create
// immutable payment-selection quotes.
func (s *PaymentSessionServiceImpl) SetExchangeRateService(exchange contracts.ExchangeRateService) {
	s.exchange = exchange
}

// SetFiatFacade injects the FiatPaymentFacade so CreateSession can provision
// fiat payment sessions. Must be called during node initialisation before any
// fiat CreateSession requests are handled.
func (s *PaymentSessionServiceImpl) SetFiatFacade(f *FiatPaymentFacade) {
	s.fiat = f
}

// SetCryptoFacade injects the CryptoPaymentFacade so CreateSession can provision
// crypto payment addresses.
//
// Phase PS crypto closure.
func (s *PaymentSessionServiceImpl) SetCryptoFacade(c *CryptoPaymentFacade) {
	s.crypto = c
}

// AddProvisioningPolicy registers a policy that authorizes new funding targets.
// Read-only session projections and already-provisioned idempotent reads do not
// invoke policies.
func (s *PaymentSessionServiceImpl) AddProvisioningPolicy(policy SessionProvisioningPolicy) {
	if policy != nil {
		s.policies = append(s.policies, policy)
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
		if !iwallet.IsPaymentCoinEnabled(req.PaymentCoin) {
			return nil, fmt.Errorf("%w: %q", ErrPaymentCoinDisabled, req.PaymentCoin)
		}
	}

	input, err := s.projector.fetchProjectInput(req.OrderID)
	if err != nil {
		return nil, fmt.Errorf("payment session: CreateSession: %w", err)
	}
	if input.cryptoAttempt != nil && input.cryptoAttempt.State == models.PaymentAttemptAuthorizationDraft && s.crypto != nil {
		expired, expiryErr := s.crypto.expireQuoteBoundSettlementAuthorizationDraft(
			ctx, input.order, input.cryptoAttempt, s.currentTime(),
		)
		if expiryErr != nil {
			return nil, expiryErr
		}
		if expired {
			input, err = s.projector.fetchProjectInput(req.OrderID)
			if err != nil {
				return nil, fmt.Errorf("payment session: reload expired authorization draft: %w", err)
			}
		}
	}
	if input.cryptoAttempt != nil && input.cryptoAttempt.State == models.PaymentAttemptAuthorizationDraft && s.crypto != nil {
		abandoned, abandonErr := s.crypto.abandonUnsupportedSettlementAuthorizationDraft(ctx, input.order, input.cryptoAttempt)
		if abandonErr != nil {
			return nil, abandonErr
		}
		if abandoned {
			input, err = s.projector.fetchProjectInput(req.OrderID)
			if err != nil {
				return nil, fmt.Errorf("payment session: reload recovered authorization draft: %w", err)
			}
		}
	}
	selectionQuote, err := s.resolveDealPaymentSelectionQuote(ctx, input.order, input.orderOpen, req)
	if err != nil {
		return nil, err
	}
	if selectionQuote != nil {
		req.AuthorizedPaymentAmount = selectionQuote.BuyerPaymentTotal
		if strings.HasPrefix(strings.ToLower(req.PaymentCoin), "fiat:") {
			amount, ok := new(big.Int).SetString(selectionQuote.BuyerPaymentTotal, 10)
			if !ok || !amount.IsInt64() || amount.Sign() <= 0 {
				return nil, fmt.Errorf("%w: quoted fiat amount is invalid", ErrDealPaymentSelectionQuoteInvalid)
			}
			if req.FiatAmountCents > 0 && req.FiatAmountCents != amount.Int64() {
				return nil, fmt.Errorf("%w: requested fiat amount does not match the quote", ErrDealPaymentAmountIntegrity)
			}
			req.FiatAmountCents = amount.Int64()
		}
	}
	if err := validateDealSessionProvisioning(input.orderOpen, req, selectionQuote); err != nil {
		return nil, err
	}
	view, err := s.projector.Project(input)
	if err != nil {
		return nil, fmt.Errorf("payment session: CreateSession: %w", err)
	}
	alreadyProvisioned := paymentSessionHasProvisionedTarget(view)
	if selectionQuote != nil && alreadyProvisioned && view.PaymentSelectionQuoteID != selectionQuote.QuoteID {
		return nil, fmt.Errorf(
			"%w: existing payment target is bound to a different quote",
			ErrDealPaymentSelectionQuoteInvalid,
		)
	}
	if selectionQuote != nil && !selectionQuote.ExpiresAt.After(s.currentTime()) &&
		(!alreadyProvisioned || view.PaymentSelectionQuoteID != selectionQuote.QuoteID) {
		return nil, fmt.Errorf("%w: quote expired before provisioning", ErrDealPaymentSelectionQuoteInvalid)
	}

	if view.PaymentReadiness.Status != payment.PaymentReadinessReadyToPay {
		if view.PaymentReadiness.Status == payment.PaymentReadinessAwaitingSellerReceipt &&
			strings.HasPrefix(req.PaymentCoin, "crypto:") &&
			createSessionCarriesRefundAddressUpdate(req) {
			if s.crypto == nil {
				return nil, ErrProvisioningNotImplemented
			}
			updated, err := s.crypto.UpdateCreateSessionRefundAddress(ctx, req)
			if err != nil {
				return nil, err
			}
			// The seller acknowledgement may land while the refund update is in
			// flight. A ready session without an address is not actionable, so
			// finish provisioning in this request instead of requiring a client
			// to detect and retry an invalid intermediate projection.
			if updated.PaymentReadiness.Status == payment.PaymentReadinessReadyToPay && updated.FundingTarget.Address == "" {
				if err := s.bindPaymentSelectionQuote(input.order, selectionQuote); err != nil {
					return nil, err
				}
				created, err := s.crypto.CreateSession(ctx, req)
				if err != nil {
					return nil, err
				}
				applyPaymentSelectionQuote(created, selectionQuote)
				if err := validateDealPaymentSession(input.orderOpen, req, selectionQuote, created); err != nil {
					return nil, err
				}
				return created, nil
			}
			return updated, nil
		}
		return view, nil
	}

	// Determine whether a session has already been provisioned:
	//   - Crypto: PaymentAddress is set (managed EVM/UTXO address persisted after GeneratePaymentInstructions).
	//   - Fiat:   sessionID key exists in ProviderData (written after CreatePayment returns).
	//
	// NOTE: ProviderData may contain only "providerID" (from coin metadata alone)
	// without a "sessionID". We intentionally require sessionID to be present
	// before treating the fiat session as already provisioned; otherwise a
	// partially-populated view would block all subsequent CreatePayment calls.
	coinSwitch := false
	if alreadyProvisioned && req.PaymentCoin != "" {
		// Guard: if the caller requests a different coin, reject instead of
		// silently returning a session for the wrong rail. The one allowed
		// exception is an unfunded provider checkout (for example, Stripe was
		// opened as the default and the buyer then chose Solana before paying).
		if view.PaymentCoin != "" && view.PaymentCoin != req.PaymentCoin {
			if !canReprovisionForCoinSwitch(view, req.PaymentCoin, input.cryptoAttempt != nil) {
				return nil, fmt.Errorf("%w: existing=%q requested=%q",
					ErrPaymentCoinMismatch, view.PaymentCoin, req.PaymentCoin)
			}
			coinSwitch = true
			alreadyProvisioned = false
		}
	}

	if alreadyProvisioned {
		if strings.HasPrefix(req.PaymentCoin, "crypto:") && createSessionCarriesRefundAddressUpdate(req) {
			if s.crypto == nil {
				return nil, ErrProvisioningNotImplemented
			}
			updated, err := s.crypto.UpdateCreateSessionRefundAddress(ctx, req)
			if err != nil {
				return nil, err
			}
			applyPaymentSelectionQuote(updated, selectionQuote)
			if err := validateDealPaymentSession(input.orderOpen, req, selectionQuote, updated); err != nil {
				return nil, err
			}
			return updated, nil
		}
		applyPaymentSelectionQuote(view, selectionQuote)
		if err := validateDealPaymentSession(input.orderOpen, req, selectionQuote, view); err != nil {
			return nil, err
		}
		return view, nil
	}

	if req.PaymentCoin != "" {
		policyInput := SessionProvisioningPolicyInput{
			OrderID:                 req.OrderID,
			PaymentCoin:             req.PaymentCoin,
			PaymentSelectionQuoteID: req.PaymentSelectionQuoteID,
			ExpiresAt:               view.ExpiresAt,
			OrderOpen:               input.orderOpen,
		}
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(req.PaymentCoin)), "fiat:") {
			policyInput.SettlementMethod = porderpb.PaymentSent_FIAT
			if strings.TrimSpace(req.Moderator) != "" {
				policyInput.SettlementMethod = porderpb.PaymentSent_MODERATED
			}
			policyInput.SettlementMethodKnown = true
		}
		for _, policy := range s.policies {
			if err := policy.AuthorizeSessionProvisioning(ctx, policyInput); err != nil {
				return nil, err
			}
		}
	}
	if err := s.bindPaymentSelectionQuote(input.order, selectionQuote); err != nil {
		return nil, fmt.Errorf("bind payment selection quote: %w", err)
	}

	// Authorization, including an atomic source-card reservation update, must
	// succeed before removing the previous funding target. If clearing or
	// reprovisioning later fails, a retry can safely resume from durable state.
	if coinSwitch {
		if err := s.clearUnfundedPaymentSessionState(ctx, req.OrderID); err != nil {
			return nil, err
		}
	}

	// Provisioning needed — route to the appropriate facade by coin prefix.
	if req.PaymentCoin != "" {
		// Fiat orders: "fiat:{provider}:{currency}"
		if strings.HasPrefix(req.PaymentCoin, "fiat:") {
			if s.fiat == nil {
				return nil, fmt.Errorf("%w: use POST /v1/fiat/{providerID}/payments", ErrFiatFacadeNotWired)
			}
			created, err := s.fiat.CreateSession(ctx, req)
			if err != nil {
				return nil, err
			}
			applyPaymentSelectionQuote(created, selectionQuote)
			if err := validateDealPaymentSession(input.orderOpen, req, selectionQuote, created); err != nil {
				return nil, err
			}
			return created, nil
		}

		// Crypto orders (managed EVM + UTXO): "crypto:{chain}:{token}"
		if strings.HasPrefix(req.PaymentCoin, "crypto:") {
			if s.crypto == nil {
				return nil, ErrProvisioningNotImplemented
			}
			if input.cryptoAttempt != nil && input.cryptoAttempt.State == models.PaymentAttemptAuthorizationDraft {
				if normalizeCoinBestEffort(input.cryptoAttempt.Currency) != normalizeCoinBestEffort(req.PaymentCoin) {
					return nil, fmt.Errorf("%w: draft=%q requested=%q",
						ErrPaymentCoinMismatch, input.cryptoAttempt.Currency, req.PaymentCoin)
				}
				// Re-enter the idempotent starter so a draft whose earlier seller
				// finalization failed can republish its offer after an upgrade.
			}
			created, err := s.crypto.CreateSession(ctx, req)
			if err != nil {
				return nil, err
			}
			applyPaymentSelectionQuote(created, selectionQuote)
			if err := validateDealPaymentSession(input.orderOpen, req, selectionQuote, created); err != nil {
				return nil, err
			}
			return created, nil
		}

		return nil, fmt.Errorf(
			"payment session: CreateSession: unsupported payment coin prefix %q",
			req.PaymentCoin)
	}

	// Read-only query with no paymentCoin — return best-effort projection.
	return view, nil
}

func paymentSessionHasProvisionedTarget(view *payment.PaymentSession) bool {
	return view != nil && (view.FundingTarget.Address != "" || fiatSessionIDFromView(view) != "")
}

func createSessionCarriesRefundAddressUpdate(req contracts.CreatePaymentSessionRequest) bool {
	return req.PayFromCustodial ||
		strings.TrimSpace(req.RefundAddress) != "" ||
		strings.TrimSpace(req.PayerAddress) != ""
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

func canReprovisionForCoinSwitch(view *payment.PaymentSession, requestedCoin string, hasFrozenAttempt bool) bool {
	if view == nil {
		return false
	}
	if hasFrozenAttempt {
		return false
	}
	if !strings.HasPrefix(requestedCoin, "crypto:") && !strings.HasPrefix(requestedCoin, "fiat:") {
		return false
	}
	if view.SettlementMode != payment.SettlementModeProviderCheckout &&
		view.SettlementMode != payment.SettlementModeAddressMonitored {
		return false
	}
	if view.PaymentProgress.FundingState != payment.FundingStateProviderProcessing &&
		view.PaymentProgress.FundingState != payment.FundingStateAwaitingFunds {
		return false
	}
	return amountStringIsZero(view.PaymentProgress.ObservedAmount)
}

func amountStringIsZero(s string) bool {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return true
	}
	rat, ok := new(big.Rat).SetString(trimmed)
	return ok && rat.Sign() == 0
}

func (s *PaymentSessionServiceImpl) clearUnfundedPaymentSessionState(_ context.Context, orderID string) error {
	if s.db == nil {
		return errors.New("payment session: cannot switch payment coin without database")
	}
	return s.db.Update(func(tx database.Tx) error {
		rows, err := tx.UpdateColumns(
			map[string]interface{}{
				"payment_address":                     "",
				"pending_payment_info":                []byte(nil),
				"payment_transaction_id":              "",
				"fiat_metadata":                       []byte(nil),
				"payment_verification_status":         "",
				"payment_verification_failure_reason": "",
				"payment_verification_failed_at":      nil,
				"total_received":                      "",
				"overpaid_amount":                     "",
				"cancel_fee_amount":                   "",
			},
			map[string]interface{}{"id = ?": orderID},
			&models.Order{},
		)
		if err != nil {
			return fmt.Errorf("payment session: clear stale payment state for coin switch: %w", err)
		}
		if rows == 0 {
			return fmt.Errorf("payment session: clear stale payment state for coin switch: order %s not found", orderID)
		}
		return nil
	})
}
