package models

import "time"

const (
	CollectibleLifecyclePaid    = "primary_sale_paid"
	CollectibleLifecycleRelease = "reservation_release"
)

// CollectibleLifecycleDelivery is a durable, idempotent work item bridging a
// Node order transition into Hosting's collectible ledger. It is written in
// the same transaction as the order transition and retried until acknowledged.
type CollectibleLifecycleDelivery struct {
	TenantMixin
	JobID         string     `gorm:"primaryKey;type:varchar(192)"`
	OrderID       string     `gorm:"type:varchar(128);not null;index"`
	Kind          string     `gorm:"type:varchar(32);not null;index"`
	Reason        string     `gorm:"type:text"`
	Attempts      int        `gorm:"not null;default:0"`
	NextAttemptAt *time.Time `gorm:"index"`
	LastError     string     `gorm:"type:text"`
	DeliveredAt   *time.Time `gorm:"index"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
