package api

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/mobazha/mobazha3.0/internal/embedded/frontend"
	"github.com/mobazha/mobazha3.0/internal/repo"
	"github.com/mobazha/mobazha3.0/internal/ssr"
	"github.com/mobazha/mobazha3.0/pkg/apitoken"
	pkgconfig "github.com/mobazha/mobazha3.0/pkg/config"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/logging"
	mcppkg "github.com/mobazha/mobazha3.0/pkg/mcp"
	"gorm.io/gorm"
)

var log = logging.MustGetLogger("API")

type contextKey string

const nodeContextKey contextKey = "node"

// normalizeLoopbackAddr rewrites wildcard listener addresses (0.0.0.0, ::,
// empty host) to a routable loopback host for in-process MCP bridge calls.
// IPv6 listeners ([::]:port) are normalized to 127.0.0.1:port because the
// gateway also listens on IPv4 wildcard in practice and 127.0.0.1 is the
// safest universally-routable target. If the address cannot be parsed it is
// returned unchanged so misconfigurations surface in logs rather than being
// silently masked.
func normalizeLoopbackAddr(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	switch host {
	case "", "0.0.0.0", "::", "[::]":
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, port)
}

type GatewayConfig struct {
	Listener        net.Listener
	AllowAllOrigins bool
	AllowedIPs      map[string]bool
	Cookie          string
	Username        string
	Password        string
	UseSSL          bool
	SSLCert         string
	SSLKey          string
	PublicOnly      bool
	HashFile        string // path to persist password hash for runtime changes (standalone)
	PlainFile       string // first-run plaintext password file (standalone)

	// CasdoorCertificate is the PEM certificate from SaaS Casdoor.
	// When set, the standalone node accepts JWT Bearer tokens in addition
	// to Basic Auth. Requests from SaaS proxy (Mini App) carry JWTs.
	CasdoorCertificate string
	// LocalPeerID is this node's libp2p peer ID, used as legacy fallback
	// for admin authorization when OwnerUserID is not configured.
	LocalPeerID string
	// OwnerUserID is the Casdoor User ID of the store owner, from
	// store_registry.owner_user_id. When set, JWT admin authorization
	// uses claims.Id == OwnerUserID instead of Properties["peerID"].
	OwnerUserID string

	// SaaSAPIURL is the SaaS platform base URL for standalone → SaaS calls
	// (e.g. store claim). Empty in SaaS mode.
	SaaSAPIURL string
	// StandaloneAPIKey is the API key for authenticating with the SaaS platform.
	StandaloneAPIKey string
	// StandaloneConnectivity is the configured network mode for standalone
	// nodes (from CLI --standaloneconnectivity). Used as fallback when the
	// CONNECTIVITY env var is not set (native binary mode).
	StandaloneConnectivity string
	// DataDir is the node's data directory (e.g. ~/.mobazha).
	// Used by native binary mode to persist domain config when Docker
	// hostconfig is unavailable.
	DataDir string

	// GormDB is an optional GORM database for local API token storage.
	// When set (standalone mode), the gateway auto-creates the api_tokens
	// table and enables /v1/auth/tokens CRUD + mbz_ token authentication.
	GormDB *gorm.DB
}

// Gateway represents an HTTP API gateway
type Gateway struct {
	listener       net.Listener
	nodeManager    coreiface.NodeManagerIface
	handler        http.Handler
	config         *GatewayConfig
	auth           authState
	jwtValidator   *JWTValidator // nil when no Casdoor cert configured
	tokenStore     *apitoken.Store // nil in SaaS mode; set for standalone
	hubs           map[string]*hub
	hubsMtx        sync.RWMutex
	shutdown       chan struct{}
	closeOnce      sync.Once
	mu             sync.RWMutex
	featureManager     *pkgconfig.FeatureManager
	guestOrderLimiter  *rateLimiter
}

