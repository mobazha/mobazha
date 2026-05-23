package core

import (
	"context"
	"testing"

	dbgorm "github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/database/dbstore"
	adapters "github.com/mobazha/mobazha3.0/internal/payment/adapters"
	"github.com/stretchr/testify/require"
)

func TestSettlementActionStore_PutLookupRoundTrip(t *testing.T) {
	db, err := dbstore.NewMemoryDB(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, dbgorm.MigrateSettlementActionModels(db))

	s := NewSettlementActionStore(db)
	require.NotNil(t, s)

	rec := adapters.ActionRecord{
		ActionID: "0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		OrderID:  "QmOrderTest",
		Action:   "relay_submit",
		ChainID:  56,
		State:    "submitted",
		TxHash:   "0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
	}
	require.NoError(t, s.Put(rec))

	got, err := s.Lookup(context.Background(), rec.ActionID)
	require.NoError(t, err)
	require.Equal(t, rec.OrderID, got.OrderID)
	require.Equal(t, rec.State, got.State)
	require.Equal(t, rec.ChainID, got.ChainID)
	require.Equal(t, rec.TxHash, got.TxHash)
}
