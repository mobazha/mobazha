//go:build purego_sqlite

package core

import (
	"database/sql"
	"fmt"

	_ "github.com/glebarez/go-sqlite"
	"go.mau.fi/util/dbutil"
)

func openMatrixCryptoDB(dbPath string) (*dbutil.Database, error) {
	dsn := fmt.Sprintf("file:%s?_txlock=immediate&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	return dbutil.NewWithDB(db, "sqlite")
}
