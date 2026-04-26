package mcpconnect

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// WriteMode determines how a client's MCP configuration is applied.
type WriteMode int

const (
	// WriteJSON merges an MCP URL + Token into the client's JSON config file.
	WriteJSON WriteMode = iota
	// WriteCLI invokes the client's own CLI to register the MCP server.
	WriteCLI
	// WriteStdio writes a stdio-bridge config that launches `mobazha mcp bridge`.
	WriteStdio
)

// MCPServerName is the identifier used when registering with AI clients.
const MCPServerName = "mobazha-store"

// CLIArgsFunc builds a structured argument list for exec.Command.
// Returning nil means the operation is not supported.
type CLIArgsFunc func(serverKey, mcpURL, token string) []string

// CLIRemoveFunc builds arguments for removing an MCP registration.
type CLIRemoveFunc func(serverKey string) []string

// ClientSpec defines detection rules and configuration templates for one AI client.
type ClientSpec struct {
	Name        string
	DisplayName string

	// ConfigPaths maps GOOS -> config file path (supports ~ and %APPDATA%).
	ConfigPaths map[string]string

	// DetectCmd is a CLI binary name to look up in PATH for installation detection.
	DetectCmd string

	// ConnectArgs returns structured arguments for CLI-based registration.
	// The first element is the executable name, rest are arguments.
	ConnectArgs CLIArgsFunc

	// DisconnectArgs returns structured arguments for CLI-based removal.
	DisconnectArgs CLIRemoveFunc

	WriteMode WriteMode

	// ConfigKey overrides the JSON key within "mcpServers". Defaults to MCPServerName.
	ConfigKey string
}

// Clients is the canonical list of supported AI clients.
// Only includes clients with verified configuration paths and behavior.
var Clients = []ClientSpec{
	{
		Name:        "cursor",
		DisplayName: "Cursor",
		ConfigPaths: map[string]string{
			"darwin":  "~/.cursor/mcp.json",
			"linux":   "~/.cursor/mcp.json",
			"windows": "%APPDATA%\\Cursor\\mcp.json",
		},
		WriteMode: WriteJSON,
	},
	{
		Name:        "claude-code",
		DisplayName: "Claude Code",
		DetectCmd:   "claude",
		ConnectArgs: func(key, url, token string) []string {
			return []string{"claude", "mcp", "add",
				"--transport", "http",
				key, url,
				"-H", "Authorization: Bearer " + token}
		},
		DisconnectArgs: func(key string) []string {
			return []string{"claude", "mcp", "remove", key}
		},
		WriteMode: WriteCLI,
	},
	{
		Name:        "claude-desktop",
		DisplayName: "Claude Desktop",
		ConfigPaths: map[string]string{
			"darwin":  "~/Library/Application Support/Claude/claude_desktop_config.json",
			"linux":   "~/.config/Claude/claude_desktop_config.json",
			"windows": "%APPDATA%\\Claude\\claude_desktop_config.json",
		},
		WriteMode: WriteStdio,
	},
	{
		Name:        "windsurf",
		DisplayName: "Windsurf",
		ConfigPaths: map[string]string{
			"darwin":  "~/.codeium/windsurf/mcp_config.json",
			"linux":   "~/.codeium/windsurf/mcp_config.json",
			"windows": "%APPDATA%\\Codeium\\windsurf\\mcp_config.json",
		},
		WriteMode: WriteJSON,
	},
	{
		Name:        "codex",
		DisplayName: "Codex CLI",
		DetectCmd:   "codex",
		ConnectArgs: func(key, url, token string) []string {
			return []string{"codex", "mcp", "add", key,
				"--transport", "http", "--url", url,
				"--header", "Authorization: Bearer " + token}
		},
		DisconnectArgs: func(key string) []string {
			return []string{"codex", "mcp", "remove", key}
		},
		WriteMode: WriteCLI,
	},
	{
		Name:        "opencode",
		DisplayName: "OpenCode",
		DetectCmd:   "opencode",
		ConnectArgs: func(key, url, token string) []string {
			return []string{"opencode", "mcp", "add", key,
				"--transport", "http", "--url", url,
				"--header", "Authorization: Bearer " + token}
		},
		DisconnectArgs: func(key string) []string {
			return []string{"opencode", "mcp", "remove", key}
		},
		WriteMode: WriteCLI,
	},
}

// ClientByName looks up a client by its canonical name (case-insensitive).
func ClientByName(name string) (*ClientSpec, bool) {
	lower := strings.ToLower(name)
	for i := range Clients {
		if strings.ToLower(Clients[i].Name) == lower {
			return &Clients[i], true
		}
	}
	return nil, false
}

// ResolvedConfigPath returns the absolute config file path for the current OS,
// expanding ~ and %APPDATA%. Returns empty string if no path is defined.
func (c *ClientSpec) ResolvedConfigPath() string {
	tmpl, ok := c.ConfigPaths[runtime.GOOS]
	if !ok {
		return ""
	}
	return expandPath(tmpl)
}

// ServerKey returns the JSON key to use in "mcpServers".
func (c *ClientSpec) ServerKey() string {
	if c.ConfigKey != "" {
		return c.ConfigKey
	}
	return MCPServerName
}

// BuildConnectArgs returns the full argument list for CLI-based connect.
func (c *ClientSpec) BuildConnectArgs(mcpURL, token string) []string {
	if c.ConnectArgs == nil {
		return nil
	}
	return c.ConnectArgs(c.ServerKey(), mcpURL, token)
}

// BuildDisconnectArgs returns the full argument list for CLI-based disconnect.
func (c *ClientSpec) BuildDisconnectArgs() []string {
	if c.DisconnectArgs == nil {
		return nil
	}
	return c.DisconnectArgs(c.ServerKey())
}

func expandPath(p string) string {
	if strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return p
		}
		return filepath.Join(home, p[2:])
	}
	if runtime.GOOS == "windows" && strings.Contains(p, "%APPDATA%") {
		appdata := os.Getenv("APPDATA")
		if appdata != "" {
			return strings.ReplaceAll(p, "%APPDATA%", appdata)
		}
	}
	return p
}
