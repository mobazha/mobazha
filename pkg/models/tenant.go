package models

// TenantMixin provides a TenantID field for multi-tenant data isolation.
// Embed this struct in GORM models that need tenant scoping.
//
// In standalone mode (single node), TenantID is empty string.
// In SaaS/hosting mode, TenantID is the user/node ID for row-level isolation.
type TenantMixin struct {
	TenantID string `gorm:"column:tenant_id;index;default:''" json:"-"`
}
