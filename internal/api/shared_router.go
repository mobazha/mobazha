package api

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"
	agentskill "github.com/mobazha/mobazha/pkg/agent/skill"
	"github.com/mobazha/mobazha/pkg/config"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/distribution"
	"github.com/mobazha/mobazha/pkg/edition"
	"github.com/mobazha/mobazha/pkg/response"
)

// SharedRouterConfig configures a SharedRouter.
type SharedRouterConfig struct {
	Resolver                      func(r *http.Request) (contracts.NodeService, error)
	FeatureManager                *config.FeatureManager
	SkillProvider                 agentskill.Provider
	AllowCORS                     bool
	DistributionPolicy            edition.Policy
	AIHTTPPolicy                  distribution.AIHTTPPolicy
	RuntimePaymentMethodsProvider func(context.Context) []edition.PaymentMethod
	TrustedProxyCIDRs             []string

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
	trustedProxies, err := newTrustedProxySet(cfg.TrustedProxyCIDRs)
	if err != nil {
		return nil, err
	}
	distributionPolicy := cfg.DistributionPolicy
	if distributionPolicy == nil {
		distributionPolicy = edition.DefaultPolicy()
	}
	g := &Gateway{
		config: &GatewayConfig{
			SkillProvider:                 cfg.SkillProvider,
			RuntimeCapabilitiesSnapshotFn: runtimeCapabilitiesFromPaymentMethods(cfg.RuntimePaymentMethodsProvider),
		},
		hubs:           make(map[string]*hub),
		hubsMtx:        sync.RWMutex{},
		editionPolicy:  distributionPolicy,
		aiHTTPPolicy:   resolveAIHTTPPolicy(cfg.AIHTTPPolicy, distributionPolicy),
		trustedProxies: trustedProxies,
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
			if cfg.RuntimePaymentMethodsProvider != nil && isRuntimeConfigRequest(req) {
				ctx = context.WithValue(ctx, platformRuntimeCapabilitiesContextKey, true)
			}
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
	if _, err := g.registerHumaAPI(r); err != nil {
		return nil, fmt.Errorf("register shared Huma API: %w", err)
	}

	r.HandleFunc("/ws/{nodeID}", g.WebsocketNodeHandler())
	r.HandleFunc("/ws", g.WebsocketDefaultHandler())

	return &SharedRouter{handler: r, routes: r, gateway: g}, nil
}

// RegisteredRoute describes one concrete method/path pair installed in the
// effective router. It is intentionally derived from chi after all Huma,
// pre-Huma, capability, and distribution-policy registration has completed.
type RegisteredRoute struct {
	Method string `json:"method"`
	Path   string `json:"path"`
}

// SharedRouter wraps the chi.Router and the underlying Gateway instance
// (needed for WebSocket hub access and event push).
type SharedRouter struct {
	handler http.Handler
	routes  chi.Routes
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

// RegisteredRoutes returns a deterministic inventory of the routes installed
// in this router. chi.Walk reports only concrete HTTP methods, so catch-all
// handlers cannot masquerade as a valid public API operation.
func (sr *SharedRouter) RegisteredRoutes() ([]RegisteredRoute, error) {
	if sr == nil || sr.routes == nil {
		return nil, nil
	}
	routes := make([]RegisteredRoute, 0, 256)
	if err := chi.Walk(sr.routes, func(method, path string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if !strings.HasPrefix(path, "/v1/") {
			return nil
		}
		routes = append(routes, RegisteredRoute{Method: strings.ToUpper(method), Path: path})
		return nil
	}); err != nil {
		return nil, err
	}
	sort.Slice(routes, func(i, j int) bool {
		if routes[i].Path != routes[j].Path {
			return routes[i].Path < routes[j].Path
		}
		return routes[i].Method < routes[j].Method
	})
	return routes, nil
}
