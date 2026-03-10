package models

import "time"

// Event records named events with a timestamp and optional string value.
type Event struct {
	TenantID string    `gorm:"column:tenant_id;primaryKey;default:''" json:"-"`
	Name     string    `gorm:"primaryKey"`
	Time     time.Time
	Value    string    `gorm:"default:''"`
}
