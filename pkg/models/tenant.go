package models

// TenantMixin provides a TenantID field for multi-tenant data isolation.
// Embed this struct in GORM models that need tenant scoping.
//
// TenantID is part of the composite primary key so that the same logical
// entity (e.g. an Order) can exist independently for each tenant (buyer
// and seller both store their own copy of the same order).
//
// In standalone mode, TenantID is the sentinel value "_default".
// In SaaS/hosting mode, TenantID is the user/node ID for row-level isolation.
type TenantMixin struct {
	TenantID string `gorm:"column:tenant_id;primaryKey;default:''" json:"-"`
}
