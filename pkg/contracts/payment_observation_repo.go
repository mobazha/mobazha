package contracts

import (
	"context"
	"errors"

	"github.com/mobazha/mobazha/pkg/models"
)

// ErrDuplicateObservation is the sentinel error returned by
// PaymentObservationRepo.InsertObservation when the composite UNIQUE index
//
//	(tenant_id, chain_namespace, chain_reference, tx_hash, event_index, observer)
//
// rejects a row because the same observer has already recorded the same chain
// event.
//
// This error is a normal part of the idempotency contract: a monitor worker
// that restarts mid-batch, or whose RPC client replays an earlier event, will
// retry the insert and MUST treat ErrDuplicateObservation as success. Callers
// should distinguish it from arbitrary database errors using errors.Is.
//
// See docs/escrow/MONITOR_DRIVEN_PAYMENT.md (v2.0) §3.1 for the full dedupe
// contract; §5.1 walks through the worker-side error handling.
var ErrDuplicateObservation = errors.New("payment observation already recorded by this observer")

// OrderRef identifies an order tuple in tenant scope. Returned by repo
// methods that need to surface multiple affected orders (e.g.
// RefreshConfirmations propagating "rows newly transitioned to confirmed"
// back to the aggregator).
type OrderRef struct {
	TenantID string
	OrderID  string
}

// PaymentObservationRepo is the storage port for the append-only fact table
// underpinning monitor-driven payment verification.
//
// The repo deliberately exposes a small, opinionated surface:
//
//   - InsertObservation             — append a new fact row (idempotent on
//     UNIQUE conflict).
//   - ListDeduplicatedConfirmed     — return one row per (chain, tx,
//     event_index) tuple, picked by observer priority. This is the SELECT
//     consumed by VerificationService.AggregateAndEmit.
//   - ListByOrder                   — return ALL rows for an order, regardless
//     of status. Intended for audit / dispute review and tests; the aggregator
//     does NOT use this entry point.
//   - RefreshConfirmations          — bulk advance pending → confirmed for
//     rows whose block has reached chain quorum, returning the affected
//     (tenant, order) tuples so callers can re-trigger aggregation.
//
// Reorg handling (MarkReverted), order-by-managed-escrow-address lookup, and chain-head
// tracking live in step 3 (PaymentMonitor) and intentionally are NOT part of
// this port. Keeping the surface narrow lets us evolve the implementation
// (Go-side dedupe → window-function dedupe, single-row UPDATE → batched
// UPDATE … RETURNING) without churning callers.
//
// All methods are tenant-scoped. The TenantID required by InsertObservation
// is taken from the provided model; read methods take it explicitly to match
// the existing OrderRepo style and to make tenant boundaries unambiguous in
// call sites.
type PaymentObservationRepo interface {
	// InsertObservation persists obs as a new row.
	//
	// Returns ErrDuplicateObservation when the dedupe tuple
	// (tenant_id, chain_namespace, chain_reference, tx_hash, event_index, observer)
	// already exists. All other errors are storage failures.
	//
	// On success the caller's *obs is left as-is (no fields are mutated by
	// the repo).
	InsertObservation(ctx context.Context, obs *models.PaymentObservation) error

	// PromoteObservationBlock advances block metadata on an existing row keyed
	// by the observation dedupe tuple when the same chain event is later
	// observed with a higher block_number (mempool → confirmed inclusion).
	// Returns true when a row was updated.
	PromoteObservationBlock(ctx context.Context, obs *models.PaymentObservation) (bool, error)

	// ListDeduplicatedConfirmed returns confirmed observations for the order,
	// deduplicated to one row per (chain_namespace, chain_reference, tx_hash,
	// event_index) tuple.
	//
	// Selection rule (matches docs/escrow/MONITOR_DRIVEN_PAYMENT.md §3.2):
	//   1. status = "confirmed" only — pending and reverted rows are filtered.
	//   2. monitor source wins over buyer_reported source.
	//   3. earliest BlockTime breaks ties between same-source observers.
	//
	// Implementations MAY perform the dedupe in SQL (DISTINCT ON / ROW_NUMBER)
	// or in Go; the contract is the result, not the mechanism.
	ListDeduplicatedConfirmed(ctx context.Context, tenantID, orderID string) ([]models.PaymentObservation, error)

	// ListByOrder returns every observation row for the order in stable order
	// (CreatedAt ascending, ID ascending as deterministic tiebreaker). Used
	// by audit / dispute review and tests; not on the verification hot path.
	ListByOrder(ctx context.Context, tenantID, orderID string) ([]models.PaymentObservation, error)

	// RefreshConfirmations advances every observation row matching
	// (chain_namespace, chain_reference) whose status is "pending" and whose
	// block has been buried by at least requiredConfirmations blocks under
	// currentBlockNumber, transitioning it to "confirmed" and updating the
	// rolling Confirmations field.
	//
	// Returns the set of (tenantID, orderID) tuples for which at least one
	// row newly transitioned to "confirmed", with no duplicates and a
	// deterministic order (TenantID asc, OrderID asc). The caller is
	// expected to invoke VerificationService.AggregateAndEmit on each tuple;
	// repos do not call into the aggregator themselves.
	//
	// requiredConfirmations is expressed as the additive depth (e.g. 12 for
	// ETH/BSC, 2 for L2s); the underlying SQL is
	//
	//   block_number > 0 AND block_number <= currentBlockNumber - requiredConfirmations
	//
	// so a row included at block N is confirmed once the chain head reaches
	// N + requiredConfirmations.
	//
	// requiredConfirmations of zero is permitted (some test/devnet flows
	// confirm at first inclusion); negative values are rejected.
	RefreshConfirmations(
		ctx context.Context,
		chainNamespace, chainReference string,
		currentBlockNumber int64,
		requiredConfirmations int,
	) ([]OrderRef, error)
}
