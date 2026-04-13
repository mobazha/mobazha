//go:build !purego_sqlite

package core

import (
	"database/sql"
	"fmt"

	"go.mau.fi/util/dbutil"
	_ "go.mau.fi/util/dbutil/litestream"
)

func openMatrixCryptoDB(dbPath string) (*dbutil.Database, error) {
	dsn := fmt.Sprintf("file:%s?_txlock=immediate", dbPath)
	db, err := sql.Open("sqlite3-fk-wal", dsn)
	if err != nil {
		return nil, err
	}
	return dbutil.NewWithDB(db, "sqlite3-fk-wal")
}
