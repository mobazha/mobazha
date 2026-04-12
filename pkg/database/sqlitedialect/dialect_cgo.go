//go:build !purego_sqlite

package sqlitedialect

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Open returns a GORM dialector for SQLite.
// This build uses mattn/go-sqlite3 (CGO) for maximum compatibility and performance.
func Open(dsn string) gorm.Dialector {
	return sqlite.Open(dsn)
}
