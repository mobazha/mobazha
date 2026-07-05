package models

import "time"

const (
	PaymentAttemptKindProviderSession       = "provider_session"
	PaymentAttemptKindDirectObservedAddress = "direct_observed_address"

	PaymentAttemptPendingExternal   = "pending_external"
	PaymentAttemptExternalCreated   = "external_created"
	PaymentAttemptLinked            = "linked"
	PaymentAttemptReconcileRequired = "reconcile_required"
	PaymentAttemptExpired           = "expired"
	PaymentAttemptAbandoning        = "abandoning"
	PaymentAttemptAbandoned         = "abandoned"
)

// PaymentAttempt is Core's durable claim for one concrete payment provisioning
// operation. It must exist before any external provider or chain create call.
type PaymentAttempt struct {
	TenantID          string     `gorm:"column:tenant_id;primaryKey;default:'';uniqueIndex:idx_payment_attempt_idempotency,priority:1"`
	AttemptID         string     `gorm:"column:attempt_id;primaryKey;size:64"`
	Kind              string     `gorm:"column:kind;size:64;not null;index:idx_payment_attempt_kind_state,priority:1"`
	PaymentSessionID  string     `gorm:"column:payment_session_id;size:255;not null;index:idx_payment_attempt_session"`
	OrderID           string     `gorm:"column:order_id;size:255;not null;index:idx_payment_attempt_order"`
	ProviderID        string     `gorm:"column:provider_id;size:64;not null;default:''"`
	Amount            int64      `gorm:"column:amount;not null;default:0"`
	AmountValue       string     `gorm:"column:amount_value;type:text;not null;default:''"`
	Currency          string     `gorm:"column:currency;size:16;not null;default:''"`
	RouteBindingID    string     `gorm:"column:route_binding_id;size:64;not null"`
	IdempotencyKey    string     `gorm:"column:idempotency_key;size:128;not null;uniqueIndex:idx_payment_attempt_idempotency,priority:2"`
	State             string     `gorm:"column:state;size:32;not null;index:idx_payment_attempt_state;index:idx_payment_attempt_kind_state,priority:2"`
	ExternalReference string     `gorm:"column:external_reference;size:255"`
	ExternalIndex     uint32     `gorm:"column:external_index;not null;default:0"`
	RequiredConfs     int        `gorm:"column:required_confirmations;not null;default:0"`
	ExpiresAt         *time.Time `gorm:"column:expires_at;index"`
	LastError         string     `gorm:"column:last_error;size:2048"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (PaymentAttempt) TableName() string { return "payment_attempts" }

// PaymentRouteBinding is immutable routing identity for an accepted attempt.
// A provider/account/configuration switch creates another attempt and binding.
type PaymentRouteBinding struct {
	TenantID                 string `gorm:"column:tenant_id;primaryKey;default:'';uniqueIndex:idx_payment_route_attempt,priority:1"`
	RouteBindingID           string `gorm:"column:route_binding_id;primaryKey;size:64"`
	AttemptID                string `gorm:"column:attempt_id;size:64;not null;uniqueIndex:idx_payment_route_attempt,priority:2"`
	ContributionID           string `gorm:"column:contribution_id;size:128;not null;index:idx_payment_route_contribution"`
	ModuleID                 string `gorm:"column:module_id;size:128;not null"`
	ImplementationGeneration string `gorm:"column:implementation_generation;size:64;not null"`
	RailKind                 string `gorm:"column:rail_kind;size:32;not null"`
	NetworkID                string `gorm:"column:network_id;size:128;not null"`
	AssetID                  string `gorm:"column:asset_id;size:255;not null"`
	ProtocolVersion          string `gorm:"column:protocol_version;size:64;not null"`
	StateSchemaVersion       string `gorm:"column:state_schema_version;size:64;not null"`
	ProviderBindingID        string `gorm:"column:provider_binding_id;size:128"`
	ExternalAccountReference string `gorm:"column:external_account_reference;size:255"`
	CreatedAt                time.Time
}

func (PaymentRouteBinding) TableName() string { return "payment_route_bindings" }
