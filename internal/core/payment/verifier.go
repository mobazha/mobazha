//go:build !private_distribution

package payment

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/mobazha/mobazha3.0/internal/core/paymentintent"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	paymentmetrics "github.com/mobazha/mobazha3.0/pkg/payment"

	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AggregatingVerifier implements the Sprint 2A step 4 of the
// Monitor-Driven Payment model (docs/escrow/MONITOR_DRIVEN_PAYMENT.md §5.2).
//
// It is the only component allowed to write Order.SerializedPaymentSent
// when the payment originates on-chain: it loads every confirmed
// PaymentObservation for the order, dedupes them through the canonical
// pkg/models.DedupePaymentObservations rule, sums the resulting amounts
// and writes the verdict (verified / pending / overpaid) back to the
// order in the same DB transaction that observed the rows.
//
// ─────────────────────────────────────────────────────────────────────────
// Concurrency contract
// ─────────────────────────────────────────────────────────────────────────
//
// The whole aggregate-and-emit cycle runs inside a single database.Tx:
//
//  1. SELECT the order row scoped by (tenant_id, id) WITH a row-level lock
//     where the dialect supports it.
//  2. SELECT confirmed observations for the order, also scoped by tenant.
//  3. Compute the running sum + decide the verdict.
//  4. UPDATE the order's payment_verification_status / serialized_payment_sent
//     / total_received / overpaid_amount.
//
// Without the row-level lock two concurrent ObservationDispatcher.OnNewBlock
// fan-outs that touch the same order could each see the same row count and
// each emit PaymentVerified, double-firing downstream FSM transitions. On
// PostgreSQL/MySQL/SQL Server the FOR UPDATE clause forces the second
// worker to wait on the first one's COMMIT, after which it re-loads the
// now-verified order and short-circuits on the IsPaymentVerified() guard
// below — that is the strict guarantee.
//
// SQLite-specific caveat: GORM's default `BEGIN` is DEFERRED, so two
// concurrent verifiers can both SELECT the same pending row before either
// of them upgrades the lock on first write. The first UPDATE wins; the
// second receives SQLITE_BUSY and is retried — but in the rare interleaving
// where both COMMIT, both will have observed `IsPaymentVerified() == false`
// and both will emit. We treat SQLite as "best-effort serialisation" and
// rely on (a) the standalone deployment running a single verifier worker
// per process, and (b) the OrderAppService subscriber dedup'ing
// PaymentVerified by status before triggering downstream side effects.
// SaaS / production runs PostgreSQL where the strict guarantee holds.
//
// ─────────────────────────────────────────────────────────────────────────
// Idempotency contract
// ─────────────────────────────────────────────────────────────────────────
//
// The function emits events.PaymentVerified at most once per order across
// the lifetime of the deployment:
//
//   - First call that sees total >= expected → builds the PaymentSent
//     envelope, marks PaymentVerificationStatus = verified, emits the
//     event AFTER the surrounding transaction commits.
//   - Subsequent calls (e.g. another deposit landing later) → see
//     IsPaymentVerified() == true on entry, refresh TotalReceived /
//     OverpaidAmount but DO NOT re-emit the event and DO NOT rewrite
//     SerializedPaymentSent (the envelope is the chain-of-trust target
//     used by the seller's order processor; rewriting it would invalidate
//     downstream signatures).
//
// Partial-state calls (total < expected) update TotalReceived in place and
// leave the order in PaymentVerificationStatusPending, so the dashboard /
// QR-refresh path can show "you've paid 6 of 10" without flipping the FSM.
//
// ─────────────────────────────────────────────────────────────────────────
// Error semantics
// ─────────────────────────────────────────────────────────────────────────
//
// Errors are surfaced verbatim so the caller (ObservationDispatcher) can
// roll up multiple AggregateAndEmit failures via errors.Join. The
// constructor panics on nil db / nil bus to fail loud at wiring time;
// AggregateAndEmit returns nil for unknown orders (a stray event that
// references an order this node has never seen — see §5.1 of the design
// doc, which prescribes "log-and-skip" for these).
type AggregatingVerifier struct {
	db                     database.Database
	bus                    events.Bus
	clock                  func() time.Time
	paymentVerifiedHandler func(orderID string, paymentSent *pb.PaymentSent)
}

// NewAggregatingVerifier wires the verifier with the tenant-scoped
// database handle (used to open the read+write transaction that locks
// the order and reads the observations) and the EventBus (used to emit
// the one-shot PaymentVerified signal after a successful first
// transition).
//
// Both arguments are required. We panic rather than return an error to
// catch wiring bugs at boot — this constructor is invoked exactly once
// per node startup, so the cost of the panic is bounded and the
// alternative (returning an error and silently no-op'ing in tests that
// forget to plumb it) has historically led to "verifier never fires"
// regressions.
func NewAggregatingVerifier(db database.Database, bus events.Bus) *AggregatingVerifier {
	if db == nil {
		panic("payment.AggregatingVerifier: db must not be nil")
	}
	if bus == nil {
		panic("payment.AggregatingVerifier: bus must not be nil")
	}
	return &AggregatingVerifier{db: db, bus: bus, clock: time.Now}
}

