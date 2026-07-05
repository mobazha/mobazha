// Package contracts defines aggregate service interfaces for the Mobazha node.
//
// These interfaces represent focused business capabilities. MobazhaNode is the
// sole implementation; SaaS and standalone modes differ only by which infrastructure
// adapters (Ports) are injected at construction time.
//
// Design principles:
//   - Use only types from pkg/ (not internal/) so external consumers can depend on them
//   - Each interface covers a single business domain
//   - NodeService aggregates all domain interfaces into a single composite
//   - MobazhaNode implements NodeService (standalone and SaaS via adapter injection)
//
// Architecture: see docs/ARCHITECTURE_ROADMAP.md for the hexagonal evolution plan.
package contracts

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/peer"
	pkgdb "github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/events"
	"github.com/mobazha/mobazha/pkg/extensions"
	"github.com/mobazha/mobazha/pkg/models"
	npb "github.com/mobazha/mobazha/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha/pkg/payment"
	postsPb "github.com/mobazha/mobazha/pkg/posts/pb"
	"github.com/mobazha/mobazha/pkg/request"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"github.com/mobazha/mobazha/pkg/webhook"
)

// CoTenantPublicDataFn resolves PublicData for a co-located tenant on the same
// SaaS host. Returns an error if the peerID is not a co-tenant. When nil
// (standalone mode), callers fall through to the normal NetDB/IPNS path.
type CoTenantPublicDataFn func(peerID peer.ID) (pkgdb.PublicData, error)

// CoTenantDigitalAssetsFn resolves the digital asset service for a co-located
// tenant on the same SaaS host. It is nil outside shared-host deployments.
type CoTenantDigitalAssetsFn func(peerID peer.ID) (DigitalAssetService, error)

// CoTenantVerifiedPaymentFn routes a verified PAYMENT_SENT message to the
// target tenant's own order service on the same SaaS host.
type CoTenantVerifiedPaymentFn func(ctx context.Context, tenantID string, orderMsg *npb.OrderMessage, tx iwallet.Transaction) bool

// IdentityService provides node identity and lifecycle operations.
type IdentityService interface {
	// GetNodeID returns the unique identifier for this node/tenant.
	GetNodeID() string

	// Identity returns the libp2p peer ID.
	Identity() peer.ID

	// UsingTestnet returns whether the node is on testnet.
	UsingTestnet() bool

	// SignMessage signs a payload with the node's identity key.
	// Returns (signature, publicKeyBytes, error).
	// Standalone: uses libp2p identity private key.
	// SaaS: delegates to KeyVault.
	SignMessage(payload []byte) ([]byte, []byte, error)

	// IsGlobalBanned checks if a peer is globally banned.
	IsGlobalBanned(peerID peer.ID) bool
}

// NotificationService handles user notifications.
type NotificationService interface {
	GetNotifications(offsetID string, limit int, typeFilters []string) ([]models.NotificationRecord, int64, error)
	MarkNotificationAsRead(notifID string) error
	MarkAllNotificationsAsRead() error
	BatchMarkNotificationsAsRead(ids []string) error
	BatchDeleteNotifications(ids []string) error
	GetNotificationsUnreadCount() (int, error)
	GetNotificationsTotalCount() (int64, error)
}

