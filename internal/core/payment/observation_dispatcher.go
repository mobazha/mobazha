//go:build !private_distribution

package payment

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/models"
)

// ObservationDispatcher bridges raw on-chain funding events into the
// append-only payment_observations fact table and re-triggers the
// VerificationService aggregator after each successful insert.
//
// Authoritative design: docs/escrow/MONITOR_DRIVEN_PAYMENT.md §5.1.
//
// ─────────────────────────────────────────────────────────────────────────
// Layering
// ─────────────────────────────────────────────────────────────────────────
//
// The Sprint 2A v2.0 Monitor-Driven Payment model has three independent
// layers:
//
//  1. chain watcher (per-chain RPC subscription / polling) — produces
//     FundingEvent values from native ManagedEscrowReceived / ERC-20 Transfer /
//     SPL transfer / EXTERNAL_PAYMENT deposit logs;
//  2. ObservationDispatcher — this type. INSERT-only persistence into
//     payment_observations + verifier kick;
//  3. VerificationService.AggregateAndEmit (Sprint 2A step 4) — DISTINCT
//     ON aggregation across observers, decides funded / partial / over-
//     paid status and emits the PaymentSent envelope when the order
//     reaches the configured threshold.
//
// The dispatcher does NOT subsume the existing pkg/managedescrow.PaymentMonitor
// in-memory state machine. That layer survives as the chain watcher's
// in-process registry (Watch / Stop / Snapshot, Sprint 1 D10). The
// dispatcher subscribes to the watcher's per-event callback path and
// produces append-only observation rows; the cumulative state used by
// hosting (funded / partial / overpaid) is now derived from observations
// via the aggregator, NOT from the watcher's running totals.
//
// ─────────────────────────────────────────────────────────────────────────
// Idempotency contract
// ─────────────────────────────────────────────────────────────────────────
//
// Chain RPC subscriptions are at-least-once and process restarts replay
// recent events. The dispatcher is safe to call any number of times for
// the same FundingEvent: PaymentObservationRepo.InsertObservation surfaces
// ErrDuplicateObservation when the dedupe tuple
// (tenant_id, chain_namespace, chain_reference, tx_hash, event_index,
// observer) collides, and the dispatcher swallows that sentinel and skips
// the verifier kick to avoid hot-looping on storms of duplicate events.
//
// Aggregator errors are returned to the caller — they DO NOT abort the
// observation insert (the row is already persisted). Hosting decides
// retry policy upstream (e.g. a periodic reconciliation worker that
// re-aggregates orders with newer observations than the last emit).
type ObservationDispatcher struct {
	repo       contracts.PaymentObservationRepo
	aggregator PaymentAggregator
	tenants    TenantResolver

	workerID string
	clock    func() time.Time
}

// NewObservationDispatcher constructs a dispatcher. workerID identifies
// the running monitor worker instance (host:port + worker pool id is a
// good convention) and is embedded in the per-row Observer field so two
// workers observing the same chain event produce distinct rows but the
// same worker replaying its event stream collapses via UNIQUE.
//
// All three dependencies are required; a nil argument is a programming
// error and panics during construction.
func NewObservationDispatcher(
	repo contracts.PaymentObservationRepo,
	aggregator PaymentAggregator,
	tenants TenantResolver,
	workerID string,
) *ObservationDispatcher {
	if repo == nil {
		panic("payment: NewObservationDispatcher requires a non-nil PaymentObservationRepo")
	}
	if aggregator == nil {
		panic("payment: NewObservationDispatcher requires a non-nil PaymentAggregator")
	}
	if tenants == nil {
		panic("payment: NewObservationDispatcher requires a non-nil TenantResolver")
	}
	if strings.TrimSpace(workerID) == "" {
		panic("payment: NewObservationDispatcher requires a non-empty workerID")
	}
	return &ObservationDispatcher{
		repo:       repo,
		aggregator: aggregator,
		tenants:    tenants,
		workerID:   workerID,
		clock:      time.Now,
	}
}