// SetClock overrides the time source for tests. Production code should
// rely on the default (time.Now); tests use this hook to pin the
// PaymentSent.Timestamp value for deterministic envelope assertions.
func (v *AggregatingVerifier) SetClock(clock func() time.Time) {
	if clock == nil {
		v.clock = time.Now
		return
	}
	v.clock = clock
}

// SetPaymentVerifiedHandler registers a callback invoked after a monitor-driven
// crypto payment is confirmed and the surrounding DB transaction commits.
func (v *AggregatingVerifier) SetPaymentVerifiedHandler(fn func(orderID string, paymentSent *pb.PaymentSent)) {
	v.paymentVerifiedHandler = fn
}

// AggregateAndEmit recomputes the funded / partial / overpaid verdict for
// the order from its current set of confirmed observations and persists
// the result. See the AggregatingVerifier doc for the full contract.
//
// Returning nil with no side effect is the correct behaviour for:
//   - tenantID/orderID empty (validation; we still reject these to fail
//     fast on caller bugs);
//   - order does not exist (we are likely racing an order delete or
//     receiving observations for a different node's order);
//   - order already verified (idempotent — this is a re-aggregation
//     triggered by a later observation arriving).
func (v *AggregatingVerifier) AggregateAndEmit(ctx context.Context, tenantID, orderID string) error {
	tenantID = strings.TrimSpace(tenantID)
	orderID = strings.TrimSpace(orderID)
	if tenantID == "" {
		return fmt.Errorf("aggregating verifier: tenantID must be set")
	}
	if orderID == "" {
		return fmt.Errorf("aggregating verifier: orderID must be set")
	}
	start := time.Now()
	defer func() {
		paymentmetrics.ObservePaymentAggregationDuration(tenantID, time.Since(start))
	}()

	// emitVerified is captured by the closure and consulted AFTER the
	// transaction commits. We never emit inside the closure: emitting
	// before COMMIT would surface a verified-but-unrecorded event to
	// subscribers if the transaction later rolled back.
	var (
		emitVerified          bool
		emitVerifiedNamespace string
		emitBusinessEvents    []interface{}
		emitHandlerOrderID    string
		emitHandlerPayment    *pb.PaymentSent
	)

	var err error
	if rawProvider, ok := v.db.(interface{ RawDB() *gorm.DB }); ok {
		raw := rawProvider.RawDB()
		if raw == nil {
			return fmt.Errorf("aggregating verifier: raw DB unavailable")
		}
		err = raw.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			return v.aggregateWithGorm(ctx, tx, func(order *models.Order) error {
				return tx.Save(order).Error
			}, tenantID, orderID, &emitVerified, &emitVerifiedNamespace, &emitBusinessEvents, &emitHandlerOrderID, &emitHandlerPayment)
		})
	} else {
		err = v.db.Update(func(tx database.Tx) error {
			gdb := tx.Read().WithContext(ctx)
			return v.aggregateWithGorm(ctx, gdb, func(order *models.Order) error {
				return tx.Save(order)
			}, tenantID, orderID, &emitVerified, &emitVerifiedNamespace, &emitBusinessEvents, &emitHandlerOrderID, &emitHandlerPayment)
		})
	}
	if err != nil {
		return err
	}

	if emitVerified {
		v.bus.Emit(events.PaymentVerified{TenantID: tenantID, OrderID: orderID})
		if v.paymentVerifiedHandler != nil && emitHandlerPayment != nil && emitHandlerOrderID != "" {
			go v.paymentVerifiedHandler(emitHandlerOrderID, emitHandlerPayment)
		}
		paymentmetrics.RecordPaymentAggregationEnvelopeEmitted(tenantID, emitVerifiedNamespace, orderID)
	}
	for _, evt := range emitBusinessEvents {
		v.bus.Emit(evt)
	}
	return nil
}

