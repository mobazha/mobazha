package models

import "time"

// OutboxEvent stores business events atomically with the transaction
// that produced them. A background poller delivers pending events to
// the in-memory EventBus and marks them as delivered.
type OutboxEvent struct {
	ID          uint64     `gorm:"primaryKey;autoIncrement"`
	TenantID    string     `gorm:"index:idx_outbox_pending,priority:1"`
	EventName   string     `gorm:"size:128;not null"`
	Payload     []byte     `gorm:"type:blob;not null"`
	CreatedAt   time.Time  `gorm:"not null"`
	DeliveredAt *time.Time `gorm:"index:idx_outbox_pending,priority:2"`
}
