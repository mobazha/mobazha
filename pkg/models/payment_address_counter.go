package models

// DirectPaymentAddressCounter tracks the next HD derivation index per chain
// for direct (non-escrow) payments. Each chain type has its own monotonically
// increasing counter to avoid address reuse across Guest Orders.
type DirectPaymentAddressCounter struct {
	TenantID  string `gorm:"column:tenant_id;primaryKey;default:'';uniqueIndex:idx_counter_tenant_chain" json:"-"`
	ID        int    `gorm:"primaryKey;autoIncrement:false" json:"id"`
	ChainKey  string `gorm:"uniqueIndex:idx_counter_tenant_chain;size:32" json:"chainKey"`
	NextIndex uint32 `json:"nextIndex"`
}

// TableName overrides the default GORM table name.
func (DirectPaymentAddressCounter) TableName() string { return "direct_payment_address_counters" }
