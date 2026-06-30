package core

import (
	"encoding/json"
	"fmt"

	aipkg "github.com/mobazha/mobazha3.0/internal/ai"
	"github.com/mobazha/mobazha3.0/pkg/models"
)

// AIProxy returns the node's AI proxy (may be nil).
func (n *MobazhaNode) AIProxy() *aipkg.Proxy {
	return n.aiProxy
}

// AIConfig returns the best chat configuration exposed by the selected
// distribution, falling back to the node-owned active provider.
func (n *MobazhaNode) AIConfig() aipkg.Config {
	if selector, ok := any(n).(aiChatConfigSelector); ok {
		cfg, _ := selector.AIConfigForChat(nil)
		return cfg
	}
	config := n.AIMultiConfig()
	return config.ActiveConfig()
}

// AIRateLimiter returns the distribution-provided limiter when available.
func (n *MobazhaNode) AIRateLimiter() *aipkg.DailyRateLimiter {
	if services, ok := any(n).(aiDistributionServices); ok {
		return services.distributionAIRateLimiter()
	}
	return nil
}

// PlatformAIConfig returns the distribution-provided AI route when available.
func (n *MobazhaNode) PlatformAIConfig() *aipkg.Config {
	if services, ok := any(n).(aiDistributionServices); ok {
		return services.distributionPlatformAIConfig()
	}
	return nil
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

type aiChatConfigSelector interface {
	AIConfigForChat([]aipkg.ChatMsg) (aipkg.Config, error)
}

type aiDistributionServices interface {
	distributionAIRateLimiter() *aipkg.DailyRateLimiter
	distributionPlatformAIConfig() *aipkg.Config
}
