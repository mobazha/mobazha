package api

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"

	"github.com/gorilla/mux"
	"github.com/ipfs/kubo/core/corehttp"
	"github.com/mobazha/mobazha3.0/internal/repo"
	pkgconfig "github.com/mobazha/mobazha3.0/pkg/config"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/op/go-logging"
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
}

// Gateway represents an HTTP API gateway
type Gateway struct {
	listener       net.Listener
	nodeManager    coreiface.NodeManagerIface
	handler        http.Handler
	config         *GatewayConfig
	auth           authState
	hubs           map[string]*hub
	hubsMtx        sync.RWMutex
	shutdown       chan struct{}
	closeOnce      sync.Once
	mu             sync.RWMutex
	featureManager *pkgconfig.FeatureManager
}

// NewGateway instantiates a new gateway. We multiplex the ob API along with the
// IPFS gateway API.
func NewGateway(nodeManager coreiface.NodeManagerIface, config *GatewayConfig, options ...corehttp.ServeOption) (*Gateway, error) {
	var (
		g = &Gateway{
			nodeManager:    nodeManager,
			config:         config,
			listener:       config.Listener,
			shutdown:       make(chan struct{}),
			hubs:           make(map[string]*hub),
			hubsMtx:        sync.RWMutex{},
			featureManager: pkgconfig.GetGlobalFeatureManager(),
		}
		topMux = http.NewServeMux()
	)

	g.auth = authState{
		username:     config.Username,
		passwordHash: config.Password,
		hashFile:     config.HashFile,
		plainFile:    config.PlainFile,
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

	var (
		err error
		mux = topMux
	)
	for _, option := range options {
		mux, err = option(nodeManager.GetIPFSNode(), config.Listener, mux)
		if err != nil {
			return nil, err
		}
	}
	g.handler = topMux
	return g, nil
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
func getChatService(r *http.Request) contracts.ChatService { return getNodeService(r).Chat() }
func getNotificationService(r *http.Request) contracts.NotificationService {
	return getNodeService(r).Notification()
}
func getOrderService(r *http.Request) contracts.OrderService     { return getNodeService(r).Order() }
func getListingService(r *http.Request) contracts.ListingService { return getNodeService(r).Listing() }
func getProfileService(r *http.Request) contracts.ProfileService { return getNodeService(r).Profile() }
func getSocialService(r *http.Request) contracts.SocialService   { return getNodeService(r).Social() }
func getWalletService(r *http.Request) contracts.WalletService   { return getNodeService(r).Wallet() }
func getMediaService(r *http.Request) contracts.MediaService     { return getNodeService(r).Media() }
func getMatrixService(r *http.Request) contracts.MatrixService   { return getNodeService(r).Matrix() }
func getPreferencesService(r *http.Request) contracts.PreferencesService {
	return getNodeService(r).Preferences()
}
func getShoppingCartService(r *http.Request) contracts.ShoppingCartService {
	return getNodeService(r).ShoppingCart()
}
func getStripeService(r *http.Request) contracts.StripeService { return getNodeService(r).Stripe() }
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
