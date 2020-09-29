package mobile

import (
	"context"
	"github.com/cpacia/openbazaar3.0/core"
	"github.com/cpacia/openbazaar3.0/repo"
	"path"
)

var defaultDataDir = repo.AppDataDir("obmobile", false)

// Config holds the mobile node configuration.
type Config struct {
	LogLevel         string
	DataDir          string
	LogDir           string
	UserAgentComment string
	APICookie        string
	IPNSResolver     string
	GatewayAddress   string
	Testnet          bool
}

// NewDefaultConfig returns a new default config file.
func NewDefaultConfig() *Config {
	return &Config{
		DataDir:          defaultDataDir,
		LogDir:           path.Join(defaultDataDir, "logs"),
		LogLevel:         "debug",
		UserAgentComment: "obmobile",
		Testnet:          false,
	}
}

// Node wraps an OpenBazaarNode in a way that can be compiled to mobile devices.
type Node struct {
	node *core.OpenBazaarNode
	done context.CancelFunc
}

// NewNode returns a new MobileNode instance.
func NewNode(cfg *Config) (*Node, error) {
	dataDir := defaultDataDir
	if cfg.DataDir != "" {
		dataDir = cfg.DataDir
	}
	logDir := path.Join(defaultDataDir, "logs")
	if cfg.LogDir != "" {
		logDir = cfg.LogDir
	}
	logLevel := "debug"
	if cfg.LogLevel != "" {
		logLevel = cfg.LogLevel
	}
	bootstrapAddrs := repo.DefaultMainnetBootstrapAddrs
	if cfg.Testnet {
		bootstrapAddrs = repo.DefaultTestnetBootstrapAddrs
	}
	snfServers := repo.DefaultMainnetSNFServers
	if cfg.Testnet {
		snfServers = repo.DefaultTestnetSNFServers
	}

	rcfg := &repo.Config{
		IPNSQuorum:             2,
		LogLevel:               logLevel,
		EnabledWallets:         []string{"BTC", "BCH", "LTC", "ZEC", "ETH"},
		DisableNATPortMap:      true,
		DataDir:                dataDir,
		LogDir:                 logDir,
		ExchangeRateProviders:  []string{"https://ticker.openbazaar.org/api"},
		DHTClientOnly:          true,
		BoostrapAddrs:          bootstrapAddrs,
		StoreAndForwardServers: snfServers,
		Testnet:                cfg.Testnet,
		UserAgentComment:       cfg.UserAgentComment,
		APICookie:              cfg.APICookie,
		IPNSResolver:           cfg.IPNSResolver,
		GatewayAddr:            cfg.GatewayAddress,
	}

	ctx, cancel := context.WithCancel(context.Background()) //nolint
	obNode, err := core.NewNode(ctx, rcfg)
	if err != nil {
		return nil, err //nolint
	}
	return &Node{node: obNode, done: cancel}, nil //nolint
}

// Start will start the MobileNode.
func (n *Node) Start() {
	n.node.Start()
}

// Stop will stop the MobileNode.
func (n *Node) Stop() {
	n.done()
	n.Stop()
}
