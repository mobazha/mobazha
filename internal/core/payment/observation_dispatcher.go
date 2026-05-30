//go:build !private_distribution

package payment

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	// aggregator == nil is valid: audit-only mode (insert observations
	// without triggering AggregateAndEmit). Used by UTXO path where the
	// legacy verification pipeline is still the source of truth.
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

// HasAggregator reports whether observation inserts trigger payment aggregation.
// When false the dispatcher is audit-only and UTXO checkout must use the legacy
// buyer-node path to advance orders.
func (d *ObservationDispatcher) HasAggregator() bool {
	return d != nil && d.aggregator != nil
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

// MultiTenantResolver is the SaaS-aware extension used when a single business
// order is materialized into multiple tenant-scoped order rows (buyer + vendor).
// Implementations return every tenant that owns the order ID so monitor-driven
// funding can fan out to each mirrored order row.
type MultiTenantResolver interface {
	ResolveTenants(ctx context.Context, orderID string) ([]string, error)
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

	TxHash       string
	TxHashSource string // chain_tx (explorer-safe) or balance_poll (internal observation id)
	EventIndex   int    // 0 for native receive; log index for ERC-20 Transfer; SPL ix index for SPL.
	EventType    string // see PaymentEventManagedEscrowReceived / PaymentEventERC20Transfer / ...

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

	BlockNumber int64     // inclusion height; 0 = mempool / unconfirmed
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
	switch models.NormalizePaymentTxHashSource(e.TxHashSource) {
	case models.PaymentTxHashSourceChainTx, models.PaymentTxHashSourceBalancePoll:
	default:
		return fmt.Errorf("%w: invalid TxHashSource %q", ErrInvalidFundingEvent, e.TxHashSource)
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
	if e.BlockNumber < 0 {
		return fmt.Errorf("%w: BlockNumber must be ≥ 0 (got %d)", ErrInvalidFundingEvent, e.BlockNumber)
	}
	if e.BlockTime.IsZero() {
		return fmt.Errorf("%w: BlockTime must be set", ErrInvalidFundingEvent)
	}
	return nil
}

// OnFundingEvent records evt as a new payment_observations row and kicks
// off a verifier re-aggregation for the affected order. The flow:
//
//  1. validate evt structurally;
//  2. resolve tenantID via TenantResolver — unknown orders are silently
//     ignored (event was not for a Mobazha-managed ManagedEscrow);
//  3. INSERT a row with Source = "monitor" and a per-worker Observer;
//  4. duplicate inserts (UNIQUE on the dedupe tuple) collapse to a
//     silent no-op so chain RPC replay / worker restart is safe;
//  5. invoke aggregator.AggregateAndEmit(tenantID, orderID).
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

	tenantIDs, err := d.resolveTenants(ctx, evt.OrderID)
	if err != nil {
		if errors.Is(err, ErrUnknownOrder) {
			// Not a Mobazha-managed ManagedEscrow — ignore the event so noise
			// from unrelated funding does not surface up the stack.
			return nil
		}
		return fmt.Errorf("payment: resolve tenant for order %s: %w", evt.OrderID, err)
	}

	var errs []error
	for _, tenantID := range tenantIDs {
		obs := buildObservation(
			tenantID, evt,
			models.PaymentObservationSourceMonitor,
			d.monitorObserver(evt.ChainNamespace, evt.ChainReference),
		)
		if err := d.insertAndKick(ctx, obs); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (d *ObservationDispatcher) resolveTenants(ctx context.Context, orderID string) ([]string, error) {
	if multi, ok := d.tenants.(MultiTenantResolver); ok {
		tenantIDs, err := multi.ResolveTenants(ctx, orderID)
		if err != nil {
			return nil, err
		}
		return normalizeTenantIDs(orderID, tenantIDs)
	}

	tenantID, err := d.tenants.ResolveTenant(ctx, orderID)
	if err != nil {
		return nil, err
	}
	return normalizeTenantIDs(orderID, []string{tenantID})
}

func normalizeTenantIDs(orderID string, tenantIDs []string) ([]string, error) {
	seen := make(map[string]struct{}, len(tenantIDs))
	out := make([]string, 0, len(tenantIDs))
	for _, tenantID := range tenantIDs {
		tenantID = strings.TrimSpace(tenantID)
		if tenantID == "" {
			continue
		}
		if _, ok := seen[tenantID]; ok {
			continue
		}
		seen[tenantID] = struct{}{}
		out = append(out, tenantID)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("payment: TenantResolver returned empty tenant set for order %s", orderID)
	}
	return out, nil
}

// OnBuyerReportedPaymentSent records a buyer-reported PAYMENT_SENT
// message as an independent payment_observations row (Source =
// "buyer_reported", Observer = "buyer:<peerID>") and kicks the
// aggregator. This is the Sprint 2A step 5 entry point that lets the
// legacy buyer-submitted PaymentSent envelope feed the same fact table
// the monitor path already uses, so AggregatingVerifier can DISTINCT-ON
// across both observers without divergent data plumbing.
//
// Authoritative design: docs/escrow/MONITOR_DRIVEN_PAYMENT.md §5.3.
//
// Differences from OnFundingEvent:
//
//   - tenantID is supplied by the caller (the Order is already loaded
//     in the message-processing transaction, so going through
//     TenantResolver is unnecessary indirection — and would bias the
//     observed tenant against whatever the Order row says, defeating
//     the cross-tenant scoping guarantee from the verifier).
//   - Observer is "buyer:<peerID>", which makes the row strictly less
//     trusted than monitor rows in the DISTINCT-ON priority rule
//     (see pkg/models.DedupePaymentObservations). When monitor +
//     buyer report the same tx, the monitor row wins.
//   - The dispatcher does NOT pull the tx from the chain itself.
//     Caller is responsible for enriching FundingEvent with the
//     verified on-chain fields (amount / from / to / blockNumber /
//     blockTime) BEFORE calling this method. The DoD's "tx 验证失败
//     拒消息" requirement is satisfied at the caller (typically
//     processPaymentSentMessage's existing ValidatePayment +
//     PVS preprocess step), not inside the dispatcher.
//
// Idempotency: same as OnFundingEvent — duplicate inserts on the
// (tenant, chain, tx, eventIndex, observer) UNIQUE tuple collapse to
// a silent no-op + skipped aggregator kick. The same buyer re-sending
// the same PAYMENT_SENT message therefore costs one INSERT attempt
// and one IndexLookup; no spurious aggregator fan-out.
func (d *ObservationDispatcher) OnBuyerReportedPaymentSent(
	ctx context.Context,
	tenantID, buyerPeerID string,
	evt FundingEvent,
) error {
	if strings.TrimSpace(tenantID) == "" {
		return fmt.Errorf("%w: empty tenantID", ErrInvalidFundingEvent)
	}
	if strings.TrimSpace(buyerPeerID) == "" {
		return fmt.Errorf("%w: empty buyerPeerID", ErrInvalidFundingEvent)
	}
	if err := evt.validate(); err != nil {
		return err
	}

	obs := buildObservation(
		tenantID, evt,
		models.PaymentObservationSourceBuyerReported,
		buyerObserver(buyerPeerID),
	)
	return d.insertAndKick(ctx, obs)
}

// insertAndKick is the shared "INSERT observation + (maybe) kick the
// aggregator" tail used by both OnFundingEvent (monitor source) and
// OnBuyerReportedPaymentSent (buyer_reported source). Centralising the
// dedup-vs-aggregate logic prevents the two entry points from drifting
// — the contract that "duplicate inserts skip the aggregator kick" is
// invariant across observation sources and is asserted by the dispatcher
// tests for both paths.
func (d *ObservationDispatcher) insertAndKick(ctx context.Context, obs *models.PaymentObservation) error {
	err := d.repo.InsertObservation(ctx, obs)
	if err != nil {
		if errors.Is(err, contracts.ErrDuplicateObservation) {
			promoted, promoteErr := d.repo.PromoteObservationBlock(ctx, obs)
			if promoteErr != nil {
				return fmt.Errorf("payment: promote observation block for order %s tx %s: %w",
					obs.OrderID, obs.TxHash, promoteErr)
			}
			if !promoted {
				// Same observer replaying the same event — already persisted.
				return nil
			}
			// Fall through to aggregator kick after mempool → confirmed promotion.
		} else {
			return fmt.Errorf("payment: insert observation for order %s tx %s: %w",
				obs.OrderID, obs.TxHash, err)
		}
	}

	if d.aggregator != nil {
		if err := d.aggregator.AggregateAndEmit(ctx, obs.TenantID, obs.OrderID); err != nil {
			return fmt.Errorf("payment: aggregate after insert (order=%s tx=%s): %w",
				obs.OrderID, obs.TxHash, err)
		}
	}
	return nil
}

// buildObservation copies the chain-side fields from evt into a fresh
// PaymentObservation row tagged with the supplied source/observer pair.
// Centralising the field copy keeps OnFundingEvent and
// OnBuyerReportedPaymentSent in lock-step: the two paths always produce
// row shapes that differ only in (Source, Observer), which is exactly
// the DISTINCT-ON axis the aggregator runs on.
func buildObservation(
	tenantID string,
	evt FundingEvent,
	source string,
	observer string,
) *models.PaymentObservation {
	obs := &models.PaymentObservation{
		ID:             uuid.NewString(),
		TenantID:       tenantID,
		OrderID:        evt.OrderID,
		ChainNamespace: evt.ChainNamespace,
		ChainReference: evt.ChainReference,
		TxHash:         evt.TxHash,
		TxHashSource:   models.NormalizePaymentTxHashSource(evt.TxHashSource),
		EventIndex:     evt.EventIndex,
		EventType:      evt.EventType,
		FromAddress:    evt.FromAddress,
		ToAddress:      evt.ToAddress,
		TokenAddress:   evt.TokenAddress,
		BlockNumber:    evt.BlockNumber,
		BlockTime:      evt.BlockTime,
		Confirmations:  0,
		Source:         source,
		Observer:       observer,
		Status:         models.PaymentObservationStatusPending,
	}
	obs.SetAmountBigInt(evt.Amount)
	return obs
}

// buyerObserver returns the observer string for a buyer-reported row.
// Format documented on models.PaymentObservation.Observer; using the
// "buyer:" prefix keeps the priority rule in
// DedupePaymentObservations purely string-based.
func buyerObserver(buyerPeerID string) string {
	return "buyer:" + buyerPeerID
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

	if d.aggregator == nil {
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
	observer := fmt.Sprintf("monitor:%s:%s:%s", namespace, reference, d.workerID)
	if len(observer) <= 64 {
		return observer
	}
	sum := sha256.Sum256([]byte(observer))
	return "monitor:" + hex.EncodeToString(sum[:16])
}
