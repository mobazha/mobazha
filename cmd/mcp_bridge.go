package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mobazha/mobazha3.0/internal/mcpconnect"
	"github.com/mobazha/mobazha3.0/internal/repo"
)

// MCPBridge starts a stdio-to-MCP bridge. AI clients like Claude Desktop
// launch this as a child process (via their JSON config) to communicate
// with the Mobazha MCP server through standard I/O.
type MCPBridge struct {
	DataDir string `short:"d" long:"datadir" description:"specify the data directory to be used"`
	Testnet bool   `short:"t" long:"testnet" description:"use the test network"`
	URL     string `long:"url" description:"MCP endpoint URL (overrides config)"`
	Token   string `long:"token" description:"Bearer token (overrides config)"`
}

func (x *MCPBridge) Execute(args []string) error {
	mcpURL := x.URL
	token := x.Token

	if mcpURL == "" || token == "" {
		dataDir := x.resolveDataDir()
		cfg, err := repo.LoadConfig(dataDir)
		if err != nil && (mcpURL == "" || token == "") {
			return fmt.Errorf("cannot load config from %s and --url/--token not provided: %w", dataDir, err)
		}

		if mcpURL == "" && cfg != nil {
			resolved, err := resolveMCPURL(cfg)
			if err != nil {
				return fmt.Errorf("resolving MCP URL: %w", err)
			}
			mcpURL = resolved
		}
	}

	if mcpURL == "" {
		return fmt.Errorf("MCP URL required: use --url or ensure node config exists")
	}
	if token == "" {
		return fmt.Errorf("Bearer token required: use --token flag")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Fprintln(os.Stderr, "bridge: shutting down...")
		cancel()
	}()

	opts := mcpconnect.BridgeOpts{
		SSEURL:        mcpURL,
		Token:         token,
		MaxRetries:    30,
		RetryInterval: 2 * time.Second,
	}

	return mcpconnect.RunBridge(ctx, opts)
}

func (x *MCPBridge) resolveDataDir() string {
	if x.DataDir != "" {
		return x.DataDir
	}
	if x.Testnet {
		return repo.DefaultHomeDir + "-testnet"
	}
	return repo.DefaultHomeDir
}
