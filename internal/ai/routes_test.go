package ai

import (
	"errors"
	"testing"
)

func TestPlatformProfile_ForGenerate_RoutesByModality(t *testing.T) {
	text := Config{Provider: "deepseek", APIKey: "text-key", Model: "deepseek-v4-flash", Enabled: true, IsPlatform: true}
	vision := Config{Provider: "qwen", APIKey: "vision-key", Model: "qwen3-vl-flash", Enabled: true, IsPlatform: true}
	profile := PlatformProfile{Text: &text, Vision: &vision}

	tests := []struct {
		name    string
		req     GenerateRequest
		wantKey string
	}{
		{
			name:    "text action uses text route",
			req:     GenerateRequest{Action: "improve_title", Title: "x"},
			wantKey: "text-key",
		},
		{
			name:    "image action uses vision route",
			req:     GenerateRequest{Action: "generate_from_images", Images: []string{"https://example.com/a.jpg"}},
			wantKey: "vision-key",
		},
		{
			name:    "images force vision even with custom action",
			req:     GenerateRequest{Action: "custom", Images: []string{"https://example.com/a.jpg"}},
			wantKey: "vision-key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := profile.ForGenerate(Config{}, tt.req)
			if err != nil {
				t.Fatalf("ForGenerate: %v", err)
			}
			if cfg.APIKey != tt.wantKey {
				t.Fatalf("expected key %q, got %q", tt.wantKey, cfg.APIKey)
			}
		})
	}
}

func TestPlatformProfile_ForGenerate_VisionMissing_ReturnsClearError(t *testing.T) {
	text := Config{Provider: "deepseek", APIKey: "text-key", Model: "deepseek-v4-flash", Enabled: true, IsPlatform: true}
	profile := PlatformProfile{Text: &text}

	_, err := profile.ForGenerate(Config{}, GenerateRequest{
		Action: "generate_from_images",
		Images: []string{"https://example.com/a.jpg"},
	})
	if !errors.Is(err, ErrVisionNotConfigured) {
		t.Fatalf("expected ErrVisionNotConfigured, got %v", err)
	}
}

func TestPlatformProfile_ForGenerate_PlatformVisionMustSupportImages(t *testing.T) {
	text := Config{Provider: "deepseek", APIKey: "text-key", Model: "deepseek-v4-flash", Enabled: true, IsPlatform: true}
	vision := Config{Provider: "deepseek", APIKey: "vision-key", Model: "deepseek-v4-pro", Enabled: true, IsPlatform: true}
	profile := PlatformProfile{Text: &text, Vision: &vision}

	if profile.VisionAvailable() {
		t.Fatal("expected deepseek vision route to be unavailable")
	}
	_, err := profile.ForGenerate(Config{}, GenerateRequest{
		Action: "generate_from_images",
		Images: []string{"https://example.com/a.jpg"},
	})
	if !errors.Is(err, ErrVisionNotConfigured) {
		t.Fatalf("expected ErrVisionNotConfigured, got %v", err)
	}
}

func TestPlatformProfile_ForGenerate_BYOKVisionSupport(t *testing.T) {
	profile := PlatformProfile{
		Text: &Config{Provider: "deepseek", APIKey: "text-key", Model: "deepseek-v4-flash", Enabled: true, IsPlatform: true},
	}
	req := GenerateRequest{Action: "generate_from_images", Images: []string{"https://example.com/a.jpg"}}

	_, err := profile.ForGenerate(Config{Provider: "deepseek", APIKey: "user-key", Model: "deepseek-v4-flash", Enabled: true}, req)
	if !errors.Is(err, ErrVisionUnsupported) {
		t.Fatalf("expected ErrVisionUnsupported, got %v", err)
	}

	cfg, err := profile.ForGenerate(Config{Provider: "qwen", APIKey: "user-key", Model: "qwen3-vl-flash", Enabled: true}, req)
	if err != nil {
		t.Fatalf("ForGenerate vision BYOK: %v", err)
	}
	if cfg.APIKey != "user-key" {
		t.Fatalf("expected BYOK key, got %q", cfg.APIKey)
	}
}
