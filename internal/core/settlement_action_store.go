package core

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	adapters "github.com/mobazha/mobazha3.0/internal/payment/adapters"
	pkgdb "github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"gorm.io/gorm"
)

// SettlementActionStore implements adapters.ActionStore and adapters.ActionRecorder
// on top of the node's tenant-scoped SQL store. Rows track backend-submitted
// settlement actions across ManagedEscrow, Solana Anchor, UTXO, and guest relay flows.
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

// Lookup implements adapters.ActionStore.
func (s *SettlementActionStore) Lookup(ctx context.Context, actionID string) (*adapters.ActionRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if actionID == "" {
		return nil, adapters.ErrActionRecordNotFound
	}
	var row models.SettlementAction
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
		ActionID:        row.ActionID,
		OrderID:         row.OrderID,
		Action:          row.ActionKind,
		ChainID:         row.ChainID,
		To:              row.To,
		Data:            row.Data,
		State:           row.State,
		TxHash:          row.TxHash,
		AttemptTxHashes: row.AttemptTxHashes,
		RelayTaskID:     row.RelayTaskID,
		Attempts:        row.Attempts,
		Confirmations:   row.Confirmations,
		LastError:       row.LastError,
		SettlementCoin:  row.SettlementCoin,
		GrossAmount:     row.GrossAmount,
		PlannedLines:    models.DecodeSettlementPayoutLines(row.PlannedLines),
		ObservedLines:   models.DecodeSettlementPayoutLines(row.ObservedLines),
		ConfirmedAt:     row.ConfirmedAt,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
	return out, nil
}

