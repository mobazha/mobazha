package api

import (
	"net/http"
	"testing"

	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/distribution"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSharedRouter_LocalAIPolicyComposesOnlyBaseAISurface(t *testing.T) {
	router, err := NewSharedRouter(SharedRouterConfig{
		Resolver: func(*http.Request) (contracts.NodeService, error) { return nil, nil },
		AIHTTPPolicy: distribution.NewAIHTTPPolicy(
			true,
			false,
			false,
			false,
		),
	})
	require.NoError(t, err)

	routes, err := router.RegisteredRoutes()
	require.NoError(t, err)

	assert.True(t, hasRegisteredRoute(routes, http.MethodGet, "/v1/ai/status"))
	assert.True(t, hasRegisteredRoute(routes, http.MethodPost, "/v1/ai/generate"))
	assert.False(t, hasRegisteredRoute(routes, http.MethodPost, "/v1/agent/chat"))
	assert.False(t, hasRegisteredRoute(routes, http.MethodGet, "/v1/agent/chat/sessions"))
}

func TestNewSharedRouter_DisabledAIPolicyOmitsAISurface(t *testing.T) {
	router, err := NewSharedRouter(SharedRouterConfig{
		Resolver: func(*http.Request) (contracts.NodeService, error) { return nil, nil },
		AIHTTPPolicy: distribution.NewAIHTTPPolicy(
			false,
			false,
			false,
			false,
		),
	})
	require.NoError(t, err)

	routes, err := router.RegisteredRoutes()
	require.NoError(t, err)

	assert.False(t, hasRegisteredRoute(routes, http.MethodGet, "/v1/ai/status"))
	assert.False(t, hasRegisteredRoute(routes, http.MethodPost, "/v1/ai/generate"))
}

func hasRegisteredRoute(routes []RegisteredRoute, method, path string) bool {
	for _, route := range routes {
		if route.Method == method && route.Path == path {
			return true
		}
	}
	return false
}
