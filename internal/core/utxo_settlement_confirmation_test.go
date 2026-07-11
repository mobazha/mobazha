package core

import (
	"context"
	"testing"
	"time"

	dbgorm "github.com/mobazha/mobazha/internal/database"
	"github.com/mobazha/mobazha/internal/database/dbstore"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	pkgutxo "github.com/mobazha/mobazha/pkg/utxo"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"github.com/stretchr/testify/require"
)

func TestRunUTXOSettlementConfirmationsOnceRequiresConfirmedMatchingOutputs(t *testing.T) {
	db, err := dbstore.NewMemoryDB(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, dbgorm.MigrateSettlementActionModels(db))
	now := time.Now().UTC()
	planned := []models.SettlementPayoutLine{
		{Type: "seller", Amount: "900", Address: "seller-address", Coin: "BTC"},
		{Type: "affiliate", Amount: "100", Address: "affiliate-address", Coin: "BTC"},
	}
	row := models.SettlementAction{
		ActionID: "sync-confirm-order-1", OrderID: "order-1", ActionKind: "confirm",
		State: "submitted", TxHash: "tx-1", SettlementCoin: "BTC", GrossAmount: "1000",
		PlannedLines: models.EncodeSettlementPayoutLines(planned), CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, db.Update(func(tx database.Tx) error { return tx.Save(&row) }))

	source := newMockUTXOPaymentSource(iwallet.ChainBitcoin)
	source.confirmations = 1
	source.AddTransaction("affiliate-address", &iwallet.Transaction{
		ID: iwallet.TransactionID("tx-1"),
		To: []iwallet.SpendInfo{
			{Address: iwallet.NewAddress("seller-address", iwallet.CoinType("BTC")), Amount: iwallet.NewAmount("900")},
			{Address: iwallet.NewAddress("affiliate-address", iwallet.CoinType("BTC")), Amount: iwallet.NewAmount("100")},
		},
	})
	monitor := pkgutxo.NewMonitor(pkgutxo.DefaultMonitorConfig())
	monitor.AddSource(iwallet.ChainBitcoin, source)
	node := &MobazhaNode{
		identityFields: identityFields{nodeID: "seller-node"},
		storageFields:  storageFields{db: db},
		chainFields:    chainFields{monitorService: monitor},
	}

	node.runUTXOSettlementConfirmationsOnce(context.Background())

	var got models.SettlementAction
	require.NoError(t, db.View(func(tx database.Tx) error {
		return tx.Read().Where("action_id = ?", row.ActionID).First(&got).Error
	}))
	require.Equal(t, "confirmed", got.State)
	require.Equal(t, 1, got.Confirmations)
	require.NotNil(t, got.ConfirmedAt)
	observed := models.DecodeSettlementPayoutLines(got.ObservedLines)
	require.Len(t, observed, 2)
	require.Equal(t, "affiliate", observed[1].Type)
	require.Equal(t, "tx-1", observed[1].TxHash)
}

func TestObservedUTXOSettlementLinesRejectsMissingAffiliateOutput(t *testing.T) {
	planned := []models.SettlementPayoutLine{{Type: "affiliate", Amount: "100", Address: "affiliate-address", Coin: "BTC"}}
	tx := iwallet.Transaction{To: []iwallet.SpendInfo{{
		Address: iwallet.NewAddress("seller-address", iwallet.CoinType("BTC")), Amount: iwallet.NewAmount("100"),
	}}}

	_, err := observedUTXOSettlementLines(planned, tx, "tx-1")

	require.Error(t, err)
	require.Contains(t, err.Error(), "planned affiliate output")
}
