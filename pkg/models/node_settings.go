package models

import "time"

// NodeSettings stores per-node key-value configuration in the database.
// TenantID is auto-injected by TenantDB.Save() in SaaS mode, or set to
// StandaloneTenantID ("_default") in standalone mode.
// Uses composite primary key (tenant_id, key) for natural upsert behavior.
type NodeSettings struct {
	TenantID  string    `gorm:"column:tenant_id;type:varchar(255);not null;default:'';primaryKey"`
	Key       string    `gorm:"column:key;type:varchar(128);not null;primaryKey"`
	Value     string    `gorm:"column:value;type:text;not null"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

func (NodeSettings) TableName() string { return "node_settings" }

const (
	SettingsKeyNotificationChannels = "notification_channels"
	SettingsKeyAIConfig             = "ai_config"
	SettingsKeyStoreConfig          = "store_config"
)
