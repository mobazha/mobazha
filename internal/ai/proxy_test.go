package ai

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProxy_Generate_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/chat/completions" {
			t.Errorf("expected /chat/completions, got %s", r.URL.Path)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer test-key" {
			t.Errorf("unexpected auth header: %s", auth)
		}

		var reqBody map[string]interface{}
		json.NewDecoder(r.Body).Decode(&reqBody)
		if reqBody["model"] != "gpt-4o" {
			t.Errorf("expected model gpt-4o, got %v", reqBody["model"])
		}

		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"content": `{"title": "Great Product", "tags": ["tag1", "tag2"]}`,
					},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	proxy := NewProxy(server.Client())
	cfg := Config{
		Provider: "openai",
		APIKey:   "test-key",
		Model:    "gpt-4o",
		BaseURL:  server.URL,
		Enabled:  true,
	}

	result, err := proxy.Generate(cfg, GenerateRequest{
		Action: "suggest_tags",
		Title:  "Test Product",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Title != "Great Product" {
		t.Errorf("expected title 'Great Product', got %q", result.Title)
	}
	if len(result.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(result.Tags))
	}
}

func TestProxy_Generate_FencedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"content": "```json\n{\"title\": \"Fenced Title\"}\n```",
					},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	proxy := NewProxy(server.Client())
	cfg := Config{APIKey: "k", BaseURL: server.URL, Enabled: true}

	result, err := proxy.Generate(cfg, GenerateRequest{Action: "improve_title", Title: "x"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Title != "Fenced Title" {
		t.Errorf("expected 'Fenced Title', got %q", result.Title)
	}
}

func TestProxy_Generate_UpstreamError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"message": "Rate limit exceeded",
			},
		})
	}))
	defer server.Close()

	proxy := NewProxy(server.Client())
	cfg := Config{APIKey: "k", BaseURL: server.URL, Enabled: true}

	_, err := proxy.Generate(cfg, GenerateRequest{Action: "suggest_tags", Title: "x"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := err.Error(); got != "AI upstream error: Rate limit exceeded" {
		t.Errorf("unexpected error: %s", got)
	}
}

func TestProxy_Generate_NotConfigured(t *testing.T) {
	proxy := NewProxy(nil)
	cfg := Config{Enabled: false}

	_, err := proxy.Generate(cfg, GenerateRequest{Action: "suggest_tags"})
	if err == nil {
		t.Fatal("expected error for unconfigured AI")
	}
}

func TestProxy_Generate_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{},
		})
	}))
	defer server.Close()

	proxy := NewProxy(server.Client())
	cfg := Config{APIKey: "k", BaseURL: server.URL, Enabled: true}

	_, err := proxy.Generate(cfg, GenerateRequest{Action: "suggest_tags", Title: "x"})
	if err == nil {
		t.Fatal("expected error for empty response")
	}
}

func TestProxy_Generate_AllActions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"content": `{"title": "T", "description": "D", "tags": ["a"], "categories": ["c"], "shortDescription": "S"}`,
					},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	proxy := NewProxy(server.Client())
	cfg := Config{APIKey: "k", BaseURL: server.URL, Enabled: true, Model: "gpt-4o"}

	actions := []GenerateRequest{
		{Action: "generate_from_images", Images: []string{"https://example.com/img.jpg"}, Language: "zh"},
		{Action: "improve_title", Title: "Old Title", Description: "Some desc"},
		{Action: "polish_description", Title: "Title", Description: "Old desc", Language: "en"},
		{Action: "suggest_tags", Title: "Title"},
	}

	for _, req := range actions {
		_, err := proxy.Generate(cfg, req)
		if err != nil {
			t.Errorf("action %s failed: %v", req.Action, err)
		}
	}
}

func TestProxy_Generate_UnknownAction(t *testing.T) {
	proxy := NewProxy(nil)
	cfg := Config{APIKey: "k", BaseURL: "http://localhost", Enabled: true}

	_, err := proxy.Generate(cfg, GenerateRequest{Action: "nonexistent"})
	if err == nil {
		t.Fatal("expected error for unknown action")
	}
}

