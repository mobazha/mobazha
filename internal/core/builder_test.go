package core

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path"
	"testing"

	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/repo"
	"github.com/mobazha/mobazha3.0/pkg/models"
)

func TestNewNode(t *testing.T) {
	dataDir := path.Join(os.TempDir(), "mobazha-test", "TestNewNode")

	cfg := repo.Config{
		DataDir:       dataDir,
		Testnet:       true,
		IPNSQuorum:    3,
		BoostrapAddrs: []string{},
		SwarmAddrs:    []string{fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", rand.Intn(65535))},
	}

	node, err := NewNode(context.Background(), &cfg, repo.DefaultNodeID)
	if err != nil {
		t.Fatal(err)
	}

	defer node.DestroyNode()

	// Load our identity key from the db and set it in the config.
	var dbIdentityKey models.Key
	err = node.repo.DB().View(func(tx database.Tx) error {
		return tx.Read().Where("name = ?", "identity").First(&dbIdentityKey).Error
	})

	id, err := repo.IdentityFromKey(dbIdentityKey.Value)
	if err != nil {
		t.Fatal(err)
	}
	if node.ipfsNode.Identity.String() != id.PeerID {
		t.Error("Incorrect identity instantiated")
	}
}
