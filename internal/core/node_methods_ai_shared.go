package core

import (
	"encoding/json"
	"fmt"

	aipkg "github.com/mobazha/mobazha/internal/ai"
	"github.com/mobazha/mobazha/pkg/models"
)

// AIProxy returns the node's AI proxy (may be nil).
func (n *MobazhaNode) AIProxy() *aipkg.Proxy {
	return n.aiProxy
}

// AIConfig returns the best chat configuration exposed by the selected
// distribution, falling back to the node-owned active provider.
func (n *MobazhaNode) AIConfig() aipkg.Config {
	cfg, _ := n.AIConfigForChat(nil)
	return cfg
}

// AIRateLimiter returns the distribution-provided limiter when available.
func (n *MobazhaNode) AIRateLimiter() *aipkg.DailyRateLimiter {
	return n.distributionAIRateLimiter()
}

// PlatformAIConfig returns the distribution-provided AI route when available.
func (n *MobazhaNode) PlatformAIConfig() *aipkg.Config {
	return n.distributionPlatformAIConfig()
}

// AIMultiConfig reads the full multi-provider config from the database.
func (n *MobazhaNode) AIMultiConfig() aipkg.MultiConfig {
	val, err := n.getSetting(models.SettingsKeyAIConfig)
	if err != nil || val == "" {
		return aipkg.MultiConfig{}
	}
	var config aipkg.MultiConfig
	if err := json.Unmarshal([]byte(val), &config); err != nil {
		return aipkg.MultiConfig{}
	}
	return config
}

// SaveAIMultiConfig persists the multi-provider AI config to the database.
func (n *MobazhaNode) SaveAIMultiConfig(config aipkg.MultiConfig) error {
	data, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("marshal AI multi config: %w", err)
	}
	return n.saveSetting(models.SettingsKeyAIConfig, string(data))
}
