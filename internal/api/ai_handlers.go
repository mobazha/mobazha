//go:build !private_distribution

package api

import (
	"encoding/json"
	"errors"
	"net/http"

	aipkg "github.com/mobazha/mobazha3.0/internal/ai"
	"github.com/mobazha/mobazha3.0/pkg/logging"
	responsePkg "github.com/mobazha/mobazha3.0/pkg/response"
)

var aiLog = logging.MustGetLogger("AI")

type aiConfigProvider interface {
	AIConfig() aipkg.Config
	AIConfigForGenerate(aipkg.GenerateRequest) (aipkg.Config, error)
	AIConfigForChat([]aipkg.ChatMsg) (aipkg.Config, error)
	AIMultiConfig() aipkg.MultiConfig
	SaveAIMultiConfig(aipkg.MultiConfig) error
	AIProxy() *aipkg.Proxy
	AIRateLimiter() *aipkg.DailyRateLimiter
	PlatformAIConfig() *aipkg.Config
	PlatformAIProfile() aipkg.PlatformProfile
}

func getAIProvider(r *http.Request) aiConfigProvider {
	node := getNodeService(r)
	if p, ok := node.(aiConfigProvider); ok {
		return p
	}
	return nil
}

func (g *Gateway) handleGETAIConfig(w http.ResponseWriter, r *http.Request) {
	p := getAIProvider(r)
	if p == nil {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "AI not available in this mode")
		return
	}

	mc := p.AIMultiConfig()
	resp := map[string]interface{}{
		"enabled":         mc.Enabled,
		"active_provider": mc.ActiveProvider,
		"providers":       mc.ProviderSummary(),
	}
	responsePkg.Success(w, resp)
}

func (g *Gateway) handlePUTAIConfig(w http.ResponseWriter, r *http.Request) {
	p := getAIProvider(r)
	if p == nil {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "AI not available in this mode")
		return
	}

	var input struct {
		Provider string `json:"provider"`
		APIKey   string `json:"api_key"`
		Model    string `json:"model"`
		BaseURL  string `json:"base_url"`
		Enabled  bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "Invalid request body")
		return
	}

	mc := p.AIMultiConfig()
	mc.Enabled = input.Enabled

	if input.Provider != "" {
		existing, hasExisting := mc.Providers[input.Provider]

		cred := aipkg.ProviderCredential{
			Model:   input.Model,
			BaseURL: input.BaseURL,
		}
		if input.APIKey != "" {
			cred.APIKey = input.APIKey
		} else if hasExisting {
			cred.APIKey = existing.APIKey
		}

		mc.SetProvider(input.Provider, cred)
		mc.ActiveProvider = input.Provider
	}

	if err := p.SaveAIMultiConfig(mc); err != nil {
		aiLog.Errorf("Failed to save AI config: %s", err)
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "Failed to save AI config")
		return
	}

	resp := map[string]interface{}{
		"enabled":         mc.Enabled,
		"active_provider": mc.ActiveProvider,
		"providers":       mc.ProviderSummary(),
	}
	responsePkg.Success(w, resp)
}

