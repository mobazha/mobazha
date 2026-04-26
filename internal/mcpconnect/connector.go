package mcpconnect

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// ConnectResult is the outcome of configuring one AI client.
type ConnectResult struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Status      string `json:"status"` // "connected", "not_installed", "already_configured", "error"
	ConfigPath  string `json:"configPath,omitempty"`
	Method      string `json:"method,omitempty"` // "json", "cli", "stdio"
	Error       string `json:"error,omitempty"`
}

// ConnectOpts holds the parameters needed to configure MCP connections.
type ConnectOpts struct {
	MCPURL        string // full MCP Streamable HTTP endpoint URL
	Token         string // long-lived API token for authentication
	BridgeBinPath string // absolute path to the mobazha binary (for stdio bridge)
	Force         bool   // overwrite existing configuration
}

// ConnectAll configures all detected AI clients.
func ConnectAll(opts ConnectOpts) []ConnectResult {
	results := make([]ConnectResult, 0, len(Clients))
	for i := range Clients {
		results = append(results, connectOne(&Clients[i], opts))
	}
	return results
}

// ConnectByName configures a single client by name.
func ConnectByName(name string, opts ConnectOpts) (ConnectResult, error) {
	client, ok := ClientByName(name)
	if !ok {
		return ConnectResult{}, fmt.Errorf("unknown client: %s", name)
	}
	return connectOne(client, opts), nil
}

func connectOne(c *ClientSpec, opts ConnectOpts) ConnectResult {
	r := ConnectResult{
		Name:        c.Name,
		DisplayName: c.DisplayName,
	}

	status := detect(c)
	if !status.Installed {
		r.Status = "not_installed"
		return r
	}

	if status.Configured && !opts.Force {
		r.Status = "already_configured"
		r.ConfigPath = status.ConfigPath
		return r
	}

	var err error
	switch c.WriteMode {
	case WriteJSON:
		err = writeJSONConfig(c, opts)
		r.Method = "json"
		r.ConfigPath = c.ResolvedConfigPath()
	case WriteCLI:
		if opts.Force {
			_ = runCLIDisconnect(c)
		}
		err = runCLIConnect(c, opts)
		r.Method = "cli"
	case WriteStdio:
		err = writeStdioConfig(c, opts)
		r.Method = "stdio"
		r.ConfigPath = c.ResolvedConfigPath()
	}

	if err != nil {
		r.Status = "error"
		r.Error = err.Error()
	} else {
		r.Status = "connected"
	}
	return r
}

// writeJSONConfig merges our MCP server entry into the client's JSON config,
// preserving all existing entries. Uses atomic writes to prevent corruption.
func writeJSONConfig(c *ClientSpec, opts ConnectOpts) error {
	configPath := c.ResolvedConfigPath()
	if configPath == "" {
		return fmt.Errorf("no config path for %s on this OS", c.Name)
	}

	config, err := readJSONConfig(configPath)
	if err != nil {
		return err
	}

	servers := ensureServersMap(config)
	servers[c.ServerKey()] = map[string]interface{}{
		"url": opts.MCPURL,
		"headers": map[string]string{
			"Authorization": "Bearer " + opts.Token,
		},
	}
	config["mcpServers"] = servers

	return writeConfigAtomic(configPath, config)
}

// writeStdioConfig writes a config entry that launches `mobazha mcp bridge`
// as a stdio transport. Uses atomic writes to prevent corruption.
func writeStdioConfig(c *ClientSpec, opts ConnectOpts) error {
	configPath := c.ResolvedConfigPath()
	if configPath == "" {
		return fmt.Errorf("no config path for %s on this OS", c.Name)
	}

	config, err := readJSONConfig(configPath)
	if err != nil {
		return err
	}

	binPath := opts.BridgeBinPath
	if binPath == "" {
		binPath, err = os.Executable()
		if err != nil {
			return fmt.Errorf("cannot determine executable path: %w", err)
		}
	}

	servers := ensureServersMap(config)
	servers[c.ServerKey()] = map[string]interface{}{
		"command": binPath,
		"args":    []string{"mcp", "bridge", "--url", opts.MCPURL, "--token", opts.Token},
	}
	config["mcpServers"] = servers

	return writeConfigAtomic(configPath, config)
}