func (v *AggregatingVerifier) aggregateWithGorm(
	ctx context.Context,
	gdb *gorm.DB,
	saveOrder func(*models.Order) error,
	tenantID, orderID string,
	emitVerified *bool,
	emitVerifiedNamespace *string,
	emitBusinessEvents *[]interface{},
	emitHandlerOrderID *string,
	emitHandlerPayment **pb.PaymentSent,
) error {
	gdb = gdb.WithContext(ctx)

	// Step 1: lock the order row. We scope the WHERE on the full composite
	// primary key (tenant_id, id) so a SaaS deployment can never accidentally
	// lock or read a different tenant's order when OrderIDs collide across
	// tenants.
	var order models.Order
	loader := gdb
	if dialectSupportsRowLock(gdb) {
		loader = gdb.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := loader.
		Where("tenant_id = ? AND id = ?", tenantID, orderID).
		First(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil // unknown order — log-and-skip per §5.1.
		}
		return fmt.Errorf("aggregating verifier: load order %s: %w", orderID, err)
	}

	alreadyVerified := order.IsPaymentVerified()
	if err := hydrateSharedManagedEscrowMetadata(gdb, &order); err != nil {
		return fmt.Errorf("aggregating verifier: hydrate shared metadata for %s: %w", orderID, err)
	}
	sharedRefund := loadSharedRefundAddress(gdb, orderID)
	if err := hydrateCotenantEscrowMetadata(gdb, &order); err != nil {
		return fmt.Errorf("aggregating verifier: hydrate co-tenant escrow metadata for %s: %w", orderID, err)
	}

	orderOpen, err := order.OrderOpenMessage()
	if err != nil {
		return fmt.Errorf("aggregating verifier: decode order_open for %s: %w", orderID, err)
	}

	expected, err := parseExpectedAmount(&order, orderOpen)
	if err != nil {
		return fmt.Errorf("aggregating verifier: order %s: %w", orderID, err)
	}

	var rows []models.PaymentObservation
	if err := gdb.
		Where("tenant_id = ? AND order_id = ? AND status IN ?",
			tenantID, orderID, observationStatusesForVerification(&order)).
		Order("block_time ASC, id ASC").
		Find(&rows).Error; err != nil {
		return fmt.Errorf("aggregating verifier: load observations for %s: %w", orderID, err)
	}

	deduped := models.DedupePaymentObservations(rows)
	total, err := sumObservations(deduped)
	if err != nil {
		return fmt.Errorf("aggregating verifier: order %s: %w", orderID, err)
	}

	order.TotalReceived = total.String()

	cmp := total.Cmp(expected)
	switch {
	case cmp < 0:
		if !alreadyVerified {
			order.MarkPaymentVerificationPending()
		}
		order.OverpaidAmount = ""

	default:
		if cmp > 0 {
			surplus := new(big.Int).Sub(total, expected)
			order.OverpaidAmount = surplus.String()
		} else {
			order.OverpaidAmount = ""
		}

		if alreadyVerified {
			promoteAfterVerification(&order, v.clock())
			if err := v.tryRecoverVerifiedCancelableAutoConfirm(
				&order, orderOpen, deduped, total, sharedRefund,
				emitBusinessEvents, emitHandlerOrderID, emitHandlerPayment,
			); err != nil {
				return fmt.Errorf("aggregating verifier: recover cancelable auto-confirm for %s: %w", orderID, err)
			}
			break
		}

		ps, err := buildAggregatedPaymentSent(orderOpen, deduped, total, &order, sharedRefund, v.clock())
		if err != nil {
			return fmt.Errorf("aggregating verifier: build PaymentSent for %s: %w", orderID, err)
		}
		if err := order.SetPaymentSent(ps); err != nil {
			return fmt.Errorf("aggregating verifier: marshal PaymentSent for %s: %w", orderID, err)
		}
		backfillResolvedBuyerRefundAddress(&order, ps, deduped)
		if order.PaymentAddress == "" && ps.ToAddress != "" {
			order.PaymentAddress = ps.ToAddress
		}
		order.MarkPaymentVerified()
		promoteAfterVerification(&order, v.clock())
		*emitVerified = true
		*emitVerifiedNamespace = deduped[0].ChainNamespace
		*emitBusinessEvents = paymentVerifiedBusinessEvents(&order, orderOpen, ps, total)
		*emitHandlerOrderID = order.ID.String()
		*emitHandlerPayment = ps
	}

	if err := saveOrder(&order); err != nil {
		return fmt.Errorf("aggregating verifier: save order %s: %w", orderID, err)
	}
	return nil
}

