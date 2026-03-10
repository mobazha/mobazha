package repo

import (
	"bytes"
	"os"
	"path"
	"reflect"
	"testing"

	config "github.com/ipfs/kubo/config"
	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
)

func TestRepo_cleanIdentityFromConfig(t *testing.T) {
	var (
		dir            = path.Join(os.TempDir(), "mobazha", "cleantest")
		configFilePath = path.Join(dir, "config")
	)

	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	if err := os.WriteFile(configFilePath, []byte(`{"Identity": "abc"}`), os.ModePerm); err != nil {
		t.Fatal(err)
	}

	if err := cleanIdentityFromConfig(dir); err != nil {
		t.Fatal(err)
	}

	cfg, err := os.ReadFile(configFilePath)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(cfg, []byte("{}")) {
		t.Error("Failed to properly clean config file")
	}
}

func TestRepo_mustDefaultConfig(t *testing.T) {
	cfg := mustDefaultConfig(true)

	// Verify testnet swarm addresses
	expectedSwarm := []string{
		"/ip4/0.0.0.0/tcp/4001",
		"/ip6/::/tcp/4001",
		"/ip4/0.0.0.0/tcp/9005/ws",
		"/ip6/::/tcp/9005/ws",
	}
	if !reflect.DeepEqual(cfg.Addresses.Swarm, expectedSwarm) {
		t.Errorf("Unexpected swarm addresses.\nExpected: %v\nGot: %v", expectedSwarm, cfg.Addresses.Swarm)
	}

	// Verify testnet gateway address
	expectedGateway := config.Strings{"/ip4/127.0.0.1/tcp/4002"}
	if !reflect.DeepEqual(cfg.Addresses.Gateway, expectedGateway) {
		t.Errorf("Unexpected gateway address.\nExpected: %v\nGot: %v", expectedGateway, cfg.Addresses.Gateway)
	}

	// Verify MDNS discovery is enabled
	if !cfg.Discovery.MDNS.Enabled {
		t.Error("MDNS discovery should be enabled")
	}

	// Verify empty bootstrap peers
	if len(cfg.Bootstrap) != 0 {
		t.Errorf("Bootstrap peers should be empty, got: %v", cfg.Bootstrap)
	}

	// Verify swarm settings
	if cfg.Swarm.EnableHolePunching != config.True {
		t.Error("HolePunching should be enabled")
	}
	if cfg.Swarm.RelayClient.Enabled != config.True {
		t.Error("RelayClient should be enabled")
	}
	if cfg.Swarm.ResourceMgr.Enabled != config.True {
		t.Error("ResourceMgr should be enabled")
	}

	// Verify identity is generated
	if cfg.Identity.PeerID == "" {
		t.Error("PeerID should not be empty")
	}
	if cfg.Identity.PrivKey == "" {
		t.Error("PrivKey should not be empty")
	}

	// Verify non-testnet config has different swarm addresses
	cfgMainnet := mustDefaultConfig(false)
	expectedMainnetSwarm := []string{
		"/ip4/0.0.0.0/tcp/5101",
		"/ip6/::/tcp/5101",
		"/ip4/0.0.0.0/tcp/7105/ws",
		"/ip6/::/tcp/7105/ws",
	}
	if !reflect.DeepEqual(cfgMainnet.Addresses.Swarm, expectedMainnetSwarm) {
		t.Errorf("Unexpected mainnet swarm addresses.\nExpected: %v\nGot: %v", expectedMainnetSwarm, cfgMainnet.Addresses.Swarm)
	}
}

func TestNewRepo(t *testing.T) {
	var dir = path.Join(os.TempDir(), "mobazha", "newRepoTest")
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
		dir      = path.Join(os.TempDir(), "mobazha", "newRepoTest")
		mnemonic = "abc"
	)
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