// runCLIConnect invokes the client's own CLI to register the MCP server
// using properly structured arguments (no shell string splitting).
func runCLIConnect(c *ClientSpec, opts ConnectOpts) error {
	args := c.BuildConnectArgs(opts.MCPURL, opts.Token)
	if len(args) == 0 {
		return fmt.Errorf("no CLI connect command for %s", c.Name)
	}

	cmd := exec.Command(args[0], args[1:]...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("CLI command failed: %w\noutput: %s", err, string(output))
	}
	return nil
}

// DisconnectAll removes MCP configuration from all clients.
func DisconnectAll() []ConnectResult {
	results := make([]ConnectResult, 0, len(Clients))
	for i := range Clients {
		results = append(results, disconnectOne(&Clients[i]))
	}
	return results
}

// DisconnectByName removes MCP configuration from a single client.
func DisconnectByName(name string) (ConnectResult, error) {
	client, ok := ClientByName(name)
	if !ok {
		return ConnectResult{}, fmt.Errorf("unknown client: %s", name)
	}
	return disconnectOne(client), nil
}

func disconnectOne(c *ClientSpec) ConnectResult {
	r := ConnectResult{
		Name:        c.Name,
		DisplayName: c.DisplayName,
	}

	status := detect(c)

	// CLI-based clients (e.g. Claude Code) store config internally,
	// so our file-based detection may miss them. Always attempt disconnect.
	if !status.Configured && c.WriteMode != WriteCLI {
		r.Status = "not_configured"
		return r
	}
	if !status.Installed && c.WriteMode == WriteCLI {
		r.Status = "not_configured"
		return r
	}

	var err error
	switch c.WriteMode {
	case WriteJSON, WriteStdio:
		err = removeJSONEntry(c)
		r.ConfigPath = c.ResolvedConfigPath()
	case WriteCLI:
		err = runCLIDisconnect(c)
	}

	if err != nil {
		r.Status = "error"
		r.Error = err.Error()
	} else {
		r.Status = "disconnected"
	}
	return r
}

func removeJSONEntry(c *ClientSpec) error {
	configPath := c.ResolvedConfigPath()
	if configPath == "" {
		return nil
	}

	config, err := readJSONConfig(configPath)
	if err != nil || len(config) == 0 {
		return nil
	}

	servers, _ := config["mcpServers"].(map[string]interface{})
	if servers == nil {
		return nil
	}

	delete(servers, c.ServerKey())

	return writeConfigAtomic(configPath, config)
}

func runCLIDisconnect(c *ClientSpec) error {
	args := c.BuildDisconnectArgs()
	if len(args) == 0 {
		return nil
	}

	cmd := exec.Command(args[0], args[1:]...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("CLI disconnect failed: %w\noutput: %s", err, string(output))
	}
	return nil
}

// --- helpers ---

// readJSONConfig reads and parses a JSON config file. Returns an empty map
// if the file doesn't exist or is empty. Strips UTF-8 BOM before parsing.
func readJSONConfig(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return make(map[string]interface{}), nil
	}
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	config := make(map[string]interface{})
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("existing config is invalid JSON: %w", err)
	}
	return config, nil
}

func ensureServersMap(config map[string]interface{}) map[string]interface{} {
	servers, _ := config["mcpServers"].(map[string]interface{})
	if servers == nil {
		servers = make(map[string]interface{})
	}
	return servers
}

// writeConfigAtomic marshals config to JSON and writes it atomically:
// write to a temp file in the same directory, then rename (atomic on POSIX).
// Files are written with 0o600 (owner-only) since they contain Bearer tokens.
func writeConfigAtomic(path string, config map[string]interface{}) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(dir, ".mobazha-mcp-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("setting file permissions: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("syncing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("atomic rename: %w", err)
	}
	return nil
}
