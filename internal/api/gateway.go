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

// Close shutsdown the Gateway listener.
func (g *Gateway) Close() error {
	close(g.shutdown)

	g.hubsMtx.Lock()
	for _, hub := range g.hubs {
		close(hub.Broadcast)
		close(hub.register)
		close(hub.unregister)
	}
	g.hubsMtx.Unlock()

	return g.listener.Close()
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
		r.HandleFunc("/v1/wallet/address", g.handleGETAddress).Methods("GET")
		r.HandleFunc("/v1/wallet/address/{coinType}", g.handleGETAddress).Methods("GET")
		r.HandleFunc("/v1/wallet/balance", g.handleGETBalance).Methods("GET")
		r.HandleFunc("/v1/wallet/balance/{coinType}", g.handleGETBalance).Methods("GET")
		r.HandleFunc("/v1/wallet/transactions/{coinType}", g.handleGETTransactions).Methods("GET")
		r.HandleFunc("/v1/wallet/spend", g.handlePOSTSpend).Methods("POST")
		r.HandleFunc("/v1/wallet/mnemonic", g.handleGETMnemonic).Methods("GET")
		r.HandleFunc("/v1/wallet/estimatefee/{coinType}", g.handleGETEstimateFee).Methods("GET")
		r.HandleFunc("/v1/wallet/fees", g.handleGETFees).Methods("GET")
		r.HandleFunc("/v1/wallet/resyncblockchain", g.handlePOSTResyncOrders).Methods("POST")
		r.HandleFunc("/v1/wallet/resyncblockchain/{coinType}", g.handlePOSTResyncBlockchain).Methods("POST")
		r.HandleFunc("/v1/wallet/status", g.handleGETWalletStatus).Methods("GET")
		r.HandleFunc("/v1/wallet/status/{coinType}", g.handleGETWalletStatus).Methods("GET")
		r.HandleFunc("/v1/wallet/status", g.handleUpdateWalletStatus).Methods("POST")
		r.HandleFunc("/v1/wallet/status/{coinType}", g.handleUpdateWalletStatus).Methods("POST")

		// Chat
		r.HandleFunc("/v1/ob/chatmessage", g.handlePOSTSendChatMessage).Methods("POST")
		r.HandleFunc("/v1/ob/groupchatmessage", g.handlePOSTSendGroupChatMessage).Methods("POST")
		r.HandleFunc("/v1/ob/typingmessage", g.handlePOSTSendTypingMessage).Methods("POST")
		r.HandleFunc("/v1/ob/grouptypingmessage", g.handlePOSTSendGroupTypingMessage).Methods("POST")
		r.HandleFunc("/v1/ob/markchatasread", g.handlePOSTMarkChatMessageAsRead).Methods("POST")
		r.HandleFunc("/v1/ob/chatconversations", g.handleGETChatConversations).Methods("GET")
		r.HandleFunc("/v1/ob/chatmessages/{peerID}", g.handleGETChatMessages).Methods("GET")
		r.HandleFunc("/v1/ob/chatmessages", g.handleGETMyChatMessages).Methods("GET")
		r.HandleFunc("/v1/ob/groupchatmessages/{orderID}", g.handleGETGroupChatMessages).Methods("GET")
		r.HandleFunc("/v1/ob/chatmessage/{messageID}", g.handleDELETEChatMessages).Methods("DELETE")
		r.HandleFunc("/v1/ob/groupchatmessages/{orderID}", g.handleDELETEGroupChatMessages).Methods("DELETE")
		r.HandleFunc("/v1/ob/chatconversation/{peerID}", g.handleDELETEChatConversation).Methods("DELETE")

		// Chat group
		r.HandleFunc("/v1/ob/chatGroup", g.handleSaveChatGroup).Methods("POST")
		r.HandleFunc("/v1/ob/chatGroup", g.handleGetChatGroup).Methods("GET")
		r.HandleFunc("/v1/ob/chatGroup", g.handleDeleteChatGroup).Methods("DELETE")

		// Notification
		r.HandleFunc("/v1/ob/notifications", g.handleGetNotifications).Methods("GET")
		r.HandleFunc("/v1/ob/marknotificationasread/{notifID}", g.handlePOSTMarkNotificationMessageAsRead).Methods("POST")
		r.HandleFunc("/v1/ob/marknotificationsasread", g.handlePOSTMarkNotificationsMessageAsRead).Methods("POST")

		// Escrow

		r.HandleFunc("/v1/escrow/instruction/releaseCancelable", g.handleGetReleaseCancelableEscrowInstructions).Methods("GET")
		r.HandleFunc("/v1/escrow/instruction/spl/initialize", g.handleGetInitializeSPLTokenInstructions).Methods("GET")
		r.HandleFunc("/v1/escrow/instruction/spl/release", g.handleGetReleaseSPLTokenInstructions).Methods("GET")

		r.HandleFunc("/v1/instructions/order/payment", g.handleGetOrderPaymentInstructions).Methods("POST")
		r.HandleFunc("/v1/instructions/order/confirm", g.handleGETOrderConfirmationInstructions).Methods("POST")
		r.HandleFunc("/v1/instructions/order/reject", g.handleGETOrderConfirmationInstructions).Methods("POST")
		r.HandleFunc("/v1/instructions/order/complete", g.handleGETOrderCompleteInstructions).Methods("POST")

		// 收款账户相关API
		r.HandleFunc("/v1/wallet/receivingaccountlist", g.GetReceivingAccounts).Methods("GET")
		r.HandleFunc("/v1/wallet/receivingaccount", g.AddReceivingAccount).Methods("POST")
		r.HandleFunc("/v1/wallet/receivingaccount", g.UpdateReceivingAccount).Methods("PUT")
		// Stripe连接URL
		r.HandleFunc("/v1/wallet/stripe/connect-url", g.GetStripeConnectURL).Methods("GET")

		// Orders
		r.HandleFunc("/v1/ob/purchases", g.handlePOSTPurchases).Methods("POST")
		r.HandleFunc("/v1/ob/purchases", g.handleGETPurchases).Methods("GET")
		r.HandleFunc("/v1/ob/sales", g.handleGETSales).Methods("GET")
		r.HandleFunc("/v1/ob/sales", g.handlePostSales).Methods("POST")
		r.HandleFunc("/v1/ob/cases", g.handleGETCases).Methods("GET")
		r.HandleFunc("/v1/ob/cases", g.handlePostCases).Methods("POST")

		r.HandleFunc("/v1/ob/case/{orderID}", g.handleGetCase).Methods("GET")
		r.HandleFunc("/v1/ob/order/{orderID}", g.handleGETOrder).Methods("GET")
		r.HandleFunc("/v1/ob/orderspend", g.handlePostSpendForOrder).Methods("POST")
		r.HandleFunc("/v1/ob/ordercancel", g.handlePOSTOrderCancel).Methods("POST")

		r.HandleFunc("/v1/order/purchase", g.handlePOSTPurchase).Methods("POST")
		r.HandleFunc("/v1/order/payment", g.handlePOSTPayment).Methods("POST")
		r.HandleFunc("/v1/order/confirm", g.handlePOSTOrderConfirmation).Methods("POST")
		r.HandleFunc("/v1/order/fulfill", g.handlePOSTOrderFulfillment).Methods("POST")
		r.HandleFunc("/v1/order/refund", g.handlePOSTOrderRefund).Methods("POST")
		r.HandleFunc("/v1/order/complete", g.handlePOSTOrderCompletion).Methods("POST")

		r.HandleFunc("/v1/ob/estimatetotal", g.handlePOSTEstimateTotal).Methods("POST")
		r.HandleFunc("/v1/ob/checkoutbreakdown", g.handlePOSTCheckoutBreakdown).Methods("POST")

		// Cart
		r.HandleFunc("/v1/ob/carts/itemsCount", g.handleGETCartsItemsCount).Methods("GET")
		r.HandleFunc("/v1/ob/carts", g.handleGETCarts).Methods("GET")
		r.HandleFunc("/v1/ob/carts", g.handleClearCarts).Methods("DELETE")
		r.HandleFunc("/v1/ob/carts/{peerID}/add", g.handleAddToCart).Methods("POST")
		r.HandleFunc("/v1/ob/carts/{peerID}/update", g.handleAddToCart).Methods("POST")
		r.HandleFunc("/v1/ob/carts/{peerID}/remove", g.handleRemoveCartItem).Methods("POST")

		// Following
		r.HandleFunc("/v1/ob/followsme/{peerID}", g.handleGETFollowsMe).Methods("GET")
		r.HandleFunc("/v1/ob/follow/{peerID}", g.handlePOSTFollow).Methods("POST")
		r.HandleFunc("/v1/ob/unfollow/{peerID}", g.handlePOSTUnFollow).Methods("POST")

		// Listings
		r.HandleFunc("/v1/ob/mylisting/{slugOrCID}", g.handleGETMyListing).Methods("GET")
		r.HandleFunc("/v1/ob/listing", g.handlePOSTListing).Methods("POST")
		r.HandleFunc("/v1/ob/listing", g.handlePUTListing).Methods("PUT")
		r.HandleFunc("/v1/ob/listing/{slug}", g.handleDELETEListing).Methods("DELETE")

		// Images
		r.HandleFunc("/v1/ob/avatar", g.handlePOSTAvatar).Methods("POST")
		r.HandleFunc("/v1/ob/header", g.handlePOSTHeader).Methods("POST")
		r.HandleFunc("/v1/ob/images", g.handlePOSTImages).Methods("POST")
		r.HandleFunc("/v1/ob/productimages", g.handlePOSTProductImage).Methods("POST")

		// File
		r.HandleFunc("/v1/ob/file", g.handlePOSTFile).Methods("POST")

		// Moderation
		r.HandleFunc("/v1/ob/opendispute", g.handlePOSTOpenDispute).Methods("POST")
		r.HandleFunc("/v1/ob/closedispute", g.handlePOSTCloseDispute).Methods("POST")
		r.HandleFunc("/v1/ob/releasefunds", g.handlePOSTReleaseFunds).Methods("POST")
		r.HandleFunc("/v1/ob/releaseescrow", g.handlePOSTReleaseEscrow).Methods("POST")

		// Profiles
		r.HandleFunc("/v1/ob/profile", g.handlePOSTProfile).Methods("POST")
		r.HandleFunc("/v1/ob/profile/{peerID}", g.handlePOSTProfile).Methods("POST")
		r.HandleFunc("/v1/ob/profile", g.handlePUTProfile).Methods("PUT")
		r.HandleFunc("/v1/ob/profile/{peerID}", g.handlePUTProfile).Methods("PUT")

		// Ratings

		// Posts
		r.HandleFunc("/v1/ob/post", g.handlePOSTPost).Methods("POST")
		r.HandleFunc("/v1/ob/post/{slug}", g.handleDELETEPost).Methods("DELETE")

		r.HandleFunc("/v1/ob/signmessage", g.handlePOSTSignMessage).Methods("POST")
		r.HandleFunc("/v1/ob/verifymessage", g.handlePOSTVerifyMessage).Methods("POST")
		r.HandleFunc("/v1/ob/hashmessage", g.handlePOSTHashMessage).Methods("POST")

		// Moderator
		r.HandleFunc("/v1/ob/moderator", g.handleSetModerator).Methods("POST")
		r.HandleFunc("/v1/ob/moderator", g.handleUnsetModerator).Methods("DELETE")
		r.HandleFunc("/v1/ob/moderators", g.handleGetModerators).Methods("GET")

		// Block a store
		r.HandleFunc("/v1/ob/blocknode/{peerID}", g.handleBlockNode).Methods("POST")
		r.HandleFunc("/v1/ob/blocknode/{peerID}", g.handleUnBlockNode).Methods("DELETE")

		r.HandleFunc("/v1/ob/config", g.handleGETConfig).Methods("GET")
		r.HandleFunc("/v1/wallet/currencies", g.handleGETCurrencies).Methods("GET")

		// Preferences
		r.HandleFunc("/v1/ob/preferences", g.handlePutUserPreferences).Methods("PUT")
		r.HandleFunc("/v1/ob/preferences", g.handleGetUserPreferences).Methods("GET")

		r.HandleFunc("/v1/ob/bulkupdatecurrency", g.handlePOSTBulkUpdateCurrency).Methods("POST")
		r.HandleFunc("/v1/ob/publish", g.handlePOSTPublish).Methods("POST")
		r.HandleFunc("/v1/ob/purgecache", g.handlePOSTPurgeCache).Methods("POST")
		r.HandleFunc("/v1/ob/shutdown", g.handlePOSTShutdown).Methods("POST")

		// Channels
		r.HandleFunc("/v1/ob/channelmessage", g.handlePOSTPublishChannelMessage).Methods("POST")
		r.HandleFunc("/v1/ob/openchannel/{topic}", g.handlePOSTOpenChannel).Methods("POST")
		r.HandleFunc("/v1/ob/closechannel/{topic}", g.handlePOSTCloseChannel).Methods("POST")
		r.HandleFunc("/v1/ob/channels", g.handleGETListChannels).Methods("GET")
		r.HandleFunc("/v1/ob/channelmessages/{topic}", g.handleGETChannelMessages).Methods("GET")
	}
	// Images
	r.HandleFunc("/v1/ob/image/{imageID}", g.handleGETImage).Methods("GET")
	r.HandleFunc("/v1/ob/avatar/{peerID}/{size}", g.handleGETAvatar).Methods("GET")
	r.HandleFunc("/v1/ob/header/{peerID}/{size}", g.handleGETHeader).Methods("GET")

	// File
	r.HandleFunc("/v1/ob/file/{fileID}", g.handleGETFile).Methods("GET")

	// Listings
	r.HandleFunc("/v1/ob/listing/{listingID}", g.handleGETListing).Methods("GET")
	r.HandleFunc("/v1/ob/listing/{peerID}/{slug}", g.handleGETListing).Methods("GET")
	r.HandleFunc("/v1/ob/listingindex/{peerID}", g.handleGETListingIndex).Methods("GET")
	r.HandleFunc("/v1/ob/listingindex", g.handleGETListingIndex).Methods("GET")

	// Profiles
	r.HandleFunc("/v1/ob/profile/{peerID}", g.handleGETProfile).Methods("GET")
	r.HandleFunc("/v1/ob/profile", g.handleGETProfile).Methods("GET")
	r.HandleFunc("/v1/ob/fetchprofiles", g.handlePOSTFetchProfiles).Methods("GET", "POST")

	// Ratings
	r.HandleFunc("/v1/ob/ratingindex/{peerIDOrSlug}", g.handleGETRatingIndex).Methods("GET")
	r.HandleFunc("/v1/ob/ratingindex", g.handleGETMyRatingIndex).Methods("GET")
	r.HandleFunc("/v1/ob/ratingindex/{peerID}/{slug}", g.handleGETPeerRatingsBySlug).Methods("GET")
	r.HandleFunc("/v1/ob/rating/{ratingID}", g.handleGETRating).Methods("GET")
	r.HandleFunc("/v1/ob/fetchratings", g.handlePOSTFetchRatings).Methods("POST")

	// Posts
	r.HandleFunc("/v1/ob/post/{slug}", g.handleGETMyPost).Methods("GET")
	r.HandleFunc("/v1/ob/post/{peerID}/{slug}", g.handleGETPost).Methods("GET")

	// Following
	r.HandleFunc("/v1/ob/followers/{peerID}", g.handleGETFollowers).Methods("GET")
	r.HandleFunc("/v1/ob/followers", g.handleGETFollowers).Methods("GET")
	r.HandleFunc("/v1/ob/following/{peerID}", g.handleGETFollowing).Methods("GET")
	r.HandleFunc("/v1/ob/following", g.handleGETFollowing).Methods("GET")

	r.HandleFunc("/v1/ob/exchangerates/{currencyCode}", g.handleGETExchangeRates).Methods("GET")
	r.HandleFunc("/v1/ob/exchangerates", g.handleGETExchangeRates).Methods("GET")

	r.HandleFunc("/v1/ob/peers", g.handleGETPeers).Methods("GET")
	return r
}

func wrapError(err error) string {
	return fmt.Sprintf(`{"error": "%s"}`, err.Error())
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
