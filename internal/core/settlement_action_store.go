package core

import (
	"context"
	"errors"
	"time"

	adapters "github.com/mobazha/mobazha3.0/internal/payment/adapters"
	pkgdb "github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"gorm.io/gorm"
)

// SettlementActionStore implements adapters.ActionStore and adapters.ActionRecorder
// on top of the node's tenant-scoped SQL store. The underlying table name is
// still managed_escrow_relay_actions for DB compatibility, but rows now track all
// backend-submitted settlement actions (ManagedEscrow, Solana Anchor, guest relay).
type SettlementActionStore struct {
	db pkgdb.Database
}

// NewSettlementActionStore constructs a DB-backed projection store.
func NewSettlementActionStore(db pkgdb.Database) *SettlementActionStore {
	if db == nil {
		return nil
	}
	return &SettlementActionStore{db: db}
}

// NewManagedEscrowRelayActionStore is kept as a compatibility alias for older tests and
// callers while the projection model name is migrated to settlement actions.
func NewManagedEscrowRelayActionStore(db pkgdb.Database) *SettlementActionStore {
	return NewSettlementActionStore(db)
}

// Lookup implements adapters.ActionStore.
func (s *SettlementActionStore) Lookup(ctx context.Context, actionID string) (*adapters.ActionRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if actionID == "" {
		return nil, adapters.ErrActionRecordNotFound
	}
	var row models.ManagedEscrowRelayAction
	err := s.db.View(func(tx pkgdb.Tx) error {
		return tx.Read().Where("action_id = ?", actionID).First(&row).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, adapters.ErrActionRecordNotFound
	}
	if err != nil {
		return nil, err
	}
	out := &adapters.ActionRecord{
		ActionID:      row.ActionID,
		OrderID:       row.OrderID,
		Action:        row.ActionKind,
		ChainID:       row.ChainID,
		State:         row.State,
		TxHash:        row.TxHash,
		RelayTaskID:   row.RelayTaskID,
		Confirmations: row.Confirmations,
		LastError:     row.LastError,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
	}
	return out, nil
}

// Put implements adapters.ActionRecorder (same semantics as MemoryActionStore.Put).
func (s *SettlementActionStore) Put(rec adapters.ActionRecord) error {
	if rec.ActionID == "" {
		return errors.New("action store: ActionID is empty")
	}
	now := time.Now().UTC()
	row := models.ManagedEscrowRelayAction{
		ActionID:      rec.ActionID,
		OrderID:       rec.OrderID,
		ActionKind:    rec.Action,
		ChainID:       rec.ChainID,
		State:         rec.State,
		TxHash:        rec.TxHash,
		RelayTaskID:   rec.RelayTaskID,
		Confirmations: rec.Confirmations,
		LastError:     rec.LastError,
		CreatedAt:     rec.CreatedAt,
		UpdatedAt:     rec.UpdatedAt,
	}
	var existing models.ManagedEscrowRelayAction
	err := s.db.View(func(tx pkgdb.Tx) error {
		e := tx.Read().Where("action_id = ?", rec.ActionID).First(&existing).Error
		if errors.Is(e, gorm.ErrRecordNotFound) {
			return nil
		}
		if e != nil {
			return e
		}
		if rec.CreatedAt.IsZero() {
			row.CreatedAt = existing.CreatedAt
		}
		return nil
	})
	if err != nil {
		return err
	}
	if row.CreatedAt.IsZero() {
		row.CreatedAt = now
	}
	row.UpdatedAt = now
	return s.db.Update(func(tx pkgdb.Tx) error {
		return tx.Save(&row)
	})
}

var (
	_ adapters.ActionStore    = (*SettlementActionStore)(nil)
	_ adapters.ActionRecorder = (*SettlementActionStore)(nil)
)