// withClock swaps the dispatcher's clock for tests. Not exported: the
// dispatcher uses time.Now only as a fallback when the chain watcher
// fails to populate FundingEvent.BlockTime, which should never happen
// in production paths.
func (d *ObservationDispatcher) withClock(clock func() time.Time) *ObservationDispatcher {
	d.clock = clock
	return d
}

// PaymentAggregator is the verifier interface invoked after each
// successful observation insert. Sprint 2A step 4 will provide the
// production implementation backed by VerificationService.AggregateAndEmit;
// step 3 only requires the contract so the dispatcher and the aggregator
// can land in separate commits.
type PaymentAggregator interface {
	// AggregateAndEmit re-runs payment aggregation for a single order.
	// The implementation MUST be safe for concurrent invocation; the
	// dispatcher does not serialize calls on the caller's behalf.
	AggregateAndEmit(ctx context.Context, tenantID, orderID string) error
}

// TenantResolver maps an OrderID to the tenant that owns it. The
// dispatcher needs the tenant id to scope the observation row; safe
// EventHandler callbacks do not carry tenant context (a ManagedEscrow address
// is shared across tenants only when the same ManagedEscrow is bound to multiple
// orders, which the data model already disallows).
//
// Implementations may hit the order DB, an in-process LRU cache, or a
// hybrid. They MUST return ErrUnknownOrder for orders that are not
// (yet) registered — the dispatcher treats unknown orders as "not a
// Mobazha-managed ManagedEscrow" and ignores the event, matching the design
// doc's "不是 Mobazha 管理的 ManagedEscrow，忽略" semantics.
type TenantResolver interface {
	ResolveTenant(ctx context.Context, orderID string) (tenantID string, err error)
}

// Sentinel errors surfaced by the dispatcher.
var (
	// ErrUnknownOrder is returned by TenantResolver implementations
	// when the orderID is not registered (or has been retired). The
	// dispatcher swallows this error and returns nil from
	// OnFundingEvent so non-Mobazha funding into a watched address
	// does not poison the call site.
	ErrUnknownOrder = errors.New("payment: unknown order")

	// ErrInvalidFundingEvent is returned when FundingEvent fails
	// structural validation (missing fields, non-positive amount, ...).
	// Distinct from a chain-side error so callers can distinguish
	// "bad input" from "chain unavailable".
	ErrInvalidFundingEvent = errors.New("payment: invalid funding event")
)

// FundingEvent is the chain-agnostic representation of a single inbound
// transfer to a watched ManagedEscrow / smart-wallet / address. Adapters in
// internal/payment/adapters convert their native chain log structures
// (eth Log + filtered Transfer; SPL Transfer ix; EXTERNAL_PAYMENT get_transfers
// row; UTXO Vout match) into FundingEvent before invoking the
// dispatcher.
//
// Field-by-field rationale matches models.PaymentObservation; see that
// model's doc comment for the storage-side contract.
type FundingEvent struct {
	OrderID string

	// CAIP-2 chain identification.
	ChainNamespace string // e.g. "eip155", "solana", "external_payment", "bip122"
	ChainReference string // e.g. "1" (mainnet ETH), "mainnet" (Solana)

	TxHash     string
	EventIndex int    // 0 for native receive; log index for ERC-20 Transfer; SPL ix index for SPL.
	EventType  string // see PaymentEventManagedEscrowReceived / PaymentEventERC20Transfer / ...

	// Address fields. ToAddress is the watched recipient (the ManagedEscrow);
	// FromAddress is evidence-only and may be empty (CEX-direct-pay,
	// EXTERNAL_PAYMENT — sender is intentionally not observable). TokenAddress is
	// empty for native gas-asset transfers and the ERC-20 / SPL token
	// contract otherwise.
	FromAddress  string
	ToAddress    string
	TokenAddress string

	// Amount in the chain's smallest unit (wei / sat / lamport / atomic
	// unit / piconero). Must be a non-nil, strictly positive *big.Int —
	// zero-value transfers are rejected as ErrInvalidFundingEvent
	// because they cannot represent a real funding contribution.
	Amount *big.Int

	BlockNumber int64
	BlockTime   time.Time
}

