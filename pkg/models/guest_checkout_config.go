package models

// GuestCheckoutConfig stores per-tenant guest checkout settings.
// Singleton per tenant (ID is always 1).
type GuestCheckoutConfig struct {
	TenantMixin
	ID             int    `json:"-" gorm:"primaryKey;autoIncrement:false"`
	Enabled        bool   `json:"enabled"`
	AcceptedCoins  string `json:"acceptedCoins"`  // comma-separated coin codes, e.g. "BTC,ETH,SOL"
	MaxOrderAmount string `json:"maxOrderAmount"` // smallest-unit string; "0" = unlimited
	PaymentTimeout int    `json:"paymentTimeout"` // minutes; 0 = use default (60)
}

func (GuestCheckoutConfig) TableName() string {
	return "guest_checkout_configs"
}
