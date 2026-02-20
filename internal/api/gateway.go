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
}

// Gateway represents an HTTP API gateway
type Gateway struct {
	listener       net.Listener
	nodeManager    coreiface.NodeManagerIface
	handler        http.Handler
	config         *GatewayConfig
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

	r := g.newV1Router()

	if config.AllowAllOrigins {
		r.Use(g.CORSAllowAllOriginsMiddleware)
	}
	r.Use(mux.CORSMethodMiddleware(r))
	r.Use(g.AuthenticationMiddleware)
	r.Use(g.NodeSelectionMiddleware)

	// Create default hub
	defaultNodeID := repo.DefaultNodeID
	defaultHub := newHub(defaultNodeID)
	g.hubs[defaultNodeID] = defaultHub
	go defaultHub.run()

	r.HandleFunc("/ws/{nodeID}", g.WebsocketNodeHandler())
	r.Handle("/ws", g.AuthenticationMiddleware(newWebsocketHandler(g.hubs[defaultNodeID])))

	topMux.Handle("/v1/", r)
	topMux.Handle("/ws/", r)
	topMux.Handle("/ws", r)

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

		err = g.listener.Close()
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
func (g *Gateway) Serve() error {
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

	if !g.config.PublicOnly {
		// Wallet
		r.HandleFunc("/v1/wallet/spend", g.handlePOSTSpend).Methods("POST")
		r.HandleFunc("/v1/wallet/mnemonic", g.handleGETMnemonic).Methods("GET")

		// Chat
		r.HandleFunc("/v1/chatmessage", g.handlePOSTSendChatMessage).Methods("POST")
		r.HandleFunc("/v1/groupchatmessage", g.handlePOSTSendGroupChatMessage).Methods("POST")
		r.HandleFunc("/v1/typingmessage", g.handlePOSTSendTypingMessage).Methods("POST")
		r.HandleFunc("/v1/grouptypingmessage", g.handlePOSTSendGroupTypingMessage).Methods("POST")
		r.HandleFunc("/v1/markchatasread", g.handlePOSTMarkChatMessageAsRead).Methods("POST")
		r.HandleFunc("/v1/chatconversations", g.handleGETChatConversations).Methods("GET")
		r.HandleFunc("/v1/orderconversations", g.handleGETOrderConversations).Methods("GET")
		r.HandleFunc("/v1/chatmessages/{peerID}", g.handleGETChatMessages).Methods("GET")
		r.HandleFunc("/v1/chatmessages", g.handleGETMyChatMessages).Methods("GET")
		r.HandleFunc("/v1/groupchatmessages/{orderID}", g.handleGETGroupChatMessages).Methods("GET")
		r.HandleFunc("/v1/chatmessage/{messageID}", g.handleDELETEChatMessages).Methods("DELETE")
		r.HandleFunc("/v1/groupchatmessages/{orderID}", g.handleDELETEGroupChatMessages).Methods("DELETE")
		r.HandleFunc("/v1/chatconversation/{peerID}", g.handleDELETEChatConversation).Methods("DELETE")

		// Chat group
		r.HandleFunc("/v1/chatgroups", g.handleGetChatGroups).Methods("GET")
		r.HandleFunc("/v1/chatGroup", g.handleSaveChatGroup).Methods("POST")
		r.HandleFunc("/v1/chatGroup", g.handleGetChatGroup).Methods("GET")
		r.HandleFunc("/v1/chatGroup", g.handleDeleteChatGroup).Methods("DELETE")

		// Matrix E2EE Key Backup (stored locally, encrypted with wallet mnemonic)
		r.HandleFunc("/v1/matrix/key-backup", g.handlePOSTMatrixKeyBackup).Methods("POST")
		r.HandleFunc("/v1/matrix/key-backup", g.handleGETMatrixKeyBackup).Methods("GET")
		r.HandleFunc("/v1/matrix/key-backup", g.handleDELETEMatrixKeyBackup).Methods("DELETE")
		r.HandleFunc("/v1/matrix/key-backup/info", g.handleGETMatrixKeyBackupInfo).Methods("GET")
		r.HandleFunc("/v1/matrix/key-backup/list", g.handleGETMatrixKeyBackupList).Methods("GET")

		// Matrix Credentials (for direct password login, decentralized)
		r.HandleFunc("/v1/matrix/credentials", g.handleGETMatrixCredentials).Methods("GET")
		r.HandleFunc("/v1/matrix/credentials", g.handlePOSTMatrixCredentials).Methods("POST")
		r.HandleFunc("/v1/matrix/password", g.handleGETMatrixPassword).Methods("GET")

		// Matrix Secrets Bundle (cross-signing keys, encrypted with node private key)
		r.HandleFunc("/v1/matrix/secrets-bundle", g.handlePOSTMatrixSecretsBundle).Methods("POST")
		r.HandleFunc("/v1/matrix/secrets-bundle", g.handleGETMatrixSecretsBundle).Methods("GET")
		r.HandleFunc("/v1/matrix/secrets-bundle", g.handleDELETEMatrixSecretsBundle).Methods("DELETE")
		r.HandleFunc("/v1/matrix/secrets-bundle/info", g.handleGETMatrixSecretsBundleInfo).Methods("GET")

		// Notification
		r.HandleFunc("/v1/notifications", g.handleGetNotifications).Methods("GET")
		r.HandleFunc("/v1/notifications/count", g.handleGetNotificationCount).Methods("GET")
		r.HandleFunc("/v1/notifications/batch", g.handleBatchNotifications).Methods("POST")
		r.HandleFunc("/v1/marknotificationasread/{notifID}", g.handlePOSTMarkNotificationMessageAsRead).Methods("POST")
		r.HandleFunc("/v1/marknotificationsasread", g.handlePOSTMarkNotificationsMessageAsRead).Methods("POST")

		// Escrow
		r.HandleFunc("/v1/instructions/order/payment", g.handleGetOrderPaymentInstructions).Methods("POST")
		r.HandleFunc("/v1/instructions/order/confirm", g.handleGETOrderConfirmationInstructions).Methods("POST")
		r.HandleFunc("/v1/instructions/order/reject", g.handleGETOrderConfirmationInstructions).Methods("POST")
		r.HandleFunc("/v1/instructions/order/refund", g.handleGETOrderRefundInstructions).Methods("POST")
		r.HandleFunc("/v1/instructions/order/complete", g.handleGETOrderCompleteInstructions).Methods("POST")
		r.HandleFunc("/v1/instructions/order/cancel", g.handleGETOrderCancelInstructions).Methods("POST")

		r.HandleFunc("/v1/instructions/dispute/release", g.handleGETReleaseFundsInstructions).Methods("POST")

		// 收款账户相关API
		r.HandleFunc("/v1/wallet/receivingaccountlist", g.GetReceivingAccounts).Methods("GET")
		r.HandleFunc("/v1/wallet/receivingaccount", g.AddReceivingAccount).Methods("POST")
		r.HandleFunc("/v1/wallet/receivingaccount", g.UpdateReceivingAccount).Methods("PUT")
		r.HandleFunc("/v1/wallet/receivingaccount/{id}", g.DeleteReceivingAccount).Methods("DELETE")
		// Stripe连接URL
		r.HandleFunc("/v1/stripe/public-key", g.GetStripePublicKey).Methods("GET")
		r.HandleFunc("/v1/stripe/connect-url", g.GetStripeConnectURL).Methods("GET")
		r.HandleFunc("/v1/stripe/account-status", g.GetStripeAccountStatus).Methods("GET")
		r.HandleFunc("/v1/stripe/payment-intent", g.CreateStripePaymentIntent).Methods("POST")
		r.HandleFunc("/v1/stripe/webhook", g.HandleStripeWebhook).Methods("POST")

		// Orders
		r.HandleFunc("/v1/purchases", g.handlePOSTPurchases).Methods("POST")
		r.HandleFunc("/v1/purchases", g.handleGETPurchases).Methods("GET")
		r.HandleFunc("/v1/sales", g.handleGETSales).Methods("GET")
		r.HandleFunc("/v1/sales", g.handlePostSales).Methods("POST")
		r.HandleFunc("/v1/cases", g.handleGETCases).Methods("GET")
		r.HandleFunc("/v1/cases", g.handlePostCases).Methods("POST")

		r.HandleFunc("/v1/case/{orderID}", g.handleGetCase).Methods("GET")
		r.HandleFunc("/v1/order/{orderID}", g.handleGETOrder).Methods("GET")
		r.HandleFunc("/v1/orderspend", g.handlePostSpendForOrder).Methods("POST")
		r.HandleFunc("/v1/order/{orderID}/payment/remaining", g.handleGETPaymentRemaining).Methods("GET")
		r.HandleFunc("/v1/order/{orderID}/payment/cancel-partial", g.handlePOSTCancelPartialPayment).Methods("POST")
		r.HandleFunc("/v1/order/{orderID}/payment/watch", g.handleDELETEPaymentWatch).Methods("DELETE")

		r.HandleFunc("/v1/order/purchase", g.handlePOSTPurchase).Methods("POST")
		r.HandleFunc("/v1/order/payment", g.handlePOSTPayment).Methods("POST")
		r.HandleFunc("/v1/order/confirm", g.handlePOSTOrderConfirmation).Methods("POST")
		r.HandleFunc("/v1/order/fulfill", g.handlePOSTOrderFulfillment).Methods("POST")
		r.HandleFunc("/v1/order/refund", g.handlePOSTOrderRefund).Methods("POST")
		r.HandleFunc("/v1/order/complete", g.handlePOSTOrderCompletion).Methods("POST")
		r.HandleFunc("/v1/order/cancel", g.handlePOSTOrderCancel).Methods("POST")

		r.HandleFunc("/v1/estimatetotal", g.handlePOSTEstimateTotal).Methods("POST")
		r.HandleFunc("/v1/checkoutbreakdown", g.handlePOSTCheckoutBreakdown).Methods("POST")

		// Moderation
		r.HandleFunc("/v1/dispute/open", g.handlePOSTOpenDispute).Methods("POST")
		r.HandleFunc("/v1/dispute/close", g.handlePOSTCloseDispute).Methods("POST")
		r.HandleFunc("/v1/dispute/release", g.handlePOSTReleaseFunds).Methods("POST")
		r.HandleFunc("/v1/dispute/releaseAfterTimeout", g.handlePOSTReleaseEscrow).Methods("POST")

		// Cart
		r.HandleFunc("/v1/carts/itemsCount", g.handleGETCartsItemsCount).Methods("GET")
		r.HandleFunc("/v1/carts", g.handleGETCarts).Methods("GET")
		r.HandleFunc("/v1/carts", g.handleClearCarts).Methods("DELETE")
		r.HandleFunc("/v1/carts/{peerID}/add", g.handleAddToCart).Methods("POST")
		r.HandleFunc("/v1/carts/{peerID}/update", g.handleAddToCart).Methods("POST")
		r.HandleFunc("/v1/carts/{peerID}/remove", g.handleRemoveCartItem).Methods("POST")

		// Following
		r.HandleFunc("/v1/followsme/{peerID}", g.handleGETFollowsMe).Methods("GET")
		r.HandleFunc("/v1/follow/{peerID}", g.handlePOSTFollow).Methods("POST")
		r.HandleFunc("/v1/unfollow/{peerID}", g.handlePOSTUnFollow).Methods("POST")

		// Listings
		r.HandleFunc("/v1/mylisting/{slugOrCID}", g.handleGETMyListing).Methods("GET")
		r.HandleFunc("/v1/listing", g.handlePOSTListing).Methods("POST")
		r.HandleFunc("/v1/listing", g.handlePUTListing).Methods("PUT")
		r.HandleFunc("/v1/listing/{slug}", g.handleDELETEListing).Methods("DELETE")

		// Listings Batch Import
		r.HandleFunc("/v1/listings/import", g.handlePOSTListingsImport).Methods("POST")
		r.HandleFunc("/v1/listings/import/json", g.handlePOSTListingsImportJSON).Methods("POST")

		// Images
		r.HandleFunc("/v1/avatar", g.handlePOSTAvatar).Methods("POST")
		r.HandleFunc("/v1/header", g.handlePOSTHeader).Methods("POST")
		r.HandleFunc("/v1/images", g.handlePOSTImages).Methods("POST")
		r.HandleFunc("/v1/productimages", g.handlePOSTProductImage).Methods("POST")

		// File
		r.HandleFunc("/v1/file", g.handlePOSTFile).Methods("POST")

		// Profiles
		r.HandleFunc("/v1/profile", g.handlePOSTProfile).Methods("POST")
		r.HandleFunc("/v1/profile/{peerID}", g.handlePOSTProfile).Methods("POST")
		r.HandleFunc("/v1/profile", g.handlePUTProfile).Methods("PUT")
		r.HandleFunc("/v1/profile/{peerID}", g.handlePUTProfile).Methods("PUT")

		// Ratings

		// Posts
		r.HandleFunc("/v1/post", g.handlePOSTPost).Methods("POST")
		r.HandleFunc("/v1/post/{slug}", g.handleDELETEPost).Methods("DELETE")

		r.HandleFunc("/v1/signmessage", g.handlePOSTSignMessage).Methods("POST")
		r.HandleFunc("/v1/verifymessage", g.handlePOSTVerifyMessage).Methods("POST")
		r.HandleFunc("/v1/hashmessage", g.handlePOSTHashMessage).Methods("POST")

		// Moderator
		r.HandleFunc("/v1/moderator", g.handleSetModerator).Methods("POST")
		r.HandleFunc("/v1/moderator", g.handleUnsetModerator).Methods("DELETE")
		r.HandleFunc("/v1/moderators", g.handleGetModerators).Methods("GET")

		// Block a store
		r.HandleFunc("/v1/blocknode/{peerID}", g.handleBlockNode).Methods("POST")
		r.HandleFunc("/v1/blocknode/{peerID}", g.handleUnBlockNode).Methods("DELETE")

		r.HandleFunc("/v1/config", g.handleGETConfig).Methods("GET")
		r.HandleFunc("/v1/systemInfo", g.handleGETSystemInfo).Methods("GET")
		r.HandleFunc("/v1/wallet/currencies", g.handleGETCurrencies).Methods("GET")

		// Preferences
		r.HandleFunc("/v1/preferences", g.handlePutUserPreferences).Methods("PUT")
		r.HandleFunc("/v1/preferences", g.handleGetUserPreferences).Methods("GET")

		r.HandleFunc("/v1/bulkupdatecurrency", g.handlePOSTBulkUpdateCurrency).Methods("POST")
		r.HandleFunc("/v1/publish", g.handlePOSTPublish).Methods("POST")
		r.HandleFunc("/v1/purgecache", g.handlePOSTPurgeCache).Methods("POST")
		r.HandleFunc("/v1/shutdown", g.handlePOSTShutdown).Methods("POST")

		// Channels
		r.HandleFunc("/v1/channelmessage", g.handlePOSTPublishChannelMessage).Methods("POST")
		r.HandleFunc("/v1/openchannel/{topic}", g.handlePOSTOpenChannel).Methods("POST")
		r.HandleFunc("/v1/closechannel/{topic}", g.handlePOSTCloseChannel).Methods("POST")
		r.HandleFunc("/v1/channels", g.handleGETListChannels).Methods("GET")
		r.HandleFunc("/v1/channelmessages/{topic}", g.handleGETChannelMessages).Methods("GET")
	}
	// Images
	r.HandleFunc("/v1/image/{imageID}", g.handleGETImage).Methods("GET")
	r.HandleFunc("/v1/avatar/{peerID}/{size}", g.handleGETAvatar).Methods("GET")
	r.HandleFunc("/v1/header/{peerID}/{size}", g.handleGETHeader).Methods("GET")

	// File
	r.HandleFunc("/v1/file/{fileID}", g.handleGETFile).Methods("GET")

	// Listings
	r.HandleFunc("/v1/listing/{listingID}", g.handleGETListing).Methods("GET")
	r.HandleFunc("/v1/listing/{peerID}/{slug}", g.handleGETListing).Methods("GET")
	r.HandleFunc("/v1/listingindex/{peerID}", g.handleGETListingIndex).Methods("GET")
	r.HandleFunc("/v1/listingindex", g.handleGETListingIndex).Methods("GET")
	r.HandleFunc("/v1/listings/template", g.handleGETListingsTemplate).Methods("GET") // Public: no auth required

	// Profiles
	r.HandleFunc("/v1/profile/{peerID}", g.handleGETProfile).Methods("GET")
	r.HandleFunc("/v1/profile", g.handleGETProfile).Methods("GET")
	r.HandleFunc("/v1/fetchprofiles", g.handlePOSTFetchProfiles).Methods("GET", "POST")

	// Ratings
	r.HandleFunc("/v1/ratingindex/{peerIDOrSlug}", g.handleGETRatingIndex).Methods("GET")
	r.HandleFunc("/v1/ratingindex", g.handleGETMyRatingIndex).Methods("GET")
	r.HandleFunc("/v1/ratingindex/{peerID}/{slug}", g.handleGETPeerRatingsBySlug).Methods("GET")
	r.HandleFunc("/v1/rating/{ratingID}", g.handleGETRating).Methods("GET")
	r.HandleFunc("/v1/fetchratings", g.handlePOSTFetchRatings).Methods("POST")

	// Posts
	r.HandleFunc("/v1/post/{slug}", g.handleGETMyPost).Methods("GET")
	r.HandleFunc("/v1/post/{peerID}/{slug}", g.handleGETPost).Methods("GET")

	// Following
	r.HandleFunc("/v1/followers/{peerID}", g.handleGETFollowers).Methods("GET")
	r.HandleFunc("/v1/followers", g.handleGETFollowers).Methods("GET")
	r.HandleFunc("/v1/following/{peerID}", g.handleGETFollowing).Methods("GET")
	r.HandleFunc("/v1/following", g.handleGETFollowing).Methods("GET")

	r.HandleFunc("/v1/exchangerates/{currencyCode}", g.handleGETExchangeRates).Methods("GET")
	r.HandleFunc("/v1/exchangerates", g.handleGETExchangeRates).Methods("GET")

	r.HandleFunc("/v1/peers", g.handleGETPeers).Methods("GET")
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

func getIdentityService(r *http.Request) contracts.IdentityService         { return getNodeService(r).IdentityInfo() }
func getChatService(r *http.Request) contracts.ChatService                 { return getNodeService(r).Chat() }
func getNotificationService(r *http.Request) contracts.NotificationService { return getNodeService(r).Notification() }
func getOrderService(r *http.Request) contracts.OrderService               { return getNodeService(r).Order() }
func getListingService(r *http.Request) contracts.ListingService           { return getNodeService(r).Listing() }
func getProfileService(r *http.Request) contracts.ProfileService           { return getNodeService(r).Profile() }
func getSocialService(r *http.Request) contracts.SocialService             { return getNodeService(r).Social() }
func getWalletService(r *http.Request) contracts.WalletService             { return getNodeService(r).Wallet() }
func getMediaService(r *http.Request) contracts.MediaService               { return getNodeService(r).Media() }
func getMatrixService(r *http.Request) contracts.MatrixService             { return getNodeService(r).Matrix() }
func getPreferencesService(r *http.Request) contracts.PreferencesService   { return getNodeService(r).Preferences() }
func getShoppingCartService(r *http.Request) contracts.ShoppingCartService { return getNodeService(r).ShoppingCart() }
func getStripeService(r *http.Request) contracts.StripeService             { return getNodeService(r).Stripe() }
func getExchangeRateService(r *http.Request) contracts.ExchangeRateService { return getNodeService(r).ExchangeRate() }

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
