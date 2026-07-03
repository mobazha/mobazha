package core

import (
	"context"
	"testing"
	"time"

	dbgorm "github.com/mobazha/mobazha/internal/database"
	"github.com/mobazha/mobazha/internal/database/dbstore"
	adapters "github.com/mobazha/mobazha/internal/payment/adapters"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/stretchr/testify/require"
)

func TestSettlementActionStore_PutLookupRoundTrip(t *testing.T) {
	db, err := dbstore.NewMemoryDB(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, dbgorm.MigrateSettlementActionModels(db))

	s := NewSettlementActionStore(db)
	require.NotNil(t, s)

	rec := adapters.ActionRecord{
		ActionID:        "0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		OrderID:         "QmOrderTest",
		Action:          "relay_submit",
		ChainID:         56,
		To:              "0x1111111111111111111111111111111111111111",
		Data:            "0xdeadbeef",
		State:           "submitted",
		TxHash:          "0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		AttemptTxHashes: "0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		Attempts:        2,
		SettlementCoin:  "crypto:eip155:1:erc20:0xdAC17F958D2ee523a2206206994597C13D831ec7",
		GrossAmount:     "292929",
		PlannedLines: []models.SettlementPayoutLine{{
			Type:    "seller",
			Amount:  "141414",
			Address: "0xb9a2226c9da66db8210edfc51ede121e977e2e39",
			Coin:    "crypto:eip155:1:erc20:0xdAC17F958D2ee523a2206206994597C13D831ec7",
		}, {
			Type:    "platform",
			Amount:  "151515",
			Address: "0x10d44982e0e50bcbf4c1df72f8c43497baf74668",
			Coin:    "crypto:eip155:1:erc20:0xdAC17F958D2ee523a2206206994597C13D831ec7",
		}},
	}
	require.NoError(t, s.Put(rec))

	got, err := s.Lookup(context.Background(), rec.ActionID)
	require.NoError(t, err)
	require.Equal(t, rec.OrderID, got.OrderID)
	require.Equal(t, rec.State, got.State)
	require.Equal(t, rec.ChainID, got.ChainID)
	require.Equal(t, rec.TxHash, got.TxHash)
	require.Equal(t, rec.AttemptTxHashes, got.AttemptTxHashes)
	require.Equal(t, rec.To, got.To)
	require.Equal(t, rec.Data, got.Data)
	require.Equal(t, rec.Attempts, got.Attempts)
	require.Equal(t, rec.SettlementCoin, got.SettlementCoin)
	require.Equal(t, rec.GrossAmount, got.GrossAmount)
	require.Equal(t, rec.PlannedLines, got.PlannedLines)
}

func TestSettlementActionStore_PutRequiresCurrentExecutionLease(t *testing.T) {
	db, err := dbstore.NewMemoryDB(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, dbgorm.MigrateSettlementActionModels(db))
	store := NewSettlementActionStore(db)
	leaseUntil := time.Now().Add(time.Minute)
	require.NoError(t, db.Update(func(tx database.Tx) error {
		return tx.Save(&models.SettlementAction{ActionID: "leased-action", IntentKey: "leased-action", OrderID: "order-1", ActionKind: "guest_release", State: "claimed", LeaseToken: "lease-a", LeaseExpiresAt: &leaseUntil})
	}))

	err = store.Put(adapters.ActionRecord{ActionID: "leased-action", State: "submitting", LeaseToken: "lease-b"})
	require.ErrorIs(t, err, adapters.ErrActionLeaseLost)

	require.NoError(t, store.Put(adapters.ActionRecord{ActionID: "leased-action", State: "submitting", LeaseToken: "lease-a"}))
	got, err := store.Lookup(context.Background(), "leased-action")
	require.NoError(t, err)
	require.Equal(t, "submitting", got.State)
	require.Equal(t, "lease-a", got.LeaseToken)
}

func TestSettlementActionStore_RejectsImmutableIntentMutation(t *testing.T) {
	db, err := dbstore.NewMemoryDB(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, dbgorm.MigrateSettlementActionModels(db))
	store := NewSettlementActionStore(db)
	leaseUntil := time.Now().Add(time.Minute)
	require.NoError(t, db.Update(func(tx database.Tx) error {
		return tx.Save(&models.SettlementAction{
			ActionID: "intent-action", IntentKey: "intent-action", IntentPayload: "payload-1",
			OrderID: "order-1", ActionKind: "guest_release", ChainID: 1,
			SettlementCoin: "ETH", GrossAmount: "42", State: "claimed",
			LeaseToken: "lease-a", LeaseExpiresAt: &leaseUntil,
		})
	}))

	err = store.Put(adapters.ActionRecord{
		ActionID: "intent-action", IntentKey: "intent-action", IntentPayload: "payload-2",
		LeaseToken: "lease-a", State: "submitting",
	})
	require.ErrorIs(t, err, adapters.ErrActionIntentConflict)
}

