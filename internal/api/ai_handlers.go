package api

import (
	"encoding/json"
	"net/http"

	aipkg "github.com/mobazha/mobazha3.0/internal/ai"
	"github.com/mobazha/mobazha3.0/pkg/logging"
	responsePkg "github.com/mobazha/mobazha3.0/pkg/response"
)

var aiLog = logging.MustGetLogger("AI")

type aiConfigProvider interface {
	AIConfig() aipkg.Config
	AIMultiConfig() aipkg.MultiConfig
	SaveAIMultiConfig(aipkg.MultiConfig) error
	AIProxy() *aipkg.Proxy
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
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, err.Error())
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

	cfg := p.AIConfig()
	if !cfg.IsValid() {
		responsePkg.Error(w, http.StatusServiceUnavailable, responsePkg.CodeServiceUnavail, "AI is not configured. Please set up your AI provider in Settings > Integrations.")
		return
	}

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

	proxy := p.AIProxy()
	if proxy == nil {
		aiLog.Errorf("AI proxy not initialized")
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "AI proxy not initialized")
		return
	}

	result, err := proxy.Generate(cfg, req)
	if err != nil {
		aiLog.Errorf("AI generate failed: %s", err)
		responsePkg.Error(w, http.StatusBadGateway, responsePkg.HttpStatusToCode(http.StatusBadGateway), err.Error())
		return
	}

	responsePkg.Success(w, result)
}