func (g *Gateway) handlePOSTAITestConnection(w http.ResponseWriter, r *http.Request) {
	p := getAIProvider(r)
	if p == nil {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "AI not available in this mode")
		return
	}

	var input struct {
		Provider string `json:"provider"`
		APIKey   string `json:"api_key"`
		Model    string `json:"model"`
		BaseURL  string `json:"base_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "Invalid request body")
		return
	}

	apiKey := input.APIKey
	if apiKey == "" {
		mc := p.AIMultiConfig()
		if cred, ok := mc.Providers[input.Provider]; ok {
			apiKey = cred.APIKey
		}
	}
	if apiKey == "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "API key is required")
		return
	}

	cfg := aipkg.Config{
		Provider: input.Provider,
		APIKey:   apiKey,
		Model:    input.Model,
		BaseURL:  input.BaseURL,
		Enabled:  true,
	}

	proxy := p.AIProxy()
	if proxy == nil {
		aiLog.Errorf("AI proxy not initialized")
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "AI proxy not initialized")
		return
	}

	err := proxy.TestConnection(cfg)
	if err != nil {
		aiLog.Warningf("AI connection test failed: %v", err)
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "AI connection test failed")
		return
	}
	responsePkg.Success(w, map[string]interface{}{"success": true})
}

func (g *Gateway) handleGETAIProviders(w http.ResponseWriter, r *http.Request) {
	responsePkg.Success(w, aipkg.SupportedProviders())
}

func (g *Gateway) handlePOSTAIGenerate(w http.ResponseWriter, r *http.Request) {
	p := getAIProvider(r)
	if p == nil {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "AI not available in this mode")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB

	var req aipkg.GenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		aiLog.Errorf("Invalid AI generate request body: %s", err)
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "Invalid request body")
		return
	}
	if req.Action == "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "Missing action field")
		return
	}

	origin := publicRequestOrigin(r)
	allowLoopbackGateway := allowLoopbackGatewayForRequest(r)
	req.Images = aipkg.ResolveImageURLs(req.Images, origin)

	cfg, err := p.AIConfigForGenerate(req)
	if err != nil {
		switch {
		case errors.Is(err, aipkg.ErrVisionUnsupported):
			responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest,
				"Configured AI provider does not support image input. Configure a vision-capable provider for image generation.")
		case errors.Is(err, aipkg.ErrVisionNotConfigured):
			responsePkg.Error(w, http.StatusServiceUnavailable, responsePkg.CodeServiceUnavail,
				"AI vision model is not configured. Please configure a vision-capable provider.")
		default:
			aiLog.Warningf("AI config resolution failed: %v", err)
			responsePkg.Error(w, http.StatusServiceUnavailable, responsePkg.CodeServiceUnavail, "AI is not configured")
		}
		return
	}
	if !cfg.IsValid() {
		responsePkg.Error(w, http.StatusServiceUnavailable, responsePkg.CodeServiceUnavail, "AI is not configured. Please set up your AI provider in Settings > Integrations.")
		return
	}

	if cfg.IsPlatform {
		nodeID := getIdentityService(r).GetNodeID()
		if rl := p.AIRateLimiter(); rl != nil {
			if ok, _ := rl.Allow(nodeID, cfg.DailyLimit); !ok {
				responsePkg.Error(w, http.StatusTooManyRequests, "RATE_LIMITED",
					"Daily AI limit reached. Configure your own API key in Settings > Integrations for unlimited usage.")
				return
			}
		}
	}

	proxy := p.AIProxy()
	if proxy == nil {
		aiLog.Errorf("AI proxy not initialized")
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "AI proxy not initialized")
		return
	}

	if aipkg.GenerateNeedsVision(req) {
		inlined, inlineErr := aipkg.InlineImageURLs(r.Context(), proxy.HTTPClient(), origin, allowLoopbackGateway, req.Images)
		if inlineErr != nil {
			aiLog.Warningf("AI image inline failed: %v", inlineErr)
			responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest,
				"Failed to load product images for AI analysis")
			return
		}
		req.Images = inlined
	}

	result, err := proxy.Generate(cfg, req)
	if err != nil {
		aiLog.Warningf("AI generate failed: %v", err)
		responsePkg.Error(w, http.StatusBadGateway, responsePkg.HttpStatusToCode(http.StatusBadGateway), "AI generation failed")
		return
	}

	if cfg.IsPlatform {
		if rl := p.AIRateLimiter(); rl != nil {
			rl.Increment(getIdentityService(r).GetNodeID())
		}
	}

	responsePkg.Success(w, result)
}

func (g *Gateway) handleGETAIStatus(w http.ResponseWriter, r *http.Request) {
	p := getAIProvider(r)
	if p == nil {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "AI not available in this mode")
		return
	}

	mc := p.AIMultiConfig()
	userCfg := mc.ActiveConfig()
	byokConfigured := userCfg.IsValid()
	profile := p.PlatformAIProfile()
	textAvailable := byokConfigured || profile.TextAvailable()
	visionAvailable := profile.VisionAvailable()
	if byokConfigured {
		visionAvailable = aipkg.ConfigSupportsVision(userCfg)
	}

	var source string
	var dailyLimit, dailyUsed int

	switch {
	case byokConfigured:
		source = "byok"
	case profile.TextAvailable() || profile.VisionAvailable():
		source = "platform"
		pCfg := p.PlatformAIConfig()
		if pCfg != nil {
			dailyLimit = pCfg.DailyLimit
		}
		if rl := p.AIRateLimiter(); rl != nil {
			dailyUsed = rl.Usage(getIdentityService(r).GetNodeID())
		}
	default:
		source = "none"
	}

	responsePkg.Success(w, map[string]interface{}{
		"available":        source != "none",
		"source":           source,
		"daily_limit":      dailyLimit,
		"daily_used":       dailyUsed,
		"byok_configured":  byokConfigured,
		"text_available":   textAvailable,
		"vision_available": visionAvailable,
	})
}
