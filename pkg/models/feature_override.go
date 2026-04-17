package models

import "time"

// FeatureOverride stores a per-tenant override for a feature flag.
//
// Composite primary key: (TenantID, FeatureKey). At most one row per
// (tenant, feature) combination.
//
// Semantics:
//   - Absence of a row → tenant has no override; resolver falls back to
//     feature.DefaultValue (see pkg/config.FeatureResolver).
//   - Presence of a row → `Enabled` is the explicit tenant-layer value.
//
// This table intentionally does NOT store per-feature business config
// (accepted coins, timeouts, etc.). It is a pure on/off override store
// that drives the feature-flag resolver. Business config lives in
// dedicated per-feature tables (e.g. GuestCheckoutConfig).
type FeatureOverride struct {
	TenantMixin
	FeatureKey string    `gorm:"primaryKey;column:feature_key;size:128" json:"featureKey"`
	Enabled    bool      `gorm:"column:enabled" json:"enabled"`
	UpdatedAt  time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updatedAt"`
	UpdatedBy  string    `gorm:"column:updated_by;size:128" json:"updatedBy"`
}

func (FeatureOverride) TableName() string { return "feature_overrides" }
