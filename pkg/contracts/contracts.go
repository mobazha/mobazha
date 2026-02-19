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
	"io"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	postsPb "github.com/mobazha/mobazha3.0/pkg/posts/pb"
	"github.com/mobazha/mobazha3.0/pkg/request"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"github.com/stripe/stripe-go/v82"
)

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
	// Standalone: uses IPFS node private key.
	// SaaS: delegates to KeyVault.
	SignMessage(payload []byte) ([]byte, []byte, error)

	// IsGlobalBanned checks if a peer is globally banned.
	IsGlobalBanned(peerID peer.ID) bool
}

// ChatService handles messaging operations.
type ChatService interface {
	SendChatMessage(to peer.ID, message string, file *models.FileInChat, orderID models.OrderID, done chan<- struct{}) (string, error)
	SendGroupChatMessage(tos []peer.ID, message string, file *models.FileInChat, orderID models.OrderID, done chan<- struct{}) (string, error)
	SendTypingMessage(to peer.ID, orderID models.OrderID) (string, error)
	SendGroupTypingMessage(tos []peer.ID, orderID models.OrderID) (string, error)
	MarkChatMessagesAsRead(peer peer.ID, orderID models.OrderID) error
	GetChatConversations() ([]models.ChatConversation, error)
	GetOrderConversations() ([]models.ChatConversation, error)
	GetChatMessagesByPeer(peer peer.ID, limit int, offsetID string) ([]models.ChatMessage, error)
	GetChatMessagesByOrderID(orderID models.OrderID, limit int, offsetID string) ([]models.ChatMessage, error)
	GetChatMessagesUnreadCountByOrderID(orderID models.OrderID) (int64, error)
	DeleteChatMessage(messageID string) error
	DeleteChatConversation(peerID peer.ID) error
	DeleteGroupChatMessages(orderID models.OrderID) error

	// Chat groups
	SaveChatGroup(chatGroup *models.ChatGroup) (string, error)
	GetChatGroup(groupID string, orderID models.OrderID) (*models.ChatGroup, error)
	GetChatGroups() ([]*models.ChatGroup, error)
	DeleteChatGroup(groupID string) error
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
	RejectOrder(orderID models.OrderID, txid iwallet.TransactionID, reason string, done chan struct{}) error
	RefundOrder(orderID models.OrderID, txid iwallet.TransactionID, done chan struct{}) error
	ConfirmOrder(orderID models.OrderID, txid iwallet.TransactionID, payoutAddress string, done chan struct{}) error
	GetConfirmOrderInstructions(orderID models.OrderID, initiatorAddress string, payoutAddress string) (coinType iwallet.CoinType, instructions any, err error)
	GetRefundOrderInstructions(orderID models.OrderID, initiatorAddress string) (coinType iwallet.CoinType, instructions any, err error)
	FulfillOrder(orderID models.OrderID, fulfillments []models.Fulfillment, done chan struct{}) error
	GetCompleteOrderInstructions(orderID models.OrderID, initiatorAddress string) (coinType iwallet.CoinType, instructions any, err error)
	CompleteOrder(orderID models.OrderID, txid iwallet.TransactionID, ratings []models.Rating, includeIDInRating bool, done chan struct{}) error
	CancelOrder(orderID models.OrderID, txid iwallet.TransactionID, done chan struct{}) error

	// ViaRelay methods: combine get-instructions + relay-execute + action into a single call.
	// Used by hosting mode where there is no frontend wallet (AppKit) to sign transactions.
	// For UTXO chains, these fall through to the standard methods (backend handles signing).
	// For EVM/Solana, these build instructions, relay via platform gas wallet, then complete the action.
	// Returns ErrRelayNotAvailable if relay service is not configured.
	RefundOrderViaRelay(orderID models.OrderID, done chan struct{}) error
	RejectOrderViaRelay(orderID models.OrderID, reason string, done chan struct{}) error
	CancelOrderViaRelay(orderID models.OrderID, done chan struct{}) error

	GetOrder(orderID string) (*models.Order, error)
	GetPurchases(stateFilters []models.OrderState, searchTerm string, sortByAscending bool, sortByRead bool, limit int, exclude []string) ([]models.Order, int64, error)
	GetSales(stateFilters []models.OrderState, searchTerm string, sortByAscending bool, sortByRead bool, limit int, exclude []string) ([]models.Order, int64, error)
	GetCase(orderID string) (*models.Case, error)
	GetCases(stateFilters []models.OrderState, searchTerm string, sortByAscending bool, sortByRead bool, limit int, exclude []string) ([]models.Case, int64, error)

	// Disputes
	OpenDispute(orderID models.OrderID, reason string, done chan struct{}) error
	CloseDispute(orderID models.OrderID, buyerPercentage, vendorPercentage float32, resolution string, done chan struct{}) error
	GetReleaseFundsInstructions(orderID models.OrderID, initiatorAddress string) (coinType iwallet.CoinType, instructions any, err error)
	ReleaseFunds(orderID models.OrderID, txid iwallet.TransactionID, done chan struct{}) error
	ReleaseFundsAfterTimeout(orderID models.OrderID, done chan struct{}) error
}

