package models

// PaymentAddressCounter tracks the next HD derivation index per chain.
// Each chain type has its own monotonically increasing counter to avoid
// address reuse across Guest Orders.
type PaymentAddressCounter struct {
	TenantMixin
	ID        int    `gorm:"primaryKey;autoIncrement:false" json:"id"`
	ChainKey  string `gorm:"uniqueIndex;size:32" json:"chainKey"`
	NextIndex uint32 `json:"nextIndex"`
}

// TableName overrides the default GORM table name.
func (PaymentAddressCounter) TableName() string { return "payment_address_counters" }
