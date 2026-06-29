package order

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/mobazha/mobazha3.0/internal/collectiblesdelivery"
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
)

const (
	outboxRetentionPeriod = 24 * time.Hour
	outboxBatchSize       = 100
)

// WriteOutboxEvent writes a business event to the outbox table within the
// caller's database transaction. The event is atomically committed with the
// business state change, guaranteeing at-least-once delivery by the poller.
func WriteOutboxEvent(tx database.Tx, evt interface{}) error {
	meta := events.LookupEvent(evt)
	if meta == nil {
		return fmt.Errorf("outbox: unregistered event type %T", evt)
	}
	payload, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("outbox: marshal %s: %w", meta.Name, err)
	}
	if err := tx.Save(&models.OutboxEvent{
		EventName: meta.Name,
		Payload:   payload,
		CreatedAt: time.Now(),
	}); err != nil {
		return err
	}
	return collectiblesdelivery.EnqueueTerminalEventByLookupTx(tx, evt)
}

// RunOutboxPollOnce delivers pending outbox events in a single pass.
// Called by the shared scheduler's NodeFn.
func (s *OrderAppService) RunOutboxPollOnce() {
	s.deliverOutboxEvents()
}

// RunOutboxCleanupOnce removes old delivered outbox events in a single pass.
func (s *OrderAppService) RunOutboxCleanupOnce() {
	s.cleanupDeliveredOutboxEvents()
}

func (s *OrderAppService) deliverOutboxEvents() {
	var pending []models.OutboxEvent
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().
			Where("delivered_at IS NULL").
			Order("id ASC").
			Limit(outboxBatchSize).
			Find(&pending).Error
	})
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "Outbox: query pending: %v", err)
		return
	}

	for i := range pending {
		s.deliverSingleOutboxEvent(&pending[i])
	}
}

func (s *OrderAppService) deliverSingleOutboxEvent(record *models.OutboxEvent) {
	evt := deserializeOutboxEvent(record.EventName, record.Payload)
	if evt != nil {
		s.eventBus.Emit(evt)
	} else {
		logger.LogErrorWithIDf(log, s.nodeID, "Outbox: cannot deserialize event %s (id=%d), skipping", record.EventName, record.ID)
	}

	err := s.db.Update(func(tx database.Tx) error {
		now := time.Now()
		return tx.Update("delivered_at", now, map[string]interface{}{
			"id = ?": record.ID,
		}, &models.OutboxEvent{})
	})
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "Outbox: mark delivered (id=%d): %v", record.ID, err)
	}
}

func (s *OrderAppService) cleanupDeliveredOutboxEvents() {
	cutoff := time.Now().Add(-outboxRetentionPeriod)
	err := s.db.Update(func(tx database.Tx) error {
		return tx.Delete("delivered_at < ?", cutoff,
			map[string]interface{}{}, &models.OutboxEvent{})
	})
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "Outbox: cleanup: %v", err)
	}
}

// deserializeOutboxEvent reconstructs a typed event from its name and JSON payload.
func deserializeOutboxEvent(name string, payload []byte) interface{} {
	meta := events.LookupByName(name)
	if meta == nil {
		return nil
	}
	t := reflect.TypeOf(meta.Sample)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	newEvt := reflect.New(t).Interface()
	if err := json.Unmarshal(payload, newEvt); err != nil {
		return nil
	}
	return newEvt
}
