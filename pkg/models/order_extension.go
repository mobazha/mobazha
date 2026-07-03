package models

import "time"

// OrderExtensionRecord stores one immutable revision of an order extension.
type OrderExtensionRecord struct {
	TenantMixin
	ExtensionID         string `gorm:"primaryKey;type:varchar(96)"`
	OrderID             string `gorm:"type:varchar(128);not null;index:idx_order_extension,priority:1"`
	ProviderID          string `gorm:"type:varchar(160);not null;index:idx_order_extension,priority:2"`
	ExtensionType       string `gorm:"type:varchar(160);not null;index:idx_order_extension,priority:3"`
	SchemaVersion       string `gorm:"type:varchar(32);not null"`
	Revision            uint64 `gorm:"primaryKey;not null"`
	ResourceID          string `gorm:"type:varchar(256);index"`
	ReservationRequired bool   `gorm:"not null;default:false"`
	SettlementPolicy    string `gorm:"type:varchar(32);not null;default:''"`
	Payload             []byte `gorm:"not null"`
	PayloadHash         string `gorm:"type:varchar(96);not null"`
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// OrderExtensionEventSequence owns the monotonic event version for one order
// extension aggregate. Core advances this row transactionally before it
// inserts a delivery, so concurrent workers cannot issue the same version.
type OrderExtensionEventSequence struct {
	TenantID    string `gorm:"column:tenant_id;primaryKey;default:''" json:"-"`
	ExtensionID string `gorm:"primaryKey;type:varchar(96)"`
	OrderID     string `gorm:"type:varchar(128);not null;index"`
	LastVersion uint64 `gorm:"not null;default:0"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// OrderExtensionReservationRecord durably binds the provider reservation made
// before payment provisioning to the Core order aggregate.
type OrderExtensionReservationRecord struct {
	TenantMixin
	OrderID            string `gorm:"primaryKey;type:varchar(128)"`
	ExtensionID        string `gorm:"primaryKey;type:varchar(96)"`
	ProviderID         string `gorm:"type:varchar(160);not null;index"`
	ReservationID      string `gorm:"type:varchar(192);not null;index"`
	ReservationVersion uint64 `gorm:"not null"`
	ExtensionRevision  uint64 `gorm:"not null"`
	Status             string `gorm:"type:varchar(64);not null"`
	PaymentCoin        string `gorm:"type:varchar(160);not null"`
	IdempotencyKey     string `gorm:"type:varchar(192);not null;index"`
	ExpiresAt          time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// ExtensionDelivery stores one durable Controller delivery attempt stream.
type ExtensionDelivery struct {
	TenantID       string `gorm:"column:tenant_id;primaryKey;default:'';uniqueIndex:uidx_extension_delivery_order_version,priority:1" json:"-"`
	EventID        string `gorm:"primaryKey;type:varchar(192)"`
	SourceID       string `gorm:"type:varchar(192);not null;index"`
	OrderRole      string `gorm:"type:varchar(32);not null"`
	ProviderID     string `gorm:"type:varchar(160);not null;index"`
	EventType      string `gorm:"type:varchar(160);not null;index"`
	EventVersion   string `gorm:"type:varchar(32);not null"`
	OrderID        string `gorm:"type:varchar(128);not null;index;uniqueIndex:uidx_extension_delivery_order_version,priority:2"`
	OrderVersion   uint64 `gorm:"not null;uniqueIndex:uidx_extension_delivery_order_version,priority:4"`
	ExtensionID    string `gorm:"type:varchar(96);not null;index;uniqueIndex:uidx_extension_delivery_order_version,priority:3"`
	IdempotencyKey string `gorm:"type:varchar(192);not null"`
	Payload        []byte
	Attempts       int        `gorm:"not null;default:0"`
	NextAttemptAt  *time.Time `gorm:"index"`
	LeaseOwner     string     `gorm:"type:varchar(192);index"`
	LeaseExpiresAt *time.Time `gorm:"index"`
	LastError      string     `gorm:"type:text"`
	DeliveredAt    *time.Time `gorm:"index"`
	DeadLetteredAt *time.Time `gorm:"index"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// SettlementAttestationRecord audits accepted provider evidence idempotently.
type SettlementAttestationRecord struct {
	TenantID                  string `gorm:"column:tenant_id;primaryKey;default:'';uniqueIndex:uidx_settlement_attestation_idempotency,priority:1;uniqueIndex:uidx_settlement_attestation_replay,priority:1" json:"-"`
	AttestationID             string `gorm:"primaryKey;type:varchar(192)"`
	IdempotencyKey            string `gorm:"type:varchar(192);not null;uniqueIndex:uidx_settlement_attestation_idempotency,priority:2"`
	ReplayFingerprint         string `gorm:"type:varchar(96);not null;uniqueIndex:uidx_settlement_attestation_replay,priority:2"`
	Issuer                    string `gorm:"type:varchar(160);not null;index"`
	OrderID                   string `gorm:"type:varchar(128);not null;index"`
	ExtensionID               string `gorm:"type:varchar(96);not null;index"`
	SettlementID              string `gorm:"type:varchar(192)"`
	ExpectedExtensionRevision uint64 `gorm:"not null"`
	ExpectedOrderStateVersion string `gorm:"type:varchar(96);not null"`
	ConditionType             string `gorm:"type:varchar(160);not null"`
	ConditionVersion          string `gorm:"type:varchar(32);not null"`
	EvidenceDigest            string `gorm:"type:varchar(256);not null"`
	ObservedAt                time.Time
	ExpiresAt                 time.Time
	AcceptedAt                time.Time
}