// OrderService handles order lifecycle management.
type OrderService interface {
	PurchaseListing(ctx context.Context, purchase *models.Purchase) (orderID models.OrderID, paymentAmount models.CurrencyValue, err error)
	EstimateOrderTotal(ctx context.Context, purchase *models.Purchase) (models.OrderTotals, error)
	GetOrderInfo(orderID models.OrderID, coinType iwallet.CoinType) (*models.OrderInfo, error)
	ProcessOrderPayment(ctx context.Context, paymentData *models.PaymentData) error
	DeclineOrder(orderID models.OrderID, txid iwallet.TransactionID, reason string, done chan struct{}) error
	RefundOrder(orderID models.OrderID, txid iwallet.TransactionID, done chan struct{}) error
	ConfirmOrder(orderID models.OrderID, txid iwallet.TransactionID, payoutAddress string, done chan struct{}) error
	// GetConfirmOrderInstructions is a legacy client-signed-only surface.
	// backend-managed EVM and other backend-submitted settlement routes must use
	// ExecuteSettlementAction instead of the old instructions flow.
	GetConfirmOrderInstructions(orderID models.OrderID, initiatorAddress string, payoutAddress string) (coinType iwallet.CoinType, instructions any, err error)
	// ExecuteSettlementAction runs backend-driven settlement intents
	// (confirm / cancel) via ChainEscrowV2. Client-signed legacy chains
	// are intentionally excluded from this surface.
	ExecuteSettlementAction(ctx context.Context, action string, orderID models.OrderID, payoutAddr string) (*payment.ActionResult, iwallet.CoinType, error)
	// GetSettlementActionStatus returns the latest known lifecycle state for a
	// previously issued settlement action.
	GetSettlementActionStatus(ctx context.Context, action string, orderID models.OrderID, actionID string) (*payment.ActionStatus, iwallet.CoinType, error)
	// GetRefundOrderInstructions is a legacy instructions surface. It remains
	// valid for client-signed chains and fiat informational responses only.
	// backend-managed EVM refund/cancel flows must use ExecuteSettlementAction.
	GetRefundOrderInstructions(orderID models.OrderID, initiatorAddress string) (coinType iwallet.CoinType, instructions any, err error)
	ShipOrder(orderID models.OrderID, shipments []models.Shipment, done chan struct{}) error
	// GetCompleteOrderInstructions is a legacy instructions surface for
	// client-signed moderated completion flows only. backend-managed moderated
	// completion stays on the backend-owned completion path instead of the
	// old instructions contract.
	GetCompleteOrderInstructions(orderID models.OrderID, initiatorAddress string) (coinType iwallet.CoinType, instructions any, err error)
	CompleteOrder(orderID models.OrderID, txid iwallet.TransactionID, ratings []models.Rating, includeIDInRating bool, done chan struct{}) error
	RateOrder(orderID models.OrderID, ratings []models.Rating, includeIDInRating bool, done chan struct{}) error
	CancelOrder(orderID models.OrderID, txid iwallet.TransactionID, done chan struct{}) error
	ExtendProtection(orderID models.OrderID) (*models.OrderProtectionInfo, error)

	// ViaRelay methods: combine get-instructions + relay-execute + action into a single call.
	// Used by hosting mode where there is no frontend wallet (AppKit) to sign transactions.
	// For UTXO chains, these fall through to the standard methods (backend handles signing).
	// For EVM/Solana, these build instructions, relay via platform gas wallet, then complete the action.
	// Returns ErrRelayNotAvailable if relay service is not configured.
	RefundOrderViaRelay(orderID models.OrderID, done chan struct{}) error
	DeclineOrderViaRelay(orderID models.OrderID, reason string, done chan struct{}) error
	CancelOrderViaRelay(orderID models.OrderID, done chan struct{}) error

	GetOrder(orderID string) (*models.Order, error)
	GetPurchases(stateFilters []models.OrderState, searchTerm string, sortByAscending bool, sortByRead bool, limit int, exclude []string) ([]models.Order, int64, error)
	GetSales(stateFilters []models.OrderState, searchTerm string, sortByAscending bool, sortByRead bool, limit int, exclude []string) ([]models.Order, int64, error)
	GetCase(orderID string) (*models.Case, error)
	GetCases(stateFilters []models.OrderState, searchTerm string, sortByAscending bool, sortByRead bool, limit int, exclude []string) ([]models.Case, int64, error)

	// Disputes
	OpenDispute(orderID models.OrderID, reason string, evidenceHashes []string, done chan struct{}) error
	OpenAfterSaleDispute(orderID models.OrderID, reason string, description string) error
	CloseDispute(orderID models.OrderID, buyerPercentage, vendorPercentage float32, resolution string, done chan struct{}) error
	// GetReleaseFundsInstructions remains a legacy instruction endpoint for
	// client-signed moderated payouts. backend-managed moderated payouts should
	// flow through backend close/release handling and escrow owner-signature
	// paths instead of the old instructions contract.
	GetReleaseFundsInstructions(orderID models.OrderID, initiatorAddress string) (coinType iwallet.CoinType, instructions any, err error)
	ReleaseFunds(orderID models.OrderID, txid iwallet.TransactionID, done chan struct{}) error
	ReleaseFundsAfterTimeout(orderID models.OrderID, done chan struct{}) error

	// Request address from a remote peer
	RequestAddress(ctx context.Context, to peer.ID, coinType iwallet.CoinType) (iwallet.Address, error)

	// SetOrderRefundAddressForPayment validates and persists the buyer-controlled
	// crypto refund destination for an order once the payout coin is known.
	SetOrderRefundAddressForPayment(ctx context.Context, orderID string, coin iwallet.CoinType, refundAddr string) error

	// QuoteCheckoutSupply performs a buyer-safe advisory supply preflight for
	// authenticated standard checkout without creating an order or holding inventory.
	QuoteCheckoutSupply(ctx context.Context, req QuoteCheckoutSupplyRequest) (*CheckoutSupplyQuoteResponse, error)
	// SummarizeListingSupply performs a seller-safe advisory supply summary for
	// authenticated admin product surfaces without creating holds.
	SummarizeListingSupply(ctx context.Context, req ListingSupplySummaryRequest) (*ListingSupplySummaryResponse, error)
}

// ConditionalSettlementService is the narrow extension-facing settlement
// surface. It is separate from OrderService so adding attestation versions
// does not expand every order-service implementation.
type ConditionalSettlementService interface {
	ExecuteConditionalSettlement(context.Context, extensions.SettlementAttestation) (*payment.ActionResult, iwallet.CoinType, error)
}

// ListingService handles product listing management.
type ListingService interface {
	SaveListing(listing *pb.Listing, done chan<- struct{}) error
	UpdateAllListings(updateFunc func(l *pb.Listing) (bool, error), done chan<- struct{}) error
	DeleteListing(slug string, done chan<- struct{}) error
	GetMyListings() (models.ListingIndex, error)
	GetListings(ctx context.Context, peerID peer.ID, reqCtx *request.Context, useCache bool) (models.ListingIndex, error)
	GetMyListingBySlug(slug string) (*pb.SignedListing, error)
	GetMyListingByCID(cid cid.Cid) (*pb.SignedListing, error)
	GetListingBySlug(ctx context.Context, peerID peer.ID, slug string, reqCtx *request.Context, useCache bool) (*pb.SignedListing, error)
	GetListingByCID(ctx context.Context, cid cid.Cid, reqCtx *request.Context) (*pb.SignedListing, error)
}

// ProfileService handles user profile management and moderation.
type ProfileService interface {
	SetProfile(profile *models.Profile, done chan<- struct{}) error
	GetMyProfile() (*models.Profile, error)
	GetProfileStats() (*models.ProfileStats, error)
	GetProfile(ctx context.Context, peerID peer.ID, reqCtx *request.Context, useCache bool) (*models.Profile, error)

	// Moderation
	SetSelfAsModerator(ctx context.Context, modInfo *models.ModeratorInfo, done chan struct{}) error
	RemoveSelfAsModerator(ctx context.Context, done chan<- struct{}) error
	GetModerators(ctx context.Context) []peer.ID
	GetModeratorsAsync(ctx context.Context) <-chan peer.ID
	GetVerifiedModerators(ctx context.Context) []peer.ID
}

