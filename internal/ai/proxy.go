package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// GenerateRequest mirrors the frontend AiGenerateRequest.
type GenerateRequest struct {
	Action       string   `json:"action"`
	Images       []string `json:"images,omitempty"`
	Title        string   `json:"title,omitempty"`
	Description  string   `json:"description,omitempty"`
	ContractType string   `json:"contractType,omitempty"`
	Language     string   `json:"language,omitempty"`
	// generate_store / refine_store fields:
	BrandName   string          `json:"brandName,omitempty"`
	BrandDesc   string          `json:"brandDescription,omitempty"`
	StoreConfig json.RawMessage `json:"storeConfig,omitempty"`
	Instruction string          `json:"instruction,omitempty"`
}

// GenerateResponse mirrors the frontend AiGenerateResponse.
type GenerateResponse struct {
	Title            string          `json:"title,omitempty"`
	Description      string          `json:"description,omitempty"`
	Tags             []string        `json:"tags,omitempty"`
	Categories       []string        `json:"categories,omitempty"`
	ShortDescription string          `json:"shortDescription,omitempty"`
	StoreConfig      json.RawMessage `json:"storeConfig,omitempty"`
}

// Proxy handles proxying AI requests to an OpenAI-compatible API.
type Proxy struct {
	client *http.Client
}

func NewProxy(client *http.Client) *Proxy {
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	return &Proxy{client: client}
}

// TestConnection sends a minimal request to verify the AI provider is reachable
// and the API key is valid.
func (p *Proxy) TestConnection(cfg Config) error {
	if cfg.APIKey == "" {
		return fmt.Errorf("API key is required")
	}

	baseURL := cfg.EffectiveBaseURL()
	if baseURL == "" {
		return fmt.Errorf("base URL is required")
	}

	body := map[string]interface{}{
		"model":      cfg.EffectiveModel(),
		"messages":   []map[string]string{{"role": "user", "content": "Hi"}},
		"max_tokens": 1,
	}
	payload, _ := json.Marshal(body)

	url := strings.TrimSuffix(baseURL, "/") + "/chat/completions"
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	client := p.client
	if client.Timeout > 15*time.Second {
		client = &http.Client{Timeout: 15 * time.Second}
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("authentication failed: invalid API key")
	}
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		var errObj struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
			Message string `json:"message"`
		}
		if json.Unmarshal(respBody, &errObj) == nil {
			if errObj.Error.Message != "" {
				return fmt.Errorf("%s", errObj.Error.Message)
			}
			if errObj.Message != "" {
				return fmt.Errorf("%s", errObj.Message)
			}
		}
		return fmt.Errorf("provider returned status %d", resp.StatusCode)
	}

	return nil
}

func isStoreAction(action string) bool {
	return action == "generate_store" || action == "refine_store"
}

func (p *Proxy) Generate(cfg Config, req GenerateRequest) (*GenerateResponse, error) {
	if !cfg.IsValid() {
		return nil, fmt.Errorf("AI is not configured")
	}

	messages, err := buildPrompt(req)
	if err != nil {
		return nil, err
	}

	maxTokens := 1024
	if isStoreAction(req.Action) {
		maxTokens = 4096
	}

	body := map[string]interface{}{
		"model":       cfg.EffectiveModel(),
		"messages":    messages,
		"temperature": 0.7,
		"max_tokens":  maxTokens,
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := strings.TrimSuffix(cfg.EffectiveBaseURL(), "/") + "/chat/completions"
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("AI upstream request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		detail := fmt.Sprintf("%s: %d", cfg.EffectiveModel(), resp.StatusCode)
		var errObj struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
			Message string `json:"message"`
		}
		if json.Unmarshal(respBody, &errObj) == nil {
			if errObj.Error.Message != "" {
				detail = errObj.Error.Message
			} else if errObj.Message != "" {
				detail = errObj.Message
			}
		}
		return nil, fmt.Errorf("AI upstream error: %s", detail)
	}

	var apiResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("parse AI response: %w", err)
	}

	if len(apiResp.Choices) == 0 || apiResp.Choices[0].Message.Content == "" {
		return nil, fmt.Errorf("empty AI response")
	}

	content := extractJSON(apiResp.Choices[0].Message.Content)
	var result GenerateResponse

	if isStoreAction(req.Action) {
		if !json.Valid([]byte(content)) {
			return nil, fmt.Errorf("invalid AI response: not valid JSON")
		}
		result.StoreConfig = json.RawMessage(content)
	} else {
		if err := json.Unmarshal([]byte(content), &result); err != nil {
			return nil, fmt.Errorf("invalid AI response format: %w", err)
		}
	}
	return &result, nil
}

