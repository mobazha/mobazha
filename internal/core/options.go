package core

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/ipfs/go-cid"
	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/pkg/request"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// NodeOption configures optional dependencies on a MobazhaNode after
// construction. Used by NewNodeWithOptions to allow hosting (SaaS) to
// inject custom adapters (e.g. KeyVaultProvider) without modifying the
// core construction flow.
type NodeOption func(*MobazhaNode)

// WithKeyProvider overrides the default fileKeyProvider.
// SaaS mode uses this to inject a KeyVault-backed implementation.
func WithKeyProvider(kp contracts.KeyProvider) NodeOption {
	return func(n *MobazhaNode) {
		n.keyProvider = kp
	}
}

// WithHostService sets the HostService for SaaS integration.
// This is extracted from the variadic parameter of NewNode for cleaner API.
func WithHostService(hs coreiface.HostService) NodeOption {
	return func(n *MobazhaNode) {
		n.hostService = hs
	}
}

// applyOptions applies NodeOption functions and sets defaults for
// fields that weren't explicitly overridden.
//
// App Service Initialization Dependency Graph
// ============================================
//
// Each service depends on services above it. Services that reference
// later-initialized services MUST use closures (not direct method values)
// to defer evaluation until call time. See callback-safety-rules.mdc.
//
//	Step | Service              | Runtime deps (via closures)             | Direct deps (must be init'd before)
//	─────┼──────────────────────┼─────────────────────────────────────────┼─────────────────────────────────────
//	 1   │ paymentService       │ profileService, orderService            │ (none)
//	 2   │ orderService         │ paymentService, listingService,         │ (none)
//	     │                      │ moderationService                       │
//	 3   │ chatService          │                                         │ (none)
//	 4   │ matrixService        │                                         │ (none)
//	 5   │ preferencesService   │ listingService                          │ (none)
//	 6   │ mediaService         │                                         │ (none)
//	 7   │ ratingsService       │                                         │ (none)
//	 8   │ notificationService  │                                         │ (none)
//	 9   │ shoppingCartService  │                                         │ (none)
//	10   │ profileService       │ paymentService                          │ (none)
//	11   │ followService        │                                         │ profileService
//	12   │ postsService         │                                         │ profileService
//	13   │ moderationService    │ listingService                          │ profileService
//	14   │ listingService       │                                         │ profileService
//
// "Runtime deps" = referenced via closures; safe even if the target is
//
//	initialized later because the closure captures `n` (not the service).
//
// "Direct deps" = the service pointer is read at init time (nil-guarded
//
//	or used directly); MUST already be non-nil.
//
// ADDING A NEW APP SERVICE — Standard Procedure:
//  1. Create init method: func (n *MobazhaNode) initXxxService()
//  2. Determine dependencies:
//     a. If depending on a service initialized AFTER this one → use closure
//     b. If depending on a service initialized BEFORE → nil-guarded direct capture is OK
//  3. Add the call to this function in the correct position
//  4. Update the dependency graph table above
//  5. Run: go build ./... && go test ./internal/core/...
func (n *MobazhaNode) applyOptions(opts []NodeOption) {
	for _, opt := range opts {
		opt(n)
	}
	if n.keyProvider == nil {
		n.keyProvider = newFileKeyProvider(
			n.ethMasterKey,
			n.escrowMasterKey,
			n.ratingMasterKey,
			n.solPrivKey,
		)
	}
	n.initPaymentService()
	n.initOrderService()
	n.initChatService()
	n.initMatrixService()
	n.initPreferencesService()
	n.initMediaService()
	n.initRatingsService()
	n.initNotificationService()
	n.initShoppingCartService()
	n.initWishlistService()
	n.initProfileService()
	n.initFollowService()
	n.initPostsService()
	n.initModerationService()
	n.initListingService()
	n.initAnalyticsService()
}

// initMatrixService creates the MatrixAppService.
func (n *MobazhaNode) initMatrixService() {
	if n.infrastructureOnly {
		return
	}
	if n.privKey == nil {
		return
	}
	n.matrixService = NewMatrixAppService(MatrixAppServiceConfig{
		DB:      n.db,
		PrivKey: n.privKey,
		PeerID:  n.peerID,
	})
}

