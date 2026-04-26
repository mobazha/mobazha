package mcpconnect

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestIsAlreadyConfigured_True(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")

	config := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			MCPServerName: map[string]interface{}{"url": "http://localhost"},
		},
	}
	data, _ := json.Marshal(config)
	os.WriteFile(path, data, 0o600)

	if !isAlreadyConfigured(path, MCPServerName) {
		t.Error("should detect existing config")
	}
}

func TestIsAlreadyConfigured_False(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")

	config := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"other-server": map[string]interface{}{"url": "http://localhost"},
		},
	}
	data, _ := json.Marshal(config)
	os.WriteFile(path, data, 0o600)

	if isAlreadyConfigured(path, MCPServerName) {
		t.Error("should not detect when key is absent")
	}
}

func TestIsAlreadyConfigured_NoFile(t *testing.T) {
	if isAlreadyConfigured("/nonexistent/path", MCPServerName) {
		t.Error("should return false for missing file")
	}
}

func TestIsAlreadyConfigured_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")
	os.WriteFile(path, []byte("not json"), 0o600)

	if isAlreadyConfigured(path, MCPServerName) {
		t.Error("should return false for invalid JSON")
	}
}

func TestConfigDirExists(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "mcp.json")

	if !configDirExists(configPath) {
		t.Error("should detect existing parent dir")
	}

	if configDirExists("/nonexistent/dir/mcp.json") {
		t.Error("should not detect nonexistent dir")
	}
}

func TestDetect_ConfigBasedClient(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "mcp.json")

	c := &ClientSpec{
		Name:        "test-client",
		DisplayName: "Test",
		ConfigPaths: map[string]string{
			runtime.GOOS: configPath,
		},
		WriteMode: WriteJSON,
	}

	status := detect(c)
	if !status.Installed {
		t.Error("should detect as installed (parent dir exists)")
	}
	if status.Configured {
		t.Error("should not be configured (no file)")
	}

	// Write a config with our server key
	config := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			MCPServerName: map[string]interface{}{"url": "http://test"},
		},
	}
	data, _ := json.Marshal(config)
	os.WriteFile(configPath, data, 0o600)

	status = detect(c)
	if !status.Configured {
		t.Error("should be configured after writing config")
	}
}

func TestClientByName_CaseInsensitive(t *testing.T) {
	if _, ok := ClientByName("CURSOR"); !ok {
		t.Error("should find cursor case-insensitively")
	}
	if _, ok := ClientByName("Claude-Code"); !ok {
		t.Error("should find claude-code case-insensitively")
	}
	if _, ok := ClientByName("nonexistent"); ok {
		t.Error("should not find nonexistent client")
	}
}

func TestServerKey_Default(t *testing.T) {
	c := &ClientSpec{Name: "test"}
	if c.ServerKey() != MCPServerName {
		t.Errorf("default ServerKey should be %q, got %q", MCPServerName, c.ServerKey())
	}
}

func TestServerKey_Custom(t *testing.T) {
	c := &ClientSpec{Name: "test", ConfigKey: "custom-key"}
	if c.ServerKey() != "custom-key" {
		t.Errorf("custom ServerKey should be 'custom-key', got %q", c.ServerKey())
	}
}
