//go:build !private_distribution

package payment

import (
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/mobazha/mobazha3.0/internal/core/paymentintent"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"gorm.io/gorm"
)

// PaymentSessionProjector assembles a payment.PaymentSession from existing
// order, shared payment-intent, payment-observation, and fiat metadata.
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
	order             *models.Order
	orderOpen         *pb.OrderOpen   // may be nil for orders not yet opened
	paymentSent       *pb.PaymentSent // may be nil for orders not yet paid
	observedAmountRaw string
	obsCount          int
	lastObsAt         *time.Time
	observations      []models.PaymentObservation
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
	paymentCoin, productMode := p.derivePaymentInfo(order, input.orderOpen, input.paymentSent)

	// ── Expected amount ───────────────────────────────────────────────────
	// Use the order's canonical expected-amount resolver so address-monitored
	// rails (UTXO / ManagedEscrow) project only their locked pending amount instead of
	// the signed listing amount from OrderOpen, which may be a different coin.
	expectedAmountRaw := strings.TrimSpace(order.ExpectedPaymentAmountString())
	expectedAmount := payment.FormatSessionAmount(expectedAmountRaw, paymentCoin)

	// ── Settlement mode & funding target ─────────────────────────────────
	settlementMode, fundingTarget := p.deriveFundingTarget(order, paymentCoin, expectedAmount, input)

	// ── Payment progress ──────────────────────────────────────────────────
	progressRows, observedAmountRaw, obsCount, lastObsAt := progressObservationSnapshot(order, input)
	progress := p.deriveProgress(order, expectedAmountRaw, paymentCoin, observedAmountRaw, obsCount, lastObsAt, progressRows)

	// ── Session status ────────────────────────────────────────────────────
	status := deriveSessionStatus(order.PaymentVerificationStatus, progress.FundingState)

	// ── Expiry (best-effort from OrderOpen timeout) ───────────────────────
	expiresAt := p.deriveExpiry(input.orderOpen)

	// ── Capabilities ──────────────────────────────────────────────────────
	caps := p.deriveCapabilities(status, productMode, settlementMode)

	readiness := payment.DerivePaymentReadiness(order, expiresAt)

	session := &payment.PaymentSession{
		SessionID:          sessionID,
		OrderID:            order.ID.String(),
		PaymentCoin:        paymentCoin,
		SettlementMode:     settlementMode,
		ProductMode:        productMode,
		Status:             status,
		ConfirmationPolicy: p.deriveConfirmationPolicy(order),
		ExpectedAmount:     expectedAmount,
		RefundAddress:      order.RefundAddress,
		ExpiresAt:          expiresAt,
		FundingTarget:      fundingTarget,
		PaymentProgress:    progress,
		Capabilities:       caps,
		PaymentReadiness:   readiness,
		UserActionRequest:  nil, // Phase B: no user action required for address_monitored
	}
	payment.ApplyBuyerPaymentReadinessGate(session)
	return session, nil
}

// ── Private derivation helpers ────────────────────────────────────────────

// derivePaymentInfo extracts canonical payment coin and product mode.
//
// Resolution order:
//  1. PaymentSent (most authoritative — order already paid)
//  2. FiatMetadata["fiat_provider"] + FiatMetadata["fiat_currency"] —
//     for fiat orders where the buyer has not yet completed payment
//     (i.e. PaymentSent_FIAT is written only after the provider confirms).
//  3. PendingPaymentInfo["Coin"] — for UTXO orders whose address is set
//     but PaymentSent has not yet been received.
func (p *PaymentSessionProjector) derivePaymentInfo(
	order *models.Order,
	orderOpen *pb.OrderOpen,
	ps *pb.PaymentSent,
) (
	paymentCoin string,
	productMode payment.ProductMode,
) {
	// ── 1. PaymentSent present ───────────────────────────────────────────
	if ps != nil {
		paymentCoin = normalizeCoinBestEffort(ps.Coin)
		spec, ok := payment.ResolveSettlementSpec(order, ps)
		if !ok {
			return paymentCoin, productMode
		}
		return paymentCoin, payment.ProductModeFromMethod(spec.Method)
	}

	// ── 2. PendingPaymentInfo (crypto address provisioned, not yet paid) ──
	//
	// A buyer may open a provider checkout first and then switch to a crypto
	// address before paying. In that case FiatMetadata can remain as stale
	// recovery data, but PaymentAddress + PendingPaymentInfo are the active
	// funding target and must win the projection.
	if strings.TrimSpace(order.PaymentAddress) != "" {
		if spec, ok := payment.ResolveSettlementSpecFromOrder(order); ok {
			coin := pendingPaymentCoin(order)
			return coin, payment.ProductModeFromMethod(spec.Method)
		}
	}

	// ── 3. FiatMetadata fallback (fiat order, awaiting buyer action) ──────
	if fiatMeta, err := order.GetFiatMetadata(); err == nil {
		provider := fiatMeta["fiat_provider"]
		if provider != "" {
			productMode := payment.ProductModeFromMethod(pb.PaymentSent_FIAT)
			if spec, ok := payment.ResolveSettlementSpecFromOrder(order); ok {
				productMode = payment.ProductModeFromMethod(spec.Method)
			}
			currency := fiatMeta["fiat_currency"]
			if currency == "" && orderOpen != nil {
				currency = orderOpen.PricingCoin
			}
			if currency != "" {
				paymentCoin = fmt.Sprintf("fiat:%s:%s",
					strings.ToLower(strings.TrimSpace(provider)),
					strings.ToUpper(strings.TrimSpace(currency)))
				return paymentCoin, productMode
			}
			// Legacy rows with provider metadata but no currency context cannot be
			// projected to a canonical fiat coin safely. Leave paymentCoin empty
			// rather than leaking the invalid external shape "fiat:{provider}".
			return "", productMode
		}
	}

	// ── 4. PendingPaymentInfo / fiat metadata (pre-PaymentSent) ───────────
	if spec, ok := payment.ResolveSettlementSpecFromOrder(order); ok {
		coin := pendingPaymentCoin(order)
		return coin, payment.ProductModeFromMethod(spec.Method)
	}

	return "", payment.ProductModeCancelable
}