// initPreferencesService creates the PreferencesAppService.
func (n *MobazhaNode) initPreferencesService() {
	if n.infrastructureOnly {
		return
	}
	n.preferencesService = NewPreferencesAppService(PreferencesAppServiceConfig{
		DB:         n.db,
		BanManager: n.banManager,
		UpdateAllListingsFunc: func(updateFunc func(l *pb.Listing) (bool, error), done chan<- struct{}) error {
			return n.listingService.UpdateAllListings(updateFunc, done)
		},
		GetMyListingsFunc: func() (models.ListingIndex, error) {
			return n.listingService.GetMyListings()
		},
	})
}

// initMediaService creates the MediaAppService with CDN-backed media storage.
func (n *MobazhaNode) initMediaService() {
	if n.infrastructureOnly {
		n.mediaService = NewMediaAppService(MediaAppServiceConfig{})
		return
	}

	var blobStore contracts.BlobStore
	if n.hostService != nil {
		blobStore = n.hostService.GetBlobStore()
	}

	n.mediaService = NewMediaAppService(MediaAppServiceConfig{
		DB:           n.db,
		ContentStore: n.contentStore,
		BlobStore:    blobStore,
		NodeID:       n.nodeID,
		Publish:      n.Publish,
		PublishFile:  n.PublishFile,
	})
}

// initRatingsService creates the RatingsAppService.
func (n *MobazhaNode) initRatingsService() {
	if n.infrastructureOnly {
		return
	}

	var getRatingIndex GetRatingIndexFromNetDBFunc
	if n.netDB != nil {
		getRatingIndex = n.netDB.GetRatingIndex
	}

	n.ratingsService = NewRatingsAppService(RatingsAppServiceConfig{
		DB:                 n.db,
		GetRatingIndex:     getRatingIndex,
		CoTenantPublicData: n.coTenantPublicDataDeferred(),
	})
}

// initNotificationService creates the NotificationAppService.
func (n *MobazhaNode) initNotificationService() {
	if n.infrastructureOnly {
		return
	}
	n.notificationService = NewNotificationAppService(NotificationAppServiceConfig{
		DB: n.db,
	})
}

// initShoppingCartService creates the ShoppingCartAppService.
func (n *MobazhaNode) initShoppingCartService() {
	if n.infrastructureOnly {
		return
	}
	n.shoppingCartService = NewShoppingCartAppService(ShoppingCartAppServiceConfig{
		DB:       n.db,
		EventBus: n.eventBus,
		NodeID:   n.nodeID,
	})
}

// initWishlistService creates the WishlistAppService and migrates the table.
func (n *MobazhaNode) initWishlistService() {
	if n.infrastructureOnly {
		return
	}
	if err := n.db.Update(func(tx database.Tx) error {
		return tx.Migrate(&models.WishlistItem{})
	}); err != nil {
		logger.LogErrorWithIDf(log, n.nodeID, "Wishlist: failed to migrate models: %v", err)
		return
	}
	n.wishlistService = NewWishlistAppService(WishlistAppServiceConfig{
		DB:     n.db,
		NodeID: n.nodeID,
	})
}

// initChatService creates the ChatAppService if the necessary
// dependencies are available. Infrastructure-only nodes skip this.
func (n *MobazhaNode) initChatService() {
	if n.infrastructureOnly {
		return
	}

	n.chatService = NewChatAppService(ChatAppServiceConfig{
		DB:             n.db,
		Messenger:      n.messenger,
		NetworkService: n.networkService,
		EventBus:       n.eventBus,
		NodeID:         n.nodeID,
		PeerID:         n.peerID,
	})
}

