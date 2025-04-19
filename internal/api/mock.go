package api

import (
	"context"
	"io"

	"github.com/gagliardetto/solana-go"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/kubo/core"
	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/multiwallet"
	"github.com/mobazha/mobazha3.0/internal/wallet"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	postsPb "github.com/mobazha/mobazha3.0/pkg/posts/pb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

type mockNode struct {
	requestAddressFunc                      func(ctx context.Context, to peer.ID, coinType iwallet.CoinType) (iwallet.Address, error)
	sendChatMessageFunc                     func(to peer.ID, message string, file *models.FileInChat, orderID models.OrderID, done chan<- struct{}) (string, error)
	sendGroupChatMessageFunc                func(tos []peer.ID, message string, file *models.FileInChat, orderID models.OrderID, done chan<- struct{}) (string, error)
	sendTypingMessageFunc                   func(to peer.ID, orderID models.OrderID) (string, error)
	sendGroupTypingMessageFunc              func(tos []peer.ID, orderID models.OrderID) (string, error)
	markChatMessagesAsReadFunc              func(peer peer.ID, orderID models.OrderID) error
	getChatConversationsFunc                func() ([]models.ChatConversation, error)
	getChatMessagesByPeerFunc               func(peer peer.ID, limit int, offsetID string) ([]models.ChatMessage, error)
	getChatMessagesByOrderIDFunc            func(orderID models.OrderID, limit int, offsetID string) ([]models.ChatMessage, error)
	getChatMessagesUnreadCountByOrderIDFunc func(orderID models.OrderID) (int64, error)
	deleteChatMessageFunc                   func(messageID string) error
	deleteChatConversationFunc              func(peerID peer.ID) error
	deleteGroupChatMessagesFunc             func(orderID models.OrderID) error
	SaveChatGroupFunc                       func(chatGroup *models.ChatGroup) (string, error)
	getChatGroupFunc                        func(groupID string, orderID models.OrderID) (*models.ChatGroup, error)
	deleteChatGroupFunc                     func(groupID string) error
	getNotificationsFunc                    func(offsetID string, limit int, typeFilters []string) ([]models.NotificationRecord, int64, error)
	markNotificationAsReadFunc              func(notifID string) error
	markAllNotificationsAsReadFunc          func() error
	getNotificationsUnreadCountFunc         func() (int, error)
	getOrderFunc                            func(orderID string) (*models.Order, error)
	getPurchasesFunc                        func(stateFilters []models.OrderState, searchTerm string, sortByAscending bool, sortByRead bool, limit int, exclude []string) ([]models.Order, int64, error)
	getSalesFunc                            func(stateFilters []models.OrderState, searchTerm string, sortByAscending bool, sortByRead bool, limit int, exclude []string) ([]models.Order, int64, error)
	getCaseFunc                             func(orderID string) (*models.Case, error)
	getCasesFunc                            func(stateFilters []models.OrderState, searchTerm string, sortByAscending bool, sortByRead bool, limit int, exclude []string) ([]models.Case, int64, error)
	confirmOrderFunc                        func(orderID models.OrderID, done chan struct{}) error
	fulfillOrderFunc                        func(orderID models.OrderID, fulfillments []models.Fulfillment, done chan struct{}) error
	completeOrderFunc                       func(orderID models.OrderID, ratings []models.Rating, includeIDInRating bool, done chan struct{}) error
	cancelOrderFunc                         func(orderID models.OrderID, done chan struct{}) error
	openDisputeFunc                         func(orderID models.OrderID, reason string, done chan struct{}) error
	closeDisputeFunc                        func(orderID models.OrderID, buyerPercentage, vendorPercentage float32, resolution string, done chan struct{}) error
	releaseFundsFunc                        func(orderID models.OrderID, done chan struct{}) error
	releaseFundsAfterTimeoutFunc            func(orderID models.OrderID, done chan struct{}) error
	followNodeFunc                          func(peerID peer.ID, done chan<- struct{}) error
	unfollowNodeFunc                        func(peerID peer.ID, done chan<- struct{}) error
	followsMeFunc                           func(peerID peer.ID) (bool, error)
	getMyFollowersFunc                      func() (models.Followers, error)
	getMyFollowingFunc                      func() (models.Following, error)
	getFollowersFunc                        func(ctx context.Context, peerID peer.ID, useCache bool) (models.Followers, error)
	getFollowingFunc                        func(ctx context.Context, peerID peer.ID, useCache bool) (models.Following, error)
	saveListingFunc                         func(listing *pb.Listing, done chan<- struct{}) error
	updateAllListingsFunc                   func(updateFunc func(l *pb.Listing) (bool, error), done chan<- struct{}) error
	deleteListingFunc                       func(slug string, done chan<- struct{}) error
	getMyListingsFunc                       func() (models.ListingIndex, error)
	getListingsFunc                         func(ctx context.Context, peerID peer.ID, useCache bool) (models.ListingIndex, error)
	getMyListingBySlugFunc                  func(slug string) (*pb.SignedListing, error)
	getMyListingByCIDFunc                   func(cid cid.Cid) (*pb.SignedListing, error)
	getListingBySlugFunc                    func(ctx context.Context, peerID peer.ID, slug string, useCache bool) (*pb.SignedListing, error)
	getListingByCIDFunc                     func(ctx context.Context, cid cid.Cid) (*pb.SignedListing, error)
	getCartsTotalItemsCountFunc             func() (int, error)
	getCartsFunc                            func() ([]models.StoreCart, error)
	addToCartFunc                           func(peerID peer.ID, item models.ShoppingCartItem) error
	removeCartItemFunc                      func(peerID peer.ID, item models.ShoppingCartItem) error
	clearCartsFunc                          func(vendorID peer.ID) error
	clearAllCartsFunc                       func() error
	getImageFunc                            func(ctx context.Context, cid cid.Cid) (io.ReadSeeker, error)
	getAvatarFunc                           func(ctx context.Context, peerID peer.ID, size models.ImageSize, useCache bool) (io.ReadSeeker, error)
	getHeaderFunc                           func(ctx context.Context, peerID peer.ID, size models.ImageSize, useCache bool) (io.ReadSeeker, error)
	setAvatarImageFunc                      func(base64ImageData string, done chan struct{}) (models.ImageHashes, error)
	setHeaderImageFunc                      func(base64ImageData string, done chan struct{}) (models.ImageHashes, error)
	setImageFunc                            func(base64ImageData string, filename string) (models.FileHash, error)
	setProductImageFunc                     func(base64ImageData string, filename string) (models.ImageHashes, error)
	addIntroVideoFunc                       func(fileData []byte, filename string) (models.FileHash, error)
	addFileFunc                             func(fileData []byte, filename string) (models.FileHash, error)
	getFileFunc                             func(ctx context.Context, cid cid.Cid) (io.ReadSeeker, error)
	setSelfAsModeratorFunc                  func(ctx context.Context, modInfo *models.ModeratorInfo, done chan struct{}) error
	setModeratorsOnListingsFunc             func(mods []peer.ID, done chan struct{}) error
	removeSelfAsModeratorFunc               func(ctx context.Context, done chan<- struct{}) error
	getModeratorsFunc                       func(ctx context.Context) []peer.ID
	getModeratorsAsyncFunc                  func(ctx context.Context) <-chan peer.ID
	getVerifiedModeratorsFunc               func(ctx context.Context) []peer.ID
	openChannel                             func(topic string) error
	closeChannel                            func(topic string) error
	listChannels                            func() []string
	publishChannelMessage                   func(ctx context.Context, topic, message string) error
	getChannelMessages                      func(ctx context.Context, topic string, from *cid.Cid, limit int) ([]models.ChannelMessage, error)
	publishFunc                             func(done chan<- struct{})
	usingTestnetFunc                        func() bool
	usingTorFunc                            func() bool
	ipfsNodeFunc                            func() *core.IpfsNode
	multiwalletFunc                         func() multiwallet.Multiwallet
	nodeIDFunc                              func() string
	identityFunc                            func() peer.ID
	subscribeEventFunc                      func(event interface{}) (events.Subscription, error)
	startFunc                               func()
	destroyNodeFunc                         func()
	eventBusFunc                            func() events.Bus
	dbFunc                                  func() database.Database
	stopFunc                                func(force bool) error
	setProfileFunc                          func(profile *models.Profile, done chan<- struct{}) error
	getMyProfileFunc                        func() (*models.Profile, error)
	getProfileFunc                          func(ctx context.Context, peerID peer.ID, useCache bool) (*models.Profile, error)
	getMyRatingsFunc                        func() (models.RatingIndex, error)
	getRatingsFunc                          func(ctx context.Context, peerID peer.ID, useCache bool) (models.RatingIndex, error)
	getRatingFunc                           func(ctx context.Context, cid cid.Cid) (*pb.Rating, error)
	purchaseFunc                            func(ctx context.Context, purchase *models.Purchase) (orderID models.OrderID, paymentAmount models.CurrencyValue, err error)
	estimateOrderTotalFunc                  func(ctx context.Context, purchase *models.Purchase) (models.OrderTotals, error)
	processOrderPaymentFunc                 func(ctx context.Context, paymentData *models.PaymentData) error
	rejectOrderFunc                         func(orderID models.OrderID, reason string, done chan struct{}) error
	refundOrderFunc                         func(orderID models.OrderID, done chan struct{}) error
	pingNodeFunc                            func(ctx context.Context, peer peer.ID) error
	getUserPreferencesFunc                  func() (*models.UserPreferences, error)
	saveUserPreferencesFunc                 func(prefs *models.UserPreferences, done chan struct{}) error
	blockNodeFunc                           func(peerID string) (bool, error)
	unblockNodeFunc                         func(peerID string) (bool, error)
	spendFunc                               func(spendData *models.SpendRequest) (*models.SpendResponse, error)
	saveTransactionMetadataFunc             func(metadata *models.TransactionMetadata) error
	getTransactionMetadataFunc              func(txid iwallet.TransactionID) (models.TransactionMetadata, error)
	getMnemonicFunc                         func() (string, error)
	updateWalletStatusFunc                  func(coinTypes []iwallet.CoinType)
	getExchangeRatesFunc                    func() *wallet.ExchangeRateProvider
	getReceivingAccountsFunc                func() ([]models.ReceivingAccount, error)
	updateReceivingAccountsFunc             func(receivingAccounts []models.ReceivingAccount) error
	getStripeConnectURLFunc                 func() (string, error)

	initializeSolEscrowFunc      func(ctx context.Context, params models.InitializeSolEscrowData) (solana.PublicKey, []solana.Instruction, error)
	releaseSolEscrowFunc         func(ctx context.Context, orderID models.OrderID, initiator solana.PublicKey) ([]solana.Instruction, error)
	initializeSPLTokenEscrowFunc func(ctx context.Context, params models.InitializeSPLTokenData) (solana.PublicKey, solana.PublicKey, []solana.Instruction, error)
	releaseSPLTokenEscrowFunc    func(ctx context.Context, orderID models.OrderID, initiator solana.PublicKey) ([]solana.Instruction, error)

	addPostFunc       func(post *postsPb.Post, done chan<- struct{}) error
	deletePostFunc    func(slug string, done chan<- struct{}) error
	postExistFunc     func(slug string) bool
	getMyPostFunc     func(slug string) (*postsPb.SignedPost, error)
	getPostBySlugFunc func(ctx context.Context, peerID peer.ID, slug string, useCache bool) (*postsPb.SignedPost, error)
	getMyPostsFunc    func() ([]models.PostData, error)
	getPostsFunc      func(ctx context.Context, peerID peer.ID, useCache bool) ([]models.PostData, error)
}

func (m *mockNode) RequestAddress(ctx context.Context, to peer.ID, coinType iwallet.CoinType) (iwallet.Address, error) {
	return m.requestAddressFunc(ctx, to, coinType)
}
func (m *mockNode) SendChatMessage(to peer.ID, message string, file *models.FileInChat, orderID models.OrderID, done chan<- struct{}) (string, error) {
	return m.sendChatMessageFunc(to, message, file, orderID, done)
}
func (m *mockNode) SendGroupChatMessage(tos []peer.ID, message string, file *models.FileInChat, orderID models.OrderID, done chan<- struct{}) (string, error) {
	return m.sendGroupChatMessageFunc(tos, message, file, orderID, done)
}
func (m *mockNode) SendTypingMessage(to peer.ID, orderID models.OrderID) (string, error) {
	return m.sendTypingMessageFunc(to, orderID)
}
func (m *mockNode) SendGroupTypingMessage(tos []peer.ID, orderID models.OrderID) (string, error) {
	return m.sendGroupTypingMessageFunc(tos, orderID)
}
func (m *mockNode) MarkChatMessagesAsRead(peer peer.ID, orderID models.OrderID) error {
	return m.markChatMessagesAsReadFunc(peer, orderID)
}
func (m *mockNode) GetChatConversations() ([]models.ChatConversation, error) {
	return m.getChatConversationsFunc()
}
func (m *mockNode) GetChatMessagesByPeer(peer peer.ID, limit int, offsetID string) ([]models.ChatMessage, error) {
	return m.getChatMessagesByPeerFunc(peer, limit, offsetID)
}
func (m *mockNode) GetChatMessagesByOrderID(orderID models.OrderID, limit int, offsetID string) ([]models.ChatMessage, error) {
	return m.getChatMessagesByOrderIDFunc(orderID, limit, offsetID)
}
func (m *mockNode) GetChatMessagesUnreadCountByOrderID(orderID models.OrderID) (int64, error) {
	return m.getChatMessagesUnreadCountByOrderIDFunc(orderID)
}
func (m *mockNode) DeleteChatMessage(messageID string) error {
	return m.deleteChatMessageFunc(messageID)
}
func (m *mockNode) DeleteChatConversation(peerID peer.ID) error {
	return m.deleteChatConversationFunc(peerID)
}
func (m *mockNode) DeleteGroupChatMessages(orderID models.OrderID) error {
	return m.deleteGroupChatMessagesFunc(orderID)
}

func (m *mockNode) SaveChatGroup(chatGroup *models.ChatGroup) (string, error) {
	return m.SaveChatGroupFunc(chatGroup)
}
func (m *mockNode) GetChatGroup(groupID string, orderID models.OrderID) (*models.ChatGroup, error) {
	return m.getChatGroupFunc(groupID, orderID)
}
func (m *mockNode) DeleteChatGroup(groupID string) error {
	return m.deleteChatGroupFunc(groupID)
}

func (m *mockNode) GetNotifications(offsetID string, limit int, typeFilters []string) ([]models.NotificationRecord, int64, error) {
	return m.getNotificationsFunc(offsetID, limit, typeFilters)
}
func (m *mockNode) MarkNotificationAsRead(notifID string) error {
	return m.markNotificationAsReadFunc(notifID)
}
func (m *mockNode) MarkAllNotificationsAsRead() error {
	return m.markAllNotificationsAsReadFunc()
}
func (m *mockNode) GetNotificationsUnreadCount() (int, error) {
	return m.getNotificationsUnreadCountFunc()
}
func (m *mockNode) GetPurchases(stateFilters []models.OrderState, searchTerm string, sortByAscending bool, sortByRead bool, limit int, exclude []string) ([]models.Order, int64, error) {
	return m.getPurchasesFunc(stateFilters, searchTerm, sortByAscending, sortByRead, limit, exclude)
}
func (m *mockNode) GetOrder(orderID string) (*models.Order, error) {
	return m.getOrderFunc(orderID)
}
func (m *mockNode) GetSales(stateFilters []models.OrderState, searchTerm string, sortByAscending bool, sortByRead bool, limit int, exclude []string) ([]models.Order, int64, error) {
	return m.getSalesFunc(stateFilters, searchTerm, sortByAscending, sortByRead, limit, exclude)
}
func (m *mockNode) GetCase(orderID string) (*models.Case, error) {
	return m.getCaseFunc(orderID)
}
func (m *mockNode) GetCases(stateFilters []models.OrderState, searchTerm string, sortByAscending bool, sortByRead bool, limit int, exclude []string) ([]models.Case, int64, error) {
	return m.getCasesFunc(stateFilters, searchTerm, sortByAscending, sortByRead, limit, exclude)
}
func (m *mockNode) ConfirmOrder(orderID models.OrderID, done chan struct{}) error {
	return m.confirmOrderFunc(orderID, done)
}
func (m *mockNode) FulfillOrder(orderID models.OrderID, fulfillments []models.Fulfillment, done chan struct{}) error {
	return m.fulfillOrderFunc(orderID, fulfillments, done)
}
func (m *mockNode) CompleteOrder(orderID models.OrderID, ratings []models.Rating, includeIDInRating bool, done chan struct{}) error {
	return m.completeOrderFunc(orderID, ratings, includeIDInRating, done)
}
func (m *mockNode) CancelOrder(orderID models.OrderID, done chan struct{}) error {
	return m.cancelOrderFunc(orderID, done)
}
func (m *mockNode) OpenDispute(orderID models.OrderID, reason string, done chan struct{}) error {
	return m.openDisputeFunc(orderID, reason, done)
}
func (m *mockNode) CloseDispute(orderID models.OrderID, buyerPercentage, vendorPercentage float32, resolution string, done chan struct{}) error {
	return m.closeDisputeFunc(orderID, buyerPercentage, vendorPercentage, resolution, done)
}
func (m *mockNode) ReleaseFunds(orderID models.OrderID, done chan struct{}) error {
	return m.releaseFundsFunc(orderID, done)
}
func (m *mockNode) ReleaseFundsAfterTimeout(orderID models.OrderID, done chan struct{}) error {
	return m.releaseFundsAfterTimeoutFunc(orderID, done)
}
func (m *mockNode) CheckOrdersForMorePayments() {
}
func (m *mockNode) FollowNode(peerID peer.ID, done chan<- struct{}) error {
	return m.followNodeFunc(peerID, done)
}
func (m *mockNode) UnfollowNode(peerID peer.ID, done chan<- struct{}) error {
	return m.unfollowNodeFunc(peerID, done)
}
func (m *mockNode) FollowsMe(peerID peer.ID) (bool, error) {
	return m.followsMeFunc(peerID)
}
func (m *mockNode) GetMyFollowers() (models.Followers, error) {
	return m.getMyFollowersFunc()
}
func (m *mockNode) GetMyFollowing() (models.Following, error) {
	return m.getMyFollowingFunc()
}
func (m *mockNode) GetFollowers(ctx context.Context, peerID peer.ID, useCache bool) (models.Followers, error) {
	return m.getFollowersFunc(ctx, peerID, useCache)
}
func (m *mockNode) GetFollowing(ctx context.Context, peerID peer.ID, useCache bool) (models.Following, error) {
	return m.getFollowingFunc(ctx, peerID, useCache)
}
func (m *mockNode) SaveListing(listing *pb.Listing, done chan<- struct{}) error {
	return m.saveListingFunc(listing, done)
}
func (m *mockNode) UpdateAllListings(updateFunc func(l *pb.Listing) (bool, error), done chan<- struct{}) error {
	return m.updateAllListingsFunc(updateFunc, done)
}
func (m *mockNode) DeleteListing(slug string, done chan<- struct{}) error {
	return m.deleteListingFunc(slug, done)
}
func (m *mockNode) GetMyListings() (models.ListingIndex, error) {
	return m.getMyListingsFunc()
}
func (m *mockNode) GetListings(ctx context.Context, peerID peer.ID, useCache bool) (models.ListingIndex, error) {
	return m.getListingsFunc(ctx, peerID, useCache)
}
func (m *mockNode) GetMyListingBySlug(slug string) (*pb.SignedListing, error) {
	return m.getMyListingBySlugFunc(slug)
}
func (m *mockNode) GetMyListingByCID(cid cid.Cid) (*pb.SignedListing, error) {
	return m.getMyListingByCIDFunc(cid)
}
func (m *mockNode) GetListingBySlug(ctx context.Context, peerID peer.ID, slug string, useCache bool) (*pb.SignedListing, error) {
	return m.getListingBySlugFunc(ctx, peerID, slug, useCache)
}
func (m *mockNode) GetListingByCID(ctx context.Context, cid cid.Cid) (*pb.SignedListing, error) {
	return m.getListingByCIDFunc(ctx, cid)
}

// ShoppingCart
func (m *mockNode) GetCartsTotalItemsCount() (int, error) {
	return m.getCartsTotalItemsCountFunc()
}

func (m *mockNode) GetCarts() ([]models.StoreCart, error) {
	return m.getCartsFunc()
}
func (m *mockNode) AddToCart(peerID peer.ID, item models.ShoppingCartItem) error {
	return m.addToCartFunc(peerID, item)
}
func (m *mockNode) RemoveCartItem(peerID peer.ID, item models.ShoppingCartItem) error {
	return m.removeCartItemFunc(peerID, item)
}
func (m *mockNode) ClearCarts(vendorID peer.ID) error {
	return m.clearCartsFunc(vendorID)
}
func (m *mockNode) ClearAllCarts() error {
	return m.clearAllCartsFunc()
}

func (m *mockNode) GetImage(ctx context.Context, cid cid.Cid) (io.ReadSeeker, error) {
	return m.getImageFunc(ctx, cid)
}
func (m *mockNode) GetAvatar(ctx context.Context, peerID peer.ID, size models.ImageSize, useCache bool) (io.ReadSeeker, error) {
	return m.getAvatarFunc(ctx, peerID, size, useCache)
}
func (m *mockNode) GetHeader(ctx context.Context, peerID peer.ID, size models.ImageSize, useCache bool) (io.ReadSeeker, error) {
	return m.getHeaderFunc(ctx, peerID, size, useCache)
}
func (m *mockNode) SetAvatarImage(base64ImageData string, done chan struct{}) (models.ImageHashes, error) {
	return m.setAvatarImageFunc(base64ImageData, done)
}
func (m *mockNode) SetHeaderImage(base64ImageData string, done chan struct{}) (models.ImageHashes, error) {
	return m.setHeaderImageFunc(base64ImageData, done)
}
func (m *mockNode) SetImage(base64ImageData string, filename string) (models.FileHash, error) {
	return m.setImageFunc(base64ImageData, filename)
}
func (m *mockNode) SetProductImage(base64ImageData string, filename string) (models.ImageHashes, error) {
	return m.setProductImageFunc(base64ImageData, filename)
}

// IntroVideo
func (m *mockNode) AddIntroVideo(fileData []byte, filename string) (models.FileHash, error) {
	return m.addIntroVideoFunc(fileData, filename)
}

// Files
func (m *mockNode) AddFile(fileData []byte, filename string) (models.FileHash, error) {
	return m.addFileFunc(fileData, filename)
}
func (m *mockNode) GetFile(ctx context.Context, cid cid.Cid) (io.ReadSeeker, error) {
	return m.getFileFunc(ctx, cid)
}

func (m *mockNode) SetSelfAsModerator(ctx context.Context, modInfo *models.ModeratorInfo, done chan struct{}) error {
	return m.setSelfAsModeratorFunc(ctx, modInfo, done)
}
func (m *mockNode) RemoveSelfAsModerator(ctx context.Context, done chan<- struct{}) error {
	return m.removeSelfAsModeratorFunc(ctx, done)
}
func (m *mockNode) GetModerators(ctx context.Context) []peer.ID {
	return m.getModeratorsFunc(ctx)
}
func (m *mockNode) GetModeratorsAsync(ctx context.Context) <-chan peer.ID {
	return m.getModeratorsAsyncFunc(ctx)
}
func (m *mockNode) GetVerifiedModerators(ctx context.Context) []peer.ID {
	return m.getVerifiedModeratorsFunc(ctx)
}
func (m *mockNode) SetModeratorsOnListings(mods []peer.ID, done chan struct{}) error {
	return m.setModeratorsOnListingsFunc(mods, done)
}
func (m *mockNode) Publish(done chan<- struct{}) {
	m.publishFunc(done)
}
func (m *mockNode) UsingTestnet() bool {
	return m.usingTestnetFunc()
}
func (m *mockNode) UsingTorMode() bool {
	return m.usingTorFunc()
}
func (m *mockNode) IPFSNode() *core.IpfsNode {
	return m.ipfsNodeFunc()
}
func (m *mockNode) Multiwallet() multiwallet.Multiwallet {
	return m.multiwalletFunc()
}
func (m *mockNode) GetNodeID() string {
	return m.nodeIDFunc()
}
func (m *mockNode) Identity() peer.ID {
	return m.identityFunc()
}
func (m *mockNode) SubscribeEvent(event interface{}) (events.Subscription, error) {
	return m.subscribeEventFunc(event)
}
func (m *mockNode) Start() {
	m.startFunc()
}
func (m *mockNode) Stop(force bool) error {
	return m.stopFunc(force)
}
func (m *mockNode) DestroyNode() {
	m.destroyNodeFunc()
}
func (m *mockNode) EventBus() events.Bus {
	return m.eventBusFunc()
}
func (m *mockNode) DB() database.Database {
	return m.dbFunc()
}
func (m *mockNode) SetProfile(profile *models.Profile, done chan<- struct{}) error {
	return m.setProfileFunc(profile, done)
}
func (m *mockNode) GetMyProfile() (*models.Profile, error) {
	return m.getMyProfileFunc()
}
func (m *mockNode) GetProfile(ctx context.Context, peerID peer.ID, useCache bool) (*models.Profile, error) {
	return m.getProfileFunc(ctx, peerID, useCache)
}
func (m *mockNode) GetMyRatings() (models.RatingIndex, error) {
	return m.getMyRatingsFunc()
}
func (m *mockNode) GetRatings(ctx context.Context, peerID peer.ID, useCache bool) (models.RatingIndex, error) {
	return m.getRatingsFunc(ctx, peerID, useCache)
}
func (m *mockNode) GetRating(ctx context.Context, cid cid.Cid) (*pb.Rating, error) {
	return m.getRatingFunc(ctx, cid)
}
func (m *mockNode) PurchaseListing(ctx context.Context, purchase *models.Purchase) (orderID models.OrderID, paymentAmount models.CurrencyValue, err error) {
	return m.purchaseFunc(ctx, purchase)
}
func (m *mockNode) EstimateOrderTotal(ctx context.Context, purchase *models.Purchase) (models.OrderTotals, error) {
	return m.estimateOrderTotalFunc(ctx, purchase)
}
func (m *mockNode) ProcessOrderPayment(ctx context.Context, paymentData *models.PaymentData) error {
	return m.processOrderPaymentFunc(ctx, paymentData)
}
func (m *mockNode) RejectOrder(orderID models.OrderID, reason string, done chan struct{}) error {
	return m.rejectOrderFunc(orderID, reason, done)
}
func (m *mockNode) RefundOrder(orderID models.OrderID, done chan struct{}) error {
	return m.refundOrderFunc(orderID, done)
}
func (m *mockNode) OpenChannel(topic string) error {
	return m.openChannel(topic)
}
func (m *mockNode) CloseChannel(topic string) error {
	return m.closeChannel(topic)
}
func (m *mockNode) ListChannels() []string {
	return m.listChannels()
}
func (m *mockNode) PublishChannelMessage(ctx context.Context, topic, message string) error {
	return m.publishChannelMessage(ctx, topic, message)
}
func (m *mockNode) GetChannelMessages(ctx context.Context, topic string, from *cid.Cid, limit int) ([]models.ChannelMessage, error) {
	return m.getChannelMessages(ctx, topic, from, limit)
}
func (m *mockNode) PingNode(ctx context.Context, peer peer.ID) error {
	return m.pingNodeFunc(ctx, peer)
}
func (m *mockNode) Spend(spendData *models.SpendRequest) (*models.SpendResponse, error) {
	return m.spendFunc(spendData)
}
func (m *mockNode) SaveTransactionMetadata(metadata *models.TransactionMetadata) error {
	return m.saveTransactionMetadataFunc(metadata)
}
func (m *mockNode) GetTransactionMetadata(txid iwallet.TransactionID) (models.TransactionMetadata, error) {
	return m.getTransactionMetadataFunc(txid)
}
func (m *mockNode) GetMnemonic() (string, error) {
	return m.getMnemonicFunc()
}
func (m *mockNode) UpdateWalletStatus(coinTypes []iwallet.CoinType) {
	m.updateWalletStatusFunc(coinTypes)
}
func (m *mockNode) GetReceivingAccounts() ([]models.ReceivingAccount, error) {
	return m.getReceivingAccountsFunc()
}
func (m *mockNode) UpdateReceivingAccounts(receivingAccounts []models.ReceivingAccount) error {
	return m.updateReceivingAccountsFunc(receivingAccounts)
}
func (m *mockNode) GetStripeConnectURL() (string, error) {
	return m.getStripeConnectURLFunc()
}

// Escrow
func (m *mockNode) InitializeSolEscrow(ctx context.Context, params models.InitializeSolEscrowData) (solana.PublicKey, []solana.Instruction, error) {
	return m.initializeSolEscrowFunc(ctx, params)
}
func (m *mockNode) ReleaseSolEscrow(ctx context.Context, orderID models.OrderID, initiator solana.PublicKey) ([]solana.Instruction, error) {
	return m.releaseSolEscrowFunc(ctx, orderID, initiator)
}
func (m *mockNode) InitializeSPLTokenEscrow(ctx context.Context, params models.InitializeSPLTokenData) (solana.PublicKey, solana.PublicKey, []solana.Instruction, error) {
	return m.initializeSPLTokenEscrowFunc(ctx, params)
}
func (m *mockNode) ReleaseSPLTokenEscrow(ctx context.Context, orderID models.OrderID, initiator solana.PublicKey) ([]solana.Instruction, error) {
	return m.releaseSPLTokenEscrowFunc(ctx, orderID, initiator)
}

func (m *mockNode) SavePreferences(prefs *models.UserPreferences, done chan struct{}) error {
	return m.saveUserPreferencesFunc(prefs, done)
}
func (m *mockNode) GetPreferences() (*models.UserPreferences, error) {
	return m.getUserPreferencesFunc()
}
func (m *mockNode) BlockNode(peerID string) (bool, error) {
	return m.blockNodeFunc(peerID)
}
func (m *mockNode) UnblockNode(peerID string) (bool, error) {
	return m.unblockNodeFunc(peerID)
}
func (m *mockNode) ExchangeRates() *wallet.ExchangeRateProvider {
	return m.getExchangeRatesFunc()
}

func (m *mockNode) AddPost(post *postsPb.Post, done chan<- struct{}) error {
	return m.addPostFunc(post, done)
}
func (m *mockNode) DeletePost(slug string, done chan<- struct{}) error {
	return m.deletePostFunc(slug, done)
}
func (m *mockNode) PostExist(slug string) bool {
	return m.postExistFunc(slug)
}
func (m *mockNode) GetMyPostBySlug(slug string) (*postsPb.SignedPost, error) {
	return m.getMyPostFunc(slug)
}
func (m *mockNode) GetPostBySlug(ctx context.Context, peerID peer.ID, slug string, useCache bool) (*postsPb.SignedPost, error) {
	return m.getPostBySlugFunc(ctx, peerID, slug, useCache)
}
func (m *mockNode) GetMyPosts() ([]models.PostData, error) {
	return m.getMyPostsFunc()
}
func (m *mockNode) GetPosts(ctx context.Context, peerID peer.ID, useCache bool) ([]models.PostData, error) {
	return m.getPostsFunc(ctx, peerID, useCache)
}
func (m *mockNode) IsGlobalBanned(peerID peer.ID) bool {
	return false
}

type mockNodeManager struct {
	nodes map[string]coreiface.CoreIface
}

func (m *mockNodeManager) GetNode(nodeID string) (coreiface.CoreIface, bool) {
	node, ok := m.nodes[nodeID]
	return node, ok
}

func (m *mockNodeManager) AddNode(nodeID string, node coreiface.CoreIface) {
	if m.nodes == nil {
		m.nodes = make(map[string]coreiface.CoreIface)
	}
	m.nodes[nodeID] = node
}

func (m *mockNodeManager) GetDefaultNode() coreiface.CoreIface {
	return m.nodes["default"]
}

func (m *mockNodeManager) GetIPFSNode() *core.IpfsNode {
	return nil
}

func (m *mockNodeManager) GetNodes() map[string]coreiface.CoreIface {
	return m.nodes
}

func (m *mockNodeManager) RemoveNode(nodeID string) {
	delete(m.nodes, nodeID)
}
