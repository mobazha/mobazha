package supervisor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// LauncherConfig represents the user-configurable settings written by the Web UI
// and monitored by the Launcher.
type LauncherConfig struct {
	AutoUpdateEnabled  bool   `json:"autoUpdateEnabled"`
	CheckIntervalMin   int    `json:"checkIntervalMinutes"`
	UpdateChannel      string `json:"updateChannel"`
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() LauncherConfig {
	return LauncherConfig{
		AutoUpdateEnabled: true,
		CheckIntervalMin:  360, // 6 hours
		UpdateChannel:     "stable",
	}
}

// ConfigManager reads and monitors launcher-config.json.
type ConfigManager struct {
	mu       sync.RWMutex
	config   LauncherConfig
	filePath string
}

func NewConfigManager(dataDir string) *ConfigManager {
	cm := &ConfigManager{
		config:   DefaultConfig(),
		filePath: filepath.Join(dataDir, "launcher-config.json"),
	}
	cm.Reload()
	return cm
}

func (cm *ConfigManager) Get() LauncherConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.config
}

// Reload reads the config file from disk. Missing file uses defaults.
func (cm *ConfigManager) Reload() {
	data, err := os.ReadFile(cm.filePath)
	if err != nil {
		return
	}
	cm.mu.Lock()
	defer cm.mu.Unlock()
	var cfg LauncherConfig
	if json.Unmarshal(data, &cfg) == nil {
		if cfg.CheckIntervalMin <= 0 {
			cfg.CheckIntervalMin = 360
		}
		if cfg.UpdateChannel == "" {
			cfg.UpdateChannel = "stable"
		}
		cm.config = cfg
	}
}

func (cm *ConfigManager) FilePath() string {
	return cm.filePath
}
