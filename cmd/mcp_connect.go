package cmd

import (
	"fmt"
	"net"
	"os"

	"github.com/mobazha/mobazha3.0/internal/mcpconnect"
	"github.com/mobazha/mobazha3.0/internal/repo"
)

// MCPConnect auto-detects installed AI clients and configures them to use
// this node's MCP server. Reads gateway URL from local config; requires
// a long-lived API token (create one via Admin UI → AI Agents).
type MCPConnect struct {
	DataDir string `short:"d" long:"datadir" description:"specify the data directory to be used"`
	Testnet bool   `short:"t" long:"testnet" description:"use the test network"`
	Force   bool   `long:"force" description:"overwrite existing MCP configurations"`
	Token   string `long:"token" description:"API token for MCP authentication (create via Admin UI)"`
}

func (x *MCPConnect) Execute(args []string) error {
	dataDir := x.resolveDataDir()

	cfg, err := repo.LoadConfig(dataDir)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	mcpURL, err := resolveMCPURL(cfg)
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("  Mobazha MCP Connect")
	fmt.Println()
	fmt.Printf("  Node gateway:   %s\n", mcpURL)

	token := x.Token
	if token == "" {
		fmt.Println()
		fmt.Println("  Token required. Create one in your Admin UI:")
		fmt.Println("    1. Open your store's /admin/ai-agents page")
		fmt.Println("    2. Click 'Connect All AI Clients' (auto-creates token + configures)")
		fmt.Println()
		fmt.Println("  Or create a token manually and pass it:")
		fmt.Println("    mobazha mcp connect --token <your-api-token>")
		fmt.Println()
		return fmt.Errorf("--token flag required")
	}

	tokenPreview := token
	if len(token) > 12 {
		tokenPreview = token[:8] + "..." + token[len(token)-4:]
	}
	fmt.Printf("  Token:          %s\n", tokenPreview)
	fmt.Println()

	binPath, _ := os.Executable()
	opts := mcpconnect.ConnectOpts{
		MCPURL:        mcpURL,
		Token:         token,
		BridgeBinPath: binPath,
		Force:         x.Force,
	}

	// If a positional arg is given, connect only that client.
	var clientName string
	if len(args) > 0 && args[0] != "" {
		clientName = args[0]
	}

	fmt.Println("  Detecting AI clients...")

	var results []mcpconnect.ConnectResult
	if clientName != "" {
		result, err := mcpconnect.ConnectByName(clientName, opts)
		if err != nil {
			return err
		}
		results = []mcpconnect.ConnectResult{result}
	} else {
		results = mcpconnect.ConnectAll(opts)
	}

	fmt.Println()
	connected := 0
	for _, r := range results {
		switch r.Status {
		case "connected":
			fmt.Printf("  [done] %-18s %s\n", r.DisplayName, statusDetail(r))
			connected++
		case "already_configured":
			fmt.Printf("  [skip] %-18s already configured (use --force to overwrite)\n", r.DisplayName)
		case "not_installed":
			fmt.Printf("  [skip] %-18s not installed\n", r.DisplayName)
		case "error":
			fmt.Printf("  [err]  %-18s %s\n", r.DisplayName, r.Error)
		}
	}

	fmt.Println()
	if connected > 0 {
		fmt.Printf("  Configured %d client(s). Restart each and try: \"List my store products\"\n", connected)
	} else {
		fmt.Println("  No clients were configured. Install an AI client and try again.")
	}
	fmt.Println()

	return nil
}

func (x *MCPConnect) resolveDataDir() string {
	if x.DataDir != "" {
		return x.DataDir
	}
	if x.Testnet {
		return repo.DefaultHomeDir + "-testnet"
	}
	return repo.DefaultHomeDir
}

func statusDetail(r mcpconnect.ConnectResult) string {
	switch r.Method {
	case "json":
		return fmt.Sprintf("wrote %s", r.ConfigPath)
	case "cli":
		return "configured via CLI"
	case "stdio":
		return fmt.Sprintf("wrote stdio config %s", r.ConfigPath)
	default:
		return "configured"
	}
}

// MCPList shows detected AI clients and their MCP configuration status.
type MCPList struct{}

func (x *MCPList) Execute(args []string) error {
	clients := mcpconnect.DetectAll()

	fmt.Println()
	fmt.Println("  AI Client Detection")
	fmt.Println()

	for _, c := range clients {
		status := "not installed"
		if c.Installed && c.Configured {
			status = "configured"
		} else if c.Installed {
			status = "installed (not configured)"
		}
		fmt.Printf("  %-18s %s\n", c.DisplayName, status)
	}
	fmt.Println()
	return nil
}

// MCPDisconnect removes MCP configuration from AI clients.
type MCPDisconnect struct{}

func (x *MCPDisconnect) Execute(args []string) error {
	var results []mcpconnect.ConnectResult

	if len(args) > 0 && args[0] != "" {
		r, err := mcpconnect.DisconnectByName(args[0])
		if err != nil {
			return err
		}
		results = []mcpconnect.ConnectResult{r}
	} else {
		results = mcpconnect.DisconnectAll()
	}

	fmt.Println()
	for _, r := range results {
		switch r.Status {
		case "disconnected":
			fmt.Printf("  [done] %-18s disconnected\n", r.DisplayName)
		case "not_installed":
			fmt.Printf("  [skip] %-18s not configured\n", r.DisplayName)
		case "error":
			fmt.Printf("  [err]  %-18s %s\n", r.DisplayName, r.Error)
		}
	}
	fmt.Println()
	return nil
}

func resolveMCPURL(cfg *repo.Config) (string, error) {
	addr := cfg.GatewayAddr
	if addr == "" {
		addr = "/ip4/127.0.0.1/tcp/" + repo.DefaultGatewayPort
	}

	host, port, err := parseMultiaddr(addr)
	if err != nil {
		return "", fmt.Errorf("invalid gateway address %q: %w", addr, err)
	}

	scheme := "http"
	base := fmt.Sprintf("%s://%s", scheme, net.JoinHostPort(host, port))
	return base + "/platform/v1/mcp", nil
}

func parseMultiaddr(ma string) (host string, port string, err error) {
	parts := splitMultiaddr(ma)
	for i := 0; i < len(parts)-1; i++ {
		switch parts[i] {
		case "ip4", "ip6":
			host = parts[i+1]
		case "tcp":
			port = parts[i+1]
		}
	}
	if host == "" || port == "" {
		return "", "", fmt.Errorf("cannot parse multiaddr: %s", ma)
	}
	return host, port, nil
}

func splitMultiaddr(ma string) []string {
	var parts []string
	for _, p := range splitOnSlash(ma) {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

func splitOnSlash(s string) []string {
	result := []string{}
	current := ""
	for _, c := range s {
		if c == '/' {
			result = append(result, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}