// WalletService provides wallet capabilities — key signing and multisig address generation.
// It does NOT include chain client operations (those go through shared infrastructure).
type WalletService interface {
	// GetMnemonic returns the mnemonic seed phrase.
	GetMnemonic() (string, error)

	// SaveTransactionMetadata saves metadata for a transaction.
	SaveTransactionMetadata(metadata *models.TransactionMetadata) error

	// GetTransactionMetadata retrieves metadata for a transaction.
	GetTransactionMetadata(txid iwallet.TransactionID) (models.TransactionMetadata, error)

	// Escrow operations
	GeneratePaymentInstructions(ctx context.Context, params models.InitializeEscrowData) (*payment.PaymentSetupResult, error)
	BuildInitEscrowInstructions(ctx context.Context, params models.InitializeEscrowData) (*models.PaymentData, iwallet.Address, any, error)
	GetUTXOPaymentInfo(ctx context.Context, orderID string, moderator string, coinType iwallet.CoinType) (*models.PaymentData, error)
	GetTotalPaidToAddress(order *models.Order) (uint64, error)
	CancelPartialPayment(orderID string) (txid string, refundedAmount uint64, err error)
	StopWatchingPayment(orderID string) error
}

// ReceivingAccountService manages external wallet addresses for receiving
// payments. Lightweight — no multiwallet or escrow dependency. Pure DB CRUD.
type ReceivingAccountService interface {
	Add(account *models.ReceivingAccount) (*models.ReceivingAccount, error)
	Update(account *models.ReceivingAccount) (*models.ReceivingAccount, error)
	Delete(id int) error
	List() ([]models.ReceivingAccount, error)
	GetByID(id int) (*models.ReceivingAccount, error)
	GetActive(chainType iwallet.ChainType) (*models.ReceivingAccount, error)
	GetByChain(chainType iwallet.ChainType) ([]models.ReceivingAccount, error)
	GetAcceptedCurrencies() ([]string, error)
}

// MediaService handles media (images, videos, files) storage and retrieval.
//
// Phase 1 refactored from 9 methods to 4. Handlers are responsible for
// base64 decoding; this interface only accepts raw []byte.
type MediaService interface {
	// UploadMedia stores raw file bytes. When opts.Variants is true,
	// 5 resized image copies (tiny/small/medium/large/original) are generated.
	UploadMedia(ctx context.Context, data []byte, filename string, opts UploadOpts) (*UploadResult, error)

	// GetMedia retrieves any media by CID.
	// Fallback: BlobStore → DB (legacy).
	GetMedia(ctx context.Context, cid cid.Cid) (io.ReadSeeker, string, error)

	// SetProfileMedia uploads an image for a profile slot (avatar/header),
	// generates variants, updates the profile record, and publishes.
	SetProfileMedia(ctx context.Context, slot ProfileSlot, imageData []byte) (*UploadResult, error)

	// GetProfileMedia retrieves a profile image (avatar/header) for the
	// given peer at the requested size.
	GetProfileMedia(ctx context.Context, peerID peer.ID, slot ProfileSlot, size models.ImageSize, useCache bool) (io.ReadSeeker, error)
}

// SocialService handles following, ratings, and posts.
type SocialService interface {
	// Following
	FollowNode(peerID peer.ID, done chan<- struct{}) error
	UnfollowNode(peerID peer.ID, done chan<- struct{}) error
	FollowsMe(peerID peer.ID) (bool, error)
	GetMyFollowers() (models.Followers, error)
	GetMyFollowing() (models.Following, error)
	GetFollowers(ctx context.Context, peerID peer.ID, reqCtx *request.Context, useCache bool) (models.Followers, error)
	GetFollowing(ctx context.Context, peerID peer.ID, reqCtx *request.Context, useCache bool) (models.Following, error)

	// Ratings
	GetMyRatings() (models.RatingIndex, error)
	GetRatings(ctx context.Context, peerID peer.ID, reqCtx *request.Context, useCache bool) (models.RatingIndex, error)
	GetRating(ctx context.Context, cid cid.Cid) (*pb.Rating, error)

	// Posts
	AddPost(post *postsPb.Post, done chan<- struct{}) error
	DeletePost(slug string, done chan<- struct{}) error
	PostExist(slug string) bool
	GetMyPosts() ([]models.PostData, error)
	GetMyPostBySlug(slug string) (*postsPb.SignedPost, error)
	GetPostBySlug(ctx context.Context, peerID peer.ID, slug string, useCache bool) (*postsPb.SignedPost, error)
	GetPosts(ctx context.Context, peerID peer.ID, useCache bool) ([]models.PostData, error)
}

// PreferencesService handles user preferences.
type PreferencesService interface {
	GetPreferences() (*models.UserPreferences, error)
	SavePreferences(prefs *models.UserPreferences, done chan struct{}) error
	SetBillingHold(h models.BillingHold) error
	BlockNode(peerID string) (bool, error)
	UnblockNode(peerID string) (bool, error)
}

// ExchangeRateService provides currency exchange rate queries.
// Both standalone and SaaS modes need exchange rates:
//   - Standalone: MobazhaNode delegates to internal ExchangeRateProvider
//   - SaaS: TenantService receives a shared ExchangeRateService via SharedInfra
type ExchangeRateService interface {
	// GetAllRates returns all exchange rates for the given base currency.
	// If breakCache is true, forces a refresh from providers.
	GetAllRates(base models.CurrencyCode, breakCache bool) (map[models.CurrencyCode]iwallet.Amount, error)

	// GetRate returns the rate for a specific currency pair.
	// Supports crypto-fiat, fiat-crypto, and fiat-fiat pairs.
	GetRate(base models.CurrencyCode, to models.CurrencyCode, breakCache bool) (iwallet.Amount, error)

	// LastUpdated returns the source timestamp of the current snapshot for the
	// base currency. Consumers forwarding rates across runtime boundaries must
	// preserve this value instead of replacing it with response time.
	LastUpdated(base models.CurrencyCode) time.Time
}

