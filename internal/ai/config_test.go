package ai

import (
	"encoding/json"
	"testing"
)

func TestLoadRemoteProviders_ValidJSON(t *testing.T) {
	defer ResetRemoteProviders()

	jsonStr := `[
		{"id":"test-provider","label":"Test","default_model":"test-model","default_base_url":"https://test.example.com/v1","models":["model-a","model-b"]},
		{"id":"custom","label":"Custom","default_model":"","default_base_url":""}
	]`

	if err := LoadRemoteProviders(jsonStr); err != nil {
		t.Fatalf("LoadRemoteProviders: unexpected error: %v", err)
	}

	providers := SupportedProviders()
	if len(providers) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(providers))
	}
	if providers[0].ID != "test-provider" {
		t.Errorf("expected first provider id 'test-provider', got %q", providers[0].ID)
	}
	if providers[0].DefaultModel != "test-model" {
		t.Errorf("expected default_model 'test-model', got %q", providers[0].DefaultModel)
	}
	if len(providers[0].Models) != 2 {
		t.Errorf("expected 2 models, got %d", len(providers[0].Models))
	}
}

func TestLoadRemoteProviders_OverridesFallback(t *testing.T) {
	defer ResetRemoteProviders()

	jsonStr := `[{"id":"only-one","label":"Only","default_model":"m","default_base_url":"https://only.example.com/v1"}]`
	if err := LoadRemoteProviders(jsonStr); err != nil {
		t.Fatal(err)
	}

	providers := SupportedProviders()
	if len(providers) != 1 {
		t.Fatalf("expected 1 remote provider, got %d", len(providers))
	}
	if providers[0].ID != "only-one" {
		t.Errorf("expected 'only-one', got %q", providers[0].ID)
	}
}

func TestLoadRemoteProviders_InvalidJSON(t *testing.T) {
	defer ResetRemoteProviders()

	err := LoadRemoteProviders("not valid json")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}

	providers := SupportedProviders()
	if len(providers) == 0 {
		t.Fatal("fallback should return non-empty providers")
	}
	found := false
	for _, p := range providers {
		if p.ID == "openai" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected fallback to contain 'openai'")
	}
}

func TestLoadRemoteProviders_EmptyArray(t *testing.T) {
	defer ResetRemoteProviders()

	if err := LoadRemoteProviders("[]"); err != nil {
		t.Fatal(err)
	}

	providers := SupportedProviders()
	if len(providers) == 0 {
		t.Fatal("empty remote array should fall back to hardcoded providers")
	}
}

func TestFallbackProviders_WhenNoRemote(t *testing.T) {
	defer ResetRemoteProviders()
	ResetRemoteProviders()

	providers := SupportedProviders()
	if len(providers) == 0 {
		t.Fatal("expected non-empty fallback providers")
	}

	ids := make(map[string]bool)
	for _, p := range providers {
		ids[p.ID] = true
	}
	for _, expected := range []string{"openai", "anthropic", "deepseek", "custom"} {
		if !ids[expected] {
			t.Errorf("fallback missing provider %q", expected)
		}
	}
}

func TestLookupProvider_RemoteTakesPrecedence(t *testing.T) {
	defer ResetRemoteProviders()

	jsonStr := `[{"id":"openai","label":"OpenAI-Remote","default_model":"gpt-5","default_base_url":"https://api.openai.com/v2","models":["gpt-5","gpt-5-mini"]}]`
	if err := LoadRemoteProviders(jsonStr); err != nil {
		t.Fatal(err)
	}

	cfg := &Config{Provider: "openai"}
	if got := cfg.EffectiveModel(); got != "gpt-5" {
		t.Errorf("expected remote model 'gpt-5', got %q", got)
	}
	if got := cfg.EffectiveBaseURL(); got != "https://api.openai.com/v2" {
		t.Errorf("expected remote base_url, got %q", got)
	}
}