// validate enforces the structural pre-conditions required by the
// payment_observations schema. Returning a sentinel error (with a
// %w-wrapped detail) lets callers (e.g. the chain watcher's metric
// pipeline) classify dispatcher rejections without parsing free-form
// strings.
func (e FundingEvent) validate() error {
	if strings.TrimSpace(e.OrderID) == "" {
		return fmt.Errorf("%w: empty OrderID", ErrInvalidFundingEvent)
	}
	if strings.TrimSpace(e.ChainNamespace) == "" {
		return fmt.Errorf("%w: empty ChainNamespace", ErrInvalidFundingEvent)
	}
	if strings.TrimSpace(e.ChainReference) == "" {
		return fmt.Errorf("%w: empty ChainReference", ErrInvalidFundingEvent)
	}
	if strings.TrimSpace(e.TxHash) == "" {
		return fmt.Errorf("%w: empty TxHash", ErrInvalidFundingEvent)
	}
	if e.EventIndex < 0 {
		return fmt.Errorf("%w: negative EventIndex %d", ErrInvalidFundingEvent, e.EventIndex)
	}
	if strings.TrimSpace(e.EventType) == "" {
		return fmt.Errorf("%w: empty EventType", ErrInvalidFundingEvent)
	}
	if strings.TrimSpace(e.ToAddress) == "" {
		return fmt.Errorf("%w: empty ToAddress", ErrInvalidFundingEvent)
	}
	if e.Amount == nil {
		return fmt.Errorf("%w: nil Amount", ErrInvalidFundingEvent)
	}
	if e.Amount.Sign() <= 0 {
		return fmt.Errorf("%w: Amount must be > 0 (got %s)", ErrInvalidFundingEvent, e.Amount.String())
	}
	if e.BlockNumber <= 0 {
		return fmt.Errorf("%w: BlockNumber must be > 0 (got %d)", ErrInvalidFundingEvent, e.BlockNumber)
	}
	if e.BlockTime.IsZero() {
		return fmt.Errorf("%w: BlockTime must be set", ErrInvalidFundingEvent)
	}
	return nil
}

// OnFundingEvent records evt as a new payment_observations row and kicks
// off a verifier re-aggregation for the affected order. The flow:
//
//	1. validate evt structurally;
//	2. resolve tenantID via TenantResolver — unknown orders are silently
//	   ignored (event was not for a Mobazha-managed ManagedEscrow);
//	3. INSERT a row with Source = "monitor" and a per-worker Observer;
//	4. duplicate inserts (UNIQUE on the dedupe tuple) collapse to a
//	   silent no-op so chain RPC replay / worker restart is safe;
//	5. invoke aggregator.AggregateAndEmit(tenantID, orderID).
//
// Aggregator errors propagate to the caller. Hosting MUST surface them
// to its retry / DLQ pipeline; the observation is already persisted so
// a periodic reconciliation worker re-aggregating orders with
// observations newer than the last emitted PaymentSent envelope is the
// standard recovery path.
func (d *ObservationDispatcher) OnFundingEvent(ctx context.Context, evt FundingEvent) error {
	if err := evt.validate(); err != nil {
		return err
	}

	tenantID, err := d.tenants.ResolveTenant(ctx, evt.OrderID)
	if err != nil {
		if errors.Is(err, ErrUnknownOrder) {
			// Not a Mobazha-managed ManagedEscrow — ignore the event so noise
			// from unrelated funding does not surface up the stack.
			return nil
		}
		return fmt.Errorf("payment: resolve tenant for order %s: %w", evt.OrderID, err)
	}
	if strings.TrimSpace(tenantID) == "" {
		return fmt.Errorf("payment: TenantResolver returned empty tenant for order %s", evt.OrderID)
	}

	obs := &models.PaymentObservation{
		ID:             uuid.NewString(),
		TenantID:       tenantID,
		OrderID:        evt.OrderID,
		ChainNamespace: evt.ChainNamespace,
		ChainReference: evt.ChainReference,
		TxHash:         evt.TxHash,
		EventIndex:     evt.EventIndex,
		EventType:      evt.EventType,
		FromAddress:    evt.FromAddress,
		ToAddress:      evt.ToAddress,
		TokenAddress:   evt.TokenAddress,
		BlockNumber:    evt.BlockNumber,
		BlockTime:      evt.BlockTime,
		Confirmations:  0,
		Source:         models.PaymentObservationSourceMonitor,
		Observer:       d.monitorObserver(evt.ChainNamespace, evt.ChainReference),
		Status:         models.PaymentObservationStatusPending,
	}
	obs.SetAmountBigInt(evt.Amount)

	if err := d.repo.InsertObservation(ctx, obs); err != nil {
		if errors.Is(err, contracts.ErrDuplicateObservation) {
			// Same worker replaying the same event — already persisted.
			// Skip the aggregator kick: a duplicate event by definition
			// produces no new content for the aggregator to consider.
			return nil
		}
		return fmt.Errorf("payment: insert observation for order %s tx %s: %w", evt.OrderID, evt.TxHash, err)
	}

	if err := d.aggregator.AggregateAndEmit(ctx, tenantID, evt.OrderID); err != nil {
		return fmt.Errorf("payment: aggregate after insert (order=%s tx=%s): %w", evt.OrderID, evt.TxHash, err)
	}
	return nil
}