// tryRecoverVerifiedCancelableAutoConfirm backfills PaymentSent funding facts
// and emits CancelablePaymentReady when a vendor order was verified early (e.g.
// via P2P PaymentSent) before UTXO settlement inputs were available.
func (v *AggregatingVerifier) tryRecoverVerifiedCancelableAutoConfirm(
	order *models.Order,
	orderOpen *pb.OrderOpen,
	observations []models.PaymentObservation,
	total *big.Int,
	sharedRefund string,
	emitBusinessEvents *[]interface{},
	emitHandlerOrderID *string,
	emitHandlerPayment **pb.PaymentSent,
) error {
	if order == nil || orderOpen == nil ||
		order.Role() != models.RoleVendor ||
		order.PaymentSettlementSignaledAt != nil ||
		len(order.SerializedOrderConfirmation) > 0 {
		return nil
	}

	ps, err := order.PaymentSentMessage()
	if err != nil {
		return nil
	}
	method, ok := paymentmetrics.ResolvedPaymentMethod(order, ps)
	if !ok || !paymentmetrics.MethodIsCancelable(method) {
		return nil
	}

	emitCancelableRecovery := func(emitPS *pb.PaymentSent) {
		events := cancelableRecoveryBusinessEvents(order, emitPS, total)
		if len(events) == 0 {
			return
		}
		now := v.clock()
		order.PaymentSettlementSignaledAt = &now
		*emitBusinessEvents = events
		*emitHandlerOrderID = order.ID.String()
		*emitHandlerPayment = emitPS
	}

	if paymentmetrics.CancelableAutoConfirmReady(order, ps) {
		emitCancelableRecovery(ps)
		return nil
	}
	if len(observations) == 0 {
		return nil
	}

	newPS, err := buildAggregatedPaymentSent(orderOpen, observations, total, order, sharedRefund, v.clock())
	if err != nil {
		return err
	}
	if !paymentmetrics.CancelableAutoConfirmReady(order, newPS) {
		return nil
	}
	if err := order.SetPaymentSent(newPS); err != nil {
		return err
	}
	backfillResolvedBuyerRefundAddress(order, newPS, observations)
	emitCancelableRecovery(newPS)
	return nil
}

// cancelableRecoveryBusinessEvents emits only CancelablePaymentReady for orders
// that were verified before auto-confirm inputs were ready. Recovery must not
// replay OrderFunded or other first-verification side effects.
func cancelableRecoveryBusinessEvents(order *models.Order, ps *pb.PaymentSent, total *big.Int) []interface{} {
	if ready := paymentmetrics.CancelablePaymentReadyEvent(order, ps, total); ready != nil {
		return []interface{}{ready}
	}
	return nil
}

func paymentVerifiedBusinessEvents(order *models.Order, orderOpen *pb.OrderOpen, ps *pb.PaymentSent, total *big.Int) []interface{} {
	if order == nil || ps == nil {
		return nil
	}
	switch order.Role() {
	case models.RoleBuyer:
		fundingTotal := ""
		if total != nil {
			fundingTotal = total.String()
		}
		return []interface{}{&events.OrderPaymentReceived{
			TenantID:     order.TenantID,
			OrderID:      order.ID.String(),
			FundingTotal: fundingTotal,
			CoinType:     ps.Coin,
		}}

	case models.RoleVendor:
		var out []interface{}
		if funded := orderFundedEvent(order, orderOpen); funded != nil {
			out = append(out, funded)
		}
		spec := ps.GetSettlementSpec()
		if spec == nil {
			return out
		}
		switch spec.GetMethod() {
		case pb.PaymentSent_CANCELABLE:
			if ready := paymentmetrics.CancelablePaymentReadyEvent(order, ps, total); ready != nil {
				out = append(out, ready)
			}
		case pb.PaymentSent_RWA_INSTANT:
			out = append(out, &events.RwaInstantBuyCompleted{
				OrderID:       order.ID.String(),
				TransactionID: ps.TransactionID,
				Coin:          ps.Coin,
			})
		}
		return out
	default:
		return nil
	}
}

func orderFundedEvent(order *models.Order, orderOpen *pb.OrderOpen) *events.OrderFunded {
	if order == nil || orderOpen == nil || len(orderOpen.Listings) == 0 {
		return nil
	}
	signed := orderOpen.Listings[0]
	if signed == nil || signed.Listing == nil || signed.Listing.Metadata == nil || signed.Listing.Item == nil {
		return nil
	}
	listing := signed.Listing
	buyerID, buyerName, buyerAvatar := "", "", ""
	if orderOpen.BuyerID != nil {
		buyerID = orderOpen.BuyerID.PeerID
		buyerName = orderOpen.BuyerID.DisplayName()
		buyerAvatar = orderOpen.BuyerID.DisplayAvatar()
	}
	funded := &events.OrderFunded{
		TenantID:    order.TenantID,
		BuyerName:   buyerName,
		BuyerAvatar: buyerAvatar,
		BuyerID:     buyerID,
		ListingType: listing.Metadata.ContractType.String(),
		OrderID:     order.ID.String(),
		Price: events.ListingPrice{
			Amount:        orderOpen.Amount,
			CurrencyCode:  orderOpen.PricingCoin,
			PriceModifier: listing.Item.CryptoListingPriceModifier,
		},
		Slug:  listing.Slug,
		Title: listing.Item.Title,
	}
	if len(listing.Item.Images) > 0 && listing.Item.Images[0] != nil {
		funded.Thumbnail = events.Thumbnail{
			Tiny:  listing.Item.Images[0].Tiny,
			Small: listing.Item.Images[0].Small,
		}
	}
	return funded
}

