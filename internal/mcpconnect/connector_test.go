package mcpconnect

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestWriteJSONConfig_NewFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "mcp.json")

	c := &ClientSpec{
		Name:      "test-client",
		WriteMode: WriteJSON,
		ConfigPaths: map[string]string{
			runtime.GOOS: configPath,
		},
	}
	opts := ConnectOpts{
		MCPURL: "http://localhost:5102/v1/mcp",
		Token:  "mbz_sk_test_token_12345",
	}

	if err := writeJSONConfig(c, opts); err != nil {
		t.Fatalf("writeJSONConfig failed: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("reading config: %v", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("unmarshaling: %v", err)
	}

	servers, ok := config["mcpServers"].(map[string]interface{})
	if !ok {
		t.Fatal("mcpServers missing or wrong type")
	}
	entry, ok := servers[MCPServerName].(map[string]interface{})
	if !ok {
		t.Fatalf("server entry %q missing", MCPServerName)
	}
	if entry["url"] != opts.MCPURL {
		t.Errorf("url = %v, want %v", entry["url"], opts.MCPURL)
	}
	headers, _ := entry["headers"].(map[string]interface{})
	if headers["Authorization"] != "Bearer "+opts.Token {
		t.Errorf("Authorization = %v, want Bearer %s", headers["Authorization"], opts.Token)
	}

	// Verify owner-only permissions
	info, _ := os.Stat(configPath)
	if info.Mode().Perm()&0o077 != 0 {
		t.Errorf("file permissions %o allow group/other access", info.Mode().Perm())
	}
}

func TestWriteJSONConfig_PreservesExisting(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "mcp.json")

	existing := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"other-server": map[string]interface{}{
				"url": "http://example.com/mcp",
			},
		},
		"customKey": "should-be-preserved",
	}
	data, _ := json.MarshalIndent(existing, "", "  ")
	os.WriteFile(configPath, data, 0o600)

	c := &ClientSpec{
		Name:      "test-client",
		WriteMode: WriteJSON,
		ConfigPaths: map[string]string{
			runtime.GOOS: configPath,
		},
	}
	opts := ConnectOpts{
		MCPURL: "http://localhost:5102/v1/mcp",
		Token:  "test-token",
	}

	if err := writeJSONConfig(c, opts); err != nil {
		t.Fatalf("writeJSONConfig failed: %v", err)
	}

	data, _ = os.ReadFile(configPath)
	var config map[string]interface{}
	json.Unmarshal(data, &config)

	if config["customKey"] != "should-be-preserved" {
		t.Error("existing customKey was lost")
	}

	servers := config["mcpServers"].(map[string]interface{})
	if _, ok := servers["other-server"]; !ok {
		t.Error("existing other-server entry was lost")
	}
	if _, ok := servers[MCPServerName]; !ok {
		t.Error("new mobazha-store entry was not added")
	}
}

func TestWriteStdioConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "claude_desktop_config.json")

	c := &ClientSpec{
		Name:      "claude-desktop",
		WriteMode: WriteStdio,
		ConfigPaths: map[string]string{
			runtime.GOOS: configPath,
		},
	}
	opts := ConnectOpts{
		MCPURL:        "http://localhost:5102/v1/mcp",
		Token:         "mbz_sk_test",
		BridgeBinPath: "/usr/local/bin/mobazha",
	}

	if err := writeStdioConfig(c, opts); err != nil {
		t.Fatalf("writeStdioConfig failed: %v", err)
	}

	data, _ := os.ReadFile(configPath)
	var config map[string]interface{}
	json.Unmarshal(data, &config)

	servers := config["mcpServers"].(map[string]interface{})
	entry := servers[MCPServerName].(map[string]interface{})

	if entry["command"] != "/usr/local/bin/mobazha" {
		t.Errorf("command = %v", entry["command"])
	}
	args, _ := entry["args"].([]interface{})
	if len(args) < 6 {
		t.Fatalf("expected at least 6 args, got %d", len(args))
	}
	if args[0] != "mcp" || args[1] != "bridge" {
		t.Errorf("first args should be [mcp bridge], got %v %v", args[0], args[1])
	}
}

func TestWriteConfigAtomic_NoClobberOnExistingFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "mcp.json")

	original := `{"mcpServers":{"other":{"url":"http://keep.me"}}}`
	os.WriteFile(configPath, []byte(original), 0o600)

	config := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"mobazha-store": map[string]interface{}{"url": "http://new"},
			"other":         map[string]interface{}{"url": "http://keep.me"},
		},
	}
	if err := writeConfigAtomic(configPath, config); err != nil {
		t.Fatalf("writeConfigAtomic: %v", err)
	}

	data, _ := os.ReadFile(configPath)
	var result map[string]interface{}
	json.Unmarshal(data, &result)

	servers := result["mcpServers"].(map[string]interface{})
	if len(servers) != 2 {
		t.Errorf("expected 2 servers, got %d", len(servers))
	}
}

func TestRemoveJSONEntry(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "mcp.json")

	config := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			MCPServerName: map[string]interface{}{"url": "http://remove.me"},
			"keep-this":   map[string]interface{}{"url": "http://keep"},
		},
	}
	data, _ := json.Marshal(config)
	os.WriteFile(configPath, data, 0o600)

	c := &ClientSpec{
		Name:      "test",
		WriteMode: WriteJSON,
		ConfigPaths: map[string]string{
			runtime.GOOS: configPath,
		},
	}

	if err := removeJSONEntry(c); err != nil {
		t.Fatalf("removeJSONEntry: %v", err)
	}

	data, _ = os.ReadFile(configPath)
	var result map[string]interface{}
	json.Unmarshal(data, &result)

	servers := result["mcpServers"].(map[string]interface{})
	if _, ok := servers[MCPServerName]; ok {
		t.Error("mobazha-store should have been removed")
	}
	if _, ok := servers["keep-this"]; !ok {
		t.Error("keep-this should still be present")
	}
}

func TestReadJSONConfig_HandlesBOM(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	bom := []byte("\xef\xbb\xbf")
	content := []byte(`{"mcpServers":{}}`)
	os.WriteFile(path, append(bom, content...), 0o600)

	config, err := readJSONConfig(path)
	if err != nil {
		t.Fatalf("readJSONConfig with BOM: %v", err)
	}
	if _, ok := config["mcpServers"]; !ok {
		t.Error("mcpServers not found after BOM stripping")
	}
}

func TestBuildConnectArgs_ClaudeCode(t *testing.T) {
	c, ok := ClientByName("claude-code")
	if !ok {
		t.Fatal("claude-code not found")
	}

	args := c.BuildConnectArgs("http://localhost:5102/v1/mcp", "mbz_sk_test")
	if len(args) == 0 {
		t.Fatal("expected non-empty args")
	}

	if args[0] != "claude" {
		t.Errorf("expected executable 'claude', got %q", args[0])
	}

	// The -H value should be a single string "Authorization: Bearer mbz_sk_test"
	// (not split across multiple args).
	found := false
	for _, a := range args {
		if a == "Authorization: Bearer mbz_sk_test" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected single 'Authorization: Bearer mbz_sk_test' arg, got: %v", args)
	}
}
