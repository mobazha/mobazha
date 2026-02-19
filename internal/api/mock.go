package api

import (
	"context"
	"io"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/kubo/core"
	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/wallet"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	postsPb "github.com/mobazha/mobazha3.0/pkg/posts/pb"
	"github.com/mobazha/mobazha3.0/pkg/request"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"github.com/stripe/stripe-go/v82"
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
	confirmOrderFunc                        func(orderID models.OrderID, txid iwallet.TransactionID, payoutAddress string, done chan struct{}) error
	getConfirmOrderInstructionsFunc         func(orderID models.OrderID, initiatorAddress string, payoutAddress string) (iwallet.CoinType, any, error)
	getRefundOrderInstructionsFunc          func(orderID models.OrderID, initiatorAddress string) (iwallet.CoinType, any, error)
	getCompleteOrderInstructionsFunc        func(orderID models.OrderID, initiatorAddress string) (iwallet.CoinType, any, error)
	fulfillOrderFunc                        func(orderID models.OrderID, fulfillments []models.Fulfillment, done chan struct{}) error
	completeOrderFunc                       func(orderID models.OrderID, txid iwallet.TransactionID, ratings []models.Rating, includeIDInRating bool, done chan struct{}) error
	cancelOrderFunc                         func(orderID models.OrderID, txid iwallet.TransactionID, done chan struct{}) error
	refundOrderViaRelayFunc                 func(orderID models.OrderID, done chan struct{}) error
	rejectOrderViaRelayFunc                 func(orderID models.OrderID, reason string, done chan struct{}) error
	cancelOrderViaRelayFunc                 func(orderID models.OrderID, done chan struct{}) error
	openDisputeFunc                         func(orderID models.OrderID, reason string, done chan struct{}) error
	closeDisputeFunc                        func(orderID models.OrderID, buyerPercentage, vendorPercentage float32, resolution string, done chan struct{}) error
	getReleaseFundsInstructionsFunc         func(orderID models.OrderID, initiatorAddress string) (iwallet.CoinType, any, error)
	releaseFundsFunc                        func(orderID models.OrderID, txid iwallet.TransactionID, done chan struct{}) error
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
	multiwalletFunc                         func() contracts.WalletOperator
	nodeIDFunc                              func() string
	identityFunc                            func() peer.ID
	subscribeEventFunc                      func(event any) (events.Subscription, error)
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
	getOrderInfoFunc                        func(orderID models.OrderID, coinType iwallet.CoinType) (*models.OrderInfo, error)
	processOrderPaymentFunc                 func(ctx context.Context, paymentData *models.PaymentData) error
	rejectOrderFunc                         func(orderID models.OrderID, txid iwallet.TransactionID, reason string, done chan struct{}) error
	refundOrderFunc                         func(orderID models.OrderID, txid iwallet.TransactionID, done chan struct{}) error
	pingNodeFunc                            func(ctx context.Context, peer peer.ID) error
	getUserPreferencesFunc                  func() (*models.UserPreferences, error)
	saveUserPreferencesFunc                 func(prefs *models.UserPreferences, done chan struct{}) error
	blockNodeFunc                           func(peerID string) (bool, error)
	unblockNodeFunc                         func(peerID string) (bool, error)
	spendFunc                               func(spendData *models.SpendRequest) (*models.SpendResponse, error)
	saveTransactionMetadataFunc             func(metadata *models.TransactionMetadata) error
	getTransactionMetadataFunc              func(txid iwallet.TransactionID) (models.TransactionMetadata, error)
	getMnemonicFunc                         func() (string, error)
	getExchangeRatesFunc                    func() *wallet.ExchangeRateProvider
	getAllRatesFunc                         func(base models.CurrencyCode, breakCache bool) (map[models.CurrencyCode]iwallet.Amount, error)

	// 收款账户相关
	addReceivingAccountFunc         func(account *models.ReceivingAccount) (*models.ReceivingAccount, error)
	updateReceivingAccountFunc      func(account *models.ReceivingAccount) (*models.ReceivingAccount, error)
	deleteReceivingAccountFunc      func(id int) error
	getReceivingAccountsFunc        func() ([]models.ReceivingAccount, error)
	getReceivingAccountByIDFunc     func(id int) (*models.ReceivingAccount, error)
	getActiveReceivingAccountFunc   func(chainType iwallet.ChainType) (*models.ReceivingAccount, error)
	getReceivingAccountsByChainFunc func(chainType iwallet.ChainType) ([]models.ReceivingAccount, error)
	getStripeConnectURLFunc         func() (string, error)

	generatePaymentInstructionsFunc func(ctx context.Context, params models.InitializeEscrowData) (*payment.PaymentSetupResult, error)
	buildInitEscrowInstructionsFunc func(ctx context.Context, params models.InitializeEscrowData) (*models.PaymentData, iwallet.Address, any, error)
	getUTXOPaymentInfoFunc          func(ctx context.Context, orderID string, moderator string, coinType iwallet.CoinType) (*models.PaymentData, error)
	getTotalPaidToAddressFunc       func(order *models.Order) (uint64, error)
	cancelPartialPaymentFunc        func(orderID string) (string, uint64, error)

	addPostFunc       func(post *postsPb.Post, done chan<- struct{}) error
	deletePostFunc    func(slug string, done chan<- struct{}) error
	postExistFunc     func(slug string) bool
	getMyPostFunc     func(slug string) (*postsPb.SignedPost, error)
	getPostBySlugFunc func(ctx context.Context, peerID peer.ID, slug string, useCache bool) (*postsPb.SignedPost, error)
	getMyPostsFunc    func() ([]models.PostData, error)
	getPostsFunc      func(ctx context.Context, peerID peer.ID, useCache bool) ([]models.PostData, error)

	// Stripe相关
	getStripePublicKeyFunc        func() (string, error)
	createStripePaymentIntentFunc func(ctx context.Context, orderID models.OrderID, amount int64, currency string) (*stripe.PaymentIntent, error)
	handleStripeWebhookFunc       func(payload []byte, signature string) error
	updateOrderPaymentStatusFunc  func(orderID models.OrderID, paymentIntentID, status string) error
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
func (m *mockNode) GetOrderConversations() ([]models.ChatConversation, error) {
	return []models.ChatConversation{}, nil
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
func (m *mockNode) GetChatGroups() ([]*models.ChatGroup, error) {
	return []*models.ChatGroup{}, nil
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
func (m *mockNode) BatchMarkNotificationsAsRead(ids []string) error {
	return nil
}
func (m *mockNode) BatchDeleteNotifications(ids []string) error {
	return nil
}
func (m *mockNode) GetNotificationsTotalCount() (int64, error) {
	return 0, nil
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
func (m *mockNode) ConfirmOrder(orderID models.OrderID, txid iwallet.TransactionID, payoutAddress string, done chan struct{}) error {
	return m.confirmOrderFunc(orderID, txid, payoutAddress, done)
}
func (m *mockNode) GetConfirmOrderInstructions(orderID models.OrderID, initiatorAddress string, payoutAddress string) (iwallet.CoinType, any, error) {
	return m.getConfirmOrderInstructionsFunc(orderID, initiatorAddress, payoutAddress)
}
func (m *mockNode) GetCompleteOrderInstructions(orderID models.OrderID, initiatorAddress string) (iwallet.CoinType, any, error) {
	return m.getCompleteOrderInstructionsFunc(orderID, initiatorAddress)
}
func (m *mockNode) GetRefundOrderInstructions(orderID models.OrderID, initiatorAddress string) (iwallet.CoinType, any, error) {
	return m.getRefundOrderInstructionsFunc(orderID, initiatorAddress)
}

func (m *mockNode) FulfillOrder(orderID models.OrderID, fulfillments []models.Fulfillment, done chan struct{}) error {
	return m.fulfillOrderFunc(orderID, fulfillments, done)
}
func (m *mockNode) CompleteOrder(orderID models.OrderID, txid iwallet.TransactionID, ratings []models.Rating, includeIDInRating bool, done chan struct{}) error {
	return m.completeOrderFunc(orderID, txid, ratings, includeIDInRating, done)
}
func (m *mockNode) CancelOrder(orderID models.OrderID, txid iwallet.TransactionID, done chan struct{}) error {
	return m.cancelOrderFunc(orderID, txid, done)
}
func (m *mockNode) RefundOrderViaRelay(orderID models.OrderID, done chan struct{}) error {
	if m.refundOrderViaRelayFunc != nil {
		return m.refundOrderViaRelayFunc(orderID, done)
	}
	return nil
}
func (m *mockNode) RejectOrderViaRelay(orderID models.OrderID, reason string, done chan struct{}) error {
	if m.rejectOrderViaRelayFunc != nil {
		return m.rejectOrderViaRelayFunc(orderID, reason, done)
	}
	return nil
}
func (m *mockNode) CancelOrderViaRelay(orderID models.OrderID, done chan struct{}) error {
	if m.cancelOrderViaRelayFunc != nil {
		return m.cancelOrderViaRelayFunc(orderID, done)
	}
	return nil
}
func (m *mockNode) OpenDispute(orderID models.OrderID, reason string, done chan struct{}) error {
	return m.openDisputeFunc(orderID, reason, done)
}
func (m *mockNode) CloseDispute(orderID models.OrderID, buyerPercentage, vendorPercentage float32, resolution string, done chan struct{}) error {
	return m.closeDisputeFunc(orderID, buyerPercentage, vendorPercentage, resolution, done)
}
func (m *mockNode) GetReleaseFundsInstructions(orderID models.OrderID, initiatorAddress string) (iwallet.CoinType, any, error) {
	return m.getReleaseFundsInstructionsFunc(orderID, initiatorAddress)
}
func (m *mockNode) ReleaseFunds(orderID models.OrderID, txid iwallet.TransactionID, done chan struct{}) error {
	return m.releaseFundsFunc(orderID, txid, done)
}
func (m *mockNode) ReleaseFundsAfterTimeout(orderID models.OrderID, done chan struct{}) error {
	return m.releaseFundsAfterTimeoutFunc(orderID, done)
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
func (m *mockNode) GetFollowers(ctx context.Context, peerID peer.ID, reqCtx *request.Context, useCache bool) (models.Followers, error) {
	return m.getFollowersFunc(ctx, peerID, useCache)
}
func (m *mockNode) GetFollowing(ctx context.Context, peerID peer.ID, reqCtx *request.Context, useCache bool) (models.Following, error) {
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
func (m *mockNode) GetListings(ctx context.Context, peerID peer.ID, reqCtx *request.Context, useCache bool) (models.ListingIndex, error) {
	return m.getListingsFunc(ctx, peerID, useCache)
}
func (m *mockNode) GetMyListingBySlug(slug string) (*pb.SignedListing, error) {
	return m.getMyListingBySlugFunc(slug)
}
func (m *mockNode) GetMyListingByCID(cid cid.Cid) (*pb.SignedListing, error) {
	return m.getMyListingByCIDFunc(cid)
}
func (m *mockNode) GetListingBySlug(ctx context.Context, peerID peer.ID, slug string, reqCtx *request.Context, useCache bool) (*pb.SignedListing, error) {
	return m.getListingBySlugFunc(ctx, peerID, slug, useCache)
}
func (m *mockNode) GetListingByCID(ctx context.Context, cid cid.Cid, reqCtx *request.Context) (*pb.SignedListing, error) {
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
func (m *mockNode) SignMessage(payload []byte) ([]byte, []byte, error) {
	return nil, nil, nil
}
func (m *mockNode) GetStripePublicKey() (string, error) {
	return "", nil
}
func (m *mockNode) GetStripeConnectURL() (string, error) {
	return "", nil
}
func (m *mockNode) CreateStripePaymentIntent(_ context.Context, _ models.OrderID, _ int64, _ string) (*stripe.PaymentIntent, error) {
	return nil, nil
}
func (m *mockNode) HandleStripeWebhook(_ []byte, _ string) error {
	return nil
}
func (m *mockNode) UsingTorMode() bool {
	return m.usingTorFunc()
}
func (m *mockNode) IPFSNode() *core.IpfsNode {
	return m.ipfsNodeFunc()
}
func (m *mockNode) Multiwallet() contracts.WalletOperator {
	return m.multiwalletFunc()
}
func (m *mockNode) GetNodeID() string {
	return m.nodeIDFunc()
}
func (m *mockNode) Identity() peer.ID {
	return m.identityFunc()
}
func (m *mockNode) SubscribeEvent(event any) (events.Subscription, error) {
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
func (m *mockNode) GetProfile(ctx context.Context, peerID peer.ID, reqCtx *request.Context, useCache bool) (*models.Profile, error) {
	return m.getProfileFunc(ctx, peerID, useCache)
}
func (m *mockNode) GetMyRatings() (models.RatingIndex, error) {
	return m.getMyRatingsFunc()
}
func (m *mockNode) GetRatings(ctx context.Context, peerID peer.ID, reqCtx *request.Context, useCache bool) (models.RatingIndex, error) {
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
func (m *mockNode) GetOrderInfo(orderID models.OrderID, coinType iwallet.CoinType) (*models.OrderInfo, error) {
	return m.getOrderInfoFunc(orderID, coinType)
}
func (m *mockNode) ProcessOrderPayment(ctx context.Context, paymentData *models.PaymentData) error {
	return m.processOrderPaymentFunc(ctx, paymentData)
}
func (m *mockNode) RejectOrder(orderID models.OrderID, txid iwallet.TransactionID, reason string, done chan struct{}) error {
	return m.rejectOrderFunc(orderID, txid, reason, done)
}
func (m *mockNode) RefundOrder(orderID models.OrderID, txid iwallet.TransactionID, done chan struct{}) error {
	return m.refundOrderFunc(orderID, txid, done)
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

// 收款账户相关
func (m *mockNode) AddReceivingAccount(account *models.ReceivingAccount) (*models.ReceivingAccount, error) {
	return m.addReceivingAccountFunc(account)
}
func (m *mockNode) UpdateReceivingAccount(account *models.ReceivingAccount) (*models.ReceivingAccount, error) {
	return m.updateReceivingAccountFunc(account)
}
func (m *mockNode) DeleteReceivingAccount(id int) error {
	return m.deleteReceivingAccountFunc(id)
}
func (m *mockNode) GetReceivingAccounts() ([]models.ReceivingAccount, error) {
	return m.getReceivingAccountsFunc()
}
func (m *mockNode) GetReceivingAccountByID(id int) (*models.ReceivingAccount, error) {
	return m.getReceivingAccountByIDFunc(id)
}
func (m *mockNode) GetActiveReceivingAccount(chainType iwallet.ChainType) (*models.ReceivingAccount, error) {
	return m.getActiveReceivingAccountFunc(chainType)
}
func (m *mockNode) GetReceivingAccountsByChain(chainType iwallet.ChainType) ([]models.ReceivingAccount, error) {
	return m.getReceivingAccountsByChainFunc(chainType)
}

// Escrow
func (m *mockNode) GeneratePaymentInstructions(ctx context.Context, params models.InitializeEscrowData) (*payment.PaymentSetupResult, error) {
	return m.generatePaymentInstructionsFunc(ctx, params)
}
func (m *mockNode) BuildInitEscrowInstructions(ctx context.Context, params models.InitializeEscrowData) (*models.PaymentData, iwallet.Address, any, error) {
	return m.buildInitEscrowInstructionsFunc(ctx, params)
}
func (m *mockNode) GetUTXOPaymentInfo(ctx context.Context, orderID string, moderator string, coinType iwallet.CoinType) (*models.PaymentData, error) {
	return m.getUTXOPaymentInfoFunc(ctx, orderID, moderator, coinType)
}
func (m *mockNode) GetTotalPaidToAddress(order *models.Order) (uint64, error) {
	return m.getTotalPaidToAddressFunc(order)
}
func (m *mockNode) CancelPartialPayment(orderID string) (string, uint64, error) {
	return m.cancelPartialPaymentFunc(orderID)
}
func (m *mockNode) StopWatchingPayment(orderID string) error {
	return nil // Mock implementation
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
func (m *mockNode) GetAllRates(base models.CurrencyCode, breakCache bool) (map[models.CurrencyCode]iwallet.Amount, error) {
	if m.getAllRatesFunc != nil {
		return m.getAllRatesFunc(base, breakCache)
	}
	return nil, nil
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

// Matrix E2EE Key Backup mock implementations
func (m *mockNode) SaveMatrixKeyBackup(deviceID string, keysJSON string) error {
	return nil
}

func (m *mockNode) GetMatrixKeyBackup(deviceID string) (*models.MatrixKeyBackupResponse, error) {
	return nil, nil
}

func (m *mockNode) GetMatrixKeyBackupInfo(deviceID string) (*models.MatrixKeyBackupInfo, error) {
	return nil, nil
}

func (m *mockNode) DeleteMatrixKeyBackup(deviceID string) error {
	return nil
}

func (m *mockNode) ListMatrixKeyBackups() ([]models.MatrixKeyBackupInfo, error) {
	return nil, nil
}

func (m *mockNode) GetMatrixCredentials() (*models.MatrixCredentialsResponse, error) {
	return &models.MatrixCredentialsResponse{
		MatrixUserID:  "@mock_user:matrix.mobazha.org",
		Password:      "mock_password",
		ServerName:    "matrix.mobazha.org",
		HomeserverURL: "https://matrix.mobazha.org",
		Registered:    true,
	}, nil
}

func (m *mockNode) SaveMatrixCredentials(matrixUserID, serverName string) error {
	return nil
}

func (m *mockNode) IsMatrixRegistered() (bool, error) {
	return true, nil
}

func (m *mockNode) GetDerivedMatrixPassword() (string, error) {
	return "mock_derived_password", nil
}

// Matrix Secrets Bundle mock implementations
func (m *mockNode) SaveMatrixSecretsBundle(deviceID string, secretsJSON string) error {
	return nil
}

func (m *mockNode) GetMatrixSecretsBundle() (*models.MatrixSecretsBundleResponse, error) {
	return nil, nil
}

func (m *mockNode) GetMatrixSecretsBundleInfo() (*models.MatrixSecretsBundleInfo, error) {
	return &models.MatrixSecretsBundleInfo{Exists: false}, nil
}

func (m *mockNode) DeleteMatrixSecretsBundle() error {
	return nil
}

type mockNodeManager struct {
	nodes map[string]contracts.NodeService
}

func (m *mockNodeManager) GetNode(nodeID string) (contracts.NodeService, bool) {
	node, ok := m.nodes[nodeID]
	return node, ok
}

func (m *mockNodeManager) AddNode(nodeID string, node contracts.NodeService) {
	if m.nodes == nil {
		m.nodes = make(map[string]contracts.NodeService)
	}
	m.nodes[nodeID] = node
}

func (m *mockNodeManager) GetDefaultNode() coreiface.CoreIface {
	node, ok := m.nodes["default"]
	if !ok {
		return nil
	}
	ci, ok := node.(coreiface.CoreIface)
	if !ok {
		return nil
	}
	return ci
}

func (m *mockNodeManager) GetIPFSNode() *core.IpfsNode {
	return nil
}

func (m *mockNodeManager) GetNodes() map[string]contracts.NodeService {
	return m.nodes
}

func (m *mockNodeManager) RemoveNode(nodeID string) {
	delete(m.nodes, nodeID)
}

func (m *mockNodeManager) GetMaxImportZipSize() int64 {
	return 300 << 20 // 300MB default
}

func (m *mockNodeManager) GetMaxImportVideoSize() int64 {
	return 15 << 20 // 15MB default
}

func (m *mockNodeManager) GetExchangeRateService() contracts.ExchangeRateService {
	return nil
}