// dialectSupportsRowLock reports whether the underlying SQL dialect
// honours `SELECT ... FOR UPDATE` row-level locking. We use a denylist
// rather than an allowlist so PG-compatible dialects we haven't met yet
// (CockroachDB, YugabyteDB, TiDB, Vitess, …) opt INTO locking by default;
// silently degrading a production database to no-lock is the worst
// failure mode.
//
// SQLite is the sole denied dialect: its driver parses FOR UPDATE but
// only enforces transaction-level locking, so GORM emitting the clause
// would be misleading without changing the actual semantics.
func dialectSupportsRowLock(db *gorm.DB) bool {
	if db == nil || db.Dialector == nil {
		return false
	}
	switch db.Dialector.Name() {
	case "sqlite", "sqlite3":
		return false
	default:
		return true
	}
}

// parseExpectedAmount extracts the order's expected payment in smallest units.
// Address-monitored routes use the locked pending payment intent; direct order
// flows use the signed OrderOpen amount.
func parseExpectedAmount(order *models.Order, orderOpen *pb.OrderOpen) (*big.Int, error) {
	if orderOpen == nil {
		return nil, errors.New("order_open is nil")
	}
	raw := strings.TrimSpace(order.ExpectedPaymentAmountString())
	if raw == "" {
		raw = strings.TrimSpace(orderOpen.GetAmount())
	}
	if raw == "" {
		return nil, errors.New("expected payment amount is empty")
	}
	v, ok := new(big.Int).SetString(raw, 10)
	if !ok {
		return nil, fmt.Errorf("expected payment amount %q is not a decimal integer", raw)
	}
	if v.Sign() <= 0 {
		return nil, fmt.Errorf("expected payment amount %q must be positive", raw)
	}
	return v, nil
}

func promoteAfterVerification(order *models.Order, now time.Time) {
	if order == nil {
		return
	}
	if order.PaidAt == nil {
		paidAt := now
		order.PaidAt = &paidAt
	}
	if recoverPaymentTimeoutCancellation(order) {
		return
	}
	switch order.State {
	case models.OrderState_PENDING,
		models.OrderState_AWAITING_PAYMENT,
		models.OrderState_AWAITING_PAYMENT_VERIFICATION,
		models.OrderState_PROCESSING_ERROR:
		order.SetFSMState(models.OrderState_PENDING)
	}
}

func recoverPaymentTimeoutCancellation(order *models.Order) bool {
	if order == nil || order.State != models.OrderState_CANCELED {
		return false
	}
	cancel, err := order.OrderCancelMessage()
	if err != nil || cancel.GetReason() != "payment_timeout" {
		return false
	}
	order.SerializedOrderCancel = nil
	order.OrderCancelSignature = ""
	order.OrderCancelAcked = false
	order.Open = true
	order.SetFSMState(models.OrderState_PENDING)
	return true
}

// sumObservations folds the dedup'd observation slice into a *big.Int.
// A row that fails AmountBigInt is a hard error: the dispatcher rejects
// such inputs at insert time, so seeing one here means the row was
// corrupted at rest and trusting it would mis-count the verdict.
func sumObservations(rows []models.PaymentObservation) (*big.Int, error) {
	total := new(big.Int)
	for i := range rows {
		amt, ok := rows[i].AmountBigInt()
		if !ok || amt == nil {
			return nil, fmt.Errorf("observation %s has invalid amount %q", rows[i].ID, rows[i].Amount)
		}
		if amt.Sign() < 0 {
			return nil, fmt.Errorf("observation %s has negative amount %q", rows[i].ID, rows[i].Amount)
		}
		total.Add(total, amt)
	}
	return total, nil
}