// NewGateway instantiates a new gateway.
func NewGateway(nodeManager coreiface.NodeManagerIface, config *GatewayConfig) (*Gateway, error) {
	var (
		g = &Gateway{
			nodeManager:    nodeManager,
			config:         config,
			listener:       config.Listener,
			shutdown:       make(chan struct{}),
			hubs:           make(map[string]*hub),
			hubsMtx:        sync.RWMutex{},
			featureManager:    pkgconfig.GetGlobalFeatureManager(),
			guestOrderLimiter: newRateLimiter(10, time.Hour),
		}
		topMux = http.NewServeMux()
	)

	g.auth = authState{
		username:     config.Username,
		passwordHash: config.Password,
		hashFile:     config.HashFile,
		plainFile:    config.PlainFile,
	}

	if config.GormDB != nil {
		if err := g.InitTokenStore(config.GormDB); err != nil {
			log.Warningf("Failed to init API token store: %v", err)
		}
	} else if config.DataDir != "" && !config.PublicOnly {
		if tokenDB, err := openTokenDB(config.DataDir); err != nil {
			log.Warningf("Failed to open token database: %v", err)
		} else if err := g.InitTokenStore(tokenDB); err != nil {
			log.Warningf("Failed to init API token store: %v", err)
		}
	}

	if config.CasdoorCertificate != "" && config.LocalPeerID != "" {
		jv, err := NewJWTValidator(config.CasdoorCertificate, config.LocalPeerID, config.OwnerUserID)
		if err != nil {
			log.Warningf("Failed to init JWT validator (JWT auth disabled): %v", err)
		} else {
			g.jwtValidator = jv
			authMode := "peerID (legacy)"
			if config.OwnerUserID != "" {
				authMode = "ownerUserID"
			}
			log.Infof("JWT authentication enabled for standalone store %s (mode: %s)", config.LocalPeerID, authMode)
		}
	}

	r := g.newV1Router()

	if config.AllowAllOrigins {
		r.Use(g.CORSAllowAllOriginsMiddleware)
	}
	r.Use(mux.CORSMethodMiddleware(r))
	// Auth is NOT applied globally — each private route is individually wrapped
	// with AuthenticationMiddleware inside registerBusinessRoutes. Public
	// storefront routes (listings, profiles, exchange rates, etc.) are served
	// without auth so that buyers can read store data on standalone nodes.
	r.Use(g.NodeSelectionMiddleware)
	// StorefrontMiddleware parses X-Storefront-* headers injected by the
	// hosting Gateway (MS-Phase-2a · MS2a.2c) into a StorefrontContext on
	// the request. It's a no-op when the headers are absent (main host or
	// internal API traffic), so adding it globally is managed_escrow.
	r.Use(g.StorefrontMiddleware)

	// Create default hub
	defaultNodeID := repo.DefaultNodeID
	defaultHub := newHub(defaultNodeID)
	g.hubs[defaultNodeID] = defaultHub
	go defaultHub.run()

	r.Handle("/ws/{nodeID}", g.AuthenticationMiddleware(g.WebsocketNodeHandler()))
	r.Handle("/ws", g.AuthenticationMiddleware(newWebsocketHandler(g.hubs[defaultNodeID])))

	topMux.Handle("/v1/", r)
	topMux.Handle("/ws/", r)
	topMux.Handle("/ws", r)

	topMux.HandleFunc("/healthz", g.handleHealthz)

	// MCP Streamable HTTP — registered on topMux so it takes priority
	// over the /v1/ catch-all. MCP tools call /v1/* Node business APIs
	// via a loopback Bridge. Auth is per-request: the BridgeFactory
	// extracts the Bearer token from each CallToolRequest and forwards
	// it to the local gateway for authentication.
	if !config.PublicOnly {
		scheme := "http"
		if config.UseSSL {
			scheme = "https"
		}
		loopbackURL := fmt.Sprintf("%s://%s", scheme, normalizeLoopbackAddr(config.Listener.Addr().String()))
		mcpOpts := &mcppkg.ServerOptions{
			Transport: "streamable-http",
			// Standalone identity endpoint. Required (with AuditLogger) for
			// SSEIdentityFunc to resolve UserID/PeerID from request headers.
			IdentityPath: "/v1/auth/identity",
			// Stdout audit logger gives standalone deployments visibility into
			// MCP tool calls without requiring a database table. SaaS hosting
			// uses DBAuditLogger; this is the standalone equivalent.
			AuditLogger: mcppkg.NewStdoutAuditLogger(),
		}
		mcpHTTPServer := mcppkg.NewStreamableHTTPMobazhaServer(loopbackURL, nil, mcpOpts)
		topMux.Handle("/v1/mcp", mcpHTTPServer)
		topMux.Handle("/v1/mcp/", mcpHTTPServer)
		log.Info("MCP Streamable HTTP endpoint registered at /v1/mcp")
	}

	// Standalone mode: reverse-proxy /platform/* to the SaaS backend so
	// that frontend calls to HOSTING_API (store-links, bots, domains, etc.)
	// reach the platform instead of falling through to the SPA catch-all.
	if config.SaaSAPIURL != "" {
		if saasTarget, err := url.Parse(config.SaaSAPIURL); err == nil {
			proxy := httputil.NewSingleHostReverseProxy(saasTarget)
			topMux.Handle("/platform/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				r.Host = saasTarget.Host
				if config.StandaloneAPIKey != "" {
					r.Header.Set("X-Standalone-Store-Key", config.StandaloneAPIKey)
				}
				proxy.ServeHTTP(w, r)
			}))
			log.Infof("Platform API proxy enabled: /platform/* → %s", config.SaaSAPIURL)
		}
	}

	// SSR: register meta injection + embed routes for standalone mode.
	// Activated when SPA directory exists (container deployment).
	if ssrHandler := ssr.NewFromEnv(config.LocalPeerID); ssrHandler != nil {
		ssrHandler.RegisterRoutes(topMux)
	} else if frontend.HasContent() {
		feHandler := frontend.NewHandler(frontend.ServerConfig{
			FeaturesSnapshotFn: featuresSnapshotFromNodeManager(nodeManager),
		})
		topMux.Handle("/", feHandler)
		log.Info("Serving embedded Web UI on /")
	}

	g.handler = topMux
	return g, nil
}

