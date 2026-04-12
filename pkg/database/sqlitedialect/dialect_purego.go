//go:build purego_sqlite

package sqlitedialect

import (
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// Open returns a GORM dialector for SQLite.
// This build uses glebarez/sqlite (modernc.org/sqlite, pure Go) for
// CGO-free cross-platform builds.
func Open(dsn string) gorm.Dialector {
	return sqlite.Open(dsn)
}