// ProviderHealthInfo contains health metrics for an exchange rate provider.
type ProviderHealthInfo struct {
	Name         string    `json:"name"`
	LastSuccess  time.Time `json:"last_success"`
	LastError    string    `json:"last_error,omitempty"`
	LastErrorAt  time.Time `json:"last_error_at,omitempty"`
	SuccessCount int64     `json:"success_count"`
	ErrorCount   int64     `json:"error_count"`
}

// ErrWishlistFull is returned when the wishlist reaches its capacity limit.
var ErrWishlistFull = errors.New("wishlist is full")

// WishlistService handles buyer wishlist operations.
type WishlistService interface {
	GetWishlist() ([]models.WishlistItem, error)
	AddToWishlist(item models.WishlistItem) (*models.WishlistItem, error)
	RemoveFromWishlist(vendorPeerID, slug string) error
	WishlistCount() (int, error)
}

// ShoppingCartService handles shopping cart operations.
type ShoppingCartService interface {
	GetCartsTotalItemsCount() (int, error)
	GetCarts() ([]models.StoreCart, error)
	AddToCart(peerID peer.ID, item models.ShoppingCartItem) error
	RemoveCartItem(peerID peer.ID, item models.ShoppingCartItem) error
	ClearCarts(vendorID peer.ID) error
	ClearAllCarts() error
}

// WebhookProvider exposes the per-node webhook subsystem (store + engine).
// Handlers obtain this via type assertion on NodeService.
type WebhookProvider interface {
	WebhookStore() webhook.EndpointStore
	WebhookEngine() *webhook.Engine
}

// DiscountProvider exposes the per-node discount subsystem.
// Handlers obtain this via type assertion on NodeService.
type DiscountProvider interface {
	Discount() DiscountService
}

// CollectionProvider exposes the per-node collection subsystem.
// Handlers obtain this via type assertion on NodeService.
type CollectionProvider interface {
	Collection() CollectionService
}

// AnalyticsService handles visitor event tracking and stats aggregation.
type AnalyticsService interface {
	TrackEvent(evt models.AnalyticsEvent) error
	TrackEvents(events []models.AnalyticsEvent) error
	GetVisitorStats(days int) (any, error)
}

// AnalyticsProvider exposes the per-node analytics subsystem.
// Handlers obtain this via type assertion on NodeService.
type AnalyticsProvider interface {
	Analytics() AnalyticsService
}

// PaymentDetectedOpts carries chain-specific metadata for HandlePaymentDetected.
// nil for chains that don't need extra metadata (UTXO/EVM/Solana — txHash
// alone identifies the payment). Currently only Monero populates this:
// XMR has no global tx index, so the watcher must communicate the block
// height for downstream confirmation polling. If a second chain needs
// chain-specific metadata, evaluate switching to a typed-per-chain method
// (e.g. HandleXMRPaymentConfirmed) or a generic map[string]any.
type PaymentDetectedOpts struct {
	// TxBlockHeight is the block height of the confirmed XMR transfer.
	// 0 means the transfer was first observed in the mempool (pool phase).
	TxBlockHeight uint64
}

// GuestOrderService exposes Guest Checkout order lifecycle operations.
// Anonymous buyers interact via HTTP (no P2P, no escrow).
//
// Handlers that need typed request/response objects use type assertion to the
// concrete *GuestOrderAppService (same pattern as WebhookProvider).
type GuestOrderService interface {
	// IsEnabled reports whether Guest Checkout is currently enabled for the
	// caller's evaluation scope (platform → tenant → node runtime). Handlers
	// and views should gate writes and list-filter against this rather than
	// consulting the legacy GuestCheckoutConfig.Enabled field.
	IsEnabled(ctx context.Context) bool

	QuoteGuestOrderSupply(ctx context.Context, req QuoteGuestOrderSupplyRequest) (*GuestOrderSupplyQuoteResponse, error)
	CreateGuestOrder(ctx context.Context, req CreateGuestOrderRequest) (*GuestOrderResponse, error)
	GetGuestOrderStatus(ctx context.Context, token string) (*GuestOrderStatusResponse, error)
	ListGuestOrders(ctx context.Context, filter GuestOrderFilter) ([]models.GuestOrder, int64, error)
	ShipGuestOrder(ctx context.Context, token string, tracking, carrier string) error
	CompleteGuestOrder(ctx context.Context, token string) error
	HandlePaymentDetected(orderToken, txHash string, opts *PaymentDetectedOpts) error
	HandleConfirmationUpdate(orderToken string, confs int) error
	// HandlePoolPayment records a mempool-only payment observation (currently
	// XMR-only). It does NOT change order state — the order remains in
	// AWAITING_PAYMENT until the transfer is mined and HandlePaymentDetected
	// fires. This preserves the invariant that PAYMENT_DETECTED implies an
	// on-chain tx, while still surfacing a "we saw your pool tx" hint to
	// the buyer via GetGuestOrderStatus. poolAmount is in atomic units.
	HandlePoolPayment(orderToken, txHash string, poolAmount uint64) error
	// HandleLatePayment records a payment that arrived but cannot fund the
	// order (partial / overpay / received after expiry). It persists the
	// txHash for seller-side recovery without changing the order state, so
	// the natural lifecycle (CleanupExpiredOrders) still applies. The status
	// argument is a free-form classifier (e.g. "partial", "overpay", "expired").
	HandleLatePayment(orderToken, txHash, status string, paid, expected uint64) error
	CleanupExpiredOrders(ctx context.Context)
	AutoCompleteOrders(ctx context.Context)
	RunGuestCleanupOnce()

	GetGuestCheckoutConfig(ctx context.Context) (*models.GuestCheckoutConfig, error)
	SaveGuestCheckoutConfig(ctx context.Context, cfg *models.GuestCheckoutConfig) error
	GetGuestCheckoutReadiness(ctx context.Context) (*GuestCheckoutReadiness, error)

	// GetAdminGuestOrder returns full order detail for the authenticated seller,
	// including the raw ShippingAddress bytes (which may be PGP ciphertext).
	// The public GetGuestOrderStatus endpoint must NOT expose this data.
	GetAdminGuestOrder(ctx context.Context, token string) (*models.GuestOrder, error)
}

