package api

import (
	"encoding/json"
	"errors"
	"net/http"

	aipkg "github.com/mobazha/mobazha/internal/ai"
	"github.com/mobazha/mobazha/pkg/distribution"
	"github.com/mobazha/mobazha/pkg/logging"
	responsePkg "github.com/mobazha/mobazha/pkg/response"
)

var aiLog = logging.MustGetLogger("AI")

type aiConfigProvider interface {
	AIConfig() aipkg.Config
	AIMultiConfig() aipkg.MultiConfig
	SaveAIMultiConfig(aipkg.MultiConfig) error
	AIProxy() *aipkg.Proxy
	AIRateLimiter() *aipkg.DailyRateLimiter
	PlatformAIConfig() *aipkg.Config
}

type aiPlatformConfigProvider interface {
	AIConfigForGenerate(aipkg.GenerateRequest) (aipkg.Config, error)
	PlatformAIProfile() aipkg.PlatformProfile
}

func getAIProvider(r *http.Request) aiConfigProvider {
	node := getNodeService(r)
	if p, ok := node.(aiConfigProvider); ok {
		return p
	}
	return nil
}

func aiConfigValidForPolicy(cfg aipkg.Config, policy distribution.AIHTTPPolicy) bool {
	if policy.AllowsRemoteAIEndpoints() {
		return cfg.IsValid()
	}
	return cfg.Enabled && aipkg.IsTrustedLocalLLMEndpoint(cfg.EffectiveBaseURL())
}

func localAIProviders() []aipkg.ProviderInfo {
	return []aipkg.ProviderInfo{{
		ID:             "custom",
		Label:          "Local LLM (Ollama)",
		DefaultModel:   "llama3.2",
		DefaultBaseURL: "http://localhost:11434/v1",
		Models:         []string{"llama3.2", "llama3.1", "llama4", "qwen2.5", "mistral", "deepseek-r1", "gemma2"},
		HelpURL:        "https://ollama.com/download",
	}}
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
	policy := g.activeAIHTTPPolicy()
	if !policy.AllowsRemoteAIEndpoints() {
		if input.BaseURL != "" && !aipkg.IsTrustedLocalLLMEndpoint(input.BaseURL) {
			responsePkg.Error(w, http.StatusForbidden, responsePkg.CodeForbidden,
				"This distribution only allows trusted local AI endpoints")
			return
		}
		if input.Provider != "" && input.Provider != "custom" && input.BaseURL == "" {
			responsePkg.Error(w, http.StatusForbidden, responsePkg.CodeForbidden,
				"This distribution requires an explicit local base_url for built-in providers")
			return
		}
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
	policy := g.activeAIHTTPPolicy()
	if !policy.AllowsRemoteAIEndpoints() &&
		(input.BaseURL == "" || !aipkg.IsTrustedLocalLLMEndpoint(input.BaseURL)) {
		responsePkg.Error(w, http.StatusForbidden, responsePkg.CodeForbidden,
			"This distribution requires an explicit trusted local AI base_url")
		return
	}

	apiKey := input.APIKey
	if apiKey == "" {
		mc := p.AIMultiConfig()
		if cred, ok := mc.Providers[input.Provider]; ok {
			apiKey = cred.APIKey
		}
	}
	if apiKey == "" && policy.AllowsRemoteAIEndpoints() {
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
	if !g.activeAIHTTPPolicy().AllowsRemoteAIEndpoints() {
		responsePkg.Success(w, localAIProviders())
		return
	}
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

	policy := g.activeAIHTTPPolicy()
	var cfg aipkg.Config
	var err error
	if policy.AllowsPlatformAIFallback() {
		platformProvider, ok := p.(aiPlatformConfigProvider)
		if !ok {
			responsePkg.Error(w, http.StatusServiceUnavailable, responsePkg.CodeServiceUnavail,
				"AI platform routing is not available")
			return
		}
		cfg, err = platformProvider.AIConfigForGenerate(req)
	} else if policy.AllowsRemoteAIEndpoints() {
		multiConfig := p.AIMultiConfig()
		cfg, err = (aipkg.PlatformProfile{}).ForGenerate(multiConfig.ActiveConfig(), req)
	} else {
		cfg = p.AIConfig()
		if !aiConfigValidForPolicy(cfg, policy) {
			err = errors.New("trusted local AI endpoint is not configured")
		}
	}
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
	if !aiConfigValidForPolicy(cfg, policy) {
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
	policy := g.activeAIHTTPPolicy()
	byokConfigured := aiConfigValidForPolicy(userCfg, policy)
	profile := aipkg.PlatformProfile{}
	if policy.AllowsPlatformAIFallback() {
		if platformProvider, ok := p.(aiPlatformConfigProvider); ok {
			profile = platformProvider.PlatformAIProfile()
		}
	}
	textAvailable := byokConfigured || profile.TextAvailable()
	visionAvailable := profile.VisionAvailable()
	if byokConfigured {
		if policy.AllowsRemoteAIEndpoints() {
			visionAvailable = aipkg.ConfigSupportsVision(userCfg)
		} else {
			var probeHTTPClient *http.Client
			if proxy := p.AIProxy(); proxy != nil {
				probeHTTPClient = proxy.HTTPClient()
			}
			visionAvailable = aipkg.ProbeOllamaSupportsVision(
				probeHTTPClient, userCfg.EffectiveBaseURL(), userCfg.EffectiveModel())
		}
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
		"supports_vision":  visionAvailable,
	})
}
