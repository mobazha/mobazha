package models

import (
	"time"
)

// NotificationRecord encapsulates one of many notifications with additional
// metadata. The actual notification is serialized as JSON so as to
// make this model suitable for the database.
type NotificationRecord struct {
	ID           string    `gorm:"primaryKey" json:"-"`
	Timestamp    time.Time `json:"timestamp"`
	Read         bool      `json:"read"`
	Type         string    `json:"type"`
	Notification []byte    `json:"notification"`
}