// getTokenStore returns the token store (nil in SaaS mode).
func (g *Gateway) getTokenStore() *apitoken.Store {
	return g.tokenStore
}

// InitTokenStore initializes the local API token store using the
// provided GORM database. Should be called during node startup for
// standalone deployments.
func (g *Gateway) InitTokenStore(db *gorm.DB) error {
	store, err := apitoken.NewStore(db)
	if err != nil {
		return fmt.Errorf("init token store: %w", err)
	}
	g.tokenStore = store
	log.Info("Local API token store initialized")
	return nil
}

// getJWTValidator returns the current jwtValidator under read-lock.
// ManagedEscrow for concurrent use with EnableJWTAuth.
func (g *Gateway) getJWTValidator() *JWTValidator {
	g.mu.RLock()
	jv := g.jwtValidator
	g.mu.RUnlock()
	return jv
}

// EnableJWTAuth initializes (or replaces) the jwtValidator at runtime.
// Called asynchronously after startup when the Casdoor certificate is
// fetched from the SaaS platform. Thread-managed_escrow.
func (g *Gateway) EnableJWTAuth(certPEM, localPeerID, ownerUserID string) error {
	jv, err := NewJWTValidator(certPEM, localPeerID, ownerUserID)
	if err != nil {
		return err
	}
	g.mu.Lock()
	g.jwtValidator = jv
	g.mu.Unlock()
	return nil
}

