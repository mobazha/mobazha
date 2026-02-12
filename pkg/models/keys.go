package models

// Key holds raw key data used by the node and stored in the
// database. In multi-tenant shared DB mode, the composite primary key
// (TenantID, Name) ensures each tenant can store keys with the same name
// (e.g. "identity", "mnemonic") without conflict.
type Key struct {
	TenantID string `gorm:"column:tenant_id;primaryKey;default:''" json:"-"`
	Name     string `gorm:"primaryKey"`
	Value    []byte
}
