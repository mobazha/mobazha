package ai

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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
	listingJSON := `{"title": "T", "description": "D", "tags": ["a"], "categories": ["c"], "shortDescription": "S"}`
	storeJSON := `{"version":1,"status":"published","theme":{"palette":"minimal","primaryColor":"#000","fontFamily":"inter","borderRadius":"md","headerStyle":"minimal"},"sections":[]}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]interface{}
		json.NewDecoder(r.Body).Decode(&reqBody)

		content := listingJSON
		msgs, _ := reqBody["messages"].([]interface{})
		if len(msgs) > 0 {
			if lastMsg, ok := msgs[len(msgs)-1].(map[string]interface{}); ok {
				if userContent, ok := lastMsg["content"].(string); ok {
					if strings.Contains(userContent, "store design") {
						content = storeJSON
					}
				}
			}
		}

		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]interface{}{"content": content}},
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
		{Action: "generate_store", BrandName: "Test", BrandDesc: "A test store"},
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
	cfg := Config{Provider: "anthropic"}
	if url := cfg.EffectiveBaseURL(); url != "https://api.anthropic.com/v1" {
		t.Errorf("unexpected base URL: %s", url)
	}
	if m := cfg.EffectiveModel(); m != "claude-sonnet-4-20250514" {
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
	for _, required := range []string{"openai", "anthropic", "custom"} {
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
		{"provider default base URL", Config{Enabled: true, APIKey: "k", Provider: "anthropic"}, true},
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

func TestProxy_Generate_StoreAction_Success(t *testing.T) {
	storeConfigJSON := `{"version":1,"status":"published","theme":{"palette":"ocean","primaryColor":"#1a3a5c","fontFamily":"inter","borderRadius":"md","headerStyle":"hero"},"sections":[{"id":"test-hero","type":"hero","props":{"title":"Welcome","height":"md","textAlign":"center"},"visible":true},{"id":"test-tabs","type":"store-tabs","props":{"tabs":["reviews","following","followers"]},"visible":true}]}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]interface{}
		json.NewDecoder(r.Body).Decode(&reqBody)

		if maxTokens, ok := reqBody["max_tokens"].(float64); ok {
			if maxTokens != 4096 {
				t.Errorf("expected max_tokens 4096 for store action, got %v", maxTokens)
			}
		}

		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]interface{}{"content": storeConfigJSON}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	proxy := NewProxy(server.Client())
	cfg := Config{APIKey: "k", BaseURL: server.URL, Enabled: true}

	result, err := proxy.Generate(cfg, GenerateRequest{
		Action:    "generate_store",
		BrandName: "Luna Botanicals",
		BrandDesc: "Organic skincare products",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.StoreConfig == nil {
		t.Fatal("expected StoreConfig to be set")
	}
	if result.Title != "" {
		t.Error("expected Title to be empty for store action")
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(result.StoreConfig, &parsed); err != nil {
		t.Fatalf("StoreConfig is not valid JSON: %v", err)
	}
	if parsed["version"] != float64(1) {
		t.Errorf("expected version 1, got %v", parsed["version"])
	}
}

func TestProxy_Generate_StoreAction_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]interface{}{"content": "This is not JSON at all"}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	proxy := NewProxy(server.Client())
	cfg := Config{APIKey: "k", BaseURL: server.URL, Enabled: true}

	_, err := proxy.Generate(cfg, GenerateRequest{
		Action:    "generate_store",
		BrandName: "Test",
		BrandDesc: "Test",
	})
	if err == nil {
		t.Fatal("expected error for invalid JSON response")
	}
	if got := err.Error(); got != "invalid AI response: not valid JSON" {
		t.Errorf("unexpected error message: %s", got)
	}
}

