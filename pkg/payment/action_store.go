package payment

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/mobazha/mobazha/pkg/models"
)

const (
	// ManagedEscrowGuestSettlementAction identifies a guest escrow payout.
	ManagedEscrowGuestSettlementAction = "guest_release"
	// ManagedEscrowGuestDeployAction identifies counterfactual guest escrow deployment.
	ManagedEscrowGuestDeployAction = "guest_managed_deploy"
)

// ActionRecord is the canonical durable projection of a backend-submitted
// settlement action. It is shared by trusted distribution adapters and Core
// persistence implementations; API callers receive the narrower ActionStatus.
type ActionRecord struct {
	ActionID        string
	IntentKey       string
	IntentPayload   string
	OrderID         string
	Action          string
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
	LeaseToken      string
	LeaseExpiresAt  *time.Time
	SettlementCoin  string
	GrossAmount     string
	PlannedLines    []models.SettlementPayoutLine
	ObservedLines   []models.SettlementPayoutLine
	ConfirmedAt     *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// ActionStore is the read-side contract used by settlement adapters. A store
// must be safe for concurrent lookups and honor context cancellation.
type ActionStore interface {
	Lookup(ctx context.Context, actionID string) (*ActionRecord, error)
}

// ActionRecorder persists settlement-action projections after a backend
// submitter accepts or updates an action.
type ActionRecorder interface {
	Put(record ActionRecord) error
}

// ErrActionRecordNotFound is returned when a settlement action is unknown.
var ErrActionRecordNotFound = errors.New("action store: record not found")

// ErrActionLeaseLost is returned when a recorder attempts to mutate an
// intent claimed by another execution lease.
var ErrActionLeaseLost = errors.New("action store: execution lease lost")

// ErrActionIntentConflict is returned when an incremental projection attempts
// to rewrite immutable business identity captured by a durable intent.
var ErrActionIntentConflict = errors.New("action store: immutable intent conflict")

// MemoryActionStore is a goroutine-safe, in-memory ActionStore and
// ActionRecorder for tests and non-durable standalone use.
type MemoryActionStore struct {
	mu      sync.RWMutex
	records map[string]ActionRecord
}

// NewMemoryActionStore constructs an empty in-memory action store.
func NewMemoryActionStore() *MemoryActionStore {
	return &MemoryActionStore{records: make(map[string]ActionRecord)}
}

// Lookup returns a defensive copy of the latest action projection.
func (s *MemoryActionStore) Lookup(ctx context.Context, actionID string) (*ActionRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if actionID == "" {
		return nil, ErrActionRecordNotFound
	}
	s.mu.RLock()
	record, ok := s.records[actionID]
	s.mu.RUnlock()
	if !ok {
		return nil, ErrActionRecordNotFound
	}
	result := record
	result.PlannedLines = append([]models.SettlementPayoutLine(nil), record.PlannedLines...)
	result.ObservedLines = append([]models.SettlementPayoutLine(nil), record.ObservedLines...)
	return &result, nil
}

// Put inserts or replaces a projection while preserving durable fields that
// an incremental update omits.
func (s *MemoryActionStore) Put(record ActionRecord) error {
	if record.ActionID == "" {
		return errors.New("action store: ActionID is empty")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.records[record.ActionID]; ok {
		if existing.LeaseToken != "" && record.LeaseToken != existing.LeaseToken {
			return ErrActionLeaseLost
		}
		if actionRecordIntentConflict(existing, record) {
			return ErrActionIntentConflict
		}
		if record.IntentKey == "" {
			record.IntentKey = existing.IntentKey
		}
		if record.IntentPayload == "" {
			record.IntentPayload = existing.IntentPayload
		}
		if record.LeaseToken == "" {
			record.LeaseToken = existing.LeaseToken
		}
		if record.LeaseExpiresAt == nil {
			record.LeaseExpiresAt = existing.LeaseExpiresAt
		}
		if record.CreatedAt.IsZero() {
			record.CreatedAt = existing.CreatedAt
		}
		if record.SettlementCoin == "" {
			record.SettlementCoin = existing.SettlementCoin
		}
		if record.GrossAmount == "" {
			record.GrossAmount = existing.GrossAmount
		}
		if len(record.PlannedLines) == 0 {
			record.PlannedLines = existing.PlannedLines
		}
		if len(record.ObservedLines) == 0 {
			record.ObservedLines = existing.ObservedLines
		}
		if record.ConfirmedAt == nil {
			record.ConfirmedAt = existing.ConfirmedAt
		}
	}
	now := time.Now().UTC()
	if record.CreatedAt.IsZero() {
		record.CreatedAt = now
	}
	record.UpdatedAt = now
	record.PlannedLines = append([]models.SettlementPayoutLine(nil), record.PlannedLines...)
	record.ObservedLines = append([]models.SettlementPayoutLine(nil), record.ObservedLines...)
	s.records[record.ActionID] = record
	return nil
}

func actionRecordIntentConflict(existing, incoming ActionRecord) bool {
	if existing.IntentKey == "" {
		return false
	}
	return (incoming.IntentKey != "" && incoming.IntentKey != existing.IntentKey) ||
		(incoming.IntentPayload != "" && incoming.IntentPayload != existing.IntentPayload) ||
		(incoming.OrderID != "" && incoming.OrderID != existing.OrderID) ||
		(incoming.Action != "" && incoming.Action != existing.Action) ||
		(incoming.ChainID != 0 && incoming.ChainID != existing.ChainID) ||
		(incoming.SettlementCoin != "" && incoming.SettlementCoin != existing.SettlementCoin) ||
		(incoming.GrossAmount != "" && incoming.GrossAmount != existing.GrossAmount)
}

// Len returns the number of stored projections.
func (s *MemoryActionStore) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.records)
}

// Reset removes all projections.
func (s *MemoryActionStore) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = make(map[string]ActionRecord)
}

var (
	_ ActionStore    = (*MemoryActionStore)(nil)
	_ ActionRecorder = (*MemoryActionStore)(nil)
)