var fencedJSONRegexp = regexp.MustCompile("(?s)```(?:json)?\\s*(.*?)```")

func validateImageURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid image URL: %w", err)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("image URL must use http or https scheme, got %q", scheme)
	}
	host := strings.ToLower(u.Hostname())
	if host == "localhost" || host == "127.0.0.1" || host == "::1" || host == "0.0.0.0" ||
		strings.HasPrefix(host, "10.") || strings.HasPrefix(host, "192.168.") ||
		strings.HasPrefix(host, "169.254.") {
		return fmt.Errorf("image URL must not point to private/local addresses")
	}
	// Check 172.16.0.0/12
	if strings.HasPrefix(host, "172.") {
		parts := strings.SplitN(host, ".", 4)
		if len(parts) >= 2 {
			if n, err := strconv.Atoi(parts[1]); err == nil && n >= 16 && n <= 31 {
				return fmt.Errorf("image URL must not point to private addresses")
			}
		}
	}
	return nil
}

func extractJSON(text string) string {
	m := fencedJSONRegexp.FindStringSubmatch(text)
	if m != nil {
		return strings.TrimSpace(m[1])
	}
	return strings.TrimSpace(text)
}

const systemPrompt = "You are an expert e-commerce product listing assistant. You help sellers create compelling, professional product listings. Always respond in valid JSON format. Do NOT wrap your response in markdown code fences."

const storeBuilderSystemPrompt = `You are an expert e-commerce store designer. Create a complete store configuration as valid JSON.

## StoreConfig Schema
{
  "version": 1,
  "status": "published",
  "theme": {
    "palette": one of "minimal"|"ocean"|"forest"|"sunset"|"midnight"|"earth"|"lavender"|"rose"|"custom",
    "primaryColor": "#hex6",
    "secondaryColor": "#hex6",
    "accentColor": "#hex6",
    "fontFamily": one of "inter"|"dm-sans"|"space-grotesk"|"playfair"|"lora"|"merriweather"|"josefin-sans"|"poppins",
    "borderRadius": one of "none"|"sm"|"md"|"lg"|"full",
    "headerStyle": one of "minimal"|"classic"|"hero"
  },
  "sections": [/* 4-8 sections */]
}

## Available Section Types
Each section: { "id": "unique-string", "type": "...", "props": {...}, "visible": true }

- hero: { "title": str, "subtitle": str?, "ctaText": str?, "height": "sm"|"md"|"lg", "textAlign": "left"|"center"|"right", "overlayOpacity": 0-1 }
- featured-products: { "title": str, "mode": "newest"|"popular"|"manual", "count": 3|4, "columns": 2|3|4 }
- product-grid: { "title": str?, "showFilters": bool, "showSearch": bool, "columns": 2|3|4, "sortDefault": "newest"|"price-asc"|"price-desc"|"name" }
- announcement-bar: { "text": str, "dismissible": bool }
- trust-badges: { "badges": [{"icon": "escrow"|"crypto"|"selfHosted"|"p2p"|"privacy", "title": str, "description": str}], "layout": "horizontal"|"grid", "style": "minimal"|"card"|"illustrated" }
- about: { "title": str, "text": str, "imagePosition": "left"|"right", "showContactInfo": bool }
- testimonials: { "title": str, "mode": "latest"|"manual", "count": 3|4 }
- faq: { "title": str, "items": [{"question": str, "answer": str}] }
- store-tabs: { "tabs": ["reviews","following","followers"] }

## Rules
1. ALWAYS include exactly one store-tabs section as the LAST section
2. Include 4-8 sections total (including store-tabs)
3. Choose theme colors, fonts, and sections that match the brand description
4. Each section id must be unique (use brand-related prefix like "brandname-hero")
5. trust-badges should include 3-5 badges relevant to the store
6. FAQ should have 2-4 relevant questions
7. Return ONLY valid JSON, no markdown fences or explanations`

type chatMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

