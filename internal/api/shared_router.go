package api

import (
	"context"
	"net/http"
	"sync"

	"github.com/go-chi/chi/v5"
	agentskill "github.com/mobazha/mobazha3.0/pkg/agent/skill"
	"github.com/mobazha/mobazha3.0/pkg/config"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/response"
)

// SharedRouterConfig configures a SharedRouter.
type SharedRouterConfig struct {
	Resolver       func(r *http.Request) (contracts.NodeService, error)
	FeatureManager *config.FeatureManager
	SkillProvider  agentskill.Provider
	AllowCORS      bool

	// PostResolverMiddleware, if set, is applied after the resolver middleware
	// has populated the request context (NodeService + AuthIdentity) but before
	// the actual route handler. This is the correct place for scope enforcement
	// in SaaS mode because the resolver sets AuthIdentity in the context.
	PostResolverMiddleware func(http.Handler) http.Handler
}

// NewSharedRouter creates an HTTP router that can be used by both hosting (SaaS)
// and standalone deployments. Handlers remain as Gateway methods; the resolver
// middleware injects the NodeService into the request context so that
// getNodeService(r) works without any handler changes.
func NewSharedRouter(cfg SharedRouterConfig) (*SharedRouter, error) {
	g := &Gateway{
		config:  &GatewayConfig{SkillProvider: cfg.SkillProvider},
		hubs:    make(map[string]*hub),
		hubsMtx: sync.RWMutex{},
	}
	if cfg.FeatureManager != nil {
		g.featureManager = cfg.FeatureManager
	}

	r := chi.NewMux()
	r.Use(maxBodySizeMiddleware(defaultMaxBodySize))

	if cfg.AllowCORS {
		r.Use(g.CORSAllowAllOriginsMiddleware)
	}

	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			node, err := cfg.Resolver(req)
			if err != nil {
				log.Warningf("Node resolver failed: %v", err)
				response.Error(w, http.StatusUnauthorized, response.CodeUnauthorized, "Authentication required")
				return
			}
			ctx := context.WithValue(req.Context(), nodeContextKey, node)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})

	if cfg.PostResolverMiddleware != nil {
		r.Use(cfg.PostResolverMiddleware)
	}

	// StorefrontMiddleware parses X-Storefront-* headers injected by the
	// hosting Gateway (MS-Phase-2a · MS2a.2c). No-op when headers are
	// absent. Mounted after node resolution so downstream handlers can
	// read both node and storefront context.
	r.Use(g.StorefrontMiddleware)

	g.registerPreHumaRoutes(r)
	g.registerHumaAPI(r)

	r.HandleFunc("/ws/{nodeID}", g.WebsocketNodeHandler())
	r.HandleFunc("/ws", g.WebsocketDefaultHandler())

	return &SharedRouter{handler: r, gateway: g}, nil
}

// SharedRouter wraps the chi.Router and the underlying Gateway instance
// (needed for WebSocket hub access and event push).
type SharedRouter struct {
	handler http.Handler
	gateway *Gateway
}

func (sr *SharedRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	sr.handler.ServeHTTP(w, r)
}

// Gateway returns the underlying *Gateway so that builder.go can register it
// via SetHTTPGateway, ensuring MobazhaNode WS pushes reach the SharedRouter's clients.
func (sr *SharedRouter) Gateway() *Gateway {
	return sr.gateway
}

// NotifyWebsockets returns the WS push function for the given node.
func (sr *SharedRouter) NotifyWebsockets(nodeID string) func(message interface{}) error {
	return sr.gateway.NotifyWebsockets(nodeID)
}