// initOrderService creates the OrderAppService if the necessary
// dependencies are available. Infrastructure-only nodes skip this.
func (n *MobazhaNode) initOrderService() {
	if n.infrastructureOnly {
		return
	}

	n.orderService = NewOrderAppService(OrderAppServiceConfig{
		DB:             n.db,
		Multiwallet:    n.multiwallet,
		Signer:         n.signer,
		OrderProcessor: n.orderProcessor,
		Messenger:      n.messenger,
		NetworkService: n.networkService,
		EventBus:       n.eventBus,
		NodeID:         n.nodeID,
		KeyProvider:    n.keyProvider,
		PeerID:         n.Identity,
		Testnet:        n.testnet,
		ExchangeRates:  n.exchangeRates,
		OrderLockMgr:   n.orderLockManager,

		GetPayoutAddr: func(coinType string) (iwallet.Address, error) {
			return n.paymentService.GetPayoutAddress(coinType)
		},
		ReleaseCancelableWithParams: func(order *models.Order, params releaseFromCancelableAddressParams) (iwallet.Tx, *iwallet.Transaction, error) {
			return n.paymentService.ReleaseFromCancelableAddressWithParams(order, params)
		},
		ReleaseCancelableFunds: func(order *models.Order, payoutAddress string) (iwallet.TransactionID, string, error) {
			if n.paymentService != nil {
				return n.paymentService.ReleaseCancelableFunds(order, payoutAddress)
			}
			return "", "", nil
		},
		ValidateListing: func(sl *pb.SignedListing) error {
			return n.listingService.validateListing(sl)
		},
		GetModeratorFee: func(totalOut iwallet.Amount, coinCode string) (iwallet.Amount, error) {
			return n.moderationService.GetModeratorFee(totalOut, coinCode)
		},
		GetListingByCID: func(ctx context.Context, c cid.Cid, reqCtx interface{}) (*pb.SignedListing, error) {
			return n.listingService.GetListingByCID(ctx, c, nil)
		},
		GetListings: func(ctx context.Context, peerID peer.ID) (models.ListingIndex, error) {
			return n.listingService.GetListings(ctx, peerID, nil, false)
		},
		FetchOrderByID: func(orderID string) (*models.Order, error) {
			return n.paymentService.FetchOrderByID(orderID)
		},
		RelayInstructions: func(orderID string, coinType iwallet.CoinType, instructions any) (string, error) {
			return n.paymentService.RelayInstructions(orderID, coinType, instructions)
		},
		DiscountResolver:           n.buildDiscountResolver(),
		DiscountRedemptionRecorder: n.buildDiscountRecorder(),
		CollectionStore: func() contracts.CollectionStore {
			if n.collectionService != nil {
				return n.collectionService.Store()
			}
			return nil
		},
	})

	if n.fiatPaymentService != nil {
		n.fiatPaymentService.SetWebhookHandler(func(ctx context.Context, event *contracts.WebhookEvent) error {
			coin := iwallet.CoinType(event.Coin)
			if coin == "" && event.Currency != "" {
				coin = iwallet.CoinType("fiat:" + event.Currency)
			}
			return n.orderService.ProcessOrderPayment(ctx, &models.PaymentData{
				OrderID:       event.OrderID,
				TransactionID: event.PaymentID,
				Coin:          coin,
				Amount:        uint64(event.Amount),
				Method:        pb.PaymentSent_FIAT,
				ProviderID:    event.ProviderID,
			})
		})
	}
}

// buildDiscountResolver returns a DiscountResolverFunc that resolves discounts
// for a vendor. In SaaS mode it crosses tenant boundaries via HostService;
// in standalone mode it uses the local node's DiscountAppService.
func (n *MobazhaNode) buildDiscountResolver() DiscountResolverFunc {
	if n.hostService != nil {
		return func(ctx context.Context, vendorPeerID string, dc DiscountContext) (*DiscountResult, error) {
			pid, err := peer.Decode(vendorPeerID)
			if err != nil {
				return nil, fmt.Errorf("invalid vendor peerID: %w", err)
			}
			svc, store, err := n.hostService.GetDiscountAccessForPeer(pid)
			if err != nil {
				return nil, err
			}
			return NewDiscountEngine(svc, store, nil).Calculate(ctx, dc)
		}
	}
	if n.discountService != nil {
		return func(ctx context.Context, vendorPeerID string, dc DiscountContext) (*DiscountResult, error) {
			store := n.discountService.Store()
			if store == nil {
				return nil, nil
			}
			var colStore contracts.CollectionStore
			if n.collectionService != nil {
				colStore = n.collectionService.Store()
			}
			return NewDiscountEngine(n.discountService, store, colStore).Calculate(ctx, dc)
		}
	}
	return nil
}

// buildDiscountRecorder returns a DiscountRedemptionRecorderFunc that records
// discount usage on the vendor's store. Uses the same SaaS/standalone split.
func (n *MobazhaNode) buildDiscountRecorder() DiscountRedemptionRecorderFunc {
	if n.hostService != nil {
		return func(ctx context.Context, vendorPeerID string, discountID string, codeID *string, orderID, customerPeerID, amount, currency string) error {
			pid, err := peer.Decode(vendorPeerID)
			if err != nil {
				return fmt.Errorf("invalid vendor peerID: %w", err)
			}
			svc, _, err := n.hostService.GetDiscountAccessForPeer(pid)
			if err != nil {
				return err
			}
			return svc.RecordRedemption(ctx, discountID, codeID, orderID, customerPeerID, amount, currency)
		}
	}
	if n.discountService != nil {
		return func(ctx context.Context, vendorPeerID string, discountID string, codeID *string, orderID, customerPeerID, amount, currency string) error {
			return n.discountService.RecordRedemption(ctx, discountID, codeID, orderID, customerPeerID, amount, currency)
		}
	}
	return nil
}

