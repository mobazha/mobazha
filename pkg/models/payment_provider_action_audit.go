package models

import "time"

const PaymentProviderActionAuditManualRetryRequested = "manual_retry_requested"

// PaymentProviderActionAudit is an append-only tenant-scoped record of an
// operator intervention. Automatic claim/outcome history remains in metrics
// and the action row; human/API-token interventions require durable identity.
type PaymentProviderActionAudit struct {
	TenantID   string    `gorm:"column:tenant_id;primaryKey;default:'';index:idx_provider_action_audit_action,priority:1"`
	AuditID    string    `gorm:"column:audit_id;primaryKey;size:64"`
	ActionID   string    `gorm:"column:action_id;size:64;not null;index:idx_provider_action_audit_action,priority:2"`
	Event      string    `gorm:"column:event;size:64;not null"`
	Actor      string    `gorm:"column:actor;size:255;not null"`
	ActionKind string    `gorm:"column:action_kind;size:32;not null"`
	ProviderID string    `gorm:"column:provider_id;size:64;not null"`
	State      string    `gorm:"column:state;size:32;not null"`
	Attempts   int       `gorm:"column:attempts;not null"`
	CreatedAt  time.Time `gorm:"column:created_at;not null;index"`
}

func (PaymentProviderActionAudit) TableName() string { return "payment_provider_action_audits" }
