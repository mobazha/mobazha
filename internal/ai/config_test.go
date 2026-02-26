package ai

import (
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
	for _, expected := range []string{"openai", "zhipu", "deepseek", "custom"} {
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