// NodeService is the top-level aggregate interface that combines all domain services.
// Both MobazhaNode (standalone) and TenantService (SaaS) implement this interface.
//
// Design: Service Accessor pattern — each domain is exposed via a typed accessor
// method (e.g. Order() OrderService) rather than flat embedding. This eliminates
// ~130 pass-through delegates on the implementor and ensures new domain methods
// never require changes to NodeService or its implementors.
//
// Note: IdentityInfo() is named to avoid conflict with IdentityService.Identity().
type NodeService interface {
	// Domain service accessors
	IdentityInfo() IdentityService
	Notification() NotificationService
	Order() OrderService
	ConditionalSettlement() ConditionalSettlementService
	Listing() ListingService
	Profile() ProfileService
	Wallet() WalletService
	Media() MediaService
	Social() SocialService
	MatrixChat() MatrixChatService
	Preferences() PreferencesService
	ExchangeRate() ExchangeRateService
	ShoppingCart() ShoppingCartService
	Wishlist() WishlistService
	GuestOrder() GuestOrderService
	ReceivingAccounts() ReceivingAccountService
	// PaymentSession returns the unified payment session service (Phase PS / B1).
	// Returns nil when the selected profile does not compose the subsystem.
	PaymentSession() PaymentSessionService

	// Cross-cutting methods (kept directly on NodeService)

	// EventBus returns the event bus for pub/sub.
	EventBus() events.Bus

	// Publish publishes the node's data to the network.
	Publish(done chan<- struct{})

	// PingNode pings a remote peer.
	PingNode(ctx context.Context, peer peer.ID) error

	// SubscribeEvent subscribes to a specific event type.
	SubscribeEvent(event any) (events.Subscription, error)
}

// PaymentRPCStatusProvider is implemented by nodes that can report local
// payment sidecar availability (for example Sovereign's monero-wallet-rpc).
type PaymentRPCStatusProvider interface {
	PaymentRPCStatus(ctx context.Context) PaymentRPCStatus
}

type PaymentRPCStatus struct {
	XMR *PaymentRPCStatusEntry `json:"xmr,omitempty"`
}

type PaymentRPCStatusEntry struct {
	Connected    bool   `json:"connected"`
	Endpoint     string `json:"endpoint,omitempty"`
	AccountIndex uint32 `json:"accountIndex,omitempty"`
	BlockHeight  uint64 `json:"blockHeight,omitempty"`
	Error        string `json:"error,omitempty"`
}

// MoneroNodePoolProvider is implemented by nodes that expose the Monero
// daemon node pool for admin / setup wizard management (Sovereign only).
//
// All methods are safe to call when the pool is not configured (e.g. the
// sovereign is in legacy single-daemon mode or NodePool bootstrap failed) —
// MoneroNodes returns an empty snapshot and mutating methods return
// ErrMoneroNodePoolUnavailable.
type MoneroNodePoolProvider interface {
	MoneroNodes(ctx context.Context) MoneroNodePoolSnapshot
	AddMoneroNode(ctx context.Context, req MoneroNodeAddRequest) (MoneroNodeInfo, error)
	RemoveMoneroNode(ctx context.Context, address string) error
	SwitchMoneroNode(ctx context.Context, address string) error
}

// ErrMoneroNodePoolUnavailable signals that the sovereign is not running with
// a NodePool (legacy single-daemon mode, or bootstrap failed).
var ErrMoneroNodePoolUnavailable = errors.New("monero NodePool: not available on this node")

// MoneroNodePoolSnapshot is the JSON envelope returned by
// GET /v1/system/monero-nodes.
type MoneroNodePoolSnapshot struct {
	// Available indicates whether a NodePool is wired up for this node.
	// When false, Active/Candidates are empty and write operations will
	// return ErrMoneroNodePoolUnavailable.
	Available bool `json:"available"`

	// Healthy is the NodePool.IsHealthy() verdict (active node bound,
	// not Suspicious, fail-streak under threshold). Independent of
	// candidate count.
	Healthy bool `json:"healthy"`

	// MonitorOn indicates the background MonitorLoop is running. False
	// briefly during startup and after StopMonitor / lifecycle shutdown.
	MonitorOn bool `json:"monitorOn"`

	// Active is the daemon currently bound by wallet-rpc, if any.
	Active *MoneroNodeInfo `json:"active,omitempty"`

	// Candidates is the full pool snapshot in insertion order
	// (seed-embedded → discovered → user-added). The Active node also
	// appears here.
	Candidates []MoneroNodeInfo `json:"candidates"`
}

// MoneroNodeInfo is the read-only snapshot of a pool candidate.
type MoneroNodeInfo struct {
	Address       string `json:"address"`
	Operator      string `json:"operator,omitempty"`
	Source        string `json:"source"` // seed-embedded / discovered / user-added
	SuccessStreak int    `json:"successStreak"`
	FailStreak    int    `json:"failStreak"`
	Suspicious    bool   `json:"suspicious"`
	LastChecked   string `json:"lastChecked,omitempty"` // RFC3339; empty if never checked
}

// MoneroNodeAddRequest is the body for POST /v1/system/monero-nodes.
type MoneroNodeAddRequest struct {
	// Address is the I2P / Tor / clearnet host:port of monerod RPC, e.g.
	// "node.example.b32.i2p:18089". Required.
	Address string `json:"address"`
	// Operator is a human-readable label (e.g. "MoneroWorld"). Optional.
	Operator string `json:"operator,omitempty"`
}

