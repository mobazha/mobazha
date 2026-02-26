package ai

import (
	"encoding/json"
	"sync"
)

// Config holds the AI service configuration for a node.
type Config struct {
	Provider string `json:"provider"`
	APIKey   string `json:"api_key"`
	Model    string `json:"model"`
	BaseURL  string `json:"base_url"`
	Enabled  bool   `json:"enabled"`
}

// fallbackProviders is the hardcoded minimum set used when remote config
// is unavailable. Kept in sync as a last-resort safety net.
var fallbackProviders = map[string]ProviderPreset{
	"openai": {
		Label: "OpenAI", DefaultModel: "gpt-4o", DefaultBaseURL: "https://api.openai.com/v1",
		Models: []string{"gpt-4o", "gpt-4o-mini", "gpt-4-turbo", "gpt-3.5-turbo"},
	},
	"zhipu": {
		Label: "智谱 GLM", DefaultModel: "glm-4v-flash", DefaultBaseURL: "https://open.bigmodel.cn/api/paas/v4",
		Models: []string{"glm-4v-flash", "glm-4-flash", "glm-4-plus", "glm-4"},
	},
	"siliconflow": {
		Label: "SiliconFlow", DefaultModel: "Pro/Qwen/Qwen2.5-VL-32B-Instruct", DefaultBaseURL: "https://api.siliconflow.cn/v1",
		Models: []string{"Pro/Qwen/Qwen2.5-VL-32B-Instruct", "Qwen/Qwen2.5-72B-Instruct", "deepseek-ai/DeepSeek-V3"},
	},
	"deepseek": {
		Label: "DeepSeek", DefaultModel: "deepseek-chat", DefaultBaseURL: "https://api.deepseek.com/v1",
		Models: []string{"deepseek-chat", "deepseek-reasoner"},
	},
	"moonshot": {
		Label: "Moonshot / Kimi", DefaultModel: "moonshot-v1-auto", DefaultBaseURL: "https://api.moonshot.cn/v1",
		Models: []string{"moonshot-v1-auto", "moonshot-v1-8k", "moonshot-v1-32k", "moonshot-v1-128k"},
	},
	"custom": {Label: "Custom (OpenAI-compatible)", DefaultModel: "", DefaultBaseURL: ""},
}

var (
	remoteMu        sync.RWMutex
	remoteProviders []ProviderInfo // populated from nodeConfig at startup
)

// ProviderPreset contains default settings for a known AI provider.
type ProviderPreset struct {
	Label          string   `json:"label"`
	DefaultModel   string   `json:"default_model"`
	DefaultBaseURL string   `json:"default_base_url"`
	Models         []string `json:"models,omitempty"`
}

// ProviderInfo is returned by the API for frontend form rendering.
type ProviderInfo struct {
	ID             string   `json:"id"`
	Label          string   `json:"label"`
	DefaultModel   string   `json:"default_model"`
	DefaultBaseURL string   `json:"default_base_url"`
	Models         []string `json:"models,omitempty"`
}

// LoadRemoteProviders parses the JSON string from nodeConfig.data.aiProviders
// and stores the result for use by SupportedProviders and Effective* methods.
// Invalid JSON is silently ignored (fallback remains active).
func LoadRemoteProviders(jsonStr string) error {
	var providers []ProviderInfo
	if err := json.Unmarshal([]byte(jsonStr), &providers); err != nil {
		return err
	}
	if len(providers) == 0 {
		return nil
	}
	remoteMu.Lock()
	remoteProviders = providers
	remoteMu.Unlock()
	return nil
}

// ResetRemoteProviders clears loaded remote providers (for testing).
func ResetRemoteProviders() {
	remoteMu.Lock()
	remoteProviders = nil
	remoteMu.Unlock()
}

// SupportedProviders returns the list of known AI providers.
// If remote providers were loaded, they are used; otherwise falls back
// to the hardcoded list.
func SupportedProviders() []ProviderInfo {
	remoteMu.RLock()
	if len(remoteProviders) > 0 {
		result := make([]ProviderInfo, len(remoteProviders))
		copy(result, remoteProviders)
		remoteMu.RUnlock()
		return result
	}
	remoteMu.RUnlock()

	order := []string{"zhipu", "siliconflow", "deepseek", "moonshot", "openai", "custom"}
	result := make([]ProviderInfo, 0, len(order))
	for _, id := range order {
		p := fallbackProviders[id]
		result = append(result, ProviderInfo{
			ID:             id,
			Label:          p.Label,
			DefaultModel:   p.DefaultModel,
			DefaultBaseURL: p.DefaultBaseURL,
			Models:         p.Models,
		})
	}
	return result
}

// lookupProvider finds provider defaults by ID, checking remote first
// then falling back to hardcoded.
func lookupProvider(providerID string) (ProviderPreset, bool) {
	remoteMu.RLock()
	for _, p := range remoteProviders {
		if p.ID == providerID {
			remoteMu.RUnlock()
			return ProviderPreset{
				Label:          p.Label,
				DefaultModel:   p.DefaultModel,
				DefaultBaseURL: p.DefaultBaseURL,
				Models:         p.Models,
			}, true
		}
	}
	remoteMu.RUnlock()

	p, ok := fallbackProviders[providerID]
	return p, ok
}

func (c *Config) IsValid() bool {
	return c.Enabled && c.APIKey != "" && c.EffectiveBaseURL() != ""
}

func (c *Config) EffectiveBaseURL() string {
	if c.BaseURL != "" {
		return c.BaseURL
	}
	if p, ok := lookupProvider(c.Provider); ok && p.DefaultBaseURL != "" {
		return p.DefaultBaseURL
	}
	return "https://api.openai.com/v1"
}

func (c *Config) EffectiveModel() string {
	if c.Model != "" {
		return c.Model
	}
	if p, ok := lookupProvider(c.Provider); ok && p.DefaultModel != "" {
		return p.DefaultModel
	}
	return "gpt-4o"
}
