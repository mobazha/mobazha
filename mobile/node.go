package mobile

import (
	"context"
	"path"

	"github.com/mobazha/mobazha3.0/internal/core"
	"github.com/mobazha/mobazha3.0/internal/repo"
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

// Node wraps an MobazhaNode in a way that can be compiled to mobile devices.
type Node struct {
	node *core.MobazhaNode
	done context.CancelFunc
}

func migrate(dataDir string) {
}

// NewNode returns a new MobileNode instance.
func NewNode(cfg *Config) (*Node, error) {
	dataDir := defaultDataDir
	if cfg.DataDir != "" {
		dataDir = cfg.DataDir
	}

	migrate(dataDir)

	logDir := path.Join(defaultDataDir, "logs")
	if cfg.LogDir != "" {
		logDir = cfg.LogDir
	}
	logLevel := "debug"
	if cfg.LogLevel != "" {
		logLevel = cfg.LogLevel
	}

	rcfg := &repo.Config{
		IPNSQuorum:        2,
		LogLevel:          logLevel,
		DisableNATPortMap: true,
		DataDir:           dataDir,
		LogDir:            logDir,
		DHTClientOnly:     true,
		Testnet:           cfg.Testnet,
		UserAgentComment:  cfg.UserAgentComment,
		APICookie:         cfg.APICookie,
		GatewayAddr:       cfg.GatewayAddress,
	}

	ctx, cancel := context.WithCancel(context.Background()) //nolint
	obNode, err := core.NewNode(ctx, rcfg, repo.DefaultNodeID)
	if err != nil {
		cancel()
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
	n.node.Stop(true)
}
