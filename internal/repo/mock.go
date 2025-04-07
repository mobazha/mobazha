package repo

import (
	"math/rand"
	"os"
	"path"
	"strconv"

	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/database/ffsqlite"
)

// MockDB returns an in-memory sqlite db.
func MockDB() (database.Database, error) {
	n := rand.Uint32()
	dataDir := path.Join(os.TempDir(), "mobazha-test", strconv.Itoa(int(n)))
	db, err := ffsqlite.NewFFMemoryDB(dataDir)
	if err != nil {
		return nil, err
	}
	if err := autoMigrateDatabase(db); err != nil {
		return nil, err
	}
	return db, nil
}

// MockRepo returns a repo which uses a tmp data directory
// and in-memory database.
func MockRepo() (*Repo, error) {
	n := rand.Uint32()
	dataDir := path.Join(os.TempDir(), "mobazha-test", strconv.Itoa(int(n)))
	return newRepo(dataDir, "", true, true)
}
