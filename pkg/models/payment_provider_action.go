package models

import "time"

const (
	PaymentProviderActionCapture = "capture"
	PaymentProviderActionRefund  = "refund"
	PaymentProviderActionCancel  = "cancel"

	PaymentProviderActionPendingExternal   = "pending_external"
	PaymentProviderActionReconcileRequired = "reconcile_required"
	PaymentProviderActionCompleted         = "completed"
)

// PaymentProviderAction is the durable intent and result projection for one
// idempotent external provider command. It is deliberately separate from the
// business-level SettlementAction lifecycle and from chain execution records.
type PaymentProviderAction struct {
	TenantID          string     `gorm:"column:tenant_id;primaryKey;default:'';index:idx_provider_action_state,priority:1;uniqueIndex:idx_provider_action_idempotency,priority:1"`
	ActionID          string     `gorm:"column:action_id;primaryKey;size:64"`
	ActionKind        string     `gorm:"column:action_kind;size:32;not null"`
	ProviderID        string     `gorm:"column:provider_id;size:64;not null"`
	AttemptID         string     `gorm:"column:attempt_id;size:64;not null;index"`
	RouteBindingID    string     `gorm:"column:route_binding_id;size:64;not null"`
	ProviderBindingID string     `gorm:"column:provider_binding_id;size:128;not null"`
	ExternalReference string     `gorm:"column:external_reference;size:255;not null"`
	IdempotencyKey    string     `gorm:"column:idempotency_key;size:128;not null;uniqueIndex:idx_provider_action_idempotency,priority:2"`
	IntentFingerprint string     `gorm:"column:intent_fingerprint;size:64;not null"`
	IntentPayload     []byte     `gorm:"column:intent_payload;not null"`
	ResultPayload     []byte     `gorm:"column:result_payload"`
	State             string     `gorm:"column:state;size:32;not null;index:idx_provider_action_state,priority:2"`
	Attempts          int        `gorm:"column:attempts;not null;default:0"`
	LastError         string     `gorm:"column:last_error;size:2048"`
	NextAttemptAt     *time.Time `gorm:"column:next_attempt_at;index"`
	LeaseOwner        string     `gorm:"column:lease_owner;size:128"`
	LeaseExpiresAt    *time.Time `gorm:"column:lease_expires_at;index:idx_provider_action_lease"`
	LastAttemptAt     *time.Time `gorm:"column:last_attempt_at"`
	CompletedAt       *time.Time `gorm:"column:completed_at"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (PaymentProviderAction) TableName() string { return "payment_provider_actions" }