func TestLookupProvider_FallbackWhenNotInRemote(t *testing.T) {
	defer ResetRemoteProviders()

	jsonStr := `[{"id":"only-remote","label":"Only","default_model":"m","default_base_url":"https://only.example.com/v1"}]`
	if err := LoadRemoteProviders(jsonStr); err != nil {
		t.Fatal(err)
	}

	cfg := &Config{Provider: "deepseek"}
	if got := cfg.EffectiveModel(); got != "deepseek-chat" {
		t.Errorf("expected fallback model 'deepseek-chat', got %q", got)
	}
	if got := cfg.EffectiveBaseURL(); got != "https://api.deepseek.com/v1" {
		t.Errorf("expected fallback base_url, got %q", got)
	}
}

func TestEffectiveModel_UserOverridesAll(t *testing.T) {
	defer ResetRemoteProviders()

	cfg := &Config{Provider: "openai", Model: "my-custom-model"}
	if got := cfg.EffectiveModel(); got != "my-custom-model" {
		t.Errorf("expected user model 'my-custom-model', got %q", got)
	}
}

func TestEffectiveBaseURL_UserOverridesAll(t *testing.T) {
	defer ResetRemoteProviders()

	cfg := &Config{Provider: "openai", BaseURL: "https://my-proxy.example.com/v1"}
	if got := cfg.EffectiveBaseURL(); got != "https://my-proxy.example.com/v1" {
		t.Errorf("expected user base_url, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// MultiConfig tests
// ---------------------------------------------------------------------------

func TestMultiConfig_UnmarshalJSON_NewFormat(t *testing.T) {
	data := `{
		"enabled": true,
		"active_provider": "anthropic",
		"providers": {
			"openai": {"api_key":"sk-xxx","model":"gpt-4o"},
			"anthropic": {"api_key":"sk-ant-xxx"}
		}
	}`
	var mc MultiConfig
	if err := json.Unmarshal([]byte(data), &mc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !mc.Enabled {
		t.Error("expected enabled=true")
	}
	if mc.ActiveProvider != "anthropic" {
		t.Errorf("expected active_provider=anthropic, got %q", mc.ActiveProvider)
	}
	if len(mc.Providers) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(mc.Providers))
	}
	if mc.Providers["openai"].APIKey != "sk-xxx" {
		t.Error("openai api_key mismatch")
	}
	if mc.Providers["anthropic"].APIKey != "sk-ant-xxx" {
		t.Error("anthropic api_key mismatch")
	}
}

func TestMultiConfig_UnmarshalJSON_LegacyFormat(t *testing.T) {
	data := `{"provider":"openai","api_key":"sk-old","model":"gpt-4","base_url":"","enabled":true}`
	var mc MultiConfig
	if err := json.Unmarshal([]byte(data), &mc); err != nil {
		t.Fatalf("unmarshal legacy: %v", err)
	}
	if !mc.Enabled {
		t.Error("expected enabled=true")
	}
	if mc.ActiveProvider != "openai" {
		t.Errorf("expected active_provider=openai, got %q", mc.ActiveProvider)
	}
	cred, ok := mc.Providers["openai"]
	if !ok {
		t.Fatal("expected openai in providers map")
	}
	if cred.APIKey != "sk-old" {
		t.Errorf("expected api_key=sk-old, got %q", cred.APIKey)
	}
	if cred.Model != "gpt-4" {
		t.Errorf("expected model=gpt-4, got %q", cred.Model)
	}
}

func TestMultiConfig_UnmarshalJSON_EmptyJSON(t *testing.T) {
	var mc MultiConfig
	if err := json.Unmarshal([]byte(`{}`), &mc); err != nil {
		t.Fatalf("unmarshal empty: %v", err)
	}
	if mc.Enabled {
		t.Error("expected enabled=false for empty")
	}
	if mc.ActiveProvider != "" {
		t.Errorf("expected empty active_provider, got %q", mc.ActiveProvider)
	}
}

func TestMultiConfig_UnmarshalJSON_NewFormatWithoutProviders(t *testing.T) {
	data := `{"enabled":true,"active_provider":"openai"}`
	var mc MultiConfig
	if err := json.Unmarshal([]byte(data), &mc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if mc.ActiveProvider != "openai" {
		t.Errorf("expected active_provider=openai, got %q", mc.ActiveProvider)
	}
	if !mc.Enabled {
		t.Error("expected enabled=true")
	}
}

func TestMultiConfig_UnmarshalJSON_NewFormatEmptyProviders(t *testing.T) {
	data := `{"enabled":true,"active_provider":"anthropic","providers":{}}`
	var mc MultiConfig
	if err := json.Unmarshal([]byte(data), &mc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if mc.ActiveProvider != "anthropic" {
		t.Errorf("expected active_provider=anthropic, got %q", mc.ActiveProvider)
	}
	if mc.Providers == nil {
		t.Error("expected non-nil providers map")
	}
}

func TestMultiConfig_ActiveConfig(t *testing.T) {
	mc := MultiConfig{
		Enabled:        true,
		ActiveProvider: "anthropic",
		Providers: map[string]ProviderCredential{
			"openai":    {APIKey: "sk-oai", Model: "gpt-4o"},
			"anthropic": {APIKey: "sk-ant", Model: "claude-sonnet-4-20250514"},
		},
	}
	cfg := mc.ActiveConfig()
	if cfg.Provider != "anthropic" {
		t.Errorf("expected provider=anthropic, got %q", cfg.Provider)
	}
	if cfg.APIKey != "sk-ant" {
		t.Errorf("expected api_key=sk-ant, got %q", cfg.APIKey)
	}
	if cfg.Model != "claude-sonnet-4-20250514" {
		t.Errorf("expected model, got %q", cfg.Model)
	}
	if !cfg.Enabled {
		t.Error("expected enabled=true")
	}
}

func TestMultiConfig_ActiveConfig_MissingProvider(t *testing.T) {
	mc := MultiConfig{
		Enabled:        true,
		ActiveProvider: "nonexistent",
		Providers:      map[string]ProviderCredential{"openai": {APIKey: "sk-oai"}},
	}
	cfg := mc.ActiveConfig()
	if cfg.Provider != "nonexistent" {
		t.Errorf("expected provider=nonexistent, got %q", cfg.Provider)
	}
	if cfg.APIKey != "" {
		t.Error("expected empty api_key for missing provider")
	}
}

func TestMultiConfig_SetProvider(t *testing.T) {
	mc := MultiConfig{}
	mc.SetProvider("openai", ProviderCredential{APIKey: "sk-new", Model: "gpt-4o"})
	if len(mc.Providers) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(mc.Providers))
	}
	if mc.Providers["openai"].APIKey != "sk-new" {
		t.Error("api_key mismatch after SetProvider")
	}

	mc.SetProvider("openai", ProviderCredential{APIKey: "sk-updated"})
	if mc.Providers["openai"].APIKey != "sk-updated" {
		t.Error("api_key should be overwritten")
	}
}

func TestMultiConfig_ProviderSummary(t *testing.T) {
	mc := MultiConfig{
		Providers: map[string]ProviderCredential{
			"openai":    {APIKey: "sk-xxx", Model: "gpt-4o"},
			"anthropic": {APIKey: "", Model: "claude-sonnet-4-20250514"},
		},
	}
	summary := mc.ProviderSummary()
	if len(summary) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(summary))
	}
	if !summary["openai"].HasAPIKey {
		t.Error("openai should have has_api_key=true")
	}
	if summary["anthropic"].HasAPIKey {
		t.Error("anthropic should have has_api_key=false")
	}
}

func TestMultiConfig_MarshalRoundtrip(t *testing.T) {
	mc := MultiConfig{
		Enabled:        true,
		ActiveProvider: "openai",
		Providers: map[string]ProviderCredential{
			"openai":    {APIKey: "sk-oai", Model: "gpt-4o", BaseURL: ""},
			"anthropic": {APIKey: "sk-ant"},
		},
	}
	data, err := json.Marshal(mc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var mc2 MultiConfig
	if err := json.Unmarshal(data, &mc2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if mc2.ActiveProvider != mc.ActiveProvider {
		t.Error("active_provider mismatch after roundtrip")
	}
	if mc2.Providers["openai"].APIKey != "sk-oai" {
		t.Error("openai api_key mismatch after roundtrip")
	}
	if mc2.Providers["anthropic"].APIKey != "sk-ant" {
		t.Error("anthropic api_key mismatch after roundtrip")
	}
}