// initPaymentService creates the PaymentAppService if the necessary
// dependencies are available. Infrastructure-only nodes skip this.
func (n *MobazhaNode) initPaymentService() {
	if n.infrastructureOnly {
		return
	}

	var evmRelay EVMRelayService
	if n.hostService != nil {
		evmRelay = n.hostService.GetEVMRelayService()
	}

	n.paymentService = NewPaymentAppService(PaymentAppServiceConfig{
		DB:          n.db,
		Multiwallet: n.multiwallet,
		EventBus:    n.eventBus,
		NodeID:      n.nodeID,
		Shutdown:    n.shutdown,

		GetProfile: func(ctx context.Context, peerID peer.ID, reqCtx *request.Context, useCache bool) (*models.Profile, error) {
			return n.profileService.GetProfile(ctx, peerID, reqCtx, useCache)
		},
		ConfirmOrder: func(orderID models.OrderID, txid iwallet.TransactionID, payoutAddress string, done chan struct{}) error {
			return n.orderService.ConfirmOrder(orderID, txid, payoutAddress, done)
		},
		FulfillOrder: func(orderID models.OrderID, fulfillments []models.Fulfillment, done chan struct{}) error {
			return n.orderService.FulfillOrder(orderID, fulfillments, done)
		},
		ReleaseCancelable: func(order *models.Order, payoutAddress ...string) (*ReleaseResult, error) {
			return n.orderService.releaseFromCancelableAddress(order, payoutAddress...)
		},
		EscrowMasterPubKey: n.escrowMasterKey.PubKey(),

		Keys: n.keyProvider,
		ProcessOrderPayment: func(ctx context.Context, paymentData *models.PaymentData) error {
			return n.orderService.ProcessOrderPayment(ctx, paymentData)
		},

		ExchangeRates: n.exchangeRates,

		GetFiatPayment: func(paymentID string, providerID string) (*contracts.PaymentDetail, error) {
			if n.fiatPaymentService == nil {
				return nil, fmt.Errorf("fiat payment service not initialized")
			}
			return n.fiatPaymentService.GetPayment(context.Background(), providerID, paymentID)
		},

		EVMRelayService: evmRelay,
		RelayAPIURL:     n.relayAPIURL,
	})
}

// initProfileService creates the ProfileAppService.
func (n *MobazhaNode) initProfileService() {
	if n.infrastructureOnly {
		return
	}

	var escrowPubKeyHex, ethPubKeyHex, solanaPubKeyStr string
	if n.escrowMasterKey != nil {
		escrowPubKeyHex = hex.EncodeToString(n.escrowMasterKey.PubKey().SerializeCompressed())
	}
	if n.ethMasterKey != nil {
		ethPubKeyHex = hex.EncodeToString(n.ethMasterKey.PubKey().SerializeCompressed())
	}
	if n.solPrivKey != nil {
		solanaPubKeyStr = n.solPrivKey.PublicKey().String()
	}

	n.profileService = NewProfileAppService(ProfileAppServiceConfig{
		DB:                     n.db,
		Publish:                n.Publish,
		NetDB:                  n.netDB,
		NodeID:                 n.nodeID,
		PeerID:                 n.peerID,
		EscrowPubKeyHex:        escrowPubKeyHex,
		ETHPubKeyHex:           ethPubKeyHex,
		SolanaPubKeyStr:        solanaPubKeyStr,
		StripeAccountID:        n.stripeAccountID,
		StoreAndForwardServers: n.storeAndForwardServers,
		CoTenantPublicData:     n.coTenantPublicDataDeferred(),
		GetAcceptedCurrencies: func() ([]string, error) {
			return n.paymentService.GetAcceptedCurrencies()
		},
	})
}

// initPostsService creates the PostsAppService.
// Must be called after initProfileService since it depends on profileService callbacks.
func (n *MobazhaNode) initPostsService() {
	if n.infrastructureOnly {
		return
	}

	var updateProfile UpdateAndSaveProfileFunc
	var getMyProfile GetMyProfileFunc
	if n.profileService != nil {
		updateProfile = n.profileService.UpdateAndSaveProfile
		getMyProfile = n.profileService.GetMyProfile
	}

	n.postsService = NewPostsAppService(PostsAppServiceConfig{
		DB:                   n.db,
		Signer:               n.signer,
		Keys:                 n.keyProvider,
		PeerID:               n.peerID,
		Publish:              n.Publish,
		UpdateAndSaveProfile: updateProfile,
		GetMyProfile:         getMyProfile,
	})
}

