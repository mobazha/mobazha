package payment

import (
	"context"
	"testing"
	"time"

	"github.com/mobazha/mobazha/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryActionStore_LookupReturnsDefensiveCopy(t *testing.T) {
	store := NewMemoryActionStore()
	record := ActionRecord{
		ActionID:      "action-1",
		PlannedLines:  []models.SettlementPayoutLine{{Address: "seller"}},
		ObservedLines: []models.SettlementPayoutLine{{Address: "buyer"}},
	}
	require.NoError(t, store.Put(record))

	first, err := store.Lookup(context.Background(), record.ActionID)
	require.NoError(t, err)
	first.PlannedLines[0].Address = "mutated"
	first.ObservedLines[0].Address = "mutated"

	second, err := store.Lookup(context.Background(), record.ActionID)
	require.NoError(t, err)
	assert.Equal(t, "seller", second.PlannedLines[0].Address)
	assert.Equal(t, "buyer", second.ObservedLines[0].Address)
}

func TestMemoryActionStore_PutPreservesIncrementalFields(t *testing.T) {
	store := NewMemoryActionStore()
	confirmedAt := time.Now().UTC().Add(-time.Minute)
	require.NoError(t, store.Put(ActionRecord{
		ActionID:       "action-1",
		SettlementCoin: "ETH",
		GrossAmount:    "42",
		PlannedLines:   []models.SettlementPayoutLine{{Address: "seller"}},
		ConfirmedAt:    &confirmedAt,
	}))
	require.NoError(t, store.Put(ActionRecord{ActionID: "action-1", State: "confirmed"}))

	record, err := store.Lookup(context.Background(), "action-1")
	require.NoError(t, err)
	assert.Equal(t, "ETH", record.SettlementCoin)
	assert.Equal(t, "42", record.GrossAmount)
	assert.Len(t, record.PlannedLines, 1)
	assert.Equal(t, confirmedAt, *record.ConfirmedAt)
	assert.Equal(t, "confirmed", record.State)
}

func TestMemoryActionStore_RejectsEmptyIDAndHonorsContext(t *testing.T) {
	store := NewMemoryActionStore()
	require.Error(t, store.Put(ActionRecord{}))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := store.Lookup(ctx, "action-1")
	require.ErrorIs(t, err, context.Canceled)
}

func TestMemoryActionStore_RejectsImmutableIntentMutation(t *testing.T) {
	store := NewMemoryActionStore()
	require.NoError(t, store.Put(ActionRecord{
		ActionID: "intent-1", IntentKey: "intent-1", IntentPayload: "payload-1",
		OrderID: "order-1", Action: ManagedEscrowGuestSettlementAction, ChainID: 1,
		SettlementCoin: "ETH", GrossAmount: "42", LeaseToken: "lease-1",
	}))
	err := store.Put(ActionRecord{
		ActionID: "intent-1", IntentKey: "intent-1", IntentPayload: "payload-2",
		LeaseToken: "lease-1", State: "submitting",
	})
	require.ErrorIs(t, err, ErrActionIntentConflict)
}
