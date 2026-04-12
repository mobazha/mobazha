package dbstore

import (
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"path"
	"runtime"
	"sync/atomic"

	"github.com/mobazha/mobazha3.0/internal/common"
	"github.com/mobazha/mobazha3.0/internal/database"
	pkgdb "github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/database/sqlitedialect"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var silentLogger = logger.New(
	log.New(os.Stdout, "\r\n", log.LstdFlags),
	logger.Config{
		LogLevel: logger.Silent,
	},
)

var ErrReadOnly = errors.New("tx is read only")

// NewSqliteDB instantiates a new SQLite-backed db which satisfies the Database interface.
// It uses TenantDB + DBPublicData for unified storage.
func NewSqliteDB(dataDir string) (database.Database, error) {
	dbPath := path.Join(dataDir, common.DatabaseFileName)
	dsn := dbPath + "?_busy_timeout=5000"
	if runtime.GOOS == "linux" {
		if key := os.Getenv("MBZ_SQLCIPHER_KEY"); key != "" {
			dsn = fmt.Sprintf("file:%s?_pragma_key=%s&_pragma_cipher_page_size=4096&_busy_timeout=5000", dbPath, url.QueryEscape(key))
		}
	}

	db, err := gorm.Open(sqlitedialect.Open(dsn), &gorm.Config{
		Logger:            silentLogger,
		AllowGlobalUpdate: true,
	})
	if err != nil {
		return nil, err
	}
	db.Exec("PRAGMA journal_mode=WAL")
	pd := NewDBPublicData(db, pkgdb.StandaloneTenantID)
	return NewTenantDBWithPublicData(db, pkgdb.StandaloneTenantID, pd)
}

var memDBCounter uint64

// NewMemoryDB instantiates a new in-memory db which satisfies the Database interface.
func NewMemoryDB(dataDir string) (database.Database, error) {
	n := atomic.AddUint64(&memDBCounter, 1)
	dsn := fmt.Sprintf("file:memdb_%d?mode=memory&cache=shared", n)
	db, err := gorm.Open(sqlitedialect.Open(dsn), &gorm.Config{
		Logger:            silentLogger,
		AllowGlobalUpdate: true,
	})
	if err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(&PublicDataRecord{}, &PublicMediaRecord{}); err != nil {
		return nil, err
	}
	pd := NewDBPublicData(db, pkgdb.StandaloneTenantID)
	return NewTenantDBWithPublicData(db, pkgdb.StandaloneTenantID, pd)
}

