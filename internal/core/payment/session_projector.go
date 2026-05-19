//go:build !private_distribution

package payment

import (
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// PaymentSessionProjector assembles a payment.PaymentSession from existing
// order, payment and fiat metadata. No new DB table is required; all data
// is sourced from models already written by the existing payment paths.
//
// Design: UNIFIED_PAYMENT_SESSION_ARCHITECTURE.md §8 + PAYMENT_SESSION_SERVICE_SPEC.md §8
//
// Data sources (read-only, projection-first):
//
//	models.Order.PaymentAddress       → FundingTarget.Address
//	models.Order.RefundAddress        → PaymentSession.RefundAddress
//	models.Order.TotalReceived        → PaymentProgress.ObservedAmount
//	models.Order.PaymentVerification* → SessionStatus derivation
//	models.Order.FiatPaymentState     → fiat FundingTarget
//	pb.PaymentSent (serialised)       → PaymentCoin, ProductMode
//	pb.OrderOpen (serialised)         → ExpectedAmount
//	PaymentObservation rows           → ObservationCount, LastObservedAt
type PaymentSessionProjector struct {
	db database.Database
}

// NewPaymentSessionProjector creates a new projector backed by db.
func NewPaymentSessionProjector(db database.Database) *PaymentSessionProjector {
	return &PaymentSessionProjector{db: db}
}

// projectOrderInput is the pre-fetched set of data needed to build a view.
// Fetched once per call to avoid repeated DB round-trips.
type projectOrderInput struct {
	order       *models.Order
	orderOpen   *pb.OrderOpen   // may be nil for orders not yet opened
	paymentSent *pb.PaymentSent // may be nil for orders not yet paid
	isManagedEscrowOrder bool            // legacy fallback when settlement spec is absent
	hasSpec     bool
	spec        payment.SettlementSpec
	obsCount    int
	lastObsAt   *time.Time
}

// Project builds a payment.PaymentSession for the given order.
// It is the single source of truth for the projection rules applied by
// PaymentSessionService.GetSession and RefreshSession.
func (p *PaymentSessionProjector) Project(input *projectOrderInput) (*payment.PaymentSession, error) {
	order := input.order

	// ── Session ID (Phase B derivation) ──────────────────────────────────
	sessionID := "ps_" + order.ID.String()

	// ── Payment coin & product mode ───────────────────────────────────────
	// Primary source: PaymentSent (already paid).
	// Fallback: FiatMetadata (fiat order not yet paid) or PendingPaymentInfo
	// (UTXO order with address set but PaymentSent not yet received).
	paymentCoin, productMode, paymentSentKind := p.derivePaymentInfo(order, input.orderOpen, input.paymentSent)

	// ── Expected amount (from OrderOpen) ─────────────────────────────────
	expectedAmount := ""
	if input.orderOpen != nil {
		expectedAmount = input.orderOpen.Amount
	}

	// ── Settlement mode & funding target ─────────────────────────────────
	settlementMode, fundingTarget := p.deriveFundingTarget(order, paymentCoin, expectedAmount, input)

	// ── Payment progress ──────────────────────────────────────────────────
	progress := p.deriveProgress(order, expectedAmount, input.obsCount, input.lastObsAt)

	// ── Session status ────────────────────────────────────────────────────
	status := deriveSessionStatus(order.PaymentVerificationStatus, progress.FundingState)

	// ── Expiry (best-effort from OrderOpen timeout) ───────────────────────
	expiresAt := p.deriveExpiry(input.orderOpen)

	// ── Capabilities ──────────────────────────────────────────────────────
	caps := p.deriveCapabilities(status, productMode, settlementMode)

	// ── Legacy compatibility ───────────────────────────────────────────────
	var legacyCompat *payment.LegacyCompatibilityView
	if paymentSentKind != "" {
		legacyCompat = &payment.LegacyCompatibilityView{
			PaymentSentKind:               paymentSentKind,
			InstructionsEndpointAvailable: true,
		}
	}

	return &payment.PaymentSession{
		SessionID:           sessionID,
		OrderID:             order.ID.String(),
		PaymentCoin:         paymentCoin,
		SettlementMode:      settlementMode,
		ProductMode:         productMode,
		Status:              status,
		ExpectedAmount:      expectedAmount,
		RefundAddress:       order.RefundAddress,
		ExpiresAt:           expiresAt,
		FundingTarget:       fundingTarget,
		PaymentProgress:     progress,
		Capabilities:        caps,
		UserActionRequest:   nil, // Phase B: no user action required for address_monitored
		LegacyCompatibility: legacyCompat,
	}, nil
}

// ── Private derivation helpers ────────────────────────────────────────────

// derivePaymentInfo extracts canonical payment coin, product mode, and the
// legacy paymentSentKind string.
//
// Resolution order:
//  1. PaymentSent (most authoritative — order already paid)
//  2. FiatMetadata["fiat_provider"] + FiatMetadata["fiat_currency"] —
//     for fiat orders where the buyer has not yet completed payment
//     (i.e. PaymentSent_FIAT is written only after the provider confirms).
//  3. PendingPaymentInfo["Coin"] — for UTXO orders whose address is set
//     but PaymentSent has not yet been received.
//
// paymentSentKind is the raw legacy string used to populate
// LegacyCompatibility for old clients (non-empty only when PaymentSent is
// present).
func (p *PaymentSessionProjector) derivePaymentInfo(
	order *models.Order,
	orderOpen *pb.OrderOpen,
	ps *pb.PaymentSent,
) (
	paymentCoin string,
	productMode payment.ProductMode,
	paymentSentKind string,
) {
	// ── 1. PaymentSent (primary) ──────────────────────────────────────────
	if ps != nil {
		paymentCoin = normalizeCoinBestEffort(ps.Coin)
		productMode = payment.ProductModeFromMethod(ps.Method)
		switch ps.Method {
		case pb.PaymentSent_MODERATED:
			paymentSentKind = "PAYMENT_SENT_MODERATED"
		case pb.PaymentSent_DIRECT:
			paymentSentKind = "PAYMENT_SENT_DIRECT"
		default:
			paymentSentKind = "PAYMENT_SENT_CANCELABLE"
		}
		return paymentCoin, productMode, paymentSentKind
	}

	// ── 2. FiatMetadata fallback (fiat order, awaiting buyer action) ──────
	if fiatMeta, err := order.GetFiatMetadata(); err == nil {
		provider := fiatMeta["fiat_provider"]
		if provider != "" {
			currency := fiatMeta["fiat_currency"]
			if currency == "" && orderOpen != nil {
				currency = orderOpen.PricingCoin
			}
			if currency != "" {
				paymentCoin = fmt.Sprintf("fiat:%s:%s",
					strings.ToLower(strings.TrimSpace(provider)),
					strings.ToUpper(strings.TrimSpace(currency)))
				return paymentCoin, payment.ProductModeFromMethod(pb.PaymentSent_FIAT), ""
			}
			// Legacy rows with provider metadata but no currency context cannot be
			// projected to a canonical fiat coin safely. Leave paymentCoin empty
			// rather than leaking the invalid external shape "fiat:{provider}".
			return "", payment.ProductModeCancelable, ""
		}
	}

	// ── 3. PendingPaymentInfo / fiat metadata (pre-PaymentSent) ───────────
	if spec, ok := payment.ResolveSettlementSpecFromOrder(order); ok {
		coin := pendingPaymentCoin(order, orderOpen)
		return coin, payment.ProductModeFromMethod(spec.Method), ""
	}

	return "", payment.ProductModeCancelable, ""
}

func pendingPaymentCoin(order *models.Order, orderOpen *pb.OrderOpen) string {
	if managed_escrowInfo, err := order.GetPendingManagedEscrowPaymentInfo(); err == nil && managed_escrowInfo != nil && managed_escrowInfo.Coin != "" {
		return normalizeCoinBestEffort(managed_escrowInfo.Coin)
	}
	if csInfo, err := order.GetPendingClientSignedPaymentInfo(); err == nil && csInfo != nil && csInfo.Coin != "" {
		return normalizeCoinBestEffort(csInfo.Coin)
	}
	if utxoInfo, err := order.GetPendingPaymentInfo(); err == nil && utxoInfo != nil && utxoInfo.Coin != "" {
		return normalizeCoinBestEffort(utxoInfo.Coin)
	}
	if orderOpen != nil {
		return normalizeCoinBestEffort(orderOpen.PricingCoin)
	}
	return ""
}

// normalizeCoinBestEffort converts a coin code to canonical format where
// possible. Delegates to iwallet.TryNormalizePaymentCoin (crypto:* via
// assetid.Normalize, fiat casing, legacy native tickers).
//
// Ambiguous legacy symbols (e.g. "USDC" without chain) are returned trimmed
// unchanged — historical order rows lack chain context.
//
// This function must not fail — it is called from a pure projection path.
//
// Phase PS B4: projection output is canonical whenever TryNormalizePaymentCoin
// succeeds.
func normalizeCoinBestEffort(coin string) string {
	if coin == "" {
		return ""
	}
	s := strings.TrimSpace(coin)
	if ct, ok := iwallet.TryNormalizePaymentCoin(s); ok {
		return string(ct)
	}
	return s
}

// settlementRail is the internal discriminator used by detectSettlementRail.
type settlementRail int

const (
	// railAddressMonitored: backend generates an address and monitors it for
	// incoming transfers. Covers UTXO chains (BTC / BCH / LTC / ZEC) and EXTERNAL_PAYMENT.
	railAddressMonitored settlementRail = iota
	// railClientSigned: buyer's local wallet must initiate the transfer and/or
	// sign escrow instructions. Covers legacy EVM (V1 ContractManager) and
	// Solana/TRON.
	//
	// Note: ManagedEscrow EVM orders are classified as railAddressMonitored upstream,
	// before detectSettlementRail is called, via the isManagedEscrowOrder flag set in
	// fetchProjectInput (Phase PS B2 resolved TD-PSS-02).
	railClientSigned
	// railUnknown: coin code not recognised; conservative fallback.
	railUnknown
)

// detectSettlementRail classifies a canonical (or best-effort normalized) coin
// into the appropriate settlement rail. Fiat coins must be resolved before
// calling this function.
//
// Primary path: canonical "crypto:*" asset ID → resolved via CoinInfoFromCoinType
// using ChainType.IsUTXOChain() / chain identity. No string-based chain list.
//
// Fallback path: non-canonical ticker strings are resolved in two steps:
//  1. Try ChainType(ticker).IsValid() — covers all native chain tickers without
//     maintaining a parallel hardcoded list here.
//  2. For well-known multi-chain token symbols (USDC etc.) that carry no
//     intrinsic chain context, conservatively classify as client-signed since all
//     current EVM/Solana deployments use the legacy instructions path.
//
// Phase PS B4: canonical policy at API ingress reduces new orders hitting this
// fallback; legacy rows may still carry ambiguous tickers until fully migrated.
func detectSettlementRail(coinCode string) settlementRail {
	if coinCode == "" {
		return railUnknown
	}

	// Primary path: canonical crypto asset ID — use CoinInfo to classify.
	if strings.HasPrefix(strings.ToLower(coinCode), "crypto:") {
		info, err := iwallet.CoinInfoFromCoinType(iwallet.CoinType(coinCode))
		if err != nil {
			return railUnknown
		}
		if info.Chain.IsUTXOChain() || info.Chain == iwallet.ChainExternalPayment {
			return railAddressMonitored
		}
		// EVM / Solana / TRON / unknown → client-signed (V1 legacy escrow).
		// ManagedEscrow EVM orders are classified upstream via isManagedEscrowOrder flag before
		// detectSettlementRail is reached (TD-PSS-02 resolved by Phase PS B2).
		return railClientSigned
	}

	// Fallback for non-canonical pass-throughs (best-effort):
	upper := strings.ToUpper(strings.TrimSpace(coinCode))

	// Step 1: native chain tickers — delegate to ChainType methods instead of
	// maintaining a parallel hardcoded list.
	chain := iwallet.ChainType(upper)
	if chain.IsValid() {
		if chain.IsUTXOChain() || chain == iwallet.ChainExternalPayment {
			return railAddressMonitored
		}
		// Valid non-UTXO, non-ExternalPayment chain (EVM / Solana / TRON etc.).
		return railClientSigned
	}

	// Step 2: multi-chain token symbols carry no intrinsic chain context.
	// Conservatively classify as client-signed; Phase B4 ingress policy will
	// make this branch dead code for new orders.
	switch upper {
	case "USDC", "USDT", "DAI", "WETH", "WBTC":
		return railClientSigned
	default:
		return railUnknown
	}
}

// deriveFundingTarget decides the SettlementMode and builds a FundingTargetView.
//
//   - Fiat orders (paymentCoin starts with "fiat:"): provider_checkout.
//   - UTXO / EXTERNAL_PAYMENT orders with a PaymentAddress: address_monitored.
//   - ManagedEscrow EVM orders (isManagedEscrowOrder == true): address_monitored (predicted ManagedEscrow addr).
//   - Legacy EVM / Solana / TRON orders with a PaymentAddress: escrow_v1
//     (buyer must sign escrow contract instructions). The address is still
//     reported in FundingTarget so the UI can display the escrow contract.
//   - Any crypto order without a PaymentAddress: escrow_v1 placeholder
//     (not yet provisioned; call CreateSession to provision).
func (p *PaymentSessionProjector) deriveFundingTarget(
	order *models.Order,
	paymentCoin string,
	expectedAmount string,
	input *projectOrderInput,
) (payment.SettlementMode, payment.FundingTargetView) {
	isManagedEscrowOrder := input.isManagedEscrowOrder
	if input.hasSpec && input.spec.UsesManagedEscrow() {
		isManagedEscrowOrder = true
	}
	// Fiat path
	if strings.HasPrefix(paymentCoin, "fiat:") {
		return payment.SettlementModeProviderCheckout, p.deriveFiatFundingTarget(order, paymentCoin, expectedAmount)
	}

	// Crypto path — address present: discriminate by rail type.
	if order.PaymentAddress != "" {
		target := payment.FundingTargetView{
			Type:    payment.FundingTargetTypeAddress,
			Address: order.PaymentAddress,
			AssetID: paymentCoin, // projector leaves canonical normalisation to the service layer
			Amount:  expectedAmount,
		}

		if input.hasSpec {
			if mode := payment.SettlementModeFromPayMode(input.spec.PayMode); mode != "" {
				return mode, target
			}
		} else if spec, ok := payment.ResolveSettlementSpecFromOrder(order); ok {
			if mode := payment.SettlementModeFromPayMode(spec.PayMode); mode != "" {
				return mode, target
			}
		}

		// Legacy fallback when persisted settlement spec is absent.
		if isManagedEscrowOrder {
			return payment.SettlementModeAddressMonitored, target
		}

		rail := detectSettlementRail(paymentCoin)
		switch rail {
		case railAddressMonitored:
			return payment.SettlementModeAddressMonitored, target
		case railClientSigned:
			// Legacy EVM/Solana/TRON: the address is the escrow contract; the
			// buyer must use their local wallet to sign and submit the on-chain
			// escrow initialisation call. escrow_v1 signals this to the frontend.
			return payment.SettlementModeEscrowV1, target
		default:
			// Unknown coin with an address: conservative fallback to address_monitored.
			// Phase B4 canonical policy enforcement will make this branch unreachable.
			return payment.SettlementModeAddressMonitored, target
		}
	}

	// Crypto path — address not yet provisioned (e.g. order opened but
	// payment not set up). Return a placeholder FundingTargetView with an
	// empty address; the actual mode is inferred from the coin's settlement
	// rail and will be confirmed once CreateSession provisions the address.
	//
	// Clients MUST NOT treat an empty fundingTarget.address as actionable.
	// Detect the unprovisioned state by:
	//   session.Status == "awaiting_funds" && fundingTarget.address == ""
	// and call POST /v1/orders/{orderID}/payment-session to provision.
	target := payment.FundingTargetView{
		Type:    payment.FundingTargetTypeAddress,
		AssetID: paymentCoin,
		Amount:  expectedAmount,
	}
	if input.hasSpec {
		if mode := payment.SettlementModeFromPayMode(input.spec.PayMode); mode != "" {
			return mode, target
		}
	} else if spec, ok := payment.ResolveSettlementSpecFromOrder(order); ok {
		if mode := payment.SettlementModeFromPayMode(spec.PayMode); mode != "" {
			return mode, target
		}
	}
	if isManagedEscrowOrder {
		return payment.SettlementModeAddressMonitored, target
	}
	rail := detectSettlementRail(paymentCoin)
	if rail == railAddressMonitored {
		return payment.SettlementModeAddressMonitored, target
	}
	return payment.SettlementModeEscrowV1, target
}

// deriveFiatFundingTarget builds a FundingTargetView for fiat provider sessions.
//
// The ProviderData map follows the unified session contract (§6.2):
//
//	providerID  — extracted from "fiat:{provider}:{currency}"
//	sessionID   — the provider-side session identifier, resolved in priority order:
//	              1. PaymentTransactionID (written after capture/webhook confirms)
//	              2. fiat_session_id from FiatMetadata (written when CreatePayment
//	                 returns, before the buyer completes the checkout)
//
// Internal metadata keys (fiat_provider, fiat_session_id, fiat_currency) are
// NOT included in ProviderData — they are implementation details and must not
// leak to the API consumer.
func (p *PaymentSessionProjector) deriveFiatFundingTarget(
	order *models.Order,
	paymentCoin string,
	expectedAmount string,
) payment.FundingTargetView {
	target := payment.FundingTargetView{
		Type:    payment.FundingTargetTypeProviderSession,
		AssetID: paymentCoin,
		Amount:  expectedAmount,
	}

	providerData := make(map[string]interface{})

	// providerID from canonical coin format "fiat:{provider}:{currency}".
	parts := strings.SplitN(paymentCoin, ":", 3)
	if len(parts) >= 2 && strings.EqualFold(parts[0], "fiat") && parts[1] != "" {
		providerData["providerID"] = strings.ToLower(parts[1])
	}

	// sessionID: PaymentTransactionID (post-capture) takes precedence;
	// fall back to fiat_session_id (pre-capture, set by CreatePayment).
	sessionID := order.PaymentTransactionID
	if sessionID == "" {
		if meta, err := order.GetFiatMetadata(); err == nil {
			sessionID = meta["fiat_session_id"]
		}
	}
	if sessionID != "" {
		providerData["sessionID"] = sessionID
	}
	if meta, err := order.GetFiatMetadata(); err == nil {
		if shouldExposeFiatRecoveryMetadata(order.PaymentTransactionID, meta) {
			mergeFiatRecoveryMetadata(providerData, meta)
		}
	}

	if len(providerData) > 0 {
		target.ProviderData = providerData
	}
	return target
}

// deriveProgress computes the PaymentProgressView from order state and
// observation metadata.
func (p *PaymentSessionProjector) deriveProgress(
	order *models.Order,
	expectedAmount string,
	obsCount int,
	lastObsAt *time.Time,
) payment.PaymentProgressView {
	observed := order.TotalReceived
	if observed == "" {
		observed = "0"
	}
	remaining := remainingAmount(observed, expectedAmount)

	// isFiatSessionActive is true only when a fiat provider session has been
	// successfully created (fiat_session_id non-empty in FiatMetadata).
	//
	// We intentionally require fiat_session_id, NOT merely len(FiatMetadata) > 0,
	// because FiatMetadata may contain only administrative keys (e.g. fiat_provider
	// written before a successful CreatePayment call) without a real provider session.
	// Using len > 0 would prematurely map such orders to ProviderProcessing when they
	// are still in AwaitingFunds state.
	//
	// FiatPaymentAppService.CreatePayment writes fiat_provider, fiat_session_id,
	// and fiat_currency atomically only when session.SessionID != ""; therefore
	// fiat_session_id non-empty is the authoritative signal that checkout has begun.
	var isFiatSessionActive bool
	if meta, merr := order.GetFiatMetadata(); merr == nil {
		isFiatSessionActive = meta["fiat_session_id"] != ""
	}
	fundingState := deriveFundingState(observed, expectedAmount, order.PaymentVerificationStatus, isFiatSessionActive)

	return payment.PaymentProgressView{
		ObservedAmount:   observed,
		RequiredAmount:   expectedAmount,
		RemainingAmount:  remaining,
		ObservationCount: obsCount,
		LastObservedAt:   lastObsAt,
		FundingState:     fundingState,
	}
}

// deriveExpiry calculates the session expiry from OrderOpen's funding timeout
// or falls back to a 24-hour window from now.
func (p *PaymentSessionProjector) deriveExpiry(orderOpen *pb.OrderOpen) time.Time {
	if orderOpen != nil && orderOpen.Timestamp != nil {
		ts := orderOpen.Timestamp.AsTime()
		// Standard 24-hour funding window from order creation.
		// Phase C will read a per-chain timeout from the order's escrow config.
		return ts.Add(24 * time.Hour)
	}
	return time.Now().Add(24 * time.Hour)
}

// deriveCapabilities decides which settlement actions are permitted in the
// current session status.
func (p *PaymentSessionProjector) deriveCapabilities(
	status payment.SessionStatus,
	productMode payment.ProductMode,
	_ payment.SettlementMode,
) payment.SessionCapabilitiesView {
	switch status {
	case payment.SessionStatusAwaitingFunds, payment.SessionStatusPartiallyFunded:
		return payment.SessionCapabilitiesView{
			CanRefresh: true,
			CanCancel:  true,
		}
	case payment.SessionStatusFundedPendingVerification:
		return payment.SessionCapabilitiesView{
			CanRefresh: true,
			CanCancel:  productMode == payment.ProductModeCancelable,
		}
	case payment.SessionStatusVerified:
		return payment.SessionCapabilitiesView{
			CanConfirm:  true,
			CanComplete: true,
			CanRefund:   true,
		}
	case payment.SessionStatusExpired, payment.SessionStatusCancelled,
		payment.SessionStatusFailed, payment.SessionStatusVerificationFailed:
		return payment.SessionCapabilitiesView{}
	default:
		return payment.SessionCapabilitiesView{CanRefresh: true}
	}
}

// ── Stateless derivation functions ───────────────────────────────────────

// deriveSessionStatus maps the current verification status and funding state
// to the top-level SessionStatus.
func deriveSessionStatus(
	verificationStatus models.PaymentVerificationStatus,
	fundingState payment.FundingState,
) payment.SessionStatus {
	switch verificationStatus {
	case models.PaymentVerificationStatusVerified:
		return payment.SessionStatusVerified
	case models.PaymentVerificationStatusFailed:
		return payment.SessionStatusVerificationFailed
	}

	// Pending or unset verification — derive from funding progress
	switch fundingState {
	case payment.FundingStateFullyFunded, payment.FundingStateOverfunded:
		return payment.SessionStatusFundedPendingVerification
	case payment.FundingStatePartiallyFunded:
		return payment.SessionStatusPartiallyFunded
	case payment.FundingStateExpiredUnfunded:
		return payment.SessionStatusExpired
	case payment.FundingStateProviderProcessing:
		return payment.SessionStatusFundedPendingVerification
	default:
		return payment.SessionStatusAwaitingFunds
	}
}

// deriveFundingState maps observed vs required amounts and verification
// status to a FundingState.
//
// isFiatSession must be true when a fiat provider session (Stripe / PayPal)
// has been successfully created for the order (i.e. fiat_session_id is present
// in FiatMetadata). Callers must NOT pass true merely because FiatMetadata is
// non-empty; only a confirmed session justifies the ProviderProcessing state.
//
// Fiat state machine (isFiatSession == true):
//
//	status == ""       → ProviderProcessing (session open, buyer not yet approved)
//	status == pending  → ProviderProcessing (authorized, not yet captured)
//	status == verified → FullyFunded        (captured + webhook confirmed)
//	status == failed   → ExpiredUnfunded    (provider-side failure)
//
// When isFiatSession == false the crypto path (on-chain amounts) is used.
func deriveFundingState(
	observed, expected string,
	verificationStatus models.PaymentVerificationStatus,
	isFiatSession bool,
) payment.FundingState {
	if isFiatSession {
		switch verificationStatus {
		case models.PaymentVerificationStatusVerified:
			return payment.FundingStateFullyFunded
		case models.PaymentVerificationStatusFailed:
			return payment.FundingStateExpiredUnfunded
		default:
			// "" and "pending" both mean the session is open but funds not yet captured.
			return payment.FundingStateProviderProcessing
		}
	}

	// Crypto path: derive from on-chain observed amounts.
	obs := parseBigInt(observed)
	exp := parseBigInt(expected)
	if exp == nil || exp.Sign() == 0 {
		return payment.FundingStateAwaitingFunds
	}
	if obs == nil || obs.Sign() == 0 {
		return payment.FundingStateAwaitingFunds
	}
	cmp := obs.Cmp(exp)
	switch {
	case cmp < 0:
		return payment.FundingStatePartiallyFunded
	case cmp == 0:
		return payment.FundingStateFullyFunded
	default:
		return payment.FundingStateOverfunded
	}
}

// remainingAmount computes max(0, expected - observed) as a decimal string.
func remainingAmount(observed, expected string) string {
	obs := parseBigInt(observed)
	exp := parseBigInt(expected)
	if obs == nil {
		obs = big.NewInt(0)
	}
	if exp == nil {
		exp = big.NewInt(0)
	}
	diff := new(big.Int).Sub(exp, obs)
	if diff.Sign() < 0 {
		return "0"
	}
	return diff.String()
}

// parseBigInt parses a decimal string into *big.Int. Returns nil on error.
func parseBigInt(s string) *big.Int {
	if s == "" {
		return nil
	}
	v, ok := new(big.Int).SetString(s, 10)
	if !ok {
		return nil
	}
	return v
}

// ── DB fetch helper ───────────────────────────────────────────────────────

// fetchProjectInput loads the data needed to project a PaymentSession.
// It returns an error if the order is not found.
func (p *PaymentSessionProjector) fetchProjectInput(orderID string) (*projectOrderInput, error) {
	var order models.Order
	err := p.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID).First(&order).Error
	})
	if err != nil {
		return nil, fmt.Errorf("payment session projector: load order %s: %w", orderID, err)
	}

	input := &projectOrderInput{order: &order}

	// OrderOpen (for expected amount, timestamp)
	if oo, err := order.OrderOpenMessage(); err == nil {
		input.orderOpen = oo
	}

	// PaymentSent (for payment coin, method)
	if ps, err := order.PaymentSentMessage(); err == nil {
		input.paymentSent = ps
	}

	if spec, ok := payment.ResolveSettlementSpecFromOrder(&order); ok {
		input.hasSpec = true
		input.spec = spec
	}
	// Legacy fallback when settlement spec is not persisted on the order.
	if !input.hasSpec {
		if managed_escrowInfo, err := order.GetPendingManagedEscrowPaymentInfo(); err == nil && managed_escrowInfo != nil {
			input.isManagedEscrowOrder = true
		}
	}

	// Observation count + last observed timestamp
	obsCount, lastObsAt, err := p.queryObservationMeta(orderID)
	if err == nil {
		input.obsCount = obsCount
		input.lastObsAt = lastObsAt
	}

	return input, nil
}

// queryObservationMeta returns the count of confirmed observations and the
// most recent block time for the given order.
func (p *PaymentSessionProjector) queryObservationMeta(orderID string) (int, *time.Time, error) {
	var count int64
	var lastBlockTime *time.Time

	err := p.db.View(func(tx database.Tx) error {
		// Count confirmed observations
		if err := tx.Read().
			Model(&models.PaymentObservation{}).
			Where("order_id = ? AND status = ?", orderID, models.PaymentObservationStatusConfirmed).
			Count(&count).Error; err != nil {
			return err
		}

		// Fetch most recent block time
		var obs models.PaymentObservation
		if err := tx.Read().
			Where("order_id = ? AND status = ?", orderID, models.PaymentObservationStatusConfirmed).
			Order("block_time DESC").
			First(&obs).Error; err == nil {
			t := obs.BlockTime
			lastBlockTime = &t
		}
		return nil
	})
	if err != nil {
		return 0, nil, err
	}
	return int(count), lastBlockTime, nil
}