// Close shutsdown the Gateway listener. ManagedEscrow to call multiple times.
func (g *Gateway) Close() error {
	var err error
	g.closeOnce.Do(func() {
		close(g.shutdown)

		g.hubsMtx.Lock()
		for _, hub := range g.hubs {
			close(hub.stop)
		}
		g.hubsMtx.Unlock()

		if g.listener != nil {
			err = g.listener.Close()
		}
	})
	return err
}

// NotifyWebsockets marshals the message to JSON and broadcasts it
// to all existing websocket connections.
func (g *Gateway) NotifyWebsockets(nodeID string) func(message interface{}) error {
	return func(message interface{}) error {
		out, err := marshalAndSanitizeJSON(message)
		if err != nil {
			return err
		}

		g.hubsMtx.RLock()
		hub, exists := g.hubs[nodeID]
		g.hubsMtx.RUnlock()

		if !exists {
			return fmt.Errorf("no hub found for node %s", nodeID)
		}

		hub.Broadcast <- out
		return nil
	}
}

// Serve begins listening on the configured address.
// When the Gateway is created by SharedRouter (hosting bridge mode), there is
// no listener — hosting manages the HTTP server externally. In that case Serve
// is a no-op.
func (g *Gateway) Serve() error {
	if g.listener == nil {
		return nil
	}
	g.guestOrderLimiter.startCleanup(g.shutdown)
	log.Infof("Gateway/API server listening on %s\n", g.listener.Addr())
	var err error
	if g.config.UseSSL {
		err = http.ListenAndServeTLS(g.listener.Addr().String(), g.config.SSLCert, g.config.SSLKey, g.handler)
	} else {
		err = http.Serve(g.listener, g.handler)
	}
	return err
}

func (g *Gateway) newV1Router() *mux.Router {
	r := mux.NewRouter()
	r.Methods("OPTIONS")
	r.Use(maxBodySizeMiddleware(defaultMaxBodySize))
	g.registerBusinessRoutes(r)
	return r
}

func wrapError(err error) string {
	return fmt.Sprintf(`{"error": "%s"}`, err.Error())
}

// getNodeService extracts contracts.NodeService from the request context.
// This works for both MobazhaNode and TenantService.
// Prefer the domain-specific getters below when the handler only needs
// a single domain's methods — they return narrower interface types.
func getNodeService(r *http.Request) contracts.NodeService {
	return r.Context().Value(nodeContextKey).(contracts.NodeService)
}

func getIdentityService(r *http.Request) contracts.IdentityService {
	return getNodeService(r).IdentityInfo()
}
func getNotificationService(r *http.Request) contracts.NotificationService {
	return getNodeService(r).Notification()
}
func getOrderService(r *http.Request) contracts.OrderService     { return getNodeService(r).Order() }
func getListingService(r *http.Request) contracts.ListingService { return getNodeService(r).Listing() }
func getProfileService(r *http.Request) contracts.ProfileService { return getNodeService(r).Profile() }
func getSocialService(r *http.Request) contracts.SocialService   { return getNodeService(r).Social() }
func getWalletService(r *http.Request) contracts.WalletService   { return getNodeService(r).Wallet() }
func getMediaService(r *http.Request) contracts.MediaService     { return getNodeService(r).Media() }
func getMatrixChatService(r *http.Request) contracts.MatrixChatService {
	return getNodeService(r).MatrixChat()
}
func getPreferencesService(r *http.Request) contracts.PreferencesService {
	return getNodeService(r).Preferences()
}
func getShoppingCartService(r *http.Request) contracts.ShoppingCartService {
	return getNodeService(r).ShoppingCart()
}
func getExchangeRateService(r *http.Request) contracts.ExchangeRateService {
	return getNodeService(r).ExchangeRate()
}

// getCoreIface attempts to extract coreiface.CoreIface from the request context.
// Returns (nil, false) if the node is a TenantService (which only implements NodeService).
// Handlers that need CoreIface-only methods (DB, Multiwallet, IPFSNode, ExchangeRates,
// Stripe, etc.) must use this with a 501 fallback for SaaS mode.
func getCoreIface(r *http.Request) (coreiface.CoreIface, bool) {
	ci, ok := r.Context().Value(nodeContextKey).(coreiface.CoreIface)
	return ci, ok
}

