package api

import (
	"context"
	"net/http"
	"sync"

	"github.com/gorilla/mux"
	"github.com/mobazha/mobazha3.0/pkg/config"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/response"
)

// SharedRouterConfig configures a SharedRouter.
type SharedRouterConfig struct {
	Resolver       func(r *http.Request) (contracts.NodeService, error)
	FeatureManager *config.FeatureManager
	AllowCORS      bool
}

// NewSharedRouter creates an HTTP router that can be used by both hosting (SaaS)
// and standalone deployments. Handlers remain as Gateway methods; the resolver
// middleware injects the NodeService into the request context so that
// getNodeService(r) works without any handler changes.
func NewSharedRouter(cfg SharedRouterConfig) (*SharedRouter, error) {
	g := &Gateway{
		config:  &GatewayConfig{},
		hubs:    make(map[string]*hub),
		hubsMtx: sync.RWMutex{},
	}
	if cfg.FeatureManager != nil {
		g.featureManager = cfg.FeatureManager
	}

	r := mux.NewRouter()
	r.Methods("OPTIONS")

	if cfg.AllowCORS {
		r.Use(g.CORSAllowAllOriginsMiddleware)
		r.Use(mux.CORSMethodMiddleware(r))
	}

	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			node, err := cfg.Resolver(req)
			if err != nil {
				response.Error(w, http.StatusUnauthorized, response.CodeUnauthorized, err.Error())
				return
			}
			ctx := context.WithValue(req.Context(), nodeContextKey, node)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})

	g.registerBusinessRoutes(r)

	r.HandleFunc("/ws/{nodeID}", g.WebsocketNodeHandler())
	r.HandleFunc("/ws", g.WebsocketDefaultHandler())

	return &SharedRouter{handler: r, gateway: g}, nil
}

// SharedRouter wraps the mux.Router and the underlying Gateway instance
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