// MoneroWalletProvider is implemented by nodes that expose XMR wallet-level
// operations (balance / transfer / sweep_all). Available only on sovereign
// builds with a working monero-wallet-rpc sidecar.
type MoneroWalletProvider interface {
	GetXMRBalance(ctx context.Context, accountIndex *uint32) (MoneroBalance, error)
	WithdrawXMR(ctx context.Context, req MoneroWithdrawRequest) (MoneroWithdrawResult, error)
	SweepAllXMR(ctx context.Context, req MoneroSweepAllRequest) (MoneroSweepAllResult, error)
}

// MoneroBalance reports the account-level balance for the XMR wallet.
//
// Balance is the total (locked + unlocked) and UnlockedBalance is the
// portion that can be spent right now — Monero locks every incoming
// output for 10 confirmations (~20 min) and temporarily locks change
// outputs after sends. UI must withdraw against UnlockedBalance, never
// Balance, or wallet-rpc will reject the transfer.
//
// Both fields are decimal piconero strings (same JS-Number rationale as
// MoneroWithdrawRequest.Amount). BlocksToUnlock is the wallet-rpc hint
// for the next-batch unlock countdown; 0 when nothing is pending.
//
// AccountIndex echoes which account this balance refers to so the
// frontend doesn't have to track the request/response pairing itself.
type MoneroBalance struct {
	Balance         string `json:"balance"`
	UnlockedBalance string `json:"unlockedBalance"`
	BlocksToUnlock  uint64 `json:"blocksToUnlock,omitempty"`
	AccountIndex    uint32 `json:"accountIndex"`
}

// ErrMoneroWalletUnavailable signals that the sovereign node has no
// configured monero-wallet-rpc client (legacy boot without XMR config,
// or wallet RPC connection failed during startup).
var ErrMoneroWalletUnavailable = errors.New("monero wallet: RPC client not available on this node")

// ErrXMRInvalidAddress is wrapped by validation failures on the XMR
// destination address (empty / wrong length). Handlers unwrap with
// errors.Is to map to HTTP 400. The wrapping error supplies the
// human-readable reason; this sentinel only carries the prefix.
var ErrXMRInvalidAddress = errors.New("xmr address")

// ErrXMRInvalidAmount is wrapped by validation failures on the XMR
// amount field (non-numeric, zero, overflow). Handlers unwrap with
// errors.Is to map to HTTP 400.
var ErrXMRInvalidAmount = errors.New("xmr amount")

// MoneroWithdrawRequest is the body for POST /v1/wallet/xmr/withdraw.
//
// Amount is a decimal string of piconero (1 XMR = 10^12 piconero). It is a
// string instead of uint64 because JavaScript Number's safe-integer range
// (2^53 ≈ 9.007e15 piconero ≈ 9007 XMR) is below the realistic XMR balance
// of a long-running sovereign — using uint64 over the wire would silently
// truncate large withdrawals. This matches the existing models.SpendRequest
// convention for UTXO/EVM chains.
//
// Priority: 0=default (wallet decides), 1=unimportant, 2=normal,
// 3=elevated, 4=priority. Higher priority => higher fee, faster inclusion.
//
// AccountIndex is a pointer so the wire can distinguish "unset" (use the
// node's startup-flag default) from an explicit 0 (the primary account on
// every standard Monero wallet). Most callers send a non-nil 0 or omit it
// entirely; multi-account sovereigns may target specific indices.
type MoneroWithdrawRequest struct {
	Address      string  `json:"address"`
	Amount       string  `json:"amount"`
	Priority     uint32  `json:"priority,omitempty"`
	AccountIndex *uint32 `json:"accountIndex,omitempty"`
}

// MoneroWithdrawResult is the response payload for a successful withdrawal.
// Amount + Fee are decimal piconero strings (same rationale as the request).
// TxKey lets the sender prove the payment off-chain to the recipient; the
// frontend should surface it as "Save this key — only share with the
// recipient if proof is needed".
type MoneroWithdrawResult struct {
	TxHash string `json:"txHash"`
	TxKey  string `json:"txKey,omitempty"`
	Amount string `json:"amount"`
	Fee    string `json:"fee"`
}

// MoneroSweepAllRequest is the body for POST /v1/wallet/xmr/sweep-all.
//
// SubaddrIndices, when non-empty, restricts the sweep to the listed
// subaddress minor indices of AccountIndex. Empty means sweep all
// subaddresses of the account.
//
// AccountIndex is a pointer for the same reason as MoneroWithdrawRequest.
type MoneroSweepAllRequest struct {
	Address        string   `json:"address"`
	Priority       uint32   `json:"priority,omitempty"`
	AccountIndex   *uint32  `json:"accountIndex,omitempty"`
	SubaddrIndices []uint32 `json:"subaddrIndices,omitempty"`
}

// MoneroSweepAllResult lists the transactions produced by a sweep.
// sweep_all commonly produces multiple transactions when the wallet
// has many outputs; all parallel slices have the same length.
// Amounts / Fees are decimal piconero strings.
type MoneroSweepAllResult struct {
	TxHashes []string `json:"txHashes"`
	TxKeys   []string `json:"txKeys,omitempty"`
	Amounts  []string `json:"amounts"`
	Fees     []string `json:"fees"`
}

// MoneroWalletSetupProvider is implemented by nodes that expose the
// first-run wallet provisioning surface for XMR. It is admin-only and
// available only through a private distribution module with a working
// monero-wallet-rpc sidecar.
//
// Lifecycle from a fresh wallet-rpc process:
//  1. GetXMRWalletSetupStatus reports Exists=false
//  2. CreateXMRWallet (or RestoreXMRWallet) provisions the wallet,
//     persists local metadata, and returns the 25-word seed
//  3. Frontend shows seed + backup quiz, then calls ConfirmXMRWalletBackup
//  4. On every subsequent boot, the node auto-opens the wallet using the
//     metadata; GetXMRWalletSetupStatus reports Exists=true.
type MoneroWalletSetupProvider interface {
	GetXMRWalletSetupStatus(ctx context.Context) (MoneroWalletSetupStatus, error)
	CreateXMRWallet(ctx context.Context, req MoneroCreateWalletRequest) (MoneroCreateWalletResult, error)
	RestoreXMRWallet(ctx context.Context, req MoneroRestoreWalletRequest) (MoneroRestoreWalletResult, error)
	ConfirmXMRWalletBackup(ctx context.Context) error
}