// NodeSelectionMiddleware adds middleware for node selection
func (g *Gateway) NodeSelectionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nodeID := r.Header.Get("X-Mobazha-Node")
		if nodeID == "" {
			// If no node is specified, use the first available node
			g.mu.RLock()
			for id := range g.nodeManager.GetNodes() {
				nodeID = id
				break
			}
			g.mu.RUnlock()
		}

		g.mu.RLock()
		node, ok := g.nodeManager.GetNode(nodeID)
		g.mu.RUnlock()

		if !ok {
			http.Error(w, "Node not found", http.StatusNotFound)
			return
		}

		// Store the selected node in request context
		ctx := context.WithValue(r.Context(), nodeContextKey, node)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// WebsocketNodeHandler handle the websocket connection for a specific node
func (g *Gateway) WebsocketNodeHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		nodeID := vars["nodeID"]
		log.Debugf("Websocket connection for node %s", nodeID)

		hub := g.EnsureHubForUser(nodeID)

		// use the existing websocketHandler
		handler := newWebsocketHandler(hub)
		handler.ServeHTTP(w, r)
	}
}

// WebsocketDefaultHandler handles /ws connections where nodeID is resolved
// from the request context (injected by the resolver middleware).
func (g *Gateway) WebsocketDefaultHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		node := getNodeService(r)
		nodeID := node.IdentityInfo().GetNodeID()
		log.Debugf("Websocket default connection for node %s", nodeID)

		hub := g.EnsureHubForUser(nodeID)
		handler := newWebsocketHandler(hub)
		handler.ServeHTTP(w, r)
	}
}

// featuresSnapshotFromNodeManager returns a resolver callback that the
// embedded frontend uses to seed window.__RUNTIME_CONFIG__.features at
// /runtime-config.js request time. It walks NodeManager → default node →
// FeaturesProvider → Resolver so toggles via PUT /v1/settings/features/{key}
// or PATCH /platform/v1/features/{key} propagate without a process restart.
// Any missing link returns an empty snapshot (fail-closed) so misconfigured
// deployments do not advertise an unreachable feature.
//
// The callback seeds the Resolver ctx with the standalone tenantID so the
// tenant layer participates in evaluation (matches handleGETFeatures).
func featuresSnapshotFromNodeManager(nm coreiface.NodeManagerIface) func(context.Context) []frontend.FeatureSnapshot {
	return func(ctx context.Context) []frontend.FeatureSnapshot {
		if nm == nil {
			return nil
		}
		def := nm.GetDefaultNode()
		if def == nil {
			return nil
		}
		fp, ok := def.(contracts.FeaturesProvider)
		if !ok {
			return nil
		}
		resolver := fp.Features()
		if resolver == nil {
			return nil
		}

		if pkgconfig.TenantIDFromContext(ctx) == "" {
			ctx = pkgconfig.ContextWithTenantID(ctx, database.StandaloneTenantID)
		}

		entries := resolver.List(ctx)
		out := make([]frontend.FeatureSnapshot, 0, len(entries))
		for _, e := range entries {
			if e.Feature == nil {
				continue
			}
			out = append(out, frontend.FeatureSnapshot{
				Key:         e.Feature.Key,
				Effective:   e.Effective,
				Overridable: overridableScopes(e.Feature),
			})
		}
		return out
	}
}

func (g *Gateway) EnsureHubForUser(nodeID string) *hub {
	g.hubsMtx.RLock()
	h, exists := g.hubs[nodeID]
	g.hubsMtx.RUnlock()

	if !exists {
		g.hubsMtx.Lock()
		// double check
		if h, exists = g.hubs[nodeID]; !exists {
			h = newHub(nodeID)
			g.hubs[nodeID] = h
			go h.run()
		}
		g.hubsMtx.Unlock()
	}

	return h
}
