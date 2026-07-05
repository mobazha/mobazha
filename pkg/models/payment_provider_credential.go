package models

import "time"

// PaymentProviderCredential is an append-only encrypted credential version.
// PaymentProviderBinding refers to it by CredentialReference; no secret
// material is copied into routing or payment-attempt records.
type PaymentProviderCredential struct {
	TenantID                 string `gorm:"column:tenant_id;primaryKey;default:'';uniqueIndex:idx_provider_credential_generation,priority:1"`
	CredentialReference      string `gorm:"column:credential_reference;primaryKey;size:255"`
	ProviderID               string `gorm:"column:provider_id;size:64;not null;uniqueIndex:idx_provider_credential_generation,priority:2"`
	ExternalAccountReference string `gorm:"column:external_account_reference;size:255;not null"`
	ConfigurationGeneration  uint64 `gorm:"column:configuration_generation;not null;uniqueIndex:idx_provider_credential_generation,priority:3"`
	ConfigurationFingerprint string `gorm:"column:configuration_fingerprint;size:64;not null"`
	EncryptionKeyVersion     uint64 `gorm:"column:encryption_key_version;not null"`
	Ciphertext               []byte `gorm:"column:ciphertext;not null"`
	CreatedAt                time.Time
}

func (PaymentProviderCredential) TableName() string { return "payment_provider_credentials" }
