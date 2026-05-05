//go:build !private_distribution

package order

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestOutboxService(t *testing.T) (*OrderAppService, *testDatabase, *mockEventBus) {
	t.Helper()
	db := newTestDatabase(t)
	require.NoError(t, db.gormDB.AutoMigrate(&models.OutboxEvent{}))
	bus := &mockEventBus{}
	shutdown := make(chan struct{})
	svc := &OrderAppService{
		db:       db,
		eventBus: bus,
		nodeID:   "test-node",
		shutdown: shutdown,
	}
	return svc, db, bus
}

func TestOutbox_WriteAndDeliver(t *testing.T) {
	svc, db, bus := newTestOutboxService(t)

	err := db.Update(func(tx database.Tx) error {
		return WriteOutboxEvent(tx, &events.OrderExpired{
			OrderID: "order-123",
			Reason:  "payment_timeout",
		})
	})
	require.NoError(t, err)

	var count int64
	db.gormDB.Model(&models.OutboxEvent{}).Where("delivered_at IS NULL").Count(&count)
	assert.Equal(t, int64(1), count, "one pending event should exist")

	svc.deliverOutboxEvents()

	require.Len(t, bus.emitted, 1)
	evt, ok := bus.emitted[0].(*events.OrderExpired)
	require.True(t, ok)
	assert.Equal(t, "order-123", evt.OrderID)
	assert.Equal(t, "payment_timeout", evt.Reason)

	db.gormDB.Model(&models.OutboxEvent{}).Where("delivered_at IS NULL").Count(&count)
	assert.Equal(t, int64(0), count, "event should be marked delivered")
}

func TestOutbox_AlreadyDeliveredIsNotReemitted(t *testing.T) {
	svc, db, bus := newTestOutboxService(t)

	err := db.Update(func(tx database.Tx) error {
		return WriteOutboxEvent(tx, &events.OrderExpired{
			OrderID: "order-once",
			Reason:  "test",
		})
	})
	require.NoError(t, err)

	svc.deliverOutboxEvents()
	require.Len(t, bus.emitted, 1)

	bus.emitted = nil
	svc.deliverOutboxEvents()
	assert.Empty(t, bus.emitted, "delivered events must not be re-emitted")
}

func TestOutbox_RestartRecovery(t *testing.T) {
	_, db, _ := newTestOutboxService(t)

	err := db.Update(func(tx database.Tx) error {
		return WriteOutboxEvent(tx, &events.OrderStaleWarning{
			OrderID:  "order-crash",
			State:    "PENDING",
			StuckFor: "10d",
		})
	})
	require.NoError(t, err)

	newBus := &mockEventBus{}
	svc2 := &OrderAppService{
		db:       db,
		eventBus: newBus,
		nodeID:   "test-node-restarted",
		shutdown: make(chan struct{}),
	}

	svc2.deliverOutboxEvents()

	require.Len(t, newBus.emitted, 1)
	evt, ok := newBus.emitted[0].(*events.OrderStaleWarning)
	require.True(t, ok)
	assert.Equal(t, "order-crash", evt.OrderID)
	assert.Equal(t, "PENDING", evt.State)
	assert.Equal(t, "10d", evt.StuckFor)
}

func TestOutbox_Cleanup(t *testing.T) {
	svc, db, _ := newTestOutboxService(t)

	now := time.Now()
	old := now.Add(-48 * time.Hour)
	recent := now.Add(-1 * time.Hour)

	db.gormDB.Create(&models.OutboxEvent{
		EventName:   "order.expired",
		Payload:     []byte(`{}`),
		CreatedAt:   old,
		DeliveredAt: &old,
	})
	db.gormDB.Create(&models.OutboxEvent{
		EventName:   "order.expired",
		Payload:     []byte(`{}`),
		CreatedAt:   recent,
		DeliveredAt: &recent,
	})

	svc.cleanupDeliveredOutboxEvents()

	var count int64
	db.gormDB.Model(&models.OutboxEvent{}).Count(&count)
	assert.Equal(t, int64(1), count, "only the recent delivered event should remain")
}

func TestOutbox_UnregisteredEventReturnsError(t *testing.T) {
	_, db, _ := newTestOutboxService(t)

	type unknownEvent struct{ Foo string }
	err := db.Update(func(tx database.Tx) error {
		return WriteOutboxEvent(tx, &unknownEvent{Foo: "bar"})
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unregistered event type")
}

func TestOutbox_DeserializeRoundTrip(t *testing.T) {
	original := &events.OrderStaleWarning{
		OrderID:  "rt-order",
		State:    "DISPUTED",
		StuckFor: "7d",
	}

	meta := events.LookupEvent(original)
	require.NotNil(t, meta)

	payload, err := json.Marshal(original)
	require.NoError(t, err)

	restored := deserializeOutboxEvent(meta.Name, payload)
	require.NotNil(t, restored)

	restoredEvt, ok := restored.(*events.OrderStaleWarning)
	require.True(t, ok)
	assert.Equal(t, "rt-order", restoredEvt.OrderID)
	assert.Equal(t, "DISPUTED", restoredEvt.State)
	assert.Equal(t, "7d", restoredEvt.StuckFor)
}
