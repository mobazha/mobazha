package api

import (
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/mobazha/mobazha/pkg/distribution"
	"github.com/stretchr/testify/assert"
)

func TestRegisterAIHTTPCapabilities_LocalOnlyRegistersCoreWithoutAgentWorkspace(t *testing.T) {
	gateway := &Gateway{aiHTTPPolicy: distribution.NewAIHTTPPolicy(true, false, false, false)}
	api := humachi.New(chi.NewMux(), huma.DefaultConfig("test", "test"))

	gateway.registerAIHTTPCapabilities(api)

	paths := api.OpenAPI().Paths
	assert.Contains(t, paths, "/v1/settings/ai")
	assert.Contains(t, paths, "/v1/ai/status")
	assert.Contains(t, paths, "/v1/ai/generate")
	assert.NotContains(t, paths, "/v1/agent/chat/sessions")
	assert.NotContains(t, paths, "/v1/agent/memories")
}

func TestRegisterAIHTTPCapabilities_DisabledRegistersNothing(t *testing.T) {
	gateway := &Gateway{aiHTTPPolicy: distribution.NewAIHTTPPolicy(false, false, false, false)}
	api := humachi.New(chi.NewMux(), huma.DefaultConfig("test", "test"))

	gateway.registerAIHTTPCapabilities(api)

	paths := api.OpenAPI().Paths
	assert.NotContains(t, paths, "/v1/settings/ai")
	assert.NotContains(t, paths, "/v1/ai/status")
	assert.NotContains(t, paths, "/v1/ai/generate")
}
