package core

import (
	"context"
	"testing"
	"time"

	dbgorm "github.com/mobazha/mobazha/internal/database"
	"github.com/mobazha/mobazha/internal/database/dbstore"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/mobazha/mobazha/pkg/payment"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"github.com/stretchr/testify/require"
)

type recordingActionReconciler struct {
	payment.ChainEscrowV2
	actionIDs []string
	status    *payment.ActionStatus
	err       error
}

func (r *recordingActionReconciler) ReconcileAction(_ context.Context, actionID string) (*payment.ActionStatus, error) {
	r.actionIDs = append(r.actionIDs, actionID)
	return r.status, r.err
}

func TestLoadPendingManagedSettlementActionsSelectsProviderOwnedRows(t *testing.T) {
	db, err := dbstore.NewMemoryDB(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, dbgorm.MigrateSettlementActionModels(db))
	now := time.Now().UTC()
	rows := []models.SettlementAction{
		{ActionID: "managed", OrderID: "order-1", ActionKind: "complete", SettlementCoin: "crypto:solana:mainnet:native", State: "submitted", CreatedAt: now, UpdatedAt: now},
		{ActionID: "evm", OrderID: "order-2", ActionKind: "complete", SettlementCoin: "crypto:eip155:56:native", ChainID: 56, State: "submitted", CreatedAt: now, UpdatedAt: now},
		{ActionID: "unclassified", OrderID: "order-3", ActionKind: "complete", State: "submitted", CreatedAt: now, UpdatedAt: now},
	}
	require.NoError(t, db.Update(func(tx database.Tx) error {
		for i := range rows {
			if err := tx.Save(&rows[i]); err != nil {
				return err
			}
		}
		return nil
	}))

	node := &MobazhaNode{storageFields: storageFields{db: db}}
	loaded, err := node.loadPendingManagedSettlementActions(context.Background(), 100)
	require.NoError(t, err)
	require.Len(t, loaded, 1)
	require.Equal(t, "managed", loaded[0].ActionID)
}

func TestRunManagedSettlementReconciliationsDispatchesToOwningStrategy(t *testing.T) {
	db, err := dbstore.NewMemoryDB(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, dbgorm.MigrateSettlementActionModels(db))
	coin, err := iwallet.RequireCanonicalNativeCoinType(iwallet.ChainSolana)
	require.NoError(t, err)
	now := time.Now().UTC()
	row := models.SettlementAction{
		ActionID: "managed-action", OrderID: "order-1", ActionKind: "complete",
		SettlementCoin: coin.String(), State: "submitted", CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, db.Update(func(tx database.Tx) error { return tx.Save(&row) }))

	reconciler := &recordingActionReconciler{status: &payment.ActionStatus{State: "submitted"}}
	registry := payment.NewRegistry()
	registry.RegisterV2(iwallet.ChainSolana, reconciler)
	node := &MobazhaNode{
		storageFields: storageFields{db: db},
		walletFields:  walletFields{paymentRegistry: registry},
	}

	node.runManagedSettlementReconciliationsOnce(context.Background())
	require.Equal(t, []string{"managed-action"}, reconciler.actionIDs)
}
