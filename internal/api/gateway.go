package api

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sort"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mobazha/mobazha/internal/embedded/frontend"
	"github.com/mobazha/mobazha/internal/repo"
	"github.com/mobazha/mobazha/internal/ssr"
	agentskill "github.com/mobazha/mobazha/pkg/agent/skill"
	"github.com/mobazha/mobazha/pkg/apitoken"
	pkgconfig "github.com/mobazha/mobazha/pkg/config"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/core/coreiface"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/distribution"
	"github.com/mobazha/mobazha/pkg/edition"
	"github.com/mobazha/mobazha/pkg/logging"
	mcppkg "github.com/mobazha/mobazha/pkg/mcp"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"gorm.io/gorm"
)

var log = logging.MustGetLogger("API")

type contextKey string

const nodeContextKey contextKey = "node"
const platformRuntimeCapabilitiesContextKey contextKey = "platform-runtime-capabilities"

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
	// AdminSessionTTL controls short-lived browser administrator sessions.
	// Zero uses the secure default (30 minutes).
	AdminSessionTTL time.Duration
	// AuthAuditSink receives structured authentication boundary events.
	// Nil uses the standard structured log sink.
	AuthAuditSink AuthAuditSink

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
	// Edition selects the distribution capability policy. Empty fails closed to
	// Community. Private compositions select broader policies explicitly.
	Edition string

	// FrontendOverrideDir serves static files from this directory before
	// falling back to the embedded SPA. Used for white-label logo/favicon.
	FrontendOverrideDir string

	// Brand holds white-label overrides loaded from brand.yaml.
	// When non-nil, the embedded frontend receives brand theming
	// via /runtime-config.js. CLI and API docs also honour the name.
	Brand *frontend.BrandSnapshot

	// GormDB is an optional GORM database for local API token storage.
	// When set (standalone mode), the gateway auto-creates the api_tokens
	// table and enables /v1/auth/tokens CRUD + mbz_ token authentication.
	GormDB *gorm.DB

	// SkillProvider supplies built-in or deployment-specific Agent skills.
	// MOBAZHA_AGENT_SKILLS_DIR, when set, is layered in front as an override.
	SkillProvider agentskill.Provider

	// TrustedHumaModules are first-party, build-time distribution extensions.
	// The gateway supplies auth/security metadata and retains middleware,
	// envelope, listener, and OpenAPI ownership.
	TrustedHumaModules   []distribution.TrustedHumaModule
	GuestPaymentPolicy   distribution.GuestPaymentPolicy
	ProductSurfacePolicy distribution.ProductSurfacePolicy
	AIHTTPPolicy         distribution.AIHTTPPolicy

	// RuntimeCapabilitiesSnapshotFn lets a distribution composition project
	// platform-level capabilities for global/public runtime-config requests.
	// Tenant-resolved requests still use the resolved node so checkout surfaces
	// remain scoped to the store that will actually serve the order.
	RuntimeCapabilitiesSnapshotFn func(context.Context, frontend.RuntimeCapabilities) frontend.RuntimeCapabilities
}

// Gateway represents an HTTP API gateway
type Gateway struct {
	listener          net.Listener
	nodeManager       coreiface.NodeManagerIface
	handler           http.Handler
	config            *GatewayConfig
	auth              authState
	adminSessions     *adminSessionStore
	authAuditSink     AuthAuditSink
	jwtValidator      *JWTValidator   // nil when no Casdoor cert configured
	tokenStore        *apitoken.Store // nil in SaaS mode; set for standalone
	hubs              map[string]*hub
	hubsMtx           sync.RWMutex
	shutdown          chan struct{}
	closeOnce         sync.Once
	mu                sync.RWMutex
	featureManager    *pkgconfig.FeatureManager
	guestOrderLimiter *rateLimiter
	authLimiter       *authRateLimiter
	editionPolicy     edition.Policy
	aiHTTPPolicy      distribution.AIHTTPPolicy
}