func pendingPaymentCoin(order *models.Order) string {
	if managed_escrowInfo, err := order.GetPendingManagedEscrowPaymentInfo(); err == nil && managed_escrowInfo != nil && managed_escrowInfo.Coin != "" {
		return normalizeCoinBestEffort(managed_escrowInfo.Coin)
	}
	if escrowInfo, err := order.GetPendingEscrowPaymentInfo(); err == nil && escrowInfo != nil && escrowInfo.Coin != "" {
		return normalizeCoinBestEffort(escrowInfo.Coin)
	}
	if utxoInfo, err := order.GetPendingPaymentInfo(); err == nil && utxoInfo != nil && utxoInfo.Coin != "" {
		return normalizeCoinBestEffort(utxoInfo.Coin)
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

func (p *PaymentSessionProjector) deriveConfirmationPolicy(order *models.Order) string {
	if order == nil {
		return ""
	}
	info, err := order.GetPendingPaymentInfo()
	if err != nil || info == nil {
		return ""
	}
	return models.NormalizePaymentConfirmationPolicy(info.ConfirmationPolicy)
}

// deriveFundingTarget decides the SettlementMode and builds a FundingTargetView.
//
//   - Fiat orders (paymentCoin starts with "fiat:"): provider_checkout.
//   - Crypto orders: address_monitored. Client-signed funding has been retired
//     from the product contract; any remaining legacy records are projected as
//     address-monitored so the frontend never re-enters the old instructions UI.
func (p *PaymentSessionProjector) deriveFundingTarget(
	order *models.Order,
	paymentCoin string,
	expectedAmount string,
	_ *projectOrderInput,
) (payment.SettlementMode, payment.FundingTargetView) {
	// Fiat path
	if strings.HasPrefix(paymentCoin, "fiat:") {
		return payment.SettlementModeProviderCheckout, p.deriveFiatFundingTarget(order, paymentCoin, expectedAmount)
	}

	// Crypto path. Empty address means the session still needs provisioning via
	// POST /v1/orders/{orderID}/payment-session; it is not an actionable target.
	target := payment.FundingTargetView{
		Type:      payment.FundingTargetTypeAddress,
		Address:   order.PaymentAddress,
		AssetID:   paymentCoin,
		Amount:    expectedAmount,
		QRPayload: buildAddressQRPayload(paymentCoin, order.PaymentAddress, expectedAmount),
	}
	return payment.SettlementModeAddressMonitored, target
}

func buildAddressQRPayload(paymentCoin, address, amount string) string {
	scheme := paymentURIScheme(paymentCoin)
	if scheme == "" {
		return ""
	}

	addr := strings.TrimSpace(address)
	if addr == "" {
		return ""
	}

	payload := addr
	if !strings.HasPrefix(strings.ToLower(addr), scheme+":") {
		payload = scheme + ":" + addr
	}

	amt := strings.TrimSpace(amount)
	if !isPositiveDecimal(amt) {
		return payload
	}
	if strings.Contains(payload, "?") {
		return payload + "&amount=" + amt
	}
	return payload + "?amount=" + amt
}

func paymentURIScheme(paymentCoin string) string {
	switch coin := strings.ToLower(strings.TrimSpace(paymentCoin)); {
	case coin == "btc", strings.HasPrefix(coin, "crypto:bip122:000000000019d6689c085ae165831e93:"), strings.HasPrefix(coin, "crypto:bitcoin:"):
		return "bitcoin"
	case coin == "ltc", strings.HasPrefix(coin, "crypto:bip122:12a765e31ffd4059bada1e25190f6e98:"), strings.HasPrefix(coin, "crypto:litecoin:"):
		return "litecoin"
	case coin == "bch", strings.HasPrefix(coin, "crypto:bitcoincash:"):
		return "bitcoincash"
	case coin == "zec", strings.HasPrefix(coin, "crypto:zcash:"):
		return "zcash"
	default:
		return ""
	}
}

func isPositiveDecimal(amount string) bool {
	if amount == "" {
		return false
	}
	v, ok := new(big.Rat).SetString(amount)
	return ok && v.Sign() > 0
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
	expectedAmountRaw string,
	paymentCoin string,
	observedAmountRaw string,
	obsCount int,
	lastObsAt *time.Time,
	observations []models.PaymentObservation,
) payment.PaymentProgressView {
	observedRaw := strings.TrimSpace(observedAmountRaw)
	if observedRaw == "" {
		observedRaw = order.TotalReceived
	}
	if strings.TrimSpace(observedRaw) == "" {
		if paymentSent, err := order.PaymentSentMessage(); err == nil && paymentSent != nil {
			observedRaw = strings.TrimSpace(paymentSent.GetAmount())
		}
	}
	if strings.TrimSpace(observedRaw) == "" {
		observedRaw = "0"
	}
	remainingRaw := remainingAmount(observedRaw, expectedAmountRaw)

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
	fundingState := deriveFundingState(
		observedRaw,
		expectedAmountRaw,
		order.PaymentVerificationStatus,
		isFiatSessionActive,
	)

	return payment.PaymentProgressView{
		ObservedAmount:   payment.FormatSessionAmount(observedRaw, paymentCoin),
		RequiredAmount:   payment.FormatSessionAmount(expectedAmountRaw, paymentCoin),
		RemainingAmount:  payment.FormatSessionAmount(remainingRaw, paymentCoin),
		ObservationCount: obsCount,
		LastObservedAt:   lastObsAt,
		FundingState:     fundingState,
		Observations:     paymentObservationViews(observations, paymentCoin),
	}
}

func paymentObservationViews(rows []models.PaymentObservation, paymentCoin string) []payment.ObservedPaymentView {
	if len(rows) == 0 {
		return nil
	}
	views := make([]payment.ObservedPaymentView, 0, len(rows))
	for i := range rows {
		row := rows[i]
		var observedAt *time.Time
		if !row.BlockTime.IsZero() {
			t := row.BlockTime
			observedAt = &t
		} else if !row.CreatedAt.IsZero() {
			t := row.CreatedAt
			observedAt = &t
		}
		views = append(views, payment.ObservedPaymentView{
			ID:             row.ID,
			TxHash:         row.TxHash,
			TxHashSource:   models.NormalizePaymentTxHashSource(row.TxHashSource),
			HasChainTxHash: row.HasChainTxHash(),
			EventIndex:     row.EventIndex,
			EventType:      row.EventType,
			Amount:         payment.FormatSessionAmount(row.Amount, paymentCoin),
			RawAmount:      row.Amount,
			ChainNamespace: row.ChainNamespace,
			ChainReference: row.ChainReference,
			FromAddress:    row.FromAddress,
			ToAddress:      row.ToAddress,
			TokenAddress:   row.TokenAddress,
			BlockNumber:    row.BlockNumber,
			Confirmations:  row.Confirmations,
			Status:         row.Status,
			Source:         row.Source,
			ObservedAt:     observedAt,
		})
	}
	return views
}

func progressObservationSnapshot(order *models.Order, input *projectOrderInput) ([]models.PaymentObservation, string, int, *time.Time) {
	if input == nil {
		return nil, "", 0, nil
	}
	rows := append([]models.PaymentObservation(nil), input.observations...)
	if order != nil {
		rows = append(rows, paymentSentFundingFactRows(order, input.paymentSent)...)
	}
	if len(rows) == 0 {
		return nil, input.observedAmountRaw, input.obsCount, input.lastObsAt
	}
	rows = models.DedupePaymentObservations(rows)
	total, err := sumObservations(rows)
	if err != nil {
		return input.observations, input.observedAmountRaw, input.obsCount, input.lastObsAt
	}
	var lastSeen *time.Time
	for i := range rows {
		t := rows[i].BlockTime
		if !t.IsZero() && (lastSeen == nil || t.After(*lastSeen)) {
			copy := t
			lastSeen = &copy
		}
	}
	if lastSeen == nil {
		lastSeen = input.lastObsAt
	}
	return rows, total.String(), len(rows), lastSeen
}

func paymentSentFundingFactRows(order *models.Order, paymentSent *pb.PaymentSent) []models.PaymentObservation {
	if order == nil {
		return nil
	}
	ps := paymentSent
	if ps == nil {
		var err error
		ps, err = order.PaymentSentMessage()
		if err != nil {
			return nil
		}
	}
	if len(ps.GetFundingFacts()) == 0 {
		return nil
	}
	rows := make([]models.PaymentObservation, 0, len(ps.GetFundingFacts()))
	for i, fact := range ps.GetFundingFacts() {
		if fact == nil {
			continue
		}
		id := strings.TrimSpace(fact.GetId())
		if id == "" {
			id = fmt.Sprintf("paymentsent:%s:%d", fact.GetTxHash(), fact.GetEventIndex())
		}
		blockTime := time.Time{}
		if fact.GetObservedAt() != nil {
			blockTime = fact.GetObservedAt().AsTime()
		}
		source := strings.TrimSpace(fact.GetSource())
		if source == "" {
			source = models.PaymentObservationSourceBuyerReported
		}
		status := strings.TrimSpace(fact.GetStatus())
		if status == "" {
			status = models.PaymentObservationStatusConfirmed
		}
		txHashSource := models.NormalizePaymentTxHashSource(fact.GetTxHashSource())
		rows = append(rows, models.PaymentObservation{
			TenantID:       order.TenantID,
			ID:             id,
			OrderID:        order.ID.String(),
			ChainNamespace: fact.GetChainNamespace(),
			ChainReference: fact.GetChainReference(),
			TxHash:         fact.GetTxHash(),
			EventIndex:     int(fact.GetEventIndex()),
			TxHashSource:   txHashSource,
			EventType:      fact.GetEventType(),
			FromAddress:    fact.GetFromAddress(),
			ToAddress:      fact.GetToAddress(),
			TokenAddress:   fact.GetTokenAddress(),
			Amount:         fact.GetAmount(),
			BlockNumber:    fact.GetBlockNumber(),
			BlockTime:      blockTime,
			Confirmations:  int(fact.GetConfirmations()),
			Status:         status,
			Source:         source,
			Observer:       fmt.Sprintf("paymentsent:%d", i),
		})
	}
	return rows
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
	if rawProvider, ok := p.db.(interface{ RawDB() *gorm.DB }); ok {
		if raw := rawProvider.RawDB(); raw != nil {
			if err := paymentintent.HydrateOrderFromSharedIntent(raw, input.order); err != nil {
				return nil, fmt.Errorf("payment session projector: hydrate shared intent %s: %w", orderID, err)
			}
		}
	}

	// OrderOpen (for expected amount, timestamp)
	if oo, err := order.OrderOpenMessage(); err == nil {
		input.orderOpen = oo
	}

	// PaymentSent (for payment coin, method)
	if ps, err := order.PaymentSentMessage(); err == nil {
		input.paymentSent = ps
	}

	// Observation progress includes pending rows so payment sessions can show
	// mempool-detected funds without marking the order chain-verified.
	observedAmountRaw, obsCount, lastObsAt, observations, err := p.queryObservationProgress(order.TenantID, orderID)
	if err == nil {
		input.observedAmountRaw = observedAmountRaw
		input.obsCount = obsCount
		input.lastObsAt = lastObsAt
		input.observations = observations
	}

	return input, nil
}

// queryObservationProgress returns deduplicated pending-or-confirmed observed
// funds for UI progress. Verification still reads confirmed rows only.
func (p *PaymentSessionProjector) queryObservationProgress(tenantID, orderID string) (string, int, *time.Time, []models.PaymentObservation, error) {
	if tenantID == "" || orderID == "" {
		return "", 0, nil, nil, fmt.Errorf("payment session projector: tenantID and orderID must be set")
	}

	var rows []models.PaymentObservation
	var lastBlockTime *time.Time

	err := p.db.View(func(tx database.Tx) error {
		return tx.Read().
			Where("tenant_id = ? AND order_id = ? AND status IN ?", tenantID, orderID, []string{
				models.PaymentObservationStatusPending,
				models.PaymentObservationStatusConfirmed,
			}).
			Order("block_time ASC, id ASC").
			Find(&rows).Error
	})
	if err != nil {
		return "", 0, nil, nil, err
	}
	rows = models.DedupePaymentObservations(rows)
	total, err := sumObservations(rows)
	if err != nil {
		return "", 0, nil, nil, err
	}
	for i := range rows {
		t := rows[i].BlockTime
		if lastBlockTime == nil || t.After(*lastBlockTime) {
			lastBlockTime = &t
		}
	}
	return total.String(), len(rows), lastBlockTime, rows, nil
}
