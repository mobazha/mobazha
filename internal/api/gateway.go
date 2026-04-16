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
	pkgconfig "github.com/mobazha/mobazha3.0/pkg/config"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/logging"
)

var log = logging.MustGetLogger("API")

type contextKey string

const nodeContextKey contextKey = "node"

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
}

// Gateway represents an HTTP API gateway
type Gateway struct {
	listener       net.Listener
	nodeManager    coreiface.NodeManagerIface
	handler        http.Handler
	config         *GatewayConfig
	auth           authState
	jwtValidator   *JWTValidator // nil when no Casdoor cert configured
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
		// Native binary mode: serve the go:embed SPA as catch-all.
		feHandler := frontend.NewHandler(frontend.ServerConfig{})
		topMux.Handle("/", feHandler)
		log.Info("Serving embedded Web UI on /")
	}

	g.handler = topMux
	return g, nil
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

// EnsureHubForUser ensures that a hub exists for the given user ID.
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
