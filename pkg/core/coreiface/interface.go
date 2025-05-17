package coreiface

import (
	"context"
	"io"

	"github.com/gagliardetto/solana-go"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/kubo/core"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/multiwallet"
	"github.com/mobazha/mobazha3.0/internal/wallet"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	postsPb "github.com/mobazha/mobazha3.0/pkg/posts/pb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// CoreIface enumerates the interface of the OpenBazaarNode object in the Core package.
// We primarily use this to get around circular imports though it should serve as the API
// contract for the Core package.
type CoreIface interface {
	GetNodeID() string

	Start()
	Stop(force bool) error
	IPFSNode() *core.IpfsNode
	Identity() peer.ID

	DestroyNode()
	DB() database.Database
	EventBus() events.Bus

	// Chat
	SendChatMessage(to peer.ID, message string, file *models.FileInChat, orderID models.OrderID, done chan<- struct{}) (string, error)
	SendGroupChatMessage(tos []peer.ID, message string, file *models.FileInChat, orderID models.OrderID, done chan<- struct{}) (string, error)
	SendTypingMessage(to peer.ID, orderID models.OrderID) (string, error)
	SendGroupTypingMessage(tos []peer.ID, orderID models.OrderID) (string, error)
	MarkChatMessagesAsRead(peer peer.ID, orderID models.OrderID) error
	GetChatConversations() ([]models.ChatConversation, error)
	GetChatMessagesByPeer(peer peer.ID, limit int, offsetID string) ([]models.ChatMessage, error)
	GetChatMessagesByOrderID(orderID models.OrderID, limit int, offsetID string) ([]models.ChatMessage, error)
	GetChatMessagesUnreadCountByOrderID(orderID models.OrderID) (int64, error)
	DeleteChatMessage(messageID string) error
	DeleteChatConversation(peerID peer.ID) error
	DeleteGroupChatMessages(orderID models.OrderID) error

	// Chat group
	SaveChatGroup(chatGroup *models.ChatGroup) (string, error)
	GetChatGroup(groupID string, orderID models.OrderID) (*models.ChatGroup, error)
	DeleteChatGroup(groupID string) error

	// Notification
	GetNotifications(offsetID string, limit int, typeFilters []string) ([]models.NotificationRecord, int64, error)
	MarkNotificationAsRead(notifID string) error
	MarkAllNotificationsAsRead() error
	GetNotificationsUnreadCount() (int, error)

	// Orders
	PurchaseListing(ctx context.Context, purchase *models.Purchase) (orderID models.OrderID, paymentAmount models.CurrencyValue, err error)
	EstimateOrderTotal(ctx context.Context, purchase *models.Purchase) (models.OrderTotals, error)
	ProcessOrderPayment(ctx context.Context, paymentData *models.PaymentData) error
	RejectOrder(orderID models.OrderID, txid iwallet.TransactionID, reason string, done chan struct{}) error
	RefundOrder(orderID models.OrderID, done chan struct{}) error
	ConfirmOrder(orderID models.OrderID, txid iwallet.TransactionID, done chan struct{}) error
	GetConfirmOrderInstructions(orderID models.OrderID, initiator solana.PublicKey) ([]solana.Instruction, error)
	GetRejectOrderInstructions(orderID models.OrderID, initiator solana.PublicKey) ([]solana.Instruction, error)
	FulfillOrder(orderID models.OrderID, fulfillments []models.Fulfillment, done chan struct{}) error
	CompleteOrder(orderID models.OrderID, ratings []models.Rating, includeIDInRating bool, done chan struct{}) error
	CancelOrder(orderID models.OrderID, done chan struct{}) error

	GetOrder(orderID string) (*models.Order, error)
	GetPurchases(stateFilters []models.OrderState, searchTerm string, sortByAscending bool, sortByRead bool, limit int, exclude []string) ([]models.Order, int64, error)
	GetSales(stateFilters []models.OrderState, searchTerm string, sortByAscending bool, sortByRead bool, limit int, exclude []string) ([]models.Order, int64, error)
	GetCase(orderID string) (*models.Case, error)
	GetCases(stateFilters []models.OrderState, searchTerm string, sortByAscending bool, sortByRead bool, limit int, exclude []string) ([]models.Case, int64, error)

	CheckOrdersForMorePayments()

	// Dispute
	OpenDispute(orderID models.OrderID, reason string, done chan struct{}) error
	CloseDispute(orderID models.OrderID, buyerPercentage, vendorPercentage float32, resolution string, done chan struct{}) error
	ReleaseFunds(orderID models.OrderID, done chan struct{}) error
	ReleaseFundsAfterTimeout(orderID models.OrderID, done chan struct{}) error

	// Following
	FollowNode(peerID peer.ID, done chan<- struct{}) error
	UnfollowNode(peerID peer.ID, done chan<- struct{}) error
	FollowsMe(peerID peer.ID) (bool, error)
	GetMyFollowers() (models.Followers, error)
	GetMyFollowing() (models.Following, error)
	GetFollowers(ctx context.Context, peerID peer.ID, useCache bool) (models.Followers, error)
	GetFollowing(ctx context.Context, peerID peer.ID, useCache bool) (models.Following, error)

	// Listings
	SaveListing(listing *pb.Listing, done chan<- struct{}) error
	UpdateAllListings(updateFunc func(l *pb.Listing) (bool, error), done chan<- struct{}) error
	DeleteListing(slug string, done chan<- struct{}) error
	SetModeratorsOnListings(mods []peer.ID, done chan struct{}) error
	GetMyListings() (models.ListingIndex, error)
	GetListings(ctx context.Context, peerID peer.ID, useCache bool) (models.ListingIndex, error)
	GetMyListingBySlug(slug string) (*pb.SignedListing, error)
	GetMyListingByCID(cid cid.Cid) (*pb.SignedListing, error)
	GetListingBySlug(ctx context.Context, peerID peer.ID, slug string, useCache bool) (*pb.SignedListing, error)
	GetListingByCID(ctx context.Context, cid cid.Cid) (*pb.SignedListing, error)

	// ShoppingCart
	GetCartsTotalItemsCount() (int, error)
	GetCarts() ([]models.StoreCart, error)
	AddToCart(peerID peer.ID, item models.ShoppingCartItem) error
	RemoveCartItem(peerID peer.ID, item models.ShoppingCartItem) error
	ClearCarts(vendorID peer.ID) error
	ClearAllCarts() error

	// Images
	GetImage(ctx context.Context, cid cid.Cid) (io.ReadSeeker, error)
	GetAvatar(ctx context.Context, peerID peer.ID, size models.ImageSize, useCache bool) (io.ReadSeeker, error)
	GetHeader(ctx context.Context, peerID peer.ID, size models.ImageSize, useCache bool) (io.ReadSeeker, error)
	SetAvatarImage(base64ImageData string, done chan struct{}) (models.ImageHashes, error)
	SetHeaderImage(base64ImageData string, done chan struct{}) (models.ImageHashes, error)
	SetImage(base64ImageData string, filename string) (models.FileHash, error)
	SetProductImage(base64ImageData string, filename string) (models.ImageHashes, error)

	// IntroVideo
	AddIntroVideo(fileData []byte, filename string) (models.FileHash, error)

	// Files
	AddFile(fileData []byte, filename string) (models.FileHash, error)
	GetFile(ctx context.Context, cid cid.Cid) (io.ReadSeeker, error)

	// Moderation
	SetSelfAsModerator(ctx context.Context, modInfo *models.ModeratorInfo, done chan struct{}) error
	RemoveSelfAsModerator(ctx context.Context, done chan<- struct{}) error
	GetModerators(ctx context.Context) []peer.ID
	GetModeratorsAsync(ctx context.Context) <-chan peer.ID
	GetVerifiedModerators(ctx context.Context) []peer.ID

	// Profiles
	SetProfile(profile *models.Profile, done chan<- struct{}) error
	GetMyProfile() (*models.Profile, error)
	GetProfile(ctx context.Context, peerID peer.ID, useCache bool) (*models.Profile, error)

	// Ratings
	GetMyRatings() (models.RatingIndex, error)
	GetRatings(ctx context.Context, peerID peer.ID, useCache bool) (models.RatingIndex, error)
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

	// Preferences
	GetPreferences() (*models.UserPreferences, error)
	SavePreferences(prefs *models.UserPreferences, done chan struct{}) error
	BlockNode(peerID string) (bool, error)
	UnblockNode(peerID string) (bool, error)

	// 收款账户相关
	AddReceivingAccount(account *models.ReceivingAccount) (*models.ReceivingAccount, error)
	UpdateReceivingAccount(account *models.ReceivingAccount) (*models.ReceivingAccount, error)
	DeleteReceivingAccount(id int) error
	GetReceivingAccounts() ([]models.ReceivingAccount, error)
	GetActiveReceivingAccount(chainType iwallet.ChainType) (*models.ReceivingAccount, error)
	GetReceivingAccountsByChain(chainType iwallet.ChainType) ([]models.ReceivingAccount, error)
	GetStripeConnectURL() (string, error)

	// Escrow
	BuildInitializeSolEscrowInstructions(ctx context.Context, params models.InitializeSolEscrowData) (*models.PaymentData, solana.PublicKey, []solana.Instruction, error)
	BuildReleaseSolEscrowInstructions(ctx context.Context, orderID models.OrderID, initiator solana.PublicKey) ([]solana.Instruction, error)
	BuildIInitializeSPLTokenEscrowInstructions(ctx context.Context, params models.InitializeSPLTokenData) (*models.PaymentData, solana.PublicKey, solana.PublicKey, []solana.Instruction, error)
	BuildIReleaseSPLTokenEscrowInstructions(ctx context.Context, orderID models.OrderID, initiator solana.PublicKey) ([]solana.Instruction, error)

	GetSOLEscrowReleaseInstructions(orderID models.OrderID, initiator solana.PublicKey, receiver solana.PublicKey) ([]solana.Instruction, error)

	// Wallet
	Multiwallet() multiwallet.Multiwallet
	SaveTransactionMetadata(metadata *models.TransactionMetadata) error
	GetTransactionMetadata(txid iwallet.TransactionID) (models.TransactionMetadata, error)
	RequestAddress(ctx context.Context, to peer.ID, coinType iwallet.CoinType) (iwallet.Address, error)
	GetMnemonic() (string, error)
	UpdateWalletStatus(coinTypes []iwallet.CoinType)

	// Misc
	UsingTestnet() bool
	UsingTorMode() bool
	ExchangeRates() *wallet.ExchangeRateProvider
	Publish(done chan<- struct{})
	PingNode(ctx context.Context, peer peer.ID) error
	SubscribeEvent(event interface{}) (events.Subscription, error)
	IsGlobalBanned(peerID peer.ID) bool
}

type NodeManagerIface interface {
	GetDefaultNode() CoreIface
	GetIPFSNode() *core.IpfsNode

	AddNode(nodeID string, node CoreIface)
	RemoveNode(nodeID string)
	GetNodes() map[string]CoreIface
	GetNode(nodeID string) (CoreIface, bool)
}
