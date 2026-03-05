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
		Models:  []string{"gpt-4o", "gpt-4o-mini", "gpt-4-turbo", "gpt-3.5-turbo"},
		HelpURL: "https://platform.openai.com/api-keys",
	},
	"anthropic": {
		Label: "Anthropic Claude", DefaultModel: "claude-sonnet-4-20250514", DefaultBaseURL: "https://api.anthropic.com/v1",
		Models:  []string{"claude-sonnet-4-20250514", "claude-3-5-sonnet-20241022", "claude-3-haiku-20250131"},
		HelpURL: "https://console.anthropic.com/settings/keys",
	},
	"gemini": {
		Label: "Google Gemini", DefaultModel: "gemini-2.0-flash", DefaultBaseURL: "https://generativelanguage.googleapis.com/v1beta/openai",
		Models:  []string{"gemini-2.0-flash", "gemini-2.0-flash-lite", "gemini-1.5-pro", "gemini-1.5-flash"},
		HelpURL: "https://aistudio.google.com/apikey",
	},
	"deepseek": {
		Label: "DeepSeek", DefaultModel: "deepseek-chat", DefaultBaseURL: "https://api.deepseek.com/v1",
		Models:  []string{"deepseek-chat", "deepseek-reasoner"},
		HelpURL: "https://platform.deepseek.com/api_keys",
	},
	"qwen": {
		Label: "Qwen (通义千问)", DefaultModel: "qwen-max", DefaultBaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1",
		Models:  []string{"qwen-max", "qwen-plus", "qwen-turbo", "qwen-vl-max"},
		HelpURL: "https://dashscope.console.aliyun.com/apiKey",
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
	HelpURL        string   `json:"help_url,omitempty"`
}

// ProviderInfo is returned by the API for frontend form rendering.
type ProviderInfo struct {
	ID             string   `json:"id"`
	Label          string   `json:"label"`
	DefaultModel   string   `json:"default_model"`
	DefaultBaseURL string   `json:"default_base_url"`
	Models         []string `json:"models,omitempty"`
	HelpURL        string   `json:"help_url,omitempty"`
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

	order := []string{"openai", "anthropic", "gemini", "deepseek", "qwen", "custom"}
	result := make([]ProviderInfo, 0, len(order))
	for _, id := range order {
		p := fallbackProviders[id]
		result = append(result, ProviderInfo{
			ID:             id,
			Label:          p.Label,
			DefaultModel:   p.DefaultModel,
			DefaultBaseURL: p.DefaultBaseURL,
			Models:         p.Models,
			HelpURL:        p.HelpURL,
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
				HelpURL:        p.HelpURL,
			}, true
		}
	}
	remoteMu.RUnlock()

	p, ok := fallbackProviders[providerID]
	return p, ok
}

// IsAnthropicProvider returns true if the provider uses the Anthropic Messages API
// instead of the OpenAI-compatible Chat Completions API.
func IsAnthropicProvider(providerID string) bool {
	return providerID == "anthropic"
}

// ---------------------------------------------------------------------------
// MultiConfig — multi-provider storage format
// ---------------------------------------------------------------------------

// ProviderCredential stores per-provider configuration (API key, model override, base URL override).
type ProviderCredential struct {
	APIKey  string `json:"api_key"`
	Model   string `json:"model,omitempty"`
	BaseURL string `json:"base_url,omitempty"`
}

// MultiConfig supports multiple configured providers with one active.
type MultiConfig struct {
	Enabled        bool                          `json:"enabled"`
	ActiveProvider string                        `json:"active_provider"`
	Providers      map[string]ProviderCredential `json:"providers,omitempty"`
}

// ActiveConfig returns the flat Config for the currently active provider,
// suitable for passing to Proxy.Generate / Proxy.TestConnection.
func (mc *MultiConfig) ActiveConfig() Config {
	if mc.Providers == nil {
		return Config{Enabled: mc.Enabled}
	}
	cred, ok := mc.Providers[mc.ActiveProvider]
	if !ok {
		return Config{Enabled: mc.Enabled, Provider: mc.ActiveProvider}
	}
	return Config{
		Provider: mc.ActiveProvider,
		APIKey:   cred.APIKey,
		Model:    cred.Model,
		BaseURL:  cred.BaseURL,
		Enabled:  mc.Enabled,
	}
}

// SetProvider saves or updates credentials for a specific provider.
func (mc *MultiConfig) SetProvider(id string, cred ProviderCredential) {
	if mc.Providers == nil {
		mc.Providers = make(map[string]ProviderCredential)
	}
	mc.Providers[id] = cred
}

// ProviderSummaryInfo is the per-provider info returned by GET (no raw API key).
type ProviderSummaryInfo struct {
	HasAPIKey bool   `json:"has_api_key"`
	Model     string `json:"model"`
	BaseURL   string `json:"base_url"`
}

// ProviderSummary returns a map of provider ID → summary for the GET response.
func (mc *MultiConfig) ProviderSummary() map[string]ProviderSummaryInfo {
	result := make(map[string]ProviderSummaryInfo, len(mc.Providers))
	for id, cred := range mc.Providers {
		result[id] = ProviderSummaryInfo{
			HasAPIKey: cred.APIKey != "",
			Model:     cred.Model,
			BaseURL:   cred.BaseURL,
		}
	}
	return result
}

// UnmarshalJSON handles both the legacy single-provider format and the new
// multi-provider format. Legacy format is detected by presence of "api_key"
// at the top level without a "providers" map.
func (mc *MultiConfig) UnmarshalJSON(data []byte) error {
	// Try new format first
	type multiConfigAlias MultiConfig
	var newFmt multiConfigAlias
	if err := json.Unmarshal(data, &newFmt); err != nil {
		return err
	}

	// New format is identified by having providers map OR active_provider field.
	// Legacy format uses "provider" (not "active_provider"), so the alias
	// correctly distinguishes them via JSON tag names.
	if newFmt.Providers != nil || newFmt.ActiveProvider != "" {
		*mc = MultiConfig(newFmt)
		return nil
	}

	// Detect legacy format: has "api_key" or "provider" at top level
	var legacy struct {
		Provider string `json:"provider"`
		APIKey   string `json:"api_key"`
		Model    string `json:"model"`
		BaseURL  string `json:"base_url"`
		Enabled  bool   `json:"enabled"`
	}
	if err := json.Unmarshal(data, &legacy); err != nil {
		return err
	}

	mc.Enabled = legacy.Enabled
	mc.ActiveProvider = legacy.Provider
	mc.Providers = make(map[string]ProviderCredential)
	if legacy.APIKey != "" || legacy.Provider != "" {
		provider := legacy.Provider
		if provider == "" {
			provider = "openai"
		}
		mc.ActiveProvider = provider
		mc.Providers[provider] = ProviderCredential{
			APIKey:  legacy.APIKey,
			Model:   legacy.Model,
			BaseURL: legacy.BaseURL,
		}
	}
	return nil
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
