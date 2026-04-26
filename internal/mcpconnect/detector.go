package mcpconnect

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
)

// ClientStatus represents the detection result for one AI client.
type ClientStatus struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Installed   bool   `json:"installed"`
	ConfigPath  string `json:"configPath,omitempty"`
	Configured  bool   `json:"configured"`
}

// DetectAll scans the system for installed AI clients and returns their status.
func DetectAll() []ClientStatus {
	results := make([]ClientStatus, 0, len(Clients))
	for i := range Clients {
		results = append(results, detect(&Clients[i]))
	}
	return results
}

// DetectOne checks a single client by name.
func DetectOne(name string) (ClientStatus, bool) {
	client, ok := ClientByName(name)
	if !ok {
		return ClientStatus{}, false
	}
	return detect(client), true
}

func detect(c *ClientSpec) ClientStatus {
	cs := ClientStatus{
		Name:        c.Name,
		DisplayName: c.DisplayName,
	}

	if c.DetectCmd != "" {
		cs.Installed = detectViaCLI(c.DetectCmd)
		// CLI clients may also have config paths for "already configured" check
		configPath := c.ResolvedConfigPath()
		if configPath != "" {
			cs.ConfigPath = configPath
		}
	} else {
		configPath := c.ResolvedConfigPath()
		if configPath != "" {
			cs.ConfigPath = configPath
			cs.Installed = configDirExists(configPath)
		}
	}

	if cs.Installed && cs.ConfigPath != "" {
		cs.Configured = isAlreadyConfigured(cs.ConfigPath, c.ServerKey())
	}

	return cs
}

// detectViaCLI checks if the binary is available in PATH.
func detectViaCLI(binaryName string) bool {
	if binaryName == "" {
		return false
	}
	_, err := exec.LookPath(binaryName)
	return err == nil
}

// configDirExists checks if the parent directory of the config file exists,
// indicating the application is probably installed.
func configDirExists(configPath string) bool {
	dir := filepath.Dir(configPath)
	info, err := os.Stat(dir)
	return err == nil && info.IsDir()
}

// isAlreadyConfigured parses the JSON config file and checks for our
// server key in the "mcpServers" object. This is more accurate than
// naive string matching.
func isAlreadyConfigured(configPath, serverKey string) bool {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return false
	}
	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return false
	}
	servers, _ := config["mcpServers"].(map[string]interface{})
	if servers == nil {
		return false
	}
	_, exists := servers[serverKey]
	return exists
}
