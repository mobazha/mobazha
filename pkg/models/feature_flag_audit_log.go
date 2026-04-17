package models

import "time"

// FeatureFlagAuditLog records every write to a feature flag override, across
// scopes (platform_global / tenant / node_runtime).
//
// Semantics:
//   - Rows are append-only; no updates, no deletes (operational records).
//   - OldValue is *bool: nil means the override had never been configured
//     before (row did not exist), distinguishing "first write" from
//     "toggle from false → true".
//   - Scope captures which layer the write targeted; TenantID is only set
//     when Scope == "tenant" (for platform_global and node_runtime writes
//     it is empty).
//
// The table is intentionally NOT TenantMixin'd:
//
//   - In hosting (platform-global writes) the table is a cross-tenant ops
//     log that must remain writable without any tenant scope.
//   - In mobazha3.0 (tenant-scoped writes) the TenantID column still carries
//     the tenant ID as a plain field, written explicitly by the caller; the
//     multi-tenant DB partitioning handled elsewhere (per-tenant database)
//     already isolates these rows. Adding TenantMixin here would collide
//     with hosting usage and make the shared schema harder to reason about.
//
// See docs/FEATURE_FLAG_ARCHITECTURE.md §5.3.
type FeatureFlagAuditLog struct {
	ID         int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Scope      string    `gorm:"column:scope;size:32;not null;index" json:"scope"`
	TenantID   string    `gorm:"column:tenant_id;size:128;index" json:"tenantId,omitempty"`
	FeatureKey string    `gorm:"column:feature_key;size:128;not null;index" json:"featureKey"`
	OldValue   *bool     `gorm:"column:old_value" json:"oldValue,omitempty"`
	NewValue   bool      `gorm:"column:new_value;not null" json:"newValue"`
	Actor      string    `gorm:"column:actor;size:128;not null" json:"actor"`
	Reason     string    `gorm:"column:reason;size:512" json:"reason,omitempty"`
	IPAddress  string    `gorm:"column:ip_address;size:64" json:"ipAddress,omitempty"`
	UserAgent  string    `gorm:"column:user_agent;size:256" json:"userAgent,omitempty"`
	CreatedAt  time.Time `gorm:"column:created_at;not null;index" json:"createdAt"`
}

// TableName pins the SQL table name so future model renames do not shift it.
func (FeatureFlagAuditLog) TableName() string { return "feature_flag_audit_logs" }
