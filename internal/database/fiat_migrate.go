package database

import (
	pkgdb "github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
)

// MigrateFiatModels creates/updates fiat payment tables. Call during repo init.
func MigrateFiatModels(db pkgdb.Database) error {
	return db.Update(func(tx pkgdb.Tx) error {
		if err := tx.Migrate(&models.FiatProviderConfig{}); err != nil {
			return err
		}
		if err := tx.Migrate(&models.PaymentProviderCredential{}); err != nil {
			return err
		}
		// Pre-launch schema reset: legacy columns were documented as encrypted
		// but stored plaintext. They are no longer authoritative and must not
		// retain secret material after the versioned credential store is enabled.
		for _, column := range []string{"secret_key", "webhook_secret"} {
			if tx.Read().Migrator().HasColumn(&models.FiatProviderConfig{}, column) {
				if err := tx.Read().Model(&models.FiatProviderConfig{}).Update(column, "").Error; err != nil {
					return err
				}
			}
		}
		// No compatibility is retained before launch. A legacy row has no
		// immutable credential reference and cannot be safely used as a partial
		// update source after its plaintext columns are scrubbed. Remove it so a
		// complete configuration request creates a fresh encrypted generation.
		if err := tx.Delete("credential_reference", "", nil, &models.FiatProviderConfig{}); err != nil {
			return err
		}
		if err := tx.Migrate(&models.ProcessedFiatEvent{}); err != nil {
			return err
		}
		if err := tx.Migrate(&models.PaymentProviderBinding{}); err != nil {
			return err
		}
		if err := tx.Migrate(&models.PaymentRouteBinding{}); err != nil {
			return err
		}
		return tx.Migrate(&models.PaymentAttempt{})
	})
}
