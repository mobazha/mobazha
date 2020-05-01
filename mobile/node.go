package mobile

import (
	"context"
	"github.com/cpacia/openbazaar3.0/core"
	"github.com/cpacia/openbazaar3.0/repo"
	"path"
)

// NewDefaultConfig returns a new default config file.
func NewDefaultConfig() repo.Config {
	cfg := repo.Config{
		IPNSQuorum:            2,
		LogLevel:              "debug",
		EnabledWallets:        []string{"BTC", "BCH", "LTC", "ZEC", "ETH"},
		DisableNATPortMap:     true,
		DataDir:               repo.AppDataDir("obmobile", false),
		ExchangeRateProviders: []string{"https://ticker.openbazaar.org/api"},
		DHTClientOnly:         true,
	}
	return cfg
}

// MobileNode wraps an OpenBazaarNode in a way that can be compiled to mobile devises.
type MobileNode struct {
	node *core.OpenBazaarNode
	done context.CancelFunc
}

// NewNode returns a new MobileNode instance.
func NewNode(cfg repo.Config) *MobileNode {
	if len(cfg.BoostrapAddrs) == 0 {
		if cfg.Testnet {
			cfg.BoostrapAddrs = repo.DefaultTestnetBootstrapAddrs
		} else {
			cfg.BoostrapAddrs = repo.DefaultMainnetBootstrapAddrs
		}
	}
	if len(cfg.StoreAndForwardServers) == 0 {
		if cfg.Testnet {
			cfg.StoreAndForwardServers = repo.DefaultTestnetSNFServers
		} else {
			cfg.StoreAndForwardServers = repo.DefaultMainnetSNFServers
		}
	}
	if cfg.LogDir == "" {
		cfg.LogDir = path.Join(cfg.DataDir, "logs")
	}

	ctx, cancel := context.WithCancel(context.Background())
	obNode, err := core.NewNode(ctx, &cfg)
	if err != nil {
		panic(err)
	}
	return &MobileNode{node: obNode, done: cancel}
}

// Start will start the MobileNode.
func (n *MobileNode) Start() {
	n.node.Start()
}

// Stop will stop the MobileNode.
func (n *MobileNode) Stop() {
	n.done()
	n.Stop()
}
