package core

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	adapters "github.com/mobazha/mobazha/internal/payment/adapters"
	pkgdb "github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	"gorm.io/gorm"
)

// SettlementActionStore implements adapters.ActionStore and adapters.ActionRecorder
// on top of the node's tenant-scoped SQL store. Rows track backend-submitted
// settlement actions across backend-managed contract rails, UTXO, and guest relay flows.
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

// ListSettlementActions loads order-scoped action snapshots for read-model
// projections such as affiliate statements. The tenant-scoped database handle
// enforces the caller's tenant boundary.
func (s *SettlementActionStore) ListSettlementActions(ctx context.Context, orderIDs []string) ([]models.SettlementActionSnapshot, error) {
	if s == nil || s.db == nil || len(orderIDs) == 0 {
		return nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	rows := make([]models.SettlementAction, 0)
	if err := s.db.View(func(tx pkgdb.Tx) error {
		return tx.Read().WithContext(ctx).
			Where("order_id IN ?", orderIDs).
			Order("updated_at DESC, action_id ASC").
			Find(&rows).Error
	}); err != nil {
		return nil, err
	}
	out := make([]models.SettlementActionSnapshot, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.Snapshot())
	}
	return out, nil
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
		ActionID: row.ActionID,
		Route: adapters.RouteIdentity{
			ContributionID: row.RouteContributionID, ModuleID: row.RouteModuleID,
			ImplementationGeneration: row.RouteImplementationGeneration,
			RailKind:                 row.RouteRailKind, NetworkID: row.RouteNetworkID, AssetID: row.RouteAssetID,
			ProtocolVersion: row.RouteProtocolVersion, StateSchemaVersion: row.RouteStateSchemaVersion,
		},
		IntentKey:       row.IntentKey,
		IntentPayload:   row.IntentPayload,
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
		LeaseToken:      row.LeaseToken,
		LeaseExpiresAt:  row.LeaseExpiresAt,
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
	if !rec.Route.IsZero() {
		if err := rec.Route.Validate(); err != nil {
			return err
		}
	}
	now := time.Now().UTC()
	row := models.SettlementAction{
		ActionID:            rec.ActionID,
		RouteContributionID: rec.Route.ContributionID, RouteModuleID: rec.Route.ModuleID,
		RouteImplementationGeneration: rec.Route.ImplementationGeneration,
		RouteRailKind:                 rec.Route.RailKind, RouteNetworkID: rec.Route.NetworkID, RouteAssetID: rec.Route.AssetID,
		RouteProtocolVersion: rec.Route.ProtocolVersion, RouteStateSchemaVersion: rec.Route.StateSchemaVersion,
		IntentKey:       rec.IntentKey,
		IntentPayload:   rec.IntentPayload,
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
		LeaseToken:      rec.LeaseToken,
		LeaseExpiresAt:  rec.LeaseExpiresAt,
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
		if existing.LeaseToken != "" && rec.LeaseToken != existing.LeaseToken {
			return adapters.ErrActionLeaseLost
		}
		if settlementActionIntentConflict(existing, rec) {
			return adapters.ErrActionIntentConflict
		}
		if row.IntentKey == "" {
			row.IntentKey = existing.IntentKey
		}
		if rec.Route.IsZero() {
			row.RouteContributionID = existing.RouteContributionID
			row.RouteModuleID = existing.RouteModuleID
			row.RouteImplementationGeneration = existing.RouteImplementationGeneration
			row.RouteRailKind = existing.RouteRailKind
			row.RouteNetworkID = existing.RouteNetworkID
			row.RouteAssetID = existing.RouteAssetID
			row.RouteProtocolVersion = existing.RouteProtocolVersion
			row.RouteStateSchemaVersion = existing.RouteStateSchemaVersion
		}
		if row.IntentPayload == "" {
			row.IntentPayload = existing.IntentPayload
		}
		if row.LeaseToken == "" {
			row.LeaseToken = existing.LeaseToken
		}
		if row.LeaseExpiresAt == nil {
			row.LeaseExpiresAt = existing.LeaseExpiresAt
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
	if existing.ActionID != "" && existing.LeaseToken != "" {
		rows, err := s.updateActionColumns(settlementActionValues(row), map[string]interface{}{
			"action_id = ?":   row.ActionID,
			"lease_token = ?": existing.LeaseToken,
		})
		if err != nil {
			return err
		}
		if rows != 1 {
			return adapters.ErrActionLeaseLost
		}
		return nil
	}
	return s.db.Update(func(tx pkgdb.Tx) error {
		return tx.Save(&row)
	})
}

func settlementActionIntentConflict(existing models.SettlementAction, incoming adapters.ActionRecord) bool {
	existingRoute := adapters.RouteIdentity{
		ContributionID: existing.RouteContributionID, ModuleID: existing.RouteModuleID,
		ImplementationGeneration: existing.RouteImplementationGeneration,
		RailKind:                 existing.RouteRailKind, NetworkID: existing.RouteNetworkID, AssetID: existing.RouteAssetID,
		ProtocolVersion: existing.RouteProtocolVersion, StateSchemaVersion: existing.RouteStateSchemaVersion,
	}
	if !existingRoute.IsZero() && !incoming.Route.IsZero() && incoming.Route != existingRoute {
		return true
	}
	if existing.IntentKey == "" {
		return false
	}
	return (incoming.IntentKey != "" && incoming.IntentKey != existing.IntentKey) ||
		(incoming.IntentPayload != "" && incoming.IntentPayload != existing.IntentPayload) ||
		(incoming.OrderID != "" && incoming.OrderID != existing.OrderID) ||
		(incoming.Action != "" && incoming.Action != existing.ActionKind) ||
		(incoming.ChainID != 0 && incoming.ChainID != existing.ChainID) ||
		(incoming.SettlementCoin != "" && incoming.SettlementCoin != existing.SettlementCoin) ||
		(incoming.GrossAmount != "" && incoming.GrossAmount != existing.GrossAmount)
}

func settlementActionValues(row models.SettlementAction) map[string]interface{} {
	return map[string]interface{}{
		"intent_key": row.IntentKey, "intent_payload": row.IntentPayload, "order_id": row.OrderID, "action_kind": row.ActionKind,
		"route_contribution_id": row.RouteContributionID, "route_module_id": row.RouteModuleID,
		"route_implementation_generation": row.RouteImplementationGeneration, "route_rail_kind": row.RouteRailKind,
		"route_network_id": row.RouteNetworkID, "route_asset_id": row.RouteAssetID,
		"route_protocol_version": row.RouteProtocolVersion, "route_state_schema_version": row.RouteStateSchemaVersion,
		"chain_id": row.ChainID, "to_address": row.To, "call_data": row.Data,
		"state": row.State, "tx_hash": row.TxHash, "attempt_tx_hashes": row.AttemptTxHashes,
		"relay_task_id": row.RelayTaskID, "attempts": row.Attempts, "confirmations": row.Confirmations,
		"last_error": row.LastError, "lease_token": row.LeaseToken, "lease_expires_at": row.LeaseExpiresAt,
		"settlement_coin": row.SettlementCoin, "gross_amount": row.GrossAmount,
		"planned_lines": row.PlannedLines, "observed_lines": row.ObservedLines,
		"confirmed_at": row.ConfirmedAt, "updated_at": row.UpdatedAt,
	}
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