func TestSettlementActionStore_RecordStatusConfirmedCopiesObservedLines(t *testing.T) {
	db, err := dbstore.NewMemoryDB(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, dbgorm.MigrateSettlementActionModels(db))

	s := NewSettlementActionStore(db)
	row := models.SettlementAction{
		ActionID:       "act-confirm-lines",
		OrderID:        "order-1",
		ActionKind:     "confirm",
		State:          "submitted",
		TxHash:         "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		SettlementCoin: "USDT",
		GrossAmount:    "292929",
		PlannedLines: models.EncodeSettlementPayoutLines([]models.SettlementPayoutLine{{
			Type:    "seller",
			Amount:  "141414",
			Address: "seller",
			Coin:    "USDT",
		}, {
			Type:    "platform",
			Amount:  "151515",
			Address: "platform",
			Coin:    "USDT",
		}}),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	require.NoError(t, db.Update(func(tx database.Tx) error {
		return tx.Save(&row)
	}))

	require.NoError(t, s.RecordStatus(row, SettlementActionStatusUpdate{
		State:         "confirmed",
		TxHash:        row.TxHash,
		Confirmations: 2,
	}))

	got, err := s.Lookup(context.Background(), row.ActionID)
	require.NoError(t, err)
	require.Equal(t, "confirmed", got.State)
	require.NotNil(t, got.ConfirmedAt)
	require.Len(t, got.ObservedLines, 2)
	require.Equal(t, "141414", got.ObservedLines[0].Amount)
	require.Equal(t, "151515", got.ObservedLines[1].Amount)
}

func TestSettlementActionStore_ClaimRetryUsesCASAndPreservesHashHistory(t *testing.T) {
	db, err := dbstore.NewMemoryDB(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, dbgorm.MigrateSettlementActionModels(db))

	s := NewSettlementActionStore(db)
	row := models.SettlementAction{
		ActionID:        "act-retry-claim",
		OrderID:         "order-1",
		ActionKind:      "confirm",
		ChainID:         56,
		To:              "0x1111111111111111111111111111111111111111",
		Data:            "0xdeadbeef",
		State:           "submitted",
		TxHash:          "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		AttemptTxHashes: "",
		Attempts:        1,
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}
	require.NoError(t, db.Update(func(tx database.Tx) error {
		return tx.Save(&row)
	}))

	history, claimed, err := s.ClaimRetry(row, 2)
	require.NoError(t, err)
	require.True(t, claimed)
	require.Contains(t, history, row.TxHash)

	_, claimedAgain, err := s.ClaimRetry(row, 2)
	require.NoError(t, err)
	require.False(t, claimedAgain)

	newHash := "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	require.NoError(t, s.RecordRetrySubmitted(row, newHash, history, 2))

	got, err := s.Lookup(context.Background(), row.ActionID)
	require.NoError(t, err)
	require.Equal(t, "submitted", got.State)
	require.Equal(t, newHash, got.TxHash)
	require.Equal(t, 2, got.Attempts)
	require.Contains(t, got.AttemptTxHashes, row.TxHash)
	require.Contains(t, got.AttemptTxHashes, newHash)
	require.Empty(t, got.LastError)
}

func TestSettlementActionStore_PreservesSolanaSignatureHistory(t *testing.T) {
	db, err := dbstore.NewMemoryDB(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, dbgorm.MigrateSettlementActionModels(db))

	s := NewSettlementActionStore(db)
	firstSig := "5j7sX3wYqW9p9hJmE2zZqY4q3YkX4tL8pQ9nS6uV1mN2"
	secondSig := "3m9kP7wYqW9p9hJmE2zZqY4q3YkX4tL8pQ9nS6uV1mA4"
	require.NoError(t, s.Put(adapters.ActionRecord{
		ActionID:        "order-solana:confirm",
		OrderID:         "order-solana",
		Action:          "confirm",
		State:           "submitted",
		TxHash:          firstSig,
		AttemptTxHashes: firstSig,
		Attempts:        1,
	}))
	require.NoError(t, s.Put(adapters.ActionRecord{
		ActionID:        "order-solana:confirm",
		OrderID:         "order-solana",
		Action:          "confirm",
		State:           "submitted",
		TxHash:          secondSig,
		AttemptTxHashes: secondSig,
		Attempts:        2,
	}))

	got, err := s.Lookup(context.Background(), "order-solana:confirm")
	require.NoError(t, err)
	require.Equal(t, secondSig, got.TxHash)
	require.Equal(t, 2, got.Attempts)
	require.Contains(t, got.AttemptTxHashes, firstSig)
	require.Contains(t, got.AttemptTxHashes, secondSig)
}

func TestSettlementActionStore_PreservesDurableIntentAcrossRetryUpdates(t *testing.T) {
	db, err := dbstore.NewMemoryDB(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, dbgorm.MigrateSettlementActionModels(db))

	s := NewSettlementActionStore(db)
	require.NoError(t, s.Put(adapters.ActionRecord{
		ActionID: "order-solana:complete",
		OrderID:  "order-solana",
		Action:   "complete",
		State:    "submitting",
		Data:     `{"releaseInfo":"{\"toAddress\":\"seller\"}"}`,
		Attempts: 1,
	}))
	require.NoError(t, s.Put(adapters.ActionRecord{
		ActionID: "order-solana:complete",
		OrderID:  "order-solana",
		Action:   "complete",
		State:    "submitted",
		TxHash:   "5j7sX3wYqW9p9hJmE2zZqY4q3YkX4tL8pQ9nS6uV1mN2",
		Attempts: 2,
	}))

	got, err := s.Lookup(context.Background(), "order-solana:complete")
	require.NoError(t, err)
	require.Equal(t, `{"releaseInfo":"{\"toAddress\":\"seller\"}"}`, got.Data)
	require.Equal(t, 2, got.Attempts)
}
