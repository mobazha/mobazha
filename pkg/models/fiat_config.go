package models

import "time"

// FiatProviderConfig stores per-tenant API credentials for a fiat payment provider.
// Used in standalone mode where sellers supply their own API keys.
// In SaaS mode, platform-level keys are in environment variables; only ReceivingAccount is used.
type FiatProviderConfig struct {
	TenantID  string `gorm:"column:tenant_id;primaryKey;default:''"`
	ID        int    `gorm:"primaryKey;autoIncrement"`
	ProviderID string `gorm:"column:provider_id;type:varchar(32);not null;uniqueIndex:idx_fiat_config_tenant_provider"`

	PublicKey     string `gorm:"column:public_key;type:text"`
	SecretKey     string `gorm:"column:secret_key;type:text"`     // AES-256-GCM encrypted
	WebhookSecret string `gorm:"column:webhook_secret;type:text"` // AES-256-GCM encrypted

	IsActive  bool      `gorm:"column:is_active;default:false"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

func (FiatProviderConfig) TableName() string { return "fiat_provider_configs" }

// ProcessedFiatEvent tracks webhook event IDs for idempotency deduplication.
// Records are cleaned up after 7 days via a periodic job.
type ProcessedFiatEvent struct {
	TenantID    string    `gorm:"column:tenant_id;primaryKey;default:''"`
	EventID     string    `gorm:"column:event_id;primaryKey;type:varchar(255)"`
	ProviderID  string    `gorm:"column:provider_id;type:varchar(32);not null"`
	ProcessedAt time.Time `gorm:"autoCreateTime"`
}

func (ProcessedFiatEvent) TableName() string { return "processed_fiat_events" }

// FiatSettings is the per-node fiat payment configuration stored in NodeSettings.
const SettingsKeyFiat = "fiat_settings"

// FiatSettingsValue is serialized as JSON into NodeSettings.Value.
type FiatSettingsValue struct {
	EnabledProviders []string `json:"enabledProviders"` // ["stripe", "paypal"]
	DefaultCurrency  string   `json:"defaultCurrency"`  // ISO 4217 (e.g. "USD")
}