// buildAggregatedPaymentSent reconstructs the legacy PaymentSent envelope
// from the dedup'd observation rows and the order context. The authoritative
// chain facts remain PaymentObservation rows; SerializedPaymentSent is a
// compatibility envelope for legacy order messages and event consumers.
//
// Field policy:
//
//   - TransactionID: only populated from a real chain tx hash. Native ManagedEscrow
//     balance polling may produce an internal observation id when no exact tx
//     can be attributed; that id is valid for verification/dedupe but must not
//     become a user-facing transaction hash.
//   - PayerAddress: taken from the latest real chain tx when available;
//     otherwise from the representative observation. Refund routing prefers
//     Order.RefundAddress; when that is empty, the verifier only infers a
//     refund target if all deduped observations have the same non-empty sender.
//   - ToAddress: also taken from the representative row — this is the
//     watched ManagedEscrow / smart-wallet / address the chain reported.
//   - Amount: the aggregated total (NOT the representative row's
//     amount). This is the value that crosses the verification
//     threshold; the seller's order processor uses it as the canonical
//     received amount.
//   - Coin: pulled from the order's pending payment intent. If legacy rows lack
//     that intent, native chain observations can recover the settlement coin.
//     The order pricing currency is intentionally never used because display
//     currency and settlement asset are different domains.
//   - Method and escrow fields: derived from the order's pending payment
//     intent. Observations are chain facts; they must not decide whether
//     an order is DIRECT, CANCELABLE, or MODERATED.
//   - PaymentTokenAddress / PlatformAddr / Chaincode: copied through from
//     the order record so the envelope round-trips intact for downstream
//     consumers.
func buildAggregatedPaymentSent(
	orderOpen *pb.OrderOpen,
	rows []models.PaymentObservation,
	total *big.Int,
	order *models.Order,
	sharedRefund string,
	now time.Time,
) (*pb.PaymentSent, error) {
	if len(rows) == 0 {
		return nil, errors.New("buildAggregatedPaymentSent: rows must be non-empty")
	}
	if total == nil {
		return nil, errors.New("buildAggregatedPaymentSent: total must not be nil")
	}
	if orderOpen == nil {
		return nil, errors.New("buildAggregatedPaymentSent: orderOpen must not be nil")
	}
	if order == nil {
		return nil, errors.New("buildAggregatedPaymentSent: order must not be nil")
	}

	rep := rows[len(rows)-1] // rows are sorted (BlockTime ASC, ID ASC) by the dedupe helper.
	for i := len(rows) - 2; i >= 0; i-- {
		// Defensive: if the upstream slice is ever returned in a
		// different order, take the row with the maximum BlockTime as
		// the representative. ID tie-breaker matches the dedupe sort.
		c := rows[i]
		if c.BlockTime.After(rep.BlockTime) ||
			(c.BlockTime.Equal(rep.BlockTime) && c.ID > rep.ID) {
			rep = c
		}
	}
	txRep, hasChainTx := latestChainTxObservation(rows)
	if hasChainTx {
		rep = txRep
	}

	chaincode := orderOpen.GetChaincode()
	coin, err := aggregatedPaymentCoin(order, rep)
	if err != nil {
		return nil, err
	}

	// The aggregated PaymentSent envelope must be byte-identical across buyer
	// and vendor mirrors. Embed buyer-declared refund targets only when they
	// live in SharedPaymentIntent or an existing shared PaymentSent envelope.
	refundAddr := aggregatedRefundAddress(order, sharedRefund)

	intent := resolveAggregatedPaymentIntent(order, rows)
	if !intent.settlementSpecOK {
		return nil, fmt.Errorf("missing settlement spec for pending escrow payment intent")
	}

	transactionID := ""
	if hasChainTx {
		transactionID = txRep.TxHash
	}
	eventTime := now
	if !rep.BlockTime.IsZero() {
		eventTime = rep.BlockTime.UTC()
	}

	ps := &pb.PaymentSent{
		TransactionID:       transactionID,
		Chaincode:           chaincode,
		ContractAddress:     intent.contractAddress,
		PayerAddress:        rep.FromAddress,
		Moderator:           intent.moderator,
		ModeratorAddress:    intent.moderatorAddress,
		Amount:              total.String(),
		Coin:                coin,
		ToAddress:           rep.ToAddress,
		Script:              intent.script,
		RefundAddress:       refundAddr,
		EscrowTimeoutHours:  intent.escrowTimeoutHours,
		PaymentTokenAddress: rep.TokenAddress,
		PlatformAmount:      intent.platformAmount,
		PlatformAddr:        intent.platformAddr,
		CancelFeeAmount:     intent.cancelFeeAmount,
		Timestamp:           timestamppb.New(eventTime),
		SettlementSpec:      intent.settlementSpec.ToPaymentSent(),
		FundingFacts:        fundingFactsFromObservations(rows),
		ConfirmationPolicy:  paymentSentConfirmationPolicy(order),
	}
	return ps, nil
}

func paymentSentConfirmationPolicy(order *models.Order) string {
	if order == nil {
		return ""
	}
	info, err := order.GetPendingPaymentInfo()
	if err != nil || info == nil {
		return ""
	}
	return models.NormalizePaymentConfirmationPolicy(info.ConfirmationPolicy)
}

