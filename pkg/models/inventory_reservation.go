package models

import "time"

// InventoryReservation locks inventory for an order until payment confirms or expires.
// Supports both Guest Orders and Standard Orders via OrderType discrimination.
type InventoryReservation struct {
	TenantMixin
	ID          int       `gorm:"primaryKey;autoIncrement:false" json:"id"`
	OrderRef    string    `gorm:"index" json:"orderRef"`
	OrderType   string    `gorm:"index" json:"orderType"`
	ListingSlug string    `gorm:"index" json:"listingSlug"`
	VariantHash string    `json:"variantHash"`
	Quantity    int       `json:"quantity"`
	ReservedAt  time.Time `json:"reservedAt"`
	ExpiresAt   time.Time `gorm:"index" json:"expiresAt"`
	Confirmed   bool      `json:"confirmed"`
	ReleasedAt  *time.Time `json:"releasedAt,omitempty"`
}

// TableName overrides the default GORM table name.
func (InventoryReservation) TableName() string { return "inventory_reservations" }

const (
	OrderTypeGuest    = "guest"
	OrderTypeStandard = "standard"
)

// IsActive returns true if the reservation is still holding inventory.
func (r *InventoryReservation) IsActive() bool {
	return r.ReleasedAt == nil && !r.Confirmed
}