// MoneroWalletSetupStatus is the response payload for
// GET /v1/system/setup-wizard/xmr-wallet.
//
// Exists reflects on-disk metadata (xmr-wallet.json) — it does NOT round-
// trip to wallet-rpc, so transient RPC outages don't make the wizard
// reappear and prompt the operator to overwrite. WalletOpen is the
// best-effort runtime signal that wallet-rpc currently has the wallet
// loaded (true after a successful open_wallet at startup or right after
// create/restore).
type MoneroWalletSetupStatus struct {
	Exists          bool   `json:"exists"`
	WalletOpen      bool   `json:"walletOpen"`
	Address         string `json:"address,omitempty"`
	BackupConfirmed bool   `json:"backupConfirmed"`
	CreatedAt       int64  `json:"createdAt,omitempty"`
}

// MoneroCreateWalletRequest is the body for the "create" action of
// POST /v1/system/setup-wizard/xmr-wallet. Language picks the seed
// wordlist; the MVP supports only "English" to keep the backup
// verification UI simple.
type MoneroCreateWalletRequest struct {
	Language string `json:"language,omitempty"`
}

// MoneroCreateWalletResult returns the new seed + address. The seed MUST
// be displayed to the operator exactly once — the server never persists
// it, and re-fetching after the wizard finishes would require querying
// wallet-rpc with admin auth which we explicitly do not expose.
type MoneroCreateWalletResult struct {
	Mnemonic string `json:"mnemonic"`
	Address  string `json:"address"`
}

// MoneroRestoreWalletRequest is the body for the "restore" action.
// Seed is the 25-word deterministic seed in the wordlist of `Language`
// (defaults to English). RestoreHeight tells wallet-rpc which block to
// resume scanning from; 0 means scan from genesis (very slow). Operators
// recovering a known wallet should provide the original creation height.
type MoneroRestoreWalletRequest struct {
	Seed          string `json:"seed"`
	Language      string `json:"language,omitempty"`
	RestoreHeight uint64 `json:"restoreHeight,omitempty"`
}

// MoneroRestoreWalletResult is returned by a successful restore. We do
// NOT echo back the seed — the caller already has it — to minimise the
// risk of accidentally logging it on the response path.
type MoneroRestoreWalletResult struct {
	Address string `json:"address"`
}

// ErrXMRWalletAlreadyExists is returned when a create/restore attempt
// would clobber an existing wallet. Handlers unwrap to HTTP 409 Conflict.
var ErrXMRWalletAlreadyExists = errors.New("xmr wallet already exists")

// ErrXMRInvalidSeed wraps malformed seed input (wrong word count, empty).
// Handlers unwrap to HTTP 400.
var ErrXMRInvalidSeed = errors.New("xmr seed")

// MoneroSecretsProvider is implemented by nodes that expose the XMR
// wallet's user-sovereignty surface — the operations OP-MP-6 needs so
// the merchant can re-derive a backup, restore on another machine, or
// hand a view-only copy to a trusted bookkeeper.
//
// Both methods are admin-only and live behind the same security tier as
// the wallet setup wizard (no apiToken). They MUST round-trip to
// monero-wallet-rpc on every call — the server never caches the seed or
// view key.
//
// Available only through a private distribution module with a working
// monero-wallet-rpc sidecar.
type MoneroSecretsProvider interface {
	GetXMRMnemonic(ctx context.Context) (MoneroMnemonicResult, error)
	GetXMRViewOnlyKeys(ctx context.Context) (MoneroViewOnlyKeysResult, error)
}

// MoneroMnemonicResult is the response payload for
// GET /v1/wallet/xmr/secrets/mnemonic.
//
// Mnemonic is the 25-word deterministic seed — the single source of
// truth for the wallet. Anyone who sees it permanently controls the
// funds. The frontend MUST display it inside an explicit "reveal" UX
// (eye icon, click-to-show) and emit a warning that screenshots /
// clipboard history / screen-share leaks compromise the wallet.
//
// CreatedAt + Address are echoed so the operator can sanity-check the
// seed they're about to write down belongs to the wallet they think it
// does. Both come from local metadata, not wallet-rpc.
type MoneroMnemonicResult struct {
	Mnemonic  string `json:"mnemonic"`
	Address   string `json:"address,omitempty"`
	CreatedAt int64  `json:"createdAt,omitempty"`
}

// MoneroViewOnlyKeysResult is the response payload for
// GET /v1/wallet/xmr/secrets/view-only.
//
// PrimaryAddress + PrivateViewKey + RestoreHeight is the canonical
// view-only triplet: feed the three into a fresh wallet (CLI:
// `monero-wallet-cli --generate-from-view-key`, GUI: "Restore wallet
// from keys") and the new wallet sees every incoming payment without
// being able to spend.
//
// CurrentHeight is captured server-side at the same moment the keys are
// returned, so the receiver knows the absolute upper bound the audit
// view is up to date with — without it the operator would have to
// open another tool to determine the chain tip. RestoreHeight is the
// historical scan-from anchor stored in xmr-wallet.json (0 = scan from
// genesis, very slow). The receiver should pick max(restoreHeight, 0)
// for a comprehensive audit; for short-window auditing they may pass a
// later height to skip historical re-scan.
type MoneroViewOnlyKeysResult struct {
	PrimaryAddress string `json:"primaryAddress"`
	PrivateViewKey string `json:"privateViewKey"`
	RestoreHeight  uint64 `json:"restoreHeight"`
	CurrentHeight  uint64 `json:"currentHeight"`
}