func fundingFactsFromObservations(rows []models.PaymentObservation) []*pb.PaymentSent_FundingFact {
	if len(rows) == 0 {
		return nil
	}
	out := make([]*pb.PaymentSent_FundingFact, 0, len(rows))
	for i := range rows {
		row := rows[i]
		var observedAt *timestamppb.Timestamp
		if !row.BlockTime.IsZero() {
			observedAt = timestamppb.New(row.BlockTime.UTC())
		} else if !row.CreatedAt.IsZero() {
			observedAt = timestamppb.New(row.CreatedAt.UTC())
		}
		out = append(out, &pb.PaymentSent_FundingFact{
			Id:             row.ID,
			ChainNamespace: row.ChainNamespace,
			ChainReference: row.ChainReference,
			TxHash:         row.TxHash,
			TxHashSource:   models.NormalizePaymentTxHashSource(row.TxHashSource),
			EventIndex:     int32(row.EventIndex),
			EventType:      row.EventType,
			FromAddress:    row.FromAddress,
			ToAddress:      row.ToAddress,
			TokenAddress:   row.TokenAddress,
			Amount:         row.Amount,
			BlockNumber:    row.BlockNumber,
			Confirmations:  int32(row.Confirmations),
			Status:         row.Status,
			Source:         row.Source,
			ObservedAt:     observedAt,
		})
	}
	return out
}

func aggregatedPaymentCoin(order *models.Order, rep models.PaymentObservation) (string, error) {
	if coin, ok := paymentmetrics.PendingPaymentCoinFromOrder(order); ok {
		return string(coin), nil
	}
	if coin, ok := paymentmetrics.PaymentCoinFromObservation(rep); ok {
		return string(coin), nil
	}
	return "", fmt.Errorf("cannot determine PaymentSent.Coin from payment intent or chain observation")
}

func latestChainTxObservation(rows []models.PaymentObservation) (models.PaymentObservation, bool) {
	var out models.PaymentObservation
	found := false
	for i := range rows {
		if !rows[i].HasChainTxHash() {
			continue
		}
		if !found ||
			rows[i].BlockTime.After(out.BlockTime) ||
			(rows[i].BlockTime.Equal(out.BlockTime) && rows[i].ID > out.ID) {
			out = rows[i]
			found = true
		}
	}
	return out, found
}

type aggregatedPaymentIntent struct {
	settlementSpec     paymentmetrics.SettlementSpec
	settlementSpecOK   bool
	contractAddress    string
	script             string
	moderator          string
	moderatorAddress   string
	platformAmount     string
	platformAddr       string
	cancelFeeAmount    string
	escrowTimeoutHours uint32
}

func resolveAggregatedPaymentIntent(order *models.Order, rows []models.PaymentObservation) aggregatedPaymentIntent {
	intent := aggregatedPaymentIntent{
		settlementSpec:   paymentmetrics.NewDirectSpec(),
		settlementSpecOK: true,
	}

	if order == nil {
		return intent
	}

	if managed_escrowInfo, err := order.GetPendingManagedEscrowPaymentInfo(); err == nil && managed_escrowInfo != nil {
		if spec, ok := paymentmetrics.ResolveSettlementSpecFromPendingManagedEscrow(managed_escrowInfo); ok {
			intent.settlementSpec = spec
		} else if managed_escrowInfo.Moderated {
			intent.settlementSpec = paymentmetrics.NewManagedEscrowSpec(true)
		} else {
			intent.settlementSpec = paymentmetrics.NewManagedEscrowSpec(false)
		}
		intent.contractAddress = managed_escrowInfo.Address
		intent.moderator = managed_escrowInfo.Moderator
		intent.moderatorAddress = managed_escrowInfo.ModeratorAddress
		intent.platformAmount = managed_escrowInfo.PlatformAmount
		intent.platformAddr = managed_escrowInfo.PlatformAddr
		intent.cancelFeeAmount = managed_escrowInfo.CancelFeeAmount
		return intent
	}

	if escrowInfo, err := order.GetPendingEscrowPaymentInfo(); err == nil && escrowInfo != nil {
		if spec, ok := paymentmetrics.ResolveSettlementSpecFromPendingEscrow(escrowInfo); ok {
			intent.settlementSpec = spec
		} else {
			intent.settlementSpecOK = false
		}
		intent.contractAddress = strings.TrimSpace(escrowInfo.ContractAddress)
		if intent.contractAddress == "" {
			intent.contractAddress = escrowInfo.EscrowAddress
		}
		if strings.TrimSpace(escrowInfo.Moderator) != "" {
			intent.moderator = escrowInfo.Moderator
		}
		return intent
	}

	pendingInfo, err := order.GetPendingPaymentInfo()
	if err == nil && pendingInfo != nil {
		return utxoAggregatedPaymentIntent(pendingInfo)
	}

	// No pending intent: observations are chain facts only. Default to DIRECT
	// for address-monitored routes that do not carry escrow settlement intent.
	return intent
}

