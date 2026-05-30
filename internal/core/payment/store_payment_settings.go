package payment

import (
	"errors"

	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"gorm.io/gorm"
)

// ResolveUtxoConfirmationPolicy returns the store-level UTXO confirmation policy.
func ResolveUtxoConfirmationPolicy(db database.Database) string {
	if db == nil {
		return models.PaymentConfirmationPolicyChainConfirmed
	}
	cfg, err := loadStorePaymentSettings(db)
	if err != nil {
		return models.PaymentConfirmationPolicyChainConfirmed
	}
	return models.NormalizePaymentConfirmationPolicy(cfg.UtxoConfirmationPolicy)
}

// GetStorePaymentSettings returns the persisted store payment settings singleton.
func GetStorePaymentSettings(db database.Database) (models.StorePaymentSettings, error) {
	if db == nil {
		return defaultStorePaymentSettings(), nil
	}
	return loadStorePaymentSettings(db)
}

// SaveStorePaymentSettings persists the store payment settings singleton.
func SaveStorePaymentSettings(db database.Database, policy string) (models.StorePaymentSettings, error) {
	cfg := models.StorePaymentSettings{
		ID:                     models.StorePaymentSettingsSingletonID,
		UtxoConfirmationPolicy: models.NormalizePaymentConfirmationPolicy(policy),
	}
	if db == nil {
		return cfg, nil
	}
	err := db.Update(func(tx database.Tx) error {
		return tx.Save(&cfg)
	})
	return cfg, err
}

func defaultStorePaymentSettings() models.StorePaymentSettings {
	return models.StorePaymentSettings{
		ID:                     models.StorePaymentSettingsSingletonID,
		UtxoConfirmationPolicy: models.PaymentConfirmationPolicyChainConfirmed,
	}
}

func loadStorePaymentSettings(db database.Database) (models.StorePaymentSettings, error) {
	var cfg models.StorePaymentSettings
	err := db.View(func(tx database.Tx) error {
		return tx.Read().First(&cfg).Error
	})
	if err == nil {
		cfg.UtxoConfirmationPolicy = models.NormalizePaymentConfirmationPolicy(cfg.UtxoConfirmationPolicy)
		return cfg, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return defaultStorePaymentSettings(), nil
	}
	return cfg, err
}
