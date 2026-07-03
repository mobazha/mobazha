package repo

import (
	"math/rand"
	"os"
	"path"
	"strconv"

	"github.com/mobazha/mobazha/internal/database/dbstore"
	"github.com/mobazha/mobazha/pkg/database"
)

// MockDB returns an in-memory sqlite db.
func MockDB() (database.Database, error) {
	n := rand.Uint32()
	dataDir := path.Join(os.TempDir(), "mobazha-test", strconv.Itoa(int(n)))
	db, err := dbstore.NewMemoryDB(dataDir)
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
	return newRepo("", dataDir, "", nil, true, true)
}