func utxoAggregatedPaymentIntent(pendingInfo *models.PendingUTXOPaymentInfo) aggregatedPaymentIntent {
	intent := aggregatedPaymentIntent{
		settlementSpec:   paymentmetrics.NewDirectSpec(),
		settlementSpecOK: true,
	}
	if pendingInfo == nil {
		return intent
	}
	if spec, ok := paymentmetrics.ResolveSettlementSpecFromPendingUTXO(pendingInfo); ok {
		intent.settlementSpec = spec
	} else if pendingInfo.Moderator != "" {
		intent.settlementSpec = paymentmetrics.NewUTXOSpec(true)
	} else {
		intent.settlementSpec = paymentmetrics.NewUTXOSpec(false)
	}
	if pendingInfo.Moderator != "" {
		intent.moderator = pendingInfo.Moderator
		intent.moderatorAddress = pendingInfo.ModeratorPubkey
		intent.escrowTimeoutHours = pendingInfo.UnlockHours
	}
	intent.script = pendingInfo.Script
	return intent
}

func aggregatedRefundAddress(order *models.Order, sharedRefund string) string {
	if order != nil {
		if existing, err := order.PaymentSentMessage(); err == nil {
			if addr := strings.TrimSpace(existing.RefundAddress); addr != "" {
				return addr
			}
		}
	}
	if addr := strings.TrimSpace(sharedRefund); addr != "" {
		return addr
	}
	return ""
}

func backfillResolvedBuyerRefundAddress(order *models.Order, paymentSent *pb.PaymentSent, observations []models.PaymentObservation) {
	if order == nil || paymentSent == nil || strings.TrimSpace(order.RefundAddress) != "" {
		return
	}
	result := paymentmetrics.ResolveBuyerRefundAddress(paymentmetrics.ResolveBuyerRefundAddressParams{
		Order:        order,
		PaymentSent:  paymentSent,
		Observations: observations,
	})
	if result.Found() {
		order.RefundAddress = result.Address
	}
}

func loadSharedRefundAddress(gdb *gorm.DB, orderID string) string {
	intent, err := paymentintent.LoadSharedPaymentIntent(gdb, orderID)
	if err != nil || intent == nil {
		return ""
	}
	return strings.TrimSpace(intent.RefundAddress)
}

func hydrateSharedManagedEscrowMetadata(gdb *gorm.DB, order *models.Order) error {
	if gdb == nil || order == nil || strings.TrimSpace(order.ID.String()) == "" {
		return nil
	}

	if hasPendingManagedEscrowPaymentInfo(order) && strings.TrimSpace(order.PaymentAddress) != "" && strings.TrimSpace(order.RefundAddress) != "" {
		return nil
	}
	return paymentintent.HydrateOrderFromSharedIntent(gdb, order)
}

func hydrateCotenantEscrowMetadata(gdb *gorm.DB, order *models.Order) error {
	if gdb == nil || order == nil || strings.TrimSpace(order.ID.String()) == "" {
		return nil
	}
	if hasPendingEscrowOrUTXOPaymentInfo(order) {
		return nil
	}

	var peers []models.Order
	if err := gdb.
		Where("id = ? AND tenant_id <> ?", order.ID, order.TenantID).
		Find(&peers).Error; err != nil {
		return err
	}
	for i := range peers {
		if !hasPendingEscrowOrUTXOPaymentInfo(&peers[i]) {
			continue
		}
		order.PendingPaymentInfo = append(order.PendingPaymentInfo[:0], peers[i].PendingPaymentInfo...)
		if strings.TrimSpace(order.PaymentAddress) == "" {
			order.PaymentAddress = peers[i].PaymentAddress
		}
		if strings.TrimSpace(order.RefundAddress) == "" {
			order.RefundAddress = peers[i].RefundAddress
		}
		return nil
	}
	return nil
}

func observationStatusesForVerification(order *models.Order) []string {
	statuses := []string{models.PaymentObservationStatusConfirmed}
	if utxoPendingAcceptsMempool(order) {
		statuses = append(statuses, models.PaymentObservationStatusPending)
	}
	return statuses
}

func utxoPendingAcceptsMempool(order *models.Order) bool {
	if order == nil {
		return false
	}
	info, err := order.GetPendingPaymentInfo()
	if err != nil || info == nil {
		return false
	}
	return models.NormalizePaymentConfirmationPolicy(info.ConfirmationPolicy) == models.PaymentConfirmationPolicyMempoolAccepted
}

func hasPendingManagedEscrowPaymentInfo(order *models.Order) bool {
	if order == nil {
		return false
	}
	info, err := order.GetPendingManagedEscrowPaymentInfo()
	return err == nil && info != nil
}

func hasPendingEscrowOrUTXOPaymentInfo(order *models.Order) bool {
	if order == nil {
		return false
	}
	if info, err := order.GetPendingEscrowPaymentInfo(); err == nil && info != nil {
		return true
	}
	info, err := order.GetPendingPaymentInfo()
	return err == nil && info != nil
}