// ListingService handles product listing management.
type ListingService interface {
	SaveListing(listing *pb.Listing, done chan<- struct{}) error
	UpdateAllListings(updateFunc func(l *pb.Listing) (bool, error), done chan<- struct{}) error
	DeleteListing(slug string, done chan<- struct{}) error
	SetModeratorsOnListings(mods []peer.ID, done chan struct{}) error
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

	// Receiving accounts
	AddReceivingAccount(account *models.ReceivingAccount) (*models.ReceivingAccount, error)
	UpdateReceivingAccount(account *models.ReceivingAccount) (*models.ReceivingAccount, error)
	DeleteReceivingAccount(id int) error
	GetReceivingAccounts() ([]models.ReceivingAccount, error)
	GetReceivingAccountByID(id int) (*models.ReceivingAccount, error)
	GetActiveReceivingAccount(chainType iwallet.ChainType) (*models.ReceivingAccount, error)
	GetReceivingAccountsByChain(chainType iwallet.ChainType) ([]models.ReceivingAccount, error)
}

// MediaService handles images, videos, and files.
type MediaService interface {
	GetImage(ctx context.Context, cid cid.Cid) (io.ReadSeeker, error)
	GetAvatar(ctx context.Context, peerID peer.ID, size models.ImageSize, useCache bool) (io.ReadSeeker, error)
	GetHeader(ctx context.Context, peerID peer.ID, size models.ImageSize, useCache bool) (io.ReadSeeker, error)
	SetAvatarImage(base64ImageData string, done chan struct{}) (models.ImageHashes, error)
	SetHeaderImage(base64ImageData string, done chan struct{}) (models.ImageHashes, error)
	SetImage(base64ImageData string, filename string) (models.FileHash, error)
	SetProductImage(base64ImageData string, filename string) (models.ImageHashes, error)
	AddIntroVideo(fileData []byte, filename string) (models.FileHash, error)
	AddFile(fileData []byte, filename string) (models.FileHash, error)
	GetFile(ctx context.Context, cid cid.Cid) (io.ReadSeeker, error)
}

// SocialService handles following, ratings, posts, and channels.
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

	// Channels
	OpenChannel(topic string) error
	CloseChannel(topic string) error
	ListChannels() []string
	PublishChannelMessage(ctx context.Context, topic, message string) error
	GetChannelMessages(ctx context.Context, topic string, from *cid.Cid, limit int) ([]models.ChannelMessage, error)
}

// MatrixService handles Matrix chat integration (E2EE key backup, secrets, credentials).
type MatrixService interface {
	// Key Backup
	SaveMatrixKeyBackup(deviceID string, keysJSON string) error
	GetMatrixKeyBackup(deviceID string) (*models.MatrixKeyBackupResponse, error)
	GetMatrixKeyBackupInfo(deviceID string) (*models.MatrixKeyBackupInfo, error)
	DeleteMatrixKeyBackup(deviceID string) error
	ListMatrixKeyBackups() ([]models.MatrixKeyBackupInfo, error)

	// Secrets Bundle
	SaveMatrixSecretsBundle(deviceID string, secretsJSON string) error
	GetMatrixSecretsBundle() (*models.MatrixSecretsBundleResponse, error)
	GetMatrixSecretsBundleInfo() (*models.MatrixSecretsBundleInfo, error)
	DeleteMatrixSecretsBundle() error

	// Credentials
	GetMatrixCredentials() (*models.MatrixCredentialsResponse, error)
	SaveMatrixCredentials(matrixUserID, serverName string) error
	IsMatrixRegistered() (bool, error)
	GetDerivedMatrixPassword() (string, error)
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
}

// StripeService handles Stripe payment integration.
type StripeService interface {
	GetStripePublicKey() (string, error)
	GetStripeConnectURL() (string, error)
	CreateStripePaymentIntent(ctx context.Context, orderID models.OrderID, amount int64, currency string) (*stripe.PaymentIntent, error)
	HandleStripeWebhook(payload []byte, signature string) error
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

// NodeService is the top-level aggregate interface that combines all domain services.
// Both MobazhaNode (standalone) and TenantService (SaaS) implement this interface.
//
// Design: Service Accessor pattern — each domain is exposed via a typed accessor
// method (e.g. Chat() ChatService) rather than flat embedding. This eliminates
// ~130 pass-through delegates on the implementor and ensures new domain methods
// never require changes to NodeService or its implementors.
//
// Note: IdentityInfo() is named to avoid conflict with IdentityService.Identity().
type NodeService interface {
	// Domain service accessors
	IdentityInfo() IdentityService
	Chat() ChatService
	Notification() NotificationService
	Order() OrderService
	Listing() ListingService
	Profile() ProfileService
	Wallet() WalletService
	Media() MediaService
	Social() SocialService
	Matrix() MatrixService
	Preferences() PreferencesService
	Stripe() StripeService
	ExchangeRate() ExchangeRateService
	ShoppingCart() ShoppingCartService

	// Cross-cutting methods (kept directly on NodeService)

	// EventBus returns the event bus for pub/sub.
	EventBus() events.Bus

	// Publish publishes the node's data to the network.
	Publish(done chan<- struct{})

	// PingNode pings a remote peer.
	PingNode(ctx context.Context, peer peer.ID) error

	// SubscribeEvent subscribes to a specific event type.
	SubscribeEvent(event any) (events.Subscription, error)

	// Request address
	RequestAddress(ctx context.Context, to peer.ID, coinType iwallet.CoinType) (iwallet.Address, error)
}