func TestBuildPrompt_GenerateStore(t *testing.T) {
	msgs, err := buildPrompt(GenerateRequest{
		Action:    "generate_store",
		BrandName: "Luna Botanicals",
		BrandDesc: "Organic skincare",
		Language:  "zh",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Role != "system" {
		t.Error("expected system role for first message")
	}
	sysContent, ok := msgs[0].Content.(string)
	if !ok {
		t.Fatal("expected string system content")
	}
	if !containsAll(sysContent, "StoreConfig Schema", "hero", "trust-badges", "store-tabs") {
		t.Error("system prompt missing expected schema content")
	}
	userContent, ok := msgs[1].Content.(string)
	if !ok {
		t.Fatal("expected string user content")
	}
	if !containsAll(userContent, "Luna Botanicals", "Organic skincare", "Chinese") {
		t.Error("user prompt missing brand info or language")
	}
}

func TestBuildPrompt_GenerateStore_MissingBrandName(t *testing.T) {
	_, err := buildPrompt(GenerateRequest{
		Action:    "generate_store",
		BrandDesc: "Some description",
	})
	if err == nil {
		t.Fatal("expected error for missing brandName")
	}
}

func TestBuildPrompt_RefineStore_MissingFields(t *testing.T) {
	_, err := buildPrompt(GenerateRequest{
		Action: "refine_store",
	})
	if err == nil {
		t.Fatal("expected error for missing storeConfig")
	}

	_, err = buildPrompt(GenerateRequest{
		Action:      "refine_store",
		StoreConfig: json.RawMessage(`{}`),
	})
	if err == nil {
		t.Fatal("expected error for missing instruction")
	}
}

func containsAll(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}

func TestProxy_Anthropic_Generate_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/messages" {
			t.Errorf("expected /messages, got %s", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "test-key" {
			t.Errorf("expected x-api-key header, got %q", r.Header.Get("x-api-key"))
		}
		if r.Header.Get("anthropic-version") == "" {
			t.Error("expected anthropic-version header")
		}
		if r.Header.Get("Authorization") != "" {
			t.Error("Anthropic should not send Authorization Bearer header")
		}

		var reqBody map[string]interface{}
		json.NewDecoder(r.Body).Decode(&reqBody)
		if reqBody["model"] != "claude-sonnet-4-20250514" {
			t.Errorf("expected model claude-sonnet-4-20250514, got %v", reqBody["model"])
		}
		if _, ok := reqBody["system"]; !ok {
			t.Error("expected top-level 'system' field for Anthropic")
		}

		resp := map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": `{"title": "Claude Product", "tags": ["ai", "test"]}`},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	proxy := NewProxy(server.Client())
	cfg := Config{
		Provider: "anthropic",
		APIKey:   "test-key",
		Model:    "claude-sonnet-4-20250514",
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
	if result.Title != "Claude Product" {
		t.Errorf("expected title 'Claude Product', got %q", result.Title)
	}
	if len(result.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(result.Tags))
	}
}

func TestProxy_Anthropic_TestConnection_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/messages" {
			t.Errorf("expected /messages, got %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": "Hi"},
			},
		})
	}))
	defer server.Close()

	proxy := NewProxy(&http.Client{})
	cfg := Config{Provider: "anthropic", APIKey: "test-key", BaseURL: server.URL, Enabled: true}
	if err := proxy.TestConnection(cfg); err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestProxy_Anthropic_AuthFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]string{"message": "invalid x-api-key"},
		})
	}))
	defer server.Close()

	proxy := NewProxy(&http.Client{})
	cfg := Config{Provider: "anthropic", APIKey: "bad-key", BaseURL: server.URL, Enabled: true}
	err := proxy.TestConnection(cfg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "authentication failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestConvertMessagesForAnthropic(t *testing.T) {
	messages := []chatMessage{
		{Role: "system", Content: "You are an assistant."},
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there!"},
	}

	system, msgs := convertMessagesForAnthropic(messages)
	if system != "You are an assistant." {
		t.Errorf("expected system prompt, got %q", system)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages (no system), got %d", len(msgs))
	}
	if msgs[0]["role"] != "user" {
		t.Errorf("expected user role, got %v", msgs[0]["role"])
	}
}

func TestConvertMessagesForAnthropic_WithImages(t *testing.T) {
	messages := []chatMessage{
		{Role: "user", Content: []interface{}{
			map[string]string{"type": "text", "text": "Describe this image"},
			map[string]interface{}{
				"type":      "image_url",
				"image_url": map[string]string{"url": "https://example.com/img.jpg", "detail": "low"},
			},
		}},
	}

	_, msgs := convertMessagesForAnthropic(messages)
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	blocks, ok := msgs[0]["content"].([]map[string]interface{})
	if !ok {
		t.Fatal("expected content to be []map[string]interface{}")
	}
	if len(blocks) != 2 {
		t.Fatalf("expected 2 content blocks, got %d", len(blocks))
	}
	if blocks[0]["type"] != "text" {
		t.Errorf("expected text block, got %v", blocks[0]["type"])
	}
	if blocks[1]["type"] != "image" {
		t.Errorf("expected image block, got %v", blocks[1]["type"])
	}
	source, ok := blocks[1]["source"].(map[string]interface{})
	if !ok {
		t.Fatal("expected source map")
	}
	if source["type"] != "url" || source["url"] != "https://example.com/img.jpg" {
		t.Errorf("unexpected image source: %v", source)
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