// initFollowService creates the FollowAppService.
// Must be called after initProfileService since it depends on profileService.
func (n *MobazhaNode) initFollowService() {
	if n.infrastructureOnly {
		return
	}

	var updateProfile UpdateAndSaveProfileFunc
	var getMyProfile GetMyProfileFunc
	if n.profileService != nil {
		updateProfile = n.profileService.UpdateAndSaveProfile
		getMyProfile = n.profileService.GetMyProfile
	}

	n.followService = NewFollowAppService(FollowAppServiceConfig{
		DB:                   n.db,
		Messenger:            n.messenger,
		EventBus:             n.eventBus,
		NodeID:               n.nodeID,
		NetDB:                n.netDB,
		CoTenantPublicData:   n.coTenantPublicDataDeferred(),
		UpdateAndSaveProfile: updateProfile,
		GetMyProfile:         getMyProfile,
	})
}

// initModerationService creates the ModerationAppService.
// Must be called after initProfileService since it depends on profileService.GetMyProfile.
func (n *MobazhaNode) initModerationService() {
	if n.infrastructureOnly {
		return
	}

	var getMyProfile GetMyProfileFunc
	if n.profileService != nil {
		getMyProfile = n.profileService.GetMyProfile
	}

	var verifiedModEndpoint string
	if n.netConfig != nil {
		verifiedModEndpoint = n.netConfig.GetVerifiedModEndpoint()
	}

	n.moderationService = NewModerationAppService(ModerationAppServiceConfig{
		DB:                  n.db,
		Publish:             n.Publish,
		NodeID:              n.nodeID,
		VerifiedModEndpoint: verifiedModEndpoint,
		ExchangeRates:       n.exchangeRates,
		GetMyProfile:        getMyProfile,
		GetAcceptedCurrencies: func() ([]string, error) {
			return n.paymentService.GetAcceptedCurrencies()
		},
		UpdateAllListings: func(updateFunc func(l *pb.Listing) (bool, error), done chan<- struct{}) error {
			return n.listingService.UpdateAllListings(updateFunc, done)
		},
	})
}

func (n *MobazhaNode) initListingService() {
	if n.infrastructureOnly {
		return
	}

	var getMyProfile GetMyProfileFunc
	var updateAndSaveProfile UpdateAndSaveProfileFunc
	if n.profileService != nil {
		getMyProfile = n.profileService.GetMyProfile
		updateAndSaveProfile = n.profileService.UpdateAndSaveProfile
	}

	var shippingStore contracts.ShippingStore
	if n.shippingService != nil {
		shippingStore = n.shippingService.Store()
	}

	n.listingService = NewListingAppService(ListingAppServiceConfig{
		DB:                   n.db,
		Signer:               n.signer,
		ContentStore:         n.contentStore,
		NetDB:                n.netDB,
		BanManager:           n.banManager,
		Keys:                 n.keyProvider,
		FeatureManager:       n.featureManager,
		LocalListingCrypto:   n.localListingCrypto,
		NodeID:               n.Identity(),
		Testnet:              n.testnet,
		Publish:              n.Publish,
		GetMyProfile:         getMyProfile,
		UpdateAndSaveProfile: updateAndSaveProfile,
		CoTenantPublicData:   n.coTenantPublicDataDeferred(),
		ShippingStore:        shippingStore,
	})

	if n.collectionService != nil {
		n.listingService.onDeleteCleanup = func(slug string) {
			if err := n.collectionService.RemoveProductFromAllCollections(context.Background(), slug); err != nil {
				log.Errorf("Collection: failed to remove product %s from collections: %v", slug, err)
			}
		}
	}
}

// initAnalyticsService creates the AnalyticsAppService and migrates the table.
func (n *MobazhaNode) initAnalyticsService() {
	if n.infrastructureOnly {
		return
	}
	if err := n.db.Update(func(tx database.Tx) error {
		return tx.Migrate(&models.AnalyticsEvent{})
	}); err != nil {
		logger.LogErrorWithIDf(log, n.nodeID, "Analytics: failed to migrate models: %v", err)
		return
	}
	n.analyticsService = NewAnalyticsAppService(AnalyticsAppServiceConfig{
		DB:       n.db,
		NodeID:   n.nodeID,
		Shutdown: n.shutdown,
	})
}