// NewGateway instantiates a new gateway.
func NewGateway(nodeManager coreiface.NodeManagerIface, config *GatewayConfig) (*Gateway, error) {
	editionPolicy, err := edition.ResolvePolicy(config.Edition)
	if err != nil {
		return nil, fmt.Errorf("resolve edition policy: %w", err)
	}
	var (
		g = &Gateway{
			nodeManager:       nodeManager,
			config:            config,
			listener:          config.Listener,
			shutdown:          make(chan struct{}),
			hubs:              make(map[string]*hub),
			hubsMtx:           sync.RWMutex{},
			featureManager:    pkgconfig.GetGlobalFeatureManager(),
			guestOrderLimiter: newRateLimiter(10, time.Hour),
			authLimiter:       newAuthRateLimiter(),
			adminSessions:     newAdminSessionStore(config.AdminSessionTTL),
			authAuditSink:     config.AuthAuditSink,
			editionPolicy:     editionPolicy,
			aiHTTPPolicy:      resolveAIHTTPPolicy(config.AIHTTPPolicy, editionPolicy),
		}
		topMux = http.NewServeMux()
	)
	if g.authAuditSink == nil {
		g.authAuditSink = logAuthAuditSink{}
	}

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

	csrfEnabled := !config.PublicOnly && config.Password != ""
	r, err := g.newV1Router(config.AllowAllOrigins, csrfEnabled)
	if err != nil {
		return nil, fmt.Errorf("register V1 API: %w", err)
	}

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
		toolProfile := mcppkg.ToolProfileFull
		if config.ProductSurfacePolicy != nil &&
			config.ProductSurfacePolicy.MCPToolCatalog() == distribution.MCPToolCatalogRestricted {
			toolProfile = mcppkg.ToolProfileRestricted
		}
		mcpOpts := &mcppkg.ServerOptions{
			Transport:   "streamable-http",
			ToolProfile: toolProfile,
			// Standalone identity endpoint. Required (with AuditLogger) for
			// SSEIdentityFunc to resolve UserID/PeerID from request headers.
			IdentityPath: "/v1/auth/identity",
			// Stdout audit logger gives standalone deployments visibility into
			// MCP tool calls without requiring a database table. SaaS hosting
			// uses DBAuditLogger; this is the standalone equivalent.
			AuditLogger:      mcppkg.NewStdoutAuditLogger(),
			Shopping:         mcppkg.LoadShoppingConfigFromEnv(),
			StoreGatewayURL:  loopbackURL,
			QuoteTokenSecret: mcppkg.LoadQuoteTokenSecretFromEnv(),
		}
		mcpHTTPServer := mcppkg.NewStreamableHTTPMobazhaServer(loopbackURL, nil, mcpOpts)
		mcpWithBodyLimit := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			const maxMCPBody = 4 << 20 // 4 MiB
			r.Body = http.MaxBytesReader(w, r.Body, maxMCPBody)
			mcpHTTPServer.ServeHTTP(w, r)
		})
		topMux.Handle("/v1/mcp", mcpWithBodyLimit)
		topMux.Handle("/v1/mcp/", mcpWithBodyLimit)
		log.Info("MCP Streamable HTTP endpoint registered at /v1/mcp")
	}

	// Standalone mode: reverse-proxy /platform/* to the SaaS backend so
	// that frontend calls to HOSTING_API (store-links, bots, domains, etc.)
	// reach the platform instead of falling through to the SPA catch-all.
	if editionPolicy.AllowsCapability(edition.CapabilityPlatformIntegration) && config.SaaSAPIURL != "" {
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
	} else if !editionPolicy.AllowsCapability(edition.CapabilityPlatformIntegration) {
		// Register an explicit negative route before the frontend catch-all so an
		// embedded SPA can never turn a disabled control-plane call into HTTP 200.
		topMux.Handle("/platform/", http.NotFoundHandler())
	}

	runtimeFrontendConfig := g.runtimeFrontendConfig()
	// Keep runtime bootstrap dynamic even when Next.js serves the application.
	topMux.Handle("/runtime-config.js", frontend.NewRuntimeConfigHandler(runtimeFrontendConfig))

	// SSR: register meta injection + embed routes for standalone mode.
	// Activated when SPA directory exists (container deployment).
	if ssrHandler := ssr.NewFromEnv(config.LocalPeerID); ssrHandler != nil {
		ssrHandler.RegisterRoutes(topMux)
	} else if frontend.HasContent() {
		runtimeFrontendConfig.OverrideDir = config.FrontendOverrideDir
		feHandler := frontend.NewHandler(runtimeFrontendConfig)
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
// Safe for concurrent use with EnableJWTAuth.
func (g *Gateway) getJWTValidator() *JWTValidator {
	g.mu.RLock()
	jv := g.jwtValidator
	g.mu.RUnlock()
	return jv
}

// EnableJWTAuth initializes (or replaces) the jwtValidator at runtime.
// Called asynchronously after startup when the Casdoor certificate is
// fetched from the SaaS platform. Thread-safe.
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

// Close shutsdown the Gateway listener. Safe to call multiple times.
func (g *Gateway) Close() error {
	var err error
	g.closeOnce.Do(func() {
		close(g.shutdown)

		if g.authLimiter != nil {
			g.authLimiter.stop()
		}

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

func (g *Gateway) newV1Router(allowAllOrigins, csrfEnabled bool) (chi.Router, error) {
	r := chi.NewMux()
	r.Use(panicRecoveryMiddleware)
	r.Use(securityHeadersMiddleware)
	r.Use(maxBodySizeMiddleware(defaultMaxBodySize))
	if allowAllOrigins {
		r.Use(g.CORSAllowAllOriginsMiddleware)
	}
	if csrfEnabled {
		r.Use(csrfOriginCheckMiddleware)
	}
	if g.nodeManager != nil {
		r.Use(g.NodeSelectionMiddleware)
	}
	r.Use(g.StorefrontMiddleware)

	// Register raw chi routes BEFORE huma so that static path segments
	// ("upload-stream") are already in the trie when huma adds the
	// parameterized sibling ("{assetID}"). Chi's radix tree resolves
	// static nodes before param nodes at the same level, but only if the
	// static node exists when the param node is inserted.
	g.registerPreHumaRoutes(r)

	if _, err := g.registerHumaAPI(r); err != nil {
		return nil, err
	}
	return r, nil
}

// securityHeadersMiddleware sets baseline HTTP security headers on every response.
func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func wrapError(err error) string {
	return fmt.Sprintf(`{"error": "%s"}`, err.Error())
}

// getNodeService extracts contracts.NodeService from the request context.
// This works for both MobazhaNode and TenantService.
// Prefer the domain-specific getters below when the handler only needs
// a single domain's methods — they return narrower interface types.
func getNodeService(r *http.Request) contracts.NodeService {
	ns, ok := nodeServiceFromContext(r.Context())
	if !ok || ns == nil {
		panic(fmt.Sprintf("BUG: nodeContextKey missing from request context: %s %s (NodeSelectionMiddleware may not have run)", r.Method, r.URL.Path))
	}
	return ns
}

func nodeServiceFromContext(ctx context.Context) (contracts.NodeService, bool) {
	if ctx == nil {
		return nil, false
	}
	node, ok := ctx.Value(nodeContextKey).(contracts.NodeService)
	return node, ok && node != nil
}

func usePlatformRuntimeCapabilities(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	usePlatform, _ := ctx.Value(platformRuntimeCapabilitiesContextKey).(bool)
	return usePlatform
}

func isRuntimeConfigRequest(req *http.Request) bool {
	if req == nil || req.URL == nil {
		return false
	}
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		return false
	}
	return req.URL.Path == "/v1/runtime-config" || req.URL.Path == "/runtime-config.js"
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
func getReceivingAccountService(r *http.Request) contracts.ReceivingAccountService {
	return getNodeService(r).ReceivingAccounts()
}
func getMediaService(r *http.Request) contracts.MediaService { return getNodeService(r).Media() }
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

// getNodeDB returns the tenant-scoped database for the resolved node.
// Prefer this over getCoreIface when only DB access is needed — SaaS nodes
// are stored as contracts.NodeService but still expose DB() on the concrete type.
func getNodeDB(r *http.Request) (database.Database, bool) {
	if ci, ok := getCoreIface(r); ok {
		return ci.DB(), true
	}
	type dbProvider interface {
		DB() database.Database
	}
	if dp, ok := getNodeService(r).(dbProvider); ok {
		return dp.DB(), true
	}
	return nil, false
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
		nodeID := chi.URLParam(r, "nodeID")
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
		var fp contracts.FeaturesProvider
		if requestNode, ok := nodeServiceFromContext(ctx); ok {
			// Hosted mode resolves a tenant-scoped node before this callback runs.
			// Never fall back to another tenant when the resolved node does not
			// expose features; the correct result is fail-closed.
			fp, _ = requestNode.(contracts.FeaturesProvider)
		} else if nm != nil {
			// Full mode: GetDefaultNode returns a CoreIface which also implements FeaturesProvider.
			if def := nm.GetDefaultNode(); def != nil {
				fp, _ = def.(contracts.FeaturesProvider)
			}

			// Defensive fallback for custom managers that expose only the narrower
			// NodeService collection.
			if fp == nil {
				for _, n := range nm.GetNodes() {
					if p, ok := n.(contracts.FeaturesProvider); ok {
						fp = p
						break
					}
				}
			}
		}

		if fp == nil {
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

func (g *Gateway) runtimeFrontendConfig() frontend.ServerConfig {
	if g == nil || g.config == nil {
		return frontend.ServerConfig{}
	}
	deploymentMode := frontend.RuntimeDeploymentStandalone
	if mode := detectDeploymentMode(); mode == "saas" {
		deploymentMode = frontend.RuntimeDeploymentHosted
	} else if mode == "sovereign" {
		deploymentMode = frontend.RuntimeDeploymentSovereign
	}
	return frontend.ServerConfig{
		SaaSURL:            g.config.SaaSAPIURL,
		Edition:            g.editionPolicy.Name(),
		Deployment:         frontend.RuntimeDeployment{Mode: deploymentMode},
		Brand:              g.config.Brand,
		FeaturesSnapshotFn: featuresSnapshotFromNodeManager(g.nodeManager),
		CapabilitiesSnapshotFn: capabilitiesSnapshotFromNodeManager(
			g.nodeManager,
			g.editionPolicy,
			g.config.GuestPaymentPolicy,
			g.config.RuntimeCapabilitiesSnapshotFn,
		),
		NeedsSetupShellFn: g.needsSetupShell,
	}
}

type fiatRegistryProvider interface {
	FiatRegistry() contracts.FiatProviderRegistry
}

type walletOperatorProvider interface {
	Multiwallet() contracts.WalletOperator
}

type activePaymentChainProvider interface {
	ActivePaymentChains() []iwallet.ChainType
}

func capabilitiesSnapshotFromNodeManager(
	nm coreiface.NodeManagerIface,
	policy edition.Policy,
	guestPaymentPolicy distribution.GuestPaymentPolicy,
	platformSnapshotFn func(context.Context, frontend.RuntimeCapabilities) frontend.RuntimeCapabilities,
) func(context.Context, frontend.RuntimeCapabilities) frontend.RuntimeCapabilities {
	return func(ctx context.Context, baseline frontend.RuntimeCapabilities) frontend.RuntimeCapabilities {
		result := baseline
		result.Payments = frontend.PaymentCapabilities{Methods: []frontend.PaymentCapability{}}

		if platformSnapshotFn != nil && usePlatformRuntimeCapabilities(ctx) {
			result = platformSnapshotFn(ctx, result)
			result.Payments.Methods = filterPaymentCapabilities(result.Payments.Methods, policy)
			sortPaymentCapabilities(result.Payments.Methods)
			return result
		}

		var node contracts.NodeService
		var walletOperator contracts.WalletOperator
		if requestNode, ok := nodeServiceFromContext(ctx); ok {
			// Shared/hosted routers resolve the tenant node into the request
			// context. Its capabilities must take precedence over the manager's
			// default/first node to avoid cross-tenant capability projection.
			node = requestNode
			if provider, ok := requestNode.(walletOperatorProvider); ok {
				walletOperator = provider.Multiwallet()
			}
		} else if platformSnapshotFn != nil {
			result = platformSnapshotFn(ctx, result)
			result.Payments.Methods = filterPaymentCapabilities(result.Payments.Methods, policy)
			sortPaymentCapabilities(result.Payments.Methods)
			return result
		} else if nm != nil {
			if def := nm.GetDefaultNode(); def != nil {
				node = def
				walletOperator = def.Multiwallet()
			} else {
				for _, candidate := range nm.GetNodes() {
					node = candidate
					if provider, ok := candidate.(walletOperatorProvider); ok {
						walletOperator = provider.Multiwallet()
					}
					break
				}
			}
		}

		seen := make(map[string]struct{})
		if walletOperator != nil {
			activeProvider, ok := node.(activePaymentChainProvider)
			if ok {
				for _, capability := range activeCryptoPaymentCapabilities(
					walletOperator.SupportedChains(),
					activeProvider.ActivePaymentChains(),
				) {
					seen["crypto:"+capability.ID] = struct{}{}
					result.Payments.Methods = append(result.Payments.Methods, capability)
				}
			}
		}

		if provider, ok := node.(fiatRegistryProvider); ok && provider.FiatRegistry() != nil {
			for _, id := range provider.FiatRegistry().Registered() {
				if id == "" {
					continue
				}
				key := "fiat:" + id
				if _, exists := seen[key]; exists {
					continue
				}
				seen[key] = struct{}{}
				result.Payments.Methods = append(result.Payments.Methods, frontend.PaymentCapability{
					ID:   id,
					Kind: "fiat",
					Flow: "provider-session",
				})
			}
		}

		// Trusted distributions may implement direct-payment assets outside the
		// built-in wallet operator (for example an isolated wallet-rpc sidecar).
		// AdvertisedPaymentCoins is already availability-filtered by the
		// distribution policy, so project it through the same edition gate.
		if guestPaymentPolicy != nil {
			for _, coin := range guestPaymentPolicy.AdvertisedPaymentCoins() {
				info, err := iwallet.CoinInfoFromCoinType(coin)
				if err != nil {
					continue
				}
				capability := frontend.PaymentCapability{
					ID:   info.String(),
					Kind: "crypto",
					Flow: paymentFlowForChain(info.Chain),
				}
				key := "crypto:" + capability.ID
				if _, exists := seen[key]; exists {
					continue
				}
				seen[key] = struct{}{}
				result.Payments.Methods = append(result.Payments.Methods, capability)
			}
		}

		result.Payments.Methods = filterPaymentCapabilities(result.Payments.Methods, policy)
		sortPaymentCapabilities(result.Payments.Methods)
		return result
	}
}

func runtimeCapabilitiesFromPaymentMethods(
	provider func(context.Context) []edition.PaymentMethod,
) func(context.Context, frontend.RuntimeCapabilities) frontend.RuntimeCapabilities {
	if provider == nil {
		return nil
	}
	return func(ctx context.Context, baseline frontend.RuntimeCapabilities) frontend.RuntimeCapabilities {
		baseline.Payments = frontend.PaymentCapabilities{Methods: []frontend.PaymentCapability{}}
		for _, method := range provider(ctx) {
			if method.ID == "" || method.Kind == "" || method.Flow == "" {
				continue
			}
			baseline.Payments.Methods = append(baseline.Payments.Methods, frontend.PaymentCapability{
				ID:          method.ID,
				Kind:        method.Kind,
				Flow:        method.Flow,
				AddressMode: method.AddressMode,
			})
		}
		return baseline
	}
}

func sortPaymentCapabilities(methods []frontend.PaymentCapability) {
	sort.Slice(methods, func(i, j int) bool {
		left := methods[i]
		right := methods[j]
		if left.Kind == right.Kind {
			return left.ID < right.ID
		}
		return left.Kind < right.Kind
	})
}

func activeCryptoPaymentCapabilities(walletChains, registeredChains []iwallet.ChainType) []frontend.PaymentCapability {
	registered := make(map[iwallet.ChainType]struct{}, len(registeredChains))
	for _, chain := range registeredChains {
		registered[chain] = struct{}{}
	}
	seen := make(map[iwallet.ChainType]struct{}, len(walletChains))
	capabilities := make([]frontend.PaymentCapability, 0, len(walletChains))
	for _, chain := range walletChains {
		if chain == iwallet.ChainMock || chain == iwallet.ChainFiat {
			continue
		}
		if _, active := registered[chain]; !active {
			continue
		}
		if _, duplicate := seen[chain]; duplicate {
			continue
		}
		seen[chain] = struct{}{}
		capability := frontend.PaymentCapability{
			ID:   chain.String(),
			Kind: "crypto",
			Flow: paymentFlowForChain(chain),
		}
		if chain == iwallet.ChainZCash {
			capability.AddressMode = "transparent"
		}
		capabilities = append(capabilities, capability)
	}
	return capabilities
}

func filterPaymentCapabilities(methods []frontend.PaymentCapability, policy edition.Policy) []frontend.PaymentCapability {
	if policy == nil {
		return []frontend.PaymentCapability{}
	}
	filtered := make([]frontend.PaymentCapability, 0, len(methods))
	for _, method := range methods {
		if !policy.AllowsPaymentMethod(edition.PaymentMethod{
			ID:          method.ID,
			Kind:        method.Kind,
			Flow:        method.Flow,
			AddressMode: method.AddressMode,
		}) {
			continue
		}
		// Preserve the complete runtime projection so additive frontend fields
		// cannot be lost when a method passes the edition gate.
		filtered = append(filtered, method)
	}
	return filtered
}

func paymentFlowForChain(chain iwallet.ChainType) string {
	switch chain {
	case iwallet.ChainBitcoin,
		iwallet.ChainBitcoinCash,
		iwallet.ChainLitecoin,
		iwallet.ChainZCash,
		iwallet.ChainMonero:
		return "address-transfer"
	default:
		return "external-wallet"
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
