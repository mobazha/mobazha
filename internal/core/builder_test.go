package core

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path"
	"testing"

	"github.com/mobazha/mobazha3.0/internal/repo"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
)

func TestNewNode(t *testing.T) {
	dataDir := path.Join(os.TempDir(), "mobazha-test", "TestNewNode")
	os.RemoveAll(dataDir)
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := repo.Config{
		DataDir:       dataDir,
		Testnet:       true,
		BoostrapAddrs: []string{},
		SwarmAddrs:    []string{fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", rand.Intn(65535))},
	}

	node, err := NewNode(context.Background(), &cfg, repo.DefaultNodeID)
	if err != nil {
		t.Fatal(err)
	}

	defer node.DestroyNode()

	var dbIdentityKey models.Key
	err = node.repo.DB().View(func(tx database.Tx) error {
		return tx.Read().Where("name = ?", "identity").First(&dbIdentityKey).Error
	})

	_, expectedPeerID, err := repo.PrivKeyAndPeerIDFromKey(dbIdentityKey.Value)
	if err != nil {
		t.Fatal(err)
	}
	if node.Identity().String() != expectedPeerID.String() {
		t.Error("Incorrect identity instantiated")
	}
}
