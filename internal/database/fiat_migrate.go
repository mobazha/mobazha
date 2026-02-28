package database

import (
	pkgdb "github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
)

// MigrateFiatModels creates/updates fiat payment tables. Call during repo init.
func MigrateFiatModels(db pkgdb.Database) error {
	return db.Update(func(tx pkgdb.Tx) error {
		if err := tx.Migrate(&models.FiatProviderConfig{}); err != nil {
			return err
		}
		return tx.Migrate(&models.ProcessedFiatEvent{})
	})
}
