package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/danielgtaylor/huma/v2"
)

// registerNodeHumaAIHTTPOperations registers the distribution-neutral AI
// settings, status, and generation contract.
func (g *Gateway) registerNodeHumaAIHTTPOperations(api huma.API) {
	type jsonBody struct {
		Body json.RawMessage `json:",omitempty"`
	}

	huma.Register(api, huma.Operation{
		OperationID: "settings-ai-get",
		Method:      http.MethodGet,
		Path:        "/v1/settings/ai",
		Summary:     "Get AI integration config",
		Tags:        []string{"ai"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodGet, "/v1/settings/ai", nil)
		rr := httptest.NewRecorder()
		g.handleGETAIConfig(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "settings-ai-put",
		Method:      http.MethodPut,
		Path:        "/v1/settings/ai",
		Summary:     "Update AI integration config",
		Tags:        []string{"ai"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *jsonBody) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPut, "/v1/settings/ai", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePUTAIConfig(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "settings-ai-providers-get",
		Method:      http.MethodGet,
		Path:        "/v1/settings/ai/providers",
		Summary:     "List supported AI providers",
		Tags:        []string{"ai"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodGet, "/v1/settings/ai/providers", nil)
		rr := httptest.NewRecorder()
		g.handleGETAIProviders(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "settings-ai-test-post",
		Method:      http.MethodPost,
		Path:        "/v1/settings/ai/test",
		Summary:     "Test AI provider connection",
		Tags:        []string{"ai"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *jsonBody) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/settings/ai/test", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTAITestConnection(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "ai-status-get",
		Method:      http.MethodGet,
		Path:        "/v1/ai/status",
		Summary:     "AI availability and quota status",
		Tags:        []string{"ai"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodGet, "/v1/ai/status", nil)
		rr := httptest.NewRecorder()
		g.handleGETAIStatus(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "ai-generate-post",
		Method:      http.MethodPost,
		Path:        "/v1/ai/generate",
		Summary:     "Generate AI content",
		Tags:        []string{"ai"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *jsonBody) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/ai/generate", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTAIGenerate(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerAIHTTPCapabilities(api huma.API) {
	policy := g.activeAIHTTPPolicy()
	if !policy.AIHTTPEnabled() {
		return
	}
	g.registerNodeHumaAIHTTPOperations(api)
	if policy.AllowsAgentWorkspace() {
		g.registerDistributionHumaAgentOperations(api)
	}
}
