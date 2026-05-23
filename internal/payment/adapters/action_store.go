package adapters

import (
	"context"
	"errors"
	"sync"
	"time"
)

// Sprint 2 D16 — Action store for ManagedEscrowAdapter.GetActionStatus.
//
// Background. Backend-submitted ManagedEscrow actions persist relay projections
// under an ActionID / ClientActionID before TxHash exists. GetActionStatus
// reads back from the same store once relay submission or confirmation
// updates arrive.
//
// D16 lands the read interface and an in-memory implementation so:
//
//   1. ManagedEscrowAdapter.GetActionStatus is no longer a stub — it can serve
//      a meaningful response the moment a store is wired in.
//   2. D17 can substitute a Postgres / SQLite-backed store without
//      touching adapter call sites.
//   3. Standalone-only deployments and integration tests get a
//      production-grade in-memory variant for free.
//
// The interface is intentionally narrow (Lookup only) — D16 does NOT
// wire writes from the action methods. The relay submission path
// (D17) handles inserts via a separate writer interface that the
// adapter never sees.

// ActionRecord is the canonical view of a tracked ManagedEscrowTx action — the
// projection that callers receive through GetActionStatus. It is a
// strict superset of payment.ActionStatus (state/txHash/confirmations/
// lastError) plus the metadata needed to correlate a record with the
// originating order across logs, dashboards, and reconciliation jobs.
//
// Field rationale:
//   - ActionID — opaque relay-task UUID; the public lookup key.
//   - OrderID + Action + ChainID — diagnostic backlinks. We avoid
//     leaking ManagedEscrowTx hash / signatures here because Lookup is hot on
//     status-poll paths and the caller already has them indexed in
//     the store row.
//   - State — relay_tasks.status vocabulary: pending | submitting |
//     submitted | confirmed | failed | abandoned. We don't enum-type
//     it because the store may add new states (e.g., "stalled" in
//     v1.3) before the adapter does.
//   - TxHash — the LATEST broadcast hash. May change across retries
//     (relay re-broadcasts with bumped gas); always reflects the
//     most recent attempt the store has on file.
//   - Confirmations — counted by the on-chain reader, not the store
//     itself; PaymentMonitor (D-Hybrid-31) updates this field.
//   - LastError — empty on success; carries the most recent failure
//     message verbatim so the operator UI can surface it. Adapters
//     do NOT redact here — log scrubbing happens at the API edge.
//   - CreatedAt / UpdatedAt — for staleness checks. Stored as
//     time.Time (UTC); tests pass time.Time{} to express "unknown".
type ActionRecord struct {
	ActionID      string
	OrderID       string
	Action        string // confirm | cancel | complete | dispute_release
	ChainID       uint64
	State         string
	TxHash        string
	RelayTaskID   string
	Confirmations int
	LastError     string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// ActionStore is the read-side contract ManagedEscrowAdapter consumes. The
// production implementation (D17) sits on top of `relay_tasks`; the
// MemoryActionStore below serves tests and standalone deployments.
//
// Error contract:
//   - Lookup returns ErrActionRecordNotFound (sentinel) when the
//     ActionID is unknown. ManagedEscrowAdapter wraps this into the exported
//     payment.ErrActionNotFound so handlers can return 404.
//   - Any other error is returned verbatim. The adapter wraps it
//     with the ActionID for log correlation but does NOT swallow.
//
// Implementations MUST be safe for concurrent Lookup. The relay
// poller fans out across orders; serialised lookups would be a
// straight-line bottleneck on order-list endpoints.
type ActionStore interface {
	// Lookup returns the latest projection for actionID. Honors ctx
	// cancellation — production stores poll Postgres / SQLite over
	// the same ctx the HTTP handler propagates.
	Lookup(ctx context.Context, actionID string) (*ActionRecord, error)
}

// ActionRecorder persists ActionRecord projections after a successful relay
// submission (RelayBridge). MemoryActionStore and SettlementActionStore both
// implement it via Put — the adapter read path stays ActionStore-only.
type ActionRecorder interface {
	Put(rec ActionRecord) error
}

// ErrActionRecordNotFound is the sentinel ActionStore implementations
// MUST return when the requested actionID is absent. ManagedEscrowAdapter
// translates this into the public payment.ErrActionNotFound.
//
// Rationale for the indirection: the adapter's public surface lives
// in pkg/payment (ErrActionNotFound is part of the V2 contract), but
// adapters and stores live in internal/. Keeping a second sentinel
// here lets callers avoid the pkg/payment import when they only
// implement an ActionStore (e.g., a test fake in a sibling package).
var ErrActionRecordNotFound = errors.New("action store: record not found")

// ────────────────────────────────────────────────────────────────
// MemoryActionStore — in-process implementation.
//
// Used by:
//   - Sprint 2 D16 unit tests (this file's test sibling exercises
//     the contract; managed_escrow_test.go uses it to drive GetActionStatus
//     happy / not-found / cancellation paths).
//   - Standalone-mode deployments that opt out of relay persistence
//     (a node operator who self-broadcasts and tracks status purely
//     in memory; they accept that a node restart drops the records).
//   - Integration tests that need a deterministic store without
//     standing up Postgres.
//
// NOT safe to use as the production SaaS store — there is no durable
// backing, no cross-process visibility, and no automatic eviction.
// Wire a relay_tasks-backed implementation in D17.
// ────────────────────────────────────────────────────────────────

// MemoryActionStore is a goroutine-safe map-backed ActionStore. The
// store accepts inserts via Put (out-of-band of the ActionStore
// interface so adapter code can never call it accidentally) and
// serves them through Lookup.
type MemoryActionStore struct {
	mu      sync.RWMutex
	records map[string]ActionRecord
}

// NewMemoryActionStore constructs an empty in-memory store. Always
// returns a non-nil pointer; callers do not need a separate init
// step. The constructor is intentionally simple — feature toggles
// (eviction TTL, max size) are deferred until usage data shows they
// are needed.
func NewMemoryActionStore() *MemoryActionStore {
	return &MemoryActionStore{
		records: make(map[string]ActionRecord),
	}
}

// Lookup implements ActionStore. Returns a defensive copy so callers
// cannot mutate the stored record by holding the returned pointer.
//
// ctx cancellation is honored opportunistically — the in-memory map
// is fast enough that we don't bother with a goroutine to watch ctx
// in flight; we check once on entry. Production stores doing real
// I/O MUST honor ctx throughout the query.
func (s *MemoryActionStore) Lookup(ctx context.Context, actionID string) (*ActionRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if actionID == "" {
		return nil, ErrActionRecordNotFound
	}
	s.mu.RLock()
	rec, ok := s.records[actionID]
	s.mu.RUnlock()
	if !ok {
		return nil, ErrActionRecordNotFound
	}
	out := rec
	return &out, nil
}

// Put inserts or replaces the record under rec.ActionID. Empty
// ActionID is rejected — that would silently shadow Lookup("").
//
// CreatedAt is set to the current time when zero so tests can omit
// it; UpdatedAt is bumped to the current time on every write.
//
// NOT exposed by the ActionStore interface — only the relay writer
// (and tests) should populate the store; adapter code stays read-
// only.
func (s *MemoryActionStore) Put(rec ActionRecord) error {
	if rec.ActionID == "" {
		return errors.New("action store: ActionID is empty")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.records[rec.ActionID]; ok {
		// Preserve the original CreatedAt across overwrites — this is
		// how the relay writer treats it (UPDATE ... SET updated_at
		// rather than recreating the row).
		if rec.CreatedAt.IsZero() {
			rec.CreatedAt = existing.CreatedAt
		}
	}
	now := time.Now().UTC()
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = now
	}
	rec.UpdatedAt = now
	s.records[rec.ActionID] = rec
	return nil
}

// Len returns the current record count. Useful for tests asserting
// store size after a sequence of writes; not exposed via ActionStore
// because production callers have no business introspecting the
// store's cardinality.
func (s *MemoryActionStore) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.records)
}

// Reset drops every record. Tests use this between subtests so the
// shared adapter wiring stays sticky. Not exposed via ActionStore
// for the same reason as Len.
func (s *MemoryActionStore) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = make(map[string]ActionRecord)
}

// satisfaction guard.
var (
	_ ActionStore    = (*MemoryActionStore)(nil)
	_ ActionRecorder = (*MemoryActionStore)(nil)
)
