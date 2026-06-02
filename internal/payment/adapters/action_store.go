package adapters

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/models"
)

// ActionRecord is the canonical view of a backend-submitted settlement action — the
// projection that callers receive through GetActionStatus. It is a
// strict superset of payment.ActionStatus (state/txHash/confirmations/
// lastError) plus the metadata needed to correlate a record with the
// originating order across logs, dashboards, and reconciliation jobs.
//
// Field rationale:
//   - ActionID — opaque relay-task UUID; the public lookup key.
//   - OrderID + Action + ChainID — diagnostic backlinks.
//   - To + Data — internal durable relay intent. They let the reconciler
//     resubmit a dropped transaction and are intentionally not exposed by
//     payment.ActionStatus.
//   - State — settlement action status vocabulary: pending | submitting |
//     submitted | confirmed | failed | abandoned. We don't enum-type
//     it because the store may add new states (e.g., "stalled" in
//     v1.3) before the adapter does.
//   - TxHash — the LATEST broadcast hash. May change across retries
//     (core resubmits a dropped relay call); always reflects the
//     most recent attempt the store has on file.
//   - Confirmations — counted by the on-chain reader, not the store
//     itself; PaymentMonitor (D-Hybrid-31) updates this field.
//   - LastError — empty on success; carries the most recent failure
//     message verbatim so the operator UI can surface it. Adapters
//     do NOT redact here — log scrubbing happens at the API edge.
//   - CreatedAt / UpdatedAt — for staleness checks. Stored as
//     time.Time (UTC); tests pass time.Time{} to express "unknown".
type ActionRecord struct {
	ActionID        string
	OrderID         string
	Action          string // confirm | cancel | complete | dispute_release
	ChainID         uint64
	To              string
	Data            string
	State           string
	TxHash          string
	AttemptTxHashes string
	RelayTaskID     string
	Attempts        int
	Confirmations   int
	LastError       string
	SettlementCoin  string
	GrossAmount     string
	PlannedLines    []models.SettlementPayoutLine
	ObservedLines   []models.SettlementPayoutLine
	ConfirmedAt     *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// ActionStore is the read-side contract settlement adapters consume. The
// production implementation persists durable command-outbox projections; the
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

// ActionRecorder persists ActionRecord projections after a backend submitter
// accepts an action. MemoryActionStore and SettlementActionStore both
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
//   - Unit tests that exercise GetActionStatus happy / not-found /
//     cancellation paths.
//   - Standalone-mode deployments that opt out of relay persistence
//     (a node operator who self-broadcasts and tracks status purely
//     in memory; they accept that a node restart drops the records).
//   - Integration tests that need a deterministic store without
//     standing up Postgres.
//
// NOT safe to use as the production SaaS store — there is no durable
// backing, no cross-process visibility, and no automatic eviction.
// Wire a durable SQL implementation for production use.
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
		if rec.SettlementCoin == "" {
			rec.SettlementCoin = existing.SettlementCoin
		}
		if rec.GrossAmount == "" {
			rec.GrossAmount = existing.GrossAmount
		}
		if len(rec.PlannedLines) == 0 {
			rec.PlannedLines = existing.PlannedLines
		}
		if len(rec.ObservedLines) == 0 {
			rec.ObservedLines = existing.ObservedLines
		}
		if rec.ConfirmedAt == nil {
			rec.ConfirmedAt = existing.ConfirmedAt
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
