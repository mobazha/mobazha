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
	pkgdb "github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	postsPb "github.com/mobazha/mobazha3.0/pkg/posts/pb"
	"github.com/mobazha/mobazha3.0/pkg/request"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"github.com/mobazha/mobazha3.0/pkg/webhook"
)

// CoTenantPublicDataFn resolves PublicData for a co-located tenant on the same
// SaaS host. Returns an error if the peerID is not a co-tenant. When nil
// (standalone mode), callers fall through to the normal NetDB/IPNS path.
type CoTenantPublicDataFn func(peerID peer.ID) (pkgdb.PublicData, error)

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
	GetConfirmOrderInstructions(orderID models.OrderID, initiatorAddress string, payoutAddress string) (coinType iwallet.CoinType, instructions any, err error)
	GetRefundOrderInstructions(orderID models.OrderID, initiatorAddress string) (coinType iwallet.CoinType, instructions any, err error)
	ShipOrder(orderID models.OrderID, shipments []models.Shipment, done chan struct{}) error
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
	GetReleaseFundsInstructions(orderID models.OrderID, initiatorAddress string) (coinType iwallet.CoinType, instructions any, err error)
	ReleaseFunds(orderID models.OrderID, txid iwallet.TransactionID, done chan struct{}) error
	ReleaseFundsAfterTimeout(orderID models.OrderID, done chan struct{}) error

	// Request address from a remote peer
	RequestAddress(ctx context.Context, to peer.ID, coinType iwallet.CoinType) (iwallet.Address, error)
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
// alone identifies the payment). Currently only ExternalPayment populates this:
// EXTERNAL_PAYMENT has no global tx index, so the watcher must communicate the block
// height for downstream confirmation polling. If a second chain needs
// chain-specific metadata, evaluate switching to a typed-per-chain method
// (e.g. HandleEXTERNAL_PAYMENTPaymentConfirmed) or a generic map[string]any.
type PaymentDetectedOpts struct {
	// TxBlockHeight is the block height of the confirmed EXTERNAL_PAYMENT transfer.
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

	CreateGuestOrder(ctx context.Context, req CreateGuestOrderRequest) (*GuestOrderResponse, error)
	GetGuestOrderStatus(ctx context.Context, token string) (*GuestOrderStatusResponse, error)
	ListGuestOrders(ctx context.Context, filter GuestOrderFilter) ([]models.GuestOrder, int64, error)
	ShipGuestOrder(ctx context.Context, token string, tracking, carrier string) error
	CompleteGuestOrder(ctx context.Context, token string) error
	HandlePaymentDetected(orderToken, txHash string, opts *PaymentDetectedOpts) error
	HandleConfirmationUpdate(orderToken string, confs int) error
	// HandlePoolPayment records a mempool-only payment observation (currently
	// EXTERNAL_PAYMENT-only). It does NOT change order state — the order remains in
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
// payment sidecar availability (for example PrivateDistribution's external_payment-wallet-rpc).
type PaymentRPCStatusProvider interface {
	PaymentRPCStatus(ctx context.Context) PaymentRPCStatus
}

type PaymentRPCStatus struct {
	EXTERNAL_PAYMENT *PaymentRPCStatusEntry `json:"external_payment,omitempty"`
}

type PaymentRPCStatusEntry struct {
	Connected    bool   `json:"connected"`
	Endpoint     string `json:"endpoint,omitempty"`
	AccountIndex uint32 `json:"accountIndex,omitempty"`
	BlockHeight  uint64 `json:"blockHeight,omitempty"`
	Error        string `json:"error,omitempty"`
}

// ExternalPaymentNodePoolProvider is implemented by nodes that expose the ExternalPayment
// daemon node pool for admin / setup wizard management (PrivateDistribution only).
//
// All methods are safe to call when the pool is not configured (e.g. the
// private_distribution is in legacy single-daemon mode or NodePool bootstrap failed) —
// ExternalPaymentNodes returns an empty snapshot and mutating methods return
// ErrExternalPaymentNodePoolUnavailable.
type ExternalPaymentNodePoolProvider interface {
	ExternalPaymentNodes(ctx context.Context) ExternalPaymentNodePoolSnapshot
	AddExternalPaymentNode(ctx context.Context, req ExternalPaymentNodeAddRequest) (ExternalPaymentNodeInfo, error)
	RemoveExternalPaymentNode(ctx context.Context, address string) error
	SwitchExternalPaymentNode(ctx context.Context, address string) error
}

// ErrExternalPaymentNodePoolUnavailable signals that the private_distribution is not running with
// a NodePool (legacy single-daemon mode, or bootstrap failed).
var ErrExternalPaymentNodePoolUnavailable = errors.New("external_payment NodePool: not available on this node")

// ExternalPaymentNodePoolSnapshot is the JSON envelope returned by
// GET /v1/system/external_payment-nodes.
type ExternalPaymentNodePoolSnapshot struct {
	// Available indicates whether a NodePool is wired up for this node.
	// When false, Active/Candidates are empty and write operations will
	// return ErrExternalPaymentNodePoolUnavailable.
	Available bool `json:"available"`

	// Healthy is the NodePool.IsHealthy() verdict (active node bound,
	// not Suspicious, fail-streak under threshold). Independent of
	// candidate count.
	Healthy bool `json:"healthy"`

	// MonitorOn indicates the background MonitorLoop is running. False
	// briefly during startup and after StopMonitor / lifecycle shutdown.
	MonitorOn bool `json:"monitorOn"`

	// Active is the daemon currently bound by wallet-rpc, if any.
	Active *ExternalPaymentNodeInfo `json:"active,omitempty"`

	// Candidates is the full pool snapshot in insertion order
	// (seed-embedded → discovered → user-added). The Active node also
	// appears here.
	Candidates []ExternalPaymentNodeInfo `json:"candidates"`
}

// ExternalPaymentNodeInfo is the read-only snapshot of a pool candidate.
type ExternalPaymentNodeInfo struct {
	Address       string `json:"address"`
	Operator      string `json:"operator,omitempty"`
	Source        string `json:"source"` // seed-embedded / discovered / user-added
	SuccessStreak int    `json:"successStreak"`
	FailStreak    int    `json:"failStreak"`
	Suspicious    bool   `json:"suspicious"`
	LastChecked   string `json:"lastChecked,omitempty"` // RFC3339; empty if never checked
}

// ExternalPaymentNodeAddRequest is the body for POST /v1/system/external_payment-nodes.
type ExternalPaymentNodeAddRequest struct {
	// Address is the I2P / Tor / clearnet host:port of external_paymentd RPC, e.g.
	// "node.example.b32.i2p:18089". Required.
	Address string `json:"address"`
	// Operator is a human-readable label (e.g. "ExternalPaymentWorld"). Optional.
	Operator string `json:"operator,omitempty"`
}

// ExternalPaymentWalletProvider is implemented by nodes that expose EXTERNAL_PAYMENT wallet-level
// operations (balance / transfer / sweep_all). Available only on private_distribution
// builds with a working external_payment-wallet-rpc sidecar.
type ExternalPaymentWalletProvider interface {
	GetEXTERNAL_PAYMENTBalance(ctx context.Context, accountIndex *uint32) (ExternalPaymentBalance, error)
	WithdrawEXTERNAL_PAYMENT(ctx context.Context, req ExternalPaymentWithdrawRequest) (ExternalPaymentWithdrawResult, error)
	SweepAllEXTERNAL_PAYMENT(ctx context.Context, req ExternalPaymentSweepAllRequest) (ExternalPaymentSweepAllResult, error)
}

// ExternalPaymentBalance reports the account-level balance for the EXTERNAL_PAYMENT wallet.
//
// Balance is the total (locked + unlocked) and UnlockedBalance is the
// portion that can be spent right now — ExternalPayment locks every incoming
// output for 10 confirmations (~20 min) and temporarily locks change
// outputs after sends. UI must withdraw against UnlockedBalance, never
// Balance, or wallet-rpc will reject the transfer.
//
// Both fields are decimal piconero strings (same JS-Number rationale as
// ExternalPaymentWithdrawRequest.Amount). BlocksToUnlock is the wallet-rpc hint
// for the next-batch unlock countdown; 0 when nothing is pending.
//
// AccountIndex echoes which account this balance refers to so the
// frontend doesn't have to track the request/response pairing itself.
type ExternalPaymentBalance struct {
	Balance         string `json:"balance"`
	UnlockedBalance string `json:"unlockedBalance"`
	BlocksToUnlock  uint64 `json:"blocksToUnlock,omitempty"`
	AccountIndex    uint32 `json:"accountIndex"`
}

// ErrExternalPaymentWalletUnavailable signals that the private_distribution node has no
// configured external_payment-wallet-rpc client (legacy boot without EXTERNAL_PAYMENT config,
// or wallet RPC connection failed during startup).
var ErrExternalPaymentWalletUnavailable = errors.New("external_payment wallet: RPC client not available on this node")

// ErrEXTERNAL_PAYMENTInvalidAddress is wrapped by validation failures on the EXTERNAL_PAYMENT
// destination address (empty / wrong length). Handlers unwrap with
// errors.Is to map to HTTP 400. The wrapping error supplies the
// human-readable reason; this sentinel only carries the prefix.
var ErrEXTERNAL_PAYMENTInvalidAddress = errors.New("external_payment address")

// ErrEXTERNAL_PAYMENTInvalidAmount is wrapped by validation failures on the EXTERNAL_PAYMENT
// amount field (non-numeric, zero, overflow). Handlers unwrap with
// errors.Is to map to HTTP 400.
var ErrEXTERNAL_PAYMENTInvalidAmount = errors.New("external_payment amount")

// ExternalPaymentWithdrawRequest is the body for POST /v1/wallet/external_payment/withdraw.
//
// Amount is a decimal string of piconero (1 EXTERNAL_PAYMENT = 10^12 piconero). It is a
// string instead of uint64 because JavaScript Number's managed_escrow-integer range
// (2^53 ≈ 9.007e15 piconero ≈ 9007 EXTERNAL_PAYMENT) is below the realistic EXTERNAL_PAYMENT balance
// of a long-running private_distribution — using uint64 over the wire would silently
// truncate large withdrawals. This matches the existing models.SpendRequest
// convention for UTXO/EVM chains.
//
// Priority: 0=default (wallet decides), 1=unimportant, 2=normal,
// 3=elevated, 4=priority. Higher priority => higher fee, faster inclusion.
//
// AccountIndex is a pointer so the wire can distinguish "unset" (use the
// node's startup-flag default) from an explicit 0 (the primary account on
// every standard ExternalPayment wallet). Most callers send a non-nil 0 or omit it
// entirely; multi-account private_distributions may target specific indices.
type ExternalPaymentWithdrawRequest struct {
	Address      string  `json:"address"`
	Amount       string  `json:"amount"`
	Priority     uint32  `json:"priority,omitempty"`
	AccountIndex *uint32 `json:"accountIndex,omitempty"`
}

// ExternalPaymentWithdrawResult is the response payload for a successful withdrawal.
// Amount + Fee are decimal piconero strings (same rationale as the request).
// TxKey lets the sender prove the payment off-chain to the recipient; the
// frontend should surface it as "Save this key — only share with the
// recipient if proof is needed".
type ExternalPaymentWithdrawResult struct {
	TxHash string `json:"txHash"`
	TxKey  string `json:"txKey,omitempty"`
	Amount string `json:"amount"`
	Fee    string `json:"fee"`
}

// ExternalPaymentSweepAllRequest is the body for POST /v1/wallet/external_payment/sweep-all.
//
// SubaddrIndices, when non-empty, restricts the sweep to the listed
// subaddress minor indices of AccountIndex. Empty means sweep all
// subaddresses of the account.
//
// AccountIndex is a pointer for the same reason as ExternalPaymentWithdrawRequest.
type ExternalPaymentSweepAllRequest struct {
	Address        string   `json:"address"`
	Priority       uint32   `json:"priority,omitempty"`
	AccountIndex   *uint32  `json:"accountIndex,omitempty"`
	SubaddrIndices []uint32 `json:"subaddrIndices,omitempty"`
}

// ExternalPaymentSweepAllResult lists the transactions produced by a sweep.
// sweep_all commonly produces multiple transactions when the wallet
// has many outputs; all parallel slices have the same length.
// Amounts / Fees are decimal piconero strings.
type ExternalPaymentSweepAllResult struct {
	TxHashes []string `json:"txHashes"`
	TxKeys   []string `json:"txKeys,omitempty"`
	Amounts  []string `json:"amounts"`
	Fees     []string `json:"fees"`
}

// ExternalPaymentWalletSetupProvider is implemented by nodes that expose the
// first-run wallet provisioning surface for EXTERNAL_PAYMENT. It is admin-only and
// available only on private_distribution builds with a working external_payment-wallet-rpc
// sidecar; SaaS nodes return ErrExternalPaymentWalletUnavailable.
//
// Lifecycle from a fresh wallet-rpc process:
//  1. GetEXTERNAL_PAYMENTWalletSetupStatus reports Exists=false
//  2. CreateEXTERNAL_PAYMENTWallet (or RestoreEXTERNAL_PAYMENTWallet) provisions the wallet,
//     persists local metadata, and returns the 25-word seed
//  3. Frontend shows seed + backup quiz, then calls ConfirmEXTERNAL_PAYMENTWalletBackup
//  4. On every subsequent boot, the node auto-opens the wallet using the
//     metadata; GetEXTERNAL_PAYMENTWalletSetupStatus reports Exists=true.
type ExternalPaymentWalletSetupProvider interface {
	GetEXTERNAL_PAYMENTWalletSetupStatus(ctx context.Context) (ExternalPaymentWalletSetupStatus, error)
	CreateEXTERNAL_PAYMENTWallet(ctx context.Context, req ExternalPaymentCreateWalletRequest) (ExternalPaymentCreateWalletResult, error)
	RestoreEXTERNAL_PAYMENTWallet(ctx context.Context, req ExternalPaymentRestoreWalletRequest) (ExternalPaymentRestoreWalletResult, error)
	ConfirmEXTERNAL_PAYMENTWalletBackup(ctx context.Context) error
}

// ExternalPaymentWalletSetupStatus is the response payload for
// GET /v1/system/setup-wizard/external_payment-wallet.
//
// Exists reflects on-disk metadata (external_payment-wallet.json) — it does NOT round-
// trip to wallet-rpc, so transient RPC outages don't make the wizard
// reappear and prompt the operator to overwrite. WalletOpen is the
// best-effort runtime signal that wallet-rpc currently has the wallet
// loaded (true after a successful open_wallet at startup or right after
// create/restore).
type ExternalPaymentWalletSetupStatus struct {
	Exists          bool   `json:"exists"`
	WalletOpen      bool   `json:"walletOpen"`
	Address         string `json:"address,omitempty"`
	BackupConfirmed bool   `json:"backupConfirmed"`
	CreatedAt       int64  `json:"createdAt,omitempty"`
}

// ExternalPaymentCreateWalletRequest is the body for the "create" action of
// POST /v1/system/setup-wizard/external_payment-wallet. Language picks the seed
// wordlist; the MVP supports only "English" to keep the backup
// verification UI simple.
type ExternalPaymentCreateWalletRequest struct {
	Language string `json:"language,omitempty"`
}

// ExternalPaymentCreateWalletResult returns the new seed + address. The seed MUST
// be displayed to the operator exactly once — the server never persists
// it, and re-fetching after the wizard finishes would require querying
// wallet-rpc with admin auth which we explicitly do not expose.
type ExternalPaymentCreateWalletResult struct {
	Mnemonic string `json:"mnemonic"`
	Address  string `json:"address"`
}

// ExternalPaymentRestoreWalletRequest is the body for the "restore" action.
// Seed is the 25-word deterministic seed in the wordlist of `Language`
// (defaults to English). RestoreHeight tells wallet-rpc which block to
// resume scanning from; 0 means scan from genesis (very slow). Operators
// recovering a known wallet should provide the original creation height.
type ExternalPaymentRestoreWalletRequest struct {
	Seed          string `json:"seed"`
	Language      string `json:"language,omitempty"`
	RestoreHeight uint64 `json:"restoreHeight,omitempty"`
}

// ExternalPaymentRestoreWalletResult is returned by a successful restore. We do
// NOT echo back the seed — the caller already has it — to minimise the
// risk of accidentally logging it on the response path.
type ExternalPaymentRestoreWalletResult struct {
	Address string `json:"address"`
}

// ErrEXTERNAL_PAYMENTWalletAlreadyExists is returned when a create/restore attempt
// would clobber an existing wallet. Handlers unwrap to HTTP 409 Conflict.
var ErrEXTERNAL_PAYMENTWalletAlreadyExists = errors.New("external_payment wallet already exists")

// ErrEXTERNAL_PAYMENTInvalidSeed wraps malformed seed input (wrong word count, empty).
// Handlers unwrap to HTTP 400.
var ErrEXTERNAL_PAYMENTInvalidSeed = errors.New("external_payment seed")

// SchedulerHooks exposes per-node worker tick methods for the scheduler
// (Phase AH-3). Both the SaaS shared scheduler and the standalone local
// scheduler call these via type assertion on NodeService, avoiding changes
// to the NodeService interface itself.
type SchedulerHooks interface {
	RunOrderTimeoutOnce(ctx context.Context)
	RunOutboxPollOnce(ctx context.Context)
	RunOutboxCleanupOnce(ctx context.Context)
	RunPaymentVerificationOnce(ctx context.Context)
	RunWebhookDeliveryOnce(ctx context.Context)
	RunWebhookCleanupOnce(ctx context.Context)
	RunAnalyticsCleanupOnce(ctx context.Context)
	RunFiatReconciliationOnce(ctx context.Context)
	RunFiatCleanupOnce(ctx context.Context)
	RunGuestOrderCleanupOnce(ctx context.Context)
	RunFollowerConnectOnce(ctx context.Context)
	RunNetDBReconcileOnce(ctx context.Context)
	RunOrderLockCleanupOnce(ctx context.Context)
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