func buildPrompt(req GenerateRequest) ([]chatMessage, error) {
	lang := req.Language
	if lang == "" {
		lang = "en"
	}
	langInstruction := fmt.Sprintf("Respond in %s.", lang)
	if lang == "zh" {
		langInstruction = "Respond in Chinese (中文)."
	}

	contractType := req.ContractType
	if contractType == "" {
		contractType = "PHYSICAL_GOOD"
	}

	switch req.Action {
	case "generate_from_images":
		content := []interface{}{
			map[string]string{
				"type": "text",
				"text": fmt.Sprintf(`Analyze the product image(s) and generate a complete product listing. %s

Return JSON with these fields:
- "title": A compelling product title (max 140 chars)
- "shortDescription": A brief summary (max 200 chars)
- "description": A detailed HTML description with features and benefits (use <p>, <ul>, <li> tags)
- "tags": An array of 5-8 relevant search tags (lowercase, hyphenated)
- "categories": An array of 1-3 product categories

Product type: %s

Return ONLY valid JSON, no markdown fences.`, langInstruction, contractType),
			},
		}
		maxImages := 4
		if len(req.Images) < maxImages {
			maxImages = len(req.Images)
		}
		for _, imgURL := range req.Images[:maxImages] {
			if err := validateImageURL(imgURL); err != nil {
				return nil, err
			}
			content = append(content, map[string]interface{}{
				"type":      "image_url",
				"image_url": map[string]string{"url": imgURL, "detail": "low"},
			})
		}
		return []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: content},
		}, nil

	case "improve_title":
		descCtx := ""
		if req.Description != "" {
			d := req.Description
			if len(d) > 300 {
				d = d[:300]
			}
			descCtx = fmt.Sprintf("\nProduct description context: \"%s\"", d)
		}
		return []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: fmt.Sprintf(`Improve this product title to be more compelling and SEO-friendly. %s

Current title: "%s"%s

Return JSON: { "title": "improved title (max 140 chars)" }
Return ONLY valid JSON, no markdown fences.`, langInstruction, req.Title, descCtx)},
		}, nil

	case "polish_description":
		return []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: fmt.Sprintf(`Polish and enhance this product description to be more professional and persuasive. %s

Product title: "%s"
Current description: "%s"

Return JSON: { "description": "polished HTML description using <p>, <ul>, <li> tags", "shortDescription": "brief summary (max 200 chars)" }
Return ONLY valid JSON, no markdown fences.`, langInstruction, req.Title, req.Description)},
		}, nil

	case "suggest_tags":
		descCtx := ""
		if req.Description != "" {
			d := req.Description
			if len(d) > 500 {
				d = d[:500]
			}
			descCtx = fmt.Sprintf("\nDescription: \"%s\"", d)
		}
		return []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: fmt.Sprintf(`Suggest relevant search tags and categories for this product. %s

Product title: "%s"%s

Return JSON: { "tags": ["tag1", "tag2", ...], "categories": ["category1", ...] }
Tags should be lowercase, hyphenated, 5-10 items. Categories 1-3 items.
Return ONLY valid JSON, no markdown fences.`, langInstruction, req.Title, descCtx)},
		}, nil

	case "generate_store":
		if req.BrandName == "" {
			return nil, fmt.Errorf("brandName is required for generate_store")
		}
		return []chatMessage{
			{Role: "system", Content: storeBuilderSystemPrompt},
			{Role: "user", Content: fmt.Sprintf(`Create a store design for this brand. %s

Brand name: "%s"
Brand description: "%s"

Return ONLY valid JSON matching the StoreConfig schema above.`, langInstruction, req.BrandName, req.BrandDesc)},
		}, nil

	case "refine_store":
		if len(req.StoreConfig) == 0 {
			return nil, fmt.Errorf("storeConfig is required for refine_store")
		}
		if req.Instruction == "" {
			return nil, fmt.Errorf("instruction is required for refine_store")
		}
		return []chatMessage{
			{Role: "system", Content: storeBuilderSystemPrompt},
			{Role: "user", Content: fmt.Sprintf(`Modify this store config according to the instruction. %s

Current config:
%s

Instruction: "%s"

Return the COMPLETE updated StoreConfig as valid JSON.`, langInstruction, string(req.StoreConfig), req.Instruction)},
		}, nil

	default:
		return nil, fmt.Errorf("unknown action: %s", req.Action)
	}
}
