package repo

import (
	"os"
	"path"
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
)

func TestNewRepo(t *testing.T) {
	var dir = path.Join(os.TempDir(), "mobazha", "newRepoTest")
	os.RemoveAll(dir)
	r, err := NewRepo("", dir, true)
	if err != nil {
		t.Fatal(err)
	}
	defer r.DestroyRepo()

	if r.DB() == nil {
		t.Error("Failed to initialize the database")
	}
}

func TestNewRepoWithCustomMnemonicSeed(t *testing.T) {
	var (
		dir      = path.Join(os.TempDir(), "mobazha", "newRepoTest2")
		mnemonic = "abc"
	)
	os.RemoveAll(dir)
	r, err := NewRepoWithCustomMnemonicSeed("", dir, mnemonic, true)
	if err != nil {
		t.Fatal(err)
	}
	defer r.DestroyRepo()

	var dbSeed models.Key
	err = r.db.View(func(tx database.Tx) error {
		return tx.Read().Where("name = ?", "mnemonic").First(&dbSeed).Error
	})
	if err != nil {
		t.Fatal(err)
	}

	if string(dbSeed.Value) != mnemonic {
		t.Errorf("Failed to set correct mnemonic. Expected %s, got %s", mnemonic, string(dbSeed.Value))
	}
}

func TestIsRepoInitialized(t *testing.T) {
	dir := path.Join(os.TempDir(), "mobazha", "isInitTest")
	os.RemoveAll(dir)

	if IsRepoInitialized(dir) {
		t.Error("Empty dir should not be initialized")
	}

	os.RemoveAll(dir)
	r, err := NewRepo("", dir, true)
	if err != nil {
		t.Fatal(err)
	}
	defer r.DestroyRepo()

	if !IsRepoInitialized(dir) {
		t.Error("Repo should be initialized after NewRepo")
	}
}
