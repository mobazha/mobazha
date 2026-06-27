package ai

import (
	"errors"
	"strings"
)

var (
	// ErrVisionNotConfigured indicates that a vision-capable platform route is missing.
	ErrVisionNotConfigured = errors.New("AI vision model is not configured")
	// ErrVisionUnsupported indicates that the active BYOK config cannot handle image input.
	ErrVisionUnsupported = errors.New("configured AI provider does not support image input")
)

// PlatformProfile contains platform-provided text and vision model routes.
type PlatformProfile struct {
	Text   *Config
	Vision *Config
}

// TextAvailable reports whether the platform text route can be used.
func (p PlatformProfile) TextAvailable() bool {
	return p.Text != nil && p.Text.IsValid()
}

// VisionAvailable reports whether the platform vision route can be used.
func (p PlatformProfile) VisionAvailable() bool {
	return p.Vision != nil && p.Vision.IsValid() && ConfigSupportsVision(*p.Vision)
}

// ForGenerate resolves the best config for a generation request.
func (p PlatformProfile) ForGenerate(userCfg Config, req GenerateRequest) (Config, error) {
	if GenerateNeedsVision(req) {
		if userCfg.IsValid() {
			if ConfigSupportsVision(userCfg) {
				return userCfg, nil
			}
			return Config{}, ErrVisionUnsupported
		}
		if p.VisionAvailable() {
			return *p.Vision, nil
		}
		return Config{}, ErrVisionNotConfigured
	}
	return p.ForChat(userCfg, nil)
}

// ForChat resolves the best config for chat messages.
func (p PlatformProfile) ForChat(userCfg Config, messages []ChatMsg) (Config, error) {
	if ChatNeedsVision(messages) {
		if userCfg.IsValid() {
			if ConfigSupportsVision(userCfg) {
				return userCfg, nil
			}
			return Config{}, ErrVisionUnsupported
		}
		if p.VisionAvailable() {
			return *p.Vision, nil
		}
		return Config{}, ErrVisionNotConfigured
	}
	if userCfg.IsValid() {
		return userCfg, nil
	}
	if p.TextAvailable() {
		return *p.Text, nil
	}
	return userCfg, nil
}

// GenerateNeedsVision reports whether a generation request includes image input.
func GenerateNeedsVision(req GenerateRequest) bool {
	return req.Action == "generate_from_images" || len(req.Images) > 0
}

// ChatNeedsVision reports whether chat messages contain image input.
func ChatNeedsVision(messages []ChatMsg) bool {
	for _, msg := range messages {
		if chatContentNeedsVision(msg.Content) {
			return true
		}
		for _, block := range msg.ContentBlocks {
			if block.Type == "image_url" && block.ImageURL != nil && strings.TrimSpace(block.ImageURL.URL) != "" {
				return true
			}
		}
	}
	return false
}

func chatContentNeedsVision(content string) bool {
	return strings.Contains(content, "image_url")
}

// ConfigSupportsVision reports whether a config is known or likely to support image input.
func ConfigSupportsVision(cfg Config) bool {
	provider := strings.ToLower(cfg.Provider)
	model := strings.ToLower(cfg.EffectiveModel())
	switch provider {
	case "deepseek":
		return false
	case "qwen":
		return strings.Contains(model, "vl") || strings.Contains(model, "vision")
	case "openai":
		return strings.Contains(model, "gpt-4o") ||
			strings.Contains(model, "gpt-4.1") ||
			strings.Contains(model, "gpt-5") ||
			strings.Contains(model, "vision")
	case "anthropic", "gemini", "custom":
		return true
	default:
		return strings.Contains(model, "vl") || strings.Contains(model, "vision")
	}
}
