package models

import "time"

const (
	PaymentProviderBindingActive  = "active"
	PaymentProviderBindingRetired = "retired"
)

// PaymentProviderBinding is an immutable, non-secret snapshot of the provider
// account and credential generation selected for new payment attempts. Secret
// material remains in the provider configuration/secret store and is addressed
// only through CredentialReference.
type PaymentProviderBinding struct {
	TenantID                 string `gorm:"column:tenant_id;primaryKey;default:'';uniqueIndex:idx_provider_binding_generation,priority:1;index:idx_provider_binding_fingerprint,priority:1"`
	BindingID                string `gorm:"column:binding_id;primaryKey;size:64"`
	ProviderID               string `gorm:"column:provider_id;size:64;not null;uniqueIndex:idx_provider_binding_generation,priority:2;index:idx_provider_binding_fingerprint,priority:2;index:idx_provider_binding_state"`
	DriverContributionID     string `gorm:"column:driver_contribution_id;size:128;not null"`
	ExternalAccountReference string `gorm:"column:external_account_reference;size:255;not null"`
	CredentialReference      string `gorm:"column:credential_reference;size:255;not null"`
	ConfigurationGeneration  uint64 `gorm:"column:configuration_generation;not null;uniqueIndex:idx_provider_binding_generation,priority:3"`
	ConfigurationFingerprint string `gorm:"column:configuration_fingerprint;size:64;not null;index:idx_provider_binding_fingerprint,priority:3"`
	Mode                     string `gorm:"column:mode;size:32;not null"`
	State                    string `gorm:"column:state;size:32;not null;index:idx_provider_binding_state"`
	CreatedAt                time.Time
	RetiredAt                *time.Time
}

func (PaymentProviderBinding) TableName() string { return "payment_provider_bindings" }
