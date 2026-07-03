package api

import (
	"path/filepath"

	"github.com/mobazha/mobazha/pkg/database/sqlitedialect"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// openTokenDB opens (or creates) a SQLite database for API token storage
// in the given data directory. The database file is separate from the
// node's main database to keep token lifecycle independent.
func openTokenDB(dataDir string) (*gorm.DB, error) {
	dbPath := filepath.Join(dataDir, "tokens.db")
	return gorm.Open(sqlitedialect.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
}