func TestConfig_Defaults(t *testing.T) {
	cfg := Config{Provider: "zhipu"}
	if url := cfg.EffectiveBaseURL(); url != "https://open.bigmodel.cn/api/paas/v4" {
		t.Errorf("unexpected base URL: %s", url)
	}
	if m := cfg.EffectiveModel(); m != "glm-4v-flash" {
		t.Errorf("unexpected model: %s", m)
	}

	cfg2 := Config{Provider: "custom", BaseURL: "https://my.api.com/v1", Model: "my-model"}
	if url := cfg2.EffectiveBaseURL(); url != "https://my.api.com/v1" {
		t.Errorf("unexpected base URL: %s", url)
	}
	if m := cfg2.EffectiveModel(); m != "my-model" {
		t.Errorf("unexpected model: %s", m)
	}
}

func TestSupportedProviders(t *testing.T) {
	providers := SupportedProviders()
	if len(providers) < 3 {
		t.Errorf("expected at least 3 providers, got %d", len(providers))
	}

	ids := make(map[string]bool)
	for _, p := range providers {
		ids[p.ID] = true
		if p.Label == "" {
			t.Errorf("provider %s has empty label", p.ID)
		}
	}
	for _, required := range []string{"openai", "zhipu", "custom"} {
		if !ids[required] {
			t.Errorf("missing required provider: %s", required)
		}
	}
}

func TestConfig_IsValid(t *testing.T) {
	tests := []struct {
		name   string
		config Config
		want   bool
	}{
		{"fully configured", Config{Enabled: true, APIKey: "k", BaseURL: "https://x.com/v1"}, true},
		{"provider default base URL", Config{Enabled: true, APIKey: "k", Provider: "zhipu"}, true},
		{"disabled", Config{Enabled: false, APIKey: "k", BaseURL: "https://x.com/v1"}, false},
		{"no key", Config{Enabled: true, BaseURL: "https://x.com/v1"}, false},
		{"custom no base URL falls back to openai", Config{Enabled: true, APIKey: "k", Provider: "custom"}, true},
		{"empty everything except enabled+key", Config{Enabled: true, APIKey: "k"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.config.IsValid(); got != tt.want {
				t.Errorf("IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProxy_TestConnection_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": "Hi"}},
			},
		})
	}))
	defer server.Close()

	proxy := NewProxy(&http.Client{})
	cfg := Config{APIKey: "test-key", BaseURL: server.URL, Enabled: true, Model: "gpt-4o"}
	err := proxy.TestConnection(cfg)
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestProxy_TestConnection_AuthFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]string{"message": "Invalid API key"},
		})
	}))
	defer server.Close()

	proxy := NewProxy(&http.Client{})
	cfg := Config{APIKey: "bad-key", BaseURL: server.URL, Enabled: true}
	err := proxy.TestConnection(cfg)
	if err == nil {
		t.Error("expected error, got nil")
	}
	if err.Error() != "authentication failed: invalid API key" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestProxy_TestConnection_NoKey(t *testing.T) {
	proxy := NewProxy(&http.Client{})
	cfg := Config{BaseURL: "https://example.com", Enabled: true}
	err := proxy.TestConnection(cfg)
	if err == nil || err.Error() != "API key is required" {
		t.Errorf("expected 'API key is required', got %v", err)
	}
}

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`{"key": "value"}`, `{"key": "value"}`},
		{"```json\n{\"key\": \"value\"}\n```", `{"key": "value"}`},
		{"```\n{\"key\": \"value\"}\n```", `{"key": "value"}`},
		{" {\"key\": \"value\"} ", `{"key": "value"}`},
	}
	for _, tt := range tests {
		got := extractJSON(tt.input)
		if got != tt.expected {
			t.Errorf("extractJSON(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
