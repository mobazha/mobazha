package models

// StorePaymentSettings holds per-store payment behavior that applies to all
// checkout paths (registered buyers, guest checkout, UTXO escrow).
// Singleton per tenant (ID is always 1).
//
// Long-term home for payment policy before it folds into StorePolicy.PaymentPolicy.
type StorePaymentSettings struct {
	TenantMixin
	ID int `json:"-" gorm:"primaryKey;autoIncrement:false"`

	// UtxoConfirmationPolicy controls when address-monitored UTXO orders may
	// advance. Empty defaults to chain_confirmed.
	UtxoConfirmationPolicy string `json:"utxoConfirmationPolicy,omitempty"`
}

func (StorePaymentSettings) TableName() string {
	return "store_payment_settings"
}

const StorePaymentSettingsSingletonID = 1