// MoneroHistoryProvider is implemented by nodes that expose the XMR
// wallet's transaction history. Used by the OP-MP-6 history page so the
// operator can audit incoming payments + outgoing withdrawals without
// running a separate Monero CLI.
//
// Like MoneroWalletProvider, this is admin-only — incoming transfers
// reveal subaddresses (which can be correlated to past orders) and
// outgoing transfers reveal recipient addresses (the operator's payout
// targets / counterparties). Anonymous reads are explicitly forbidden.
//
// Available only through a private distribution module with a working
// monero-wallet-rpc sidecar.
type MoneroHistoryProvider interface {
	ListXMRTransfers(ctx context.Context, req ListXMRTransfersRequest) (ListXMRTransfersResult, error)
}

// ListXMRTransfersRequest is the body for GET /v1/wallet/xmr/transfers.
//
// AccountIndex is a pointer for the same reason as MoneroWithdrawRequest:
// nil means "use the node's startup default" so multi-account sovereigns
// can rely on the same default everywhere. Most callers omit it.
//
// In/Out/Pool/Pending/Failed mirror the wallet-rpc get_transfers bucket
// flags. The server MUST default unset flags to (In=true, Out=true)
// because that's the natural "show me everything" view; explicit zeroes
// from the client are honoured and may produce an empty list. At least
// one bucket flag must end up true after defaulting; otherwise the
// handler returns 400 (lifted from the underlying client guard).
type ListXMRTransfersRequest struct {
	AccountIndex *uint32 `json:"accountIndex,omitempty"`
	In           bool    `json:"in"`
	Out          bool    `json:"out"`
	Pool         bool    `json:"pool"`
	Pending      bool    `json:"pending"`
	Failed       bool    `json:"failed"`
}

// ListXMRTransfersResult lists the matching transfers in wallet-rpc
// order (in→out→pool→pending→failed). The frontend is responsible for
// any user-driven sort (e.g. timestamp-desc) — keeping the wire format
// stable means the contract test stays simple.
//
// AccountIndex echoes which account the response refers to so the
// frontend doesn't have to track the request/response pairing itself.
type ListXMRTransfersResult struct {
	Transfers    []XMRTransferEntry `json:"transfers"`
	AccountIndex uint32             `json:"accountIndex"`
}

// XMRTransferEntry is the administration projection exposed through the
// public contracts package. Payment observation uses the provider-neutral
// distribution runtime instead.
//
// Amount + Fee are decimal piconero strings, same rationale as
// MoneroWithdrawRequest.Amount: a long-running sovereign can accumulate
// > 9007 XMR, beyond JS Number safe-integer range. UI MUST format from
// strings; never parse to Number for display.
type XMRTransferEntry struct {
	TxHash        string                 `json:"txHash"`
	Direction     string                 `json:"direction"` // in / out / pool / pending / failed
	Amount        string                 `json:"amount"`    // piconero, decimal string
	Fee           string                 `json:"fee"`       // piconero, decimal string; "0" for incoming
	Height        uint64                 `json:"height"`    // 0 for pool/pending/failed
	Confirmations uint64                 `json:"confirmations"`
	Timestamp     int64                  `json:"timestamp"` // unix seconds; 0 if unknown
	SubAddrIndex  uint32                 `json:"subAddrIndex"`
	Destinations  []XMRTransferRecipient `json:"destinations,omitempty"` // outgoing only
	Note          string                 `json:"note,omitempty"`
}

// XMRTransferRecipient is one destination leg of an outgoing transfer.
// Amount is a decimal piconero string (same rationale as XMRTransferEntry).
type XMRTransferRecipient struct {
	Address string `json:"address"`
	Amount  string `json:"amount"`
}

// SchedulerHooks exposes per-node worker tick methods for the scheduler
// (Phase AH-3). Both the SaaS shared scheduler and the standalone local
// scheduler call these via type assertion on NodeService, avoiding changes
// to the NodeService interface itself.
type SchedulerHooks interface {
	RunOrderTimeoutOnce(ctx context.Context)
	RunOutboxPollOnce(ctx context.Context)
	RunOutboxCleanupOnce(ctx context.Context)
	RunPaymentReconcileScanOnce(ctx context.Context)
	RunPaymentVerificationOnce(ctx context.Context)
	RunSettlementActionConfirmationsOnce(ctx context.Context)
	RunWebhookDeliveryOnce(ctx context.Context)
	RunWebhookCleanupOnce(ctx context.Context)
	RunAnalyticsCleanupOnce(ctx context.Context)
	RunFiatReconciliationOnce(ctx context.Context)
	RunFiatCleanupOnce(ctx context.Context)
	RunGuestOrderCleanupOnce(ctx context.Context)
	RunFollowerConnectOnce(ctx context.Context)
	RunNetDBReconcileOnce(ctx context.Context)
	RunOrderLockCleanupOnce(ctx context.Context)
	RunCollateralCredentialRefreshOnce(ctx context.Context)
	RunSupplyChainRetryOnce(ctx context.Context)
	RunSupplyChainReconcileOnce(ctx context.Context)
	RunSupplyChainCleanupOnce(ctx context.Context)
	RunSupplyChainInventoryCheckOnce(ctx context.Context)
	RunSupplyChainPriceDriftOnce(ctx context.Context)
}

// NodeRegistry exposes a race-free snapshot of all active NodeService instances.
//
// Designed for the shared scheduler (Phase AH-3) so that process-wide periodic
// jobs can iterate over active tenants without holding the SharedManager mutex
// or risking concurrent map mutation.
//
// Implementations must return a fresh slice on every call (not the underlying
// map values) so callers may iterate concurrently with AddNode / RemoveNode.
type NodeRegistry interface {
	// GetNodesSnapshot returns a point-in-time copy of all active NodeService
	// instances. The returned slice is safe to iterate concurrently with
	// registry mutations; node ordering is not guaranteed.
	GetNodesSnapshot() []NodeService
}