// Put implements adapters.ActionRecorder (same semantics as MemoryActionStore.Put).
func (s *SettlementActionStore) Put(rec adapters.ActionRecord) error {
	if rec.ActionID == "" {
		return errors.New("action store: ActionID is empty")
	}
	now := time.Now().UTC()
	row := models.SettlementAction{
		ActionID:        rec.ActionID,
		OrderID:         rec.OrderID,
		ActionKind:      rec.Action,
		ChainID:         rec.ChainID,
		To:              rec.To,
		Data:            rec.Data,
		State:           rec.State,
		TxHash:          rec.TxHash,
		AttemptTxHashes: rec.AttemptTxHashes,
		RelayTaskID:     rec.RelayTaskID,
		Attempts:        rec.Attempts,
		Confirmations:   rec.Confirmations,
		LastError:       rec.LastError,
		SettlementCoin:  rec.SettlementCoin,
		GrossAmount:     rec.GrossAmount,
		PlannedLines:    models.EncodeSettlementPayoutLines(rec.PlannedLines),
		ObservedLines:   models.EncodeSettlementPayoutLines(rec.ObservedLines),
		ConfirmedAt:     rec.ConfirmedAt,
		CreatedAt:       rec.CreatedAt,
		UpdatedAt:       rec.UpdatedAt,
	}
	var existing models.SettlementAction
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
		if row.TxHash == "" {
			row.TxHash = existing.TxHash
		}
		if row.To == "" {
			row.To = existing.To
		}
		if row.Data == "" {
			row.Data = existing.Data
		}
		if row.Attempts < existing.Attempts {
			row.Attempts = existing.Attempts
		}
		if row.SettlementCoin == "" {
			row.SettlementCoin = existing.SettlementCoin
		}
		if row.GrossAmount == "" {
			row.GrossAmount = existing.GrossAmount
		}
		if len(row.PlannedLines) == 0 {
			row.PlannedLines = existing.PlannedLines
		}
		if len(row.ObservedLines) == 0 {
			row.ObservedLines = existing.ObservedLines
		}
		if row.ConfirmedAt == nil {
			row.ConfirmedAt = existing.ConfirmedAt
		}
		row.AttemptTxHashes = mergeSettlementActionTxHashes(existing.AttemptTxHashes, row.AttemptTxHashes, existing.TxHash, row.TxHash)
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

func (s *SettlementActionStore) ClaimRetry(row models.SettlementAction, nextAttempt int) (string, bool, error) {
	if s == nil || s.db == nil {
		return "", false, nil
	}
	attemptTxHashes := mergeSettlementActionTxHashes(row.AttemptTxHashes, row.TxHash)
	rows, err := s.updateActionColumns(
		map[string]interface{}{
			"attempt_tx_hashes": attemptTxHashes,
			"attempts":          nextAttempt,
			"last_error":        "relay retry in progress",
			"updated_at":        time.Now().UTC(),
		},
		map[string]interface{}{
			"action_id = ?": row.ActionID,
			"state = ?":     row.State,
			"tx_hash = ?":   row.TxHash,
			"attempts = ?":  row.Attempts,
		},
	)
	return attemptTxHashes, rows == 1, err
}

func (s *SettlementActionStore) DeferRetry(row models.SettlementAction, reason string) error {
	if s == nil || s.db == nil {
		return nil
	}
	state := row.State
	if state == "" {
		state = "submitted"
	}
	_, err := s.updateActionColumns(
		map[string]interface{}{
			"state":      state,
			"last_error": reason,
			"updated_at": time.Now().UTC(),
		},
		map[string]interface{}{
			"action_id = ?": row.ActionID,
		},
	)
	return err
}

func (s *SettlementActionStore) RecordRetrySubmitted(row models.SettlementAction, txHash, attemptTxHashes string, attempts int) error {
	if s == nil || s.db == nil {
		return nil
	}
	_, err := s.updateActionColumns(
		map[string]interface{}{
			"state":             "submitted",
			"tx_hash":           txHash,
			"attempt_tx_hashes": mergeSettlementActionTxHashes(attemptTxHashes, txHash),
			"attempts":          attempts,
			"confirmations":     0,
			"last_error":        "",
			"updated_at":        time.Now().UTC(),
		},
		map[string]interface{}{
			"action_id = ?": row.ActionID,
		},
	)
	return err
}

func (s *SettlementActionStore) MarkTerminal(row models.SettlementAction, state, reason string) error {
	return s.RecordStatus(row, SettlementActionStatusUpdate{
		State:     state,
		LastError: reason,
	})
}

type SettlementActionStatusUpdate struct {
	State         string
	TxHash        string
	Confirmations int
	LastError     string
	ObservedLines []models.SettlementPayoutLine
}

func (s *SettlementActionStore) RecordStatus(row models.SettlementAction, update SettlementActionStatusUpdate) error {
	if s == nil || s.db == nil {
		return nil
	}
	values := map[string]interface{}{
		"confirmations": update.Confirmations,
		"last_error":    update.LastError,
		"updated_at":    time.Now().UTC(),
	}
	if update.State != "" {
		values["state"] = update.State
	}
	if update.TxHash != "" {
		values["tx_hash"] = update.TxHash
		values["attempt_tx_hashes"] = mergeSettlementActionTxHashes(row.AttemptTxHashes, row.TxHash, update.TxHash)
	}
	if strings.EqualFold(update.State, "confirmed") {
		now := time.Now().UTC()
		values["confirmed_at"] = now
		if len(update.ObservedLines) > 0 {
			values["observed_lines"] = models.EncodeSettlementPayoutLines(update.ObservedLines)
		} else if len(row.ObservedLines) == 0 && len(row.PlannedLines) > 0 {
			values["observed_lines"] = row.PlannedLines
		}
	}
	_, err := s.updateActionColumns(
		values,
		map[string]interface{}{
			"action_id = ?": row.ActionID,
		},
	)
	return err
}

func (s *SettlementActionStore) updateActionColumns(values, where map[string]interface{}) (int64, error) {
	var rows int64
	err := s.db.Update(func(tx pkgdb.Tx) error {
		affected, err := tx.UpdateColumns(values, where, &models.SettlementAction{})
		if err != nil {
			return err
		}
		rows = affected
		return nil
	})
	return rows, err
}

var (
	_ adapters.ActionStore    = (*SettlementActionStore)(nil)
	_ adapters.ActionRecorder = (*SettlementActionStore)(nil)
)

func mergeSettlementActionTxHashes(parts ...string) string {
	var out []string
	seen := make(map[string]struct{})
	add := func(raw string) {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return
		}
		key := raw
		value := raw
		if common.IsHexHash(raw) {
			hash := common.HexToHash(raw)
			if hash == (common.Hash{}) {
				return
			}
			key = strings.ToLower(hash.Hex())
			value = hash.Hex()
		}
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	for _, part := range parts {
		for _, raw := range strings.FieldsFunc(part, func(r rune) bool {
			return r == ',' || r == '\n' || r == '\r' || r == '\t' || r == ' '
		}) {
			add(raw)
		}
	}
	return strings.Join(out, "\n")
}