// OnNewBlock advances confirmations on a per-chain basis and re-aggregates
// every order whose pending observations crossed the confirmation
// threshold during this call.
//
// Concretely:
//
//   - the dispatcher invokes repo.RefreshConfirmations to flip pending
//     rows whose block depth >= requiredConfirmations to confirmed;
//   - for each (tenantID, orderID) the repo reports as newly affected
//     it invokes aggregator.AggregateAndEmit;
//   - aggregator failures are accumulated via errors.Join and returned
//     after iterating every affected order — partial progress is
//     acceptable and the next block tick will re-evaluate.
//
// The dispatcher MUST NOT short-circuit on the first aggregator error:
// stopping on the Nth order leaves orders 1..N-1 already-emitted and
// orders N+1..M permanently stale (until the next block, which may
// arrive minutes later).
//
// requiredConfirmations is the chain-specific quorum that hosting passes
// in (12 for ETH/BSC mainnet, 2 for L2s, 10 for BTC mainnet, ...). The
// dispatcher does NOT hard-code these values — it would couple the
// SaaS payment loop to chain-specific config that lives in pkg/managedescrow
// chain policies / coin metadata.
//
// Returns nil if RefreshConfirmations succeeds and no aggregator
// failed; otherwise returns an aggregated error that wraps
// (RefreshConfirmations error | per-order aggregator errors).
func (d *ObservationDispatcher) OnNewBlock(
	ctx context.Context,
	chainNamespace, chainReference string,
	currentBlockNumber int64,
	requiredConfirmations int,
) error {
	if strings.TrimSpace(chainNamespace) == "" {
		return fmt.Errorf("payment: empty chainNamespace")
	}
	if strings.TrimSpace(chainReference) == "" {
		return fmt.Errorf("payment: empty chainReference")
	}

	affected, err := d.repo.RefreshConfirmations(ctx, chainNamespace, chainReference, currentBlockNumber, requiredConfirmations)
	if err != nil {
		return fmt.Errorf("payment: refresh confirmations on %s:%s: %w", chainNamespace, chainReference, err)
	}
	if len(affected) == 0 {
		return nil
	}

	var errs []error
	for _, ref := range affected {
		if err := d.aggregator.AggregateAndEmit(ctx, ref.TenantID, ref.OrderID); err != nil {
			errs = append(errs, fmt.Errorf("payment: aggregate after refresh (tenant=%s order=%s): %w",
				ref.TenantID, ref.OrderID, err))
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// monitorObserver constructs the per-row Observer string. The format
// (monitor:<namespace>:<reference>:<workerID>) matches the convention
// documented in models.PaymentObservation.Observer and lets the
// aggregator's DISTINCT ON priority rule cleanly prefer monitor-source
// rows over buyer_reported rows.
func (d *ObservationDispatcher) monitorObserver(namespace, reference string) string {
	return fmt.Sprintf("monitor:%s:%s:%s", namespace, reference, d.workerID)
}
