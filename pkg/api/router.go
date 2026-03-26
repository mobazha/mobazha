// Package api provides a bridge that exposes internal/api's SharedRouter to
// external consumers (e.g. mobazha_hosting). This follows the same pattern as
// pkg/core/node.go which re-exports internal/core.MobazhaNode.
package api

import (
	"context"
	"net/http"

	internalapi "github.com/mobazha/mobazha3.0/internal/api"
	pkgconfig "github.com/mobazha/mobazha3.0/pkg/config"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/response"
)

// Gateway is a type alias for the internal API Gateway, following the same
// bridge pattern as pkg/core.MobazhaNode = internal/core.MobazhaNode.
type Gateway = internalapi.Gateway

// NodeResolver resolves the target NodeService from an HTTP request.
// SaaS mode: JWT → userID → EnsureNodeForUser → NodeService
// Standalone mode: header/default node → NodeService
type NodeResolver func(r *http.Request) (contracts.NodeService, error)

// RouterConfig configures the shared API router.
type RouterConfig struct {
	Resolver       NodeResolver
	FeatureManager *pkgconfig.FeatureManager
	AllowCORS      bool

	// PostResolverMiddleware is applied after the resolver has populated the
	// request context but before route handlers. Use this for scope enforcement
	// that depends on AuthIdentity set by the resolver.
	PostResolverMiddleware func(http.Handler) http.Handler
}

// Router wraps the internal SharedRouter and exposes it as an http.Handler.
type Router struct {
	inner *internalapi.SharedRouter
}

// NewRouter creates a Router containing all business API routes.
// Hosting calls this function to get a handler that can be mounted directly,
// eliminating the need for a reverse proxy.
func NewRouter(cfg RouterConfig) (*Router, error) {
	sr, err := internalapi.NewSharedRouter(internalapi.SharedRouterConfig{
		Resolver:               cfg.Resolver,
		FeatureManager:         cfg.FeatureManager,
		AllowCORS:              cfg.AllowCORS,
		PostResolverMiddleware: cfg.PostResolverMiddleware,
	})
	if err != nil {
		return nil, err
	}
	return &Router{inner: sr}, nil
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.inner.ServeHTTP(w, req)
}

// Gateway returns the underlying *Gateway for builder.go integration via
// SetSharedHTTPGateway. This ensures that MobazhaNode internal event pushes
// (NotifyWebsockets) reach the SharedRouter's WebSocket clients.
func (r *Router) Gateway() *Gateway {
	return r.inner.Gateway()
}

// NotifyWebsockets returns the WS push function for the given node.
func (r *Router) NotifyWebsockets(nodeID string) func(message interface{}) error {
	return r.inner.NotifyWebsockets(nodeID)
}

// StartMatrixChatEventBridge subscribes to Matrix chat events from the given
// MatrixChatService and forwards them to WebSocket clients identified by nodeID.
// This must be called once per node creation (not per request).
func (r *Router) StartMatrixChatEventBridge(ctx context.Context, nodeID string, svc contracts.MatrixChatService) {
	r.inner.Gateway().StartMatrixChatEventBridge(ctx, nodeID, svc)
}

// ErrorResponse writes a JSON error response. Re-exported from pkg/response
// for external consumers that need a consistent error format.
func ErrorResponse(w http.ResponseWriter, errorCode int, reason string) {
	response.Error(w, errorCode, response.HttpStatusToCode(errorCode), reason)
}
