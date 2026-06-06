package database

import (
	pkgdb "github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
)

// MigrateSettlementActionModels creates/updates backend-submitted settlement
// action projection tables.
func MigrateSettlementActionModels(db pkgdb.Database) error {
	return db.Update(func(tx pkgdb.Tx) error {
		return tx.Migrate(&models.SettlementAction{})
	})
}
