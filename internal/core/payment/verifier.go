//go:build !private_distribution

package payment

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

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
	db    database.Database
	bus   events.Bus
	clock func() time.Time
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
			}, tenantID, orderID, &emitVerified, &emitVerifiedNamespace)
		})
	} else {
		err = v.db.Update(func(tx database.Tx) error {
			gdb := tx.Read().WithContext(ctx)
			return v.aggregateWithGorm(ctx, gdb, func(order *models.Order) error {
				return tx.Save(order)
			}, tenantID, orderID, &emitVerified, &emitVerifiedNamespace)
		})
	}
	if err != nil {
		return err
	}

	if emitVerified {
		v.bus.Emit(events.PaymentVerified{TenantID: tenantID, OrderID: orderID})
		paymentmetrics.RecordPaymentAggregationEnvelopeEmitted(tenantID, emitVerifiedNamespace, orderID)
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
		Where("tenant_id = ? AND order_id = ? AND status = ?",
			tenantID, orderID, models.PaymentObservationStatusConfirmed).
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
			break
		}

		ps, err := buildAggregatedPaymentSent(orderOpen, deduped, total, &order, v.clock())
		if err != nil {
			return fmt.Errorf("aggregating verifier: build PaymentSent for %s: %w", orderID, err)
		}
		if err := order.SetPaymentSent(ps); err != nil {
			return fmt.Errorf("aggregating verifier: marshal PaymentSent for %s: %w", orderID, err)
		}
		if order.RefundAddress == "" && ps.RefundAddress != "" {
			order.RefundAddress = ps.RefundAddress
		}
		if order.PaymentAddress == "" && ps.ToAddress != "" {
			order.PaymentAddress = ps.ToAddress
		}
		order.MarkPaymentVerified()
		promoteAfterVerification(&order, v.clock())
		*emitVerified = true
		*emitVerifiedNamespace = deduped[0].ChainNamespace
	}

	if err := saveOrder(&order); err != nil {
		return fmt.Errorf("aggregating verifier: save order %s: %w", orderID, err)
	}
	return nil
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
// Address-monitored routes use the locked pending payment intent; legacy rows
// fall back to the signed OrderOpen amount.
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
	switch order.State {
	case models.OrderState_PENDING,
		models.OrderState_AWAITING_PAYMENT,
		models.OrderState_AWAITING_PAYMENT_VERIFICATION,
		models.OrderState_PROCESSING_ERROR:
		order.SetFSMState(models.OrderState_PENDING)
	}
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
// from the dedup'd observation rows and the order context. Downstream
// code (internal/orders/payment_sent.go, external auditors, dispute
// flows) treats SerializedPaymentSent as the source of truth for chain
// movements, so we have to populate every field a manually-submitted
// PaymentSent message would carry.
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
//   - Coin: pulled from the order's pending payment intent. OrderOpen.PricingCoin
//     is the pricing asset for marketplace value, while address-monitored
//     crypto flows lock the actual settlement asset in PendingPaymentInfo.
//     Legacy rows without pending intent fall back to OrderOpen.PricingCoin.
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
	coin := pendingPaymentCoin(order, orderOpen)

	// Use buyer-declared refund address when available. When absent, infer
	// only from a single unambiguous observed sender; otherwise leave empty
	// so downstream refund paths require an explicit override.
	refundAddr := order.RefundAddress
	if refundAddr == "" {
		refundAddr = inferredRefundAddress(rows)
	}

	intent := resolveAggregatedPaymentIntent(order, rows)

	transactionID := ""
	if hasChainTx {
		transactionID = txRep.TxHash
	}

	ps := &pb.PaymentSent{
		TransactionID:       transactionID,
		Chaincode:           chaincode,
		Method:              intent.method,
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
		Timestamp:           timestamppb.New(now),
	}
	return ps, nil
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
	method             pb.PaymentSent_Method
	contractAddress    string
	script             string
	moderator          string
	moderatorAddress   string
	escrowTimeoutHours uint32
}

func resolveAggregatedPaymentIntent(order *models.Order, rows []models.PaymentObservation) aggregatedPaymentIntent {
	intent := aggregatedPaymentIntent{method: pb.PaymentSent_DIRECT}

	if order == nil {
		return intent
	}

	if managed_escrowInfo, err := order.GetPendingManagedEscrowPaymentInfo(); err == nil && managed_escrowInfo != nil {
		if spec, ok := paymentmetrics.ResolveSettlementSpecFromPendingManagedEscrow(managed_escrowInfo); ok {
			intent.method = spec.Method
		} else if managed_escrowInfo.Moderated {
			intent.method = pb.PaymentSent_MODERATED
		} else {
			intent.method = pb.PaymentSent_CANCELABLE
		}
		intent.contractAddress = managed_escrowInfo.Address
		return intent
	}

	if csInfo, err := order.GetPendingClientSignedPaymentInfo(); err == nil && csInfo != nil {
		if spec, ok := paymentmetrics.ResolveSettlementSpecFromPendingClientSigned(csInfo); ok {
			intent.method = spec.Method
		} else if strings.TrimSpace(csInfo.Moderator) != "" {
			intent.method = pb.PaymentSent_MODERATED
		} else {
			intent.method = pb.PaymentSent_CANCELABLE
		}
		intent.contractAddress = csInfo.EscrowAddress
		if strings.TrimSpace(csInfo.Moderator) != "" {
			intent.moderator = csInfo.Moderator
		}
		return intent
	}

	pendingInfo, err := order.GetPendingPaymentInfo()
	if err == nil && pendingInfo != nil {
		return utxoAggregatedPaymentIntent(pendingInfo)
	}

	// No pending intent: observations are chain facts only. Default to DIRECT
	// for unknown routes (legacy rows without SettlementSpec persistence).
	return intent
}

func utxoAggregatedPaymentIntent(pendingInfo *models.PendingUTXOPaymentInfo) aggregatedPaymentIntent {
	intent := aggregatedPaymentIntent{method: pb.PaymentSent_DIRECT}
	if pendingInfo == nil {
		return intent
	}
	if spec, ok := paymentmetrics.ResolveSettlementSpecFromPendingUTXO(pendingInfo); ok {
		intent.method = spec.Method
	} else if pendingInfo.Moderator != "" {
		intent.method = pb.PaymentSent_MODERATED
	} else {
		intent.method = pb.PaymentSent_CANCELABLE
	}
	if pendingInfo.Moderator != "" {
		intent.moderator = pendingInfo.Moderator
		intent.moderatorAddress = pendingInfo.ModeratorPubkey
		intent.escrowTimeoutHours = pendingInfo.UnlockHours
	}
	intent.script = pendingInfo.Script
	return intent
}

func inferredRefundAddress(rows []models.PaymentObservation) string {
	candidate := ""
	for i := range rows {
		addr := strings.TrimSpace(rows[i].FromAddress)
		if addr == "" {
			return ""
		}
		if candidate == "" {
			candidate = addr
			continue
		}
		if !strings.EqualFold(candidate, addr) {
			return ""
		}
	}
	return candidate
}
