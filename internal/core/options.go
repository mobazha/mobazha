package core

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/internal/storage"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
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
// Services are wired via narrow interfaces (role interfaces). Circular
// dependencies and late-init services use setter injection.
//
//	Step | Service              | Injected via constructor                | Injected via setter (after init)
//	─────┼──────────────────────┼─────────────────────────────────────────┼──────────────────────────────────────
//	 1   │ profileService       │                                         │
//	 2   │ moderationService    │                                         │
//	 3   │ listingService       │                                         │
//	 4   │ paymentService       │ profileService (PeerProfileReader)       │ fiatPaymentService (FiatPaymentQuery)
//	 5   │ orderService         │ paymentService (EscrowOperations)        │
//	     │                      │ listingService (ListingQuery)            │
//	     │                      │ moderationService (ModeratorQuery)       │
//	 6   │ chatService          │                                         │
//	 7   │ matrixService        │                                         │
//	 8   │ preferencesService   │                                         │
//	 9   │ mediaService         │                                         │
//	10   │ ratingsService       │                                         │
//	11   │ notificationService  │                                         │
//	12   │ shoppingCartService  │                                         │
//	13   │ followService        │                                         │
//	14   │ postsService         │                                         │
//
// ADDING A NEW APP SERVICE — Standard Procedure:
//  1. Create init method: func (n *MobazhaNode) initXxxService()
//  2. Determine dependencies:
//     a. If depending on a service initialized BEFORE → pass via constructor (narrow interface)
//     b. If circular dependency → use setter injection after both are initialized
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
			n.tronMasterKey,
		)
	}
	n.initProfileService()
	n.initModerationService()
	n.initListingService()
	n.initPaymentService()
	n.initPaymentVerificationService()
	n.initOrderService()
	n.wireServiceSetters()
	n.initChatService()
	n.initMatrixChatService()
	n.initPreferencesService()
	n.initMediaService()
	n.initRatingsService()
	n.initNotificationService()
	n.initShoppingCartService()
	n.initWishlistService()
	n.initFollowService()
	n.initPostsService()
	n.initAnalyticsService()
}

// initPaymentVerificationService creates the PaymentVerificationService.
// Registry is nil at construction time; wired later by registerPaymentStrategies()
// during Start(). multiwallet is available from the builder.
func (n *MobazhaNode) initPaymentVerificationService() {
	if n.infrastructureOnly {
		return
	}
	if n.paymentVerificationService != nil {
		return
	}
	n.paymentVerificationService = NewPaymentVerificationService(
		n.paymentRegistry,
		n.multiwallet,
		nil,
	)
}

// wireServiceSetters resolves late-init wiring via setter injection,
// after all primary services are constructed.
func (n *MobazhaNode) wireServiceSetters() {
	if n.paymentService != nil && n.fiatPaymentService != nil {
		n.paymentService.SetFiatPaymentQuery(n.fiatPaymentService)
	}
	if n.paymentService != nil && n.paymentVerificationService != nil {
		n.paymentService.SetVerificationService(n.paymentVerificationService)
	}
	if n.orderService != nil && n.paymentVerificationService != nil {
		n.orderService.SetPaymentVerifier(n.paymentVerificationService)
	}
	if n.orderService != nil && n.fiatPaymentService != nil {
		n.orderService.SetFiatOps(n.fiatPaymentService)
	}
	if n.paymentService != nil && n.orderProcessor != nil {
		n.paymentService.SetPaymentRecorder(n.orderProcessor)
	}
	if n.paymentService != nil && n.orderService != nil {
		n.paymentService.SetPaymentVerifiedHandler(func(orderID string, paymentSent *pb.PaymentSent) {
			amount, _ := strconv.ParseUint(paymentSent.Amount, 10, 64)
			pd := &models.PaymentData{
				OrderID:       orderID,
				TransactionID: paymentSent.TransactionID,
				Coin:          iwallet.CoinType(paymentSent.Coin),
				Amount:        amount,
				Method:        paymentSent.Method,
			}
			n.orderService.RelayPaymentToBuyer(context.Background(), orderID, pd)
		})
	}
	if n.fiatPaymentService != nil && n.orderService != nil {
		n.fiatPaymentService.SetWebhookHandler(func(ctx context.Context, event *contracts.WebhookEvent) error {
			pd, err := buildFiatPaymentData(event)
			if err != nil {
				return err
			}
			if err := n.orderService.ProcessOrderPayment(ctx, pd); err != nil {
				return err
			}
			go n.orderService.RelayPaymentToBuyer(context.Background(), event.OrderID, pd)
			return nil
		})
	}
}

// buildFiatPaymentData converts a fiat WebhookEvent into a PaymentData struct.
// Extracted from the webhook handler closure to keep wiring logic thin.
func buildFiatPaymentData(event *contracts.WebhookEvent) (*models.PaymentData, error) {
	providerID := strings.ToLower(strings.TrimSpace(event.ProviderID))
	if providerID == "" {
		return nil, fmt.Errorf("fiat webhook provider ID is empty")
	}
	currency := strings.ToUpper(strings.TrimSpace(event.Currency))
	if currency == "" {
		return nil, fmt.Errorf("fiat webhook currency is empty")
	}
	if strings.TrimSpace(event.OrderID) == "" {
		return nil, fmt.Errorf("fiat webhook order ID is empty")
	}
	if strings.TrimSpace(event.PaymentID) == "" {
		return nil, fmt.Errorf("fiat webhook payment ID is empty")
	}

	coin := iwallet.CoinType(fmt.Sprintf("fiat:%s:%s", providerID, currency))

	return &models.PaymentData{
		OrderID:       event.OrderID,
		TransactionID: event.PaymentID,
		Coin:          coin,
		Amount:        uint64(event.Amount),
		Method:        pb.PaymentSent_FIAT,
		ProviderID:    providerID,
	}, nil
}

// initMatrixChatService creates the mautrix-go backed Matrix chat service.
// The service is created but not started here; Start() is called during node startup
// or lazily on first use (SaaS mode). Matrix config (homeserver URL, server name)
// is provided via SharedManager in SaaS mode or repo config in standalone mode.
//
// When matrixCryptoStore is set (SaaS multi-tenant), the mautrixChatService
// uses a shared PostgreSQL *dbutil.Database instead of per-tenant SQLite.
// Tenant isolation is via CryptoHelper.DBAccountID = peerID.
func (n *MobazhaNode) initMatrixChatService() {
	if n.infrastructureOnly {
		logger.LogInfoWithID(log, n.nodeID, "Matrix chat: skipped (infrastructure-only)")
		return
	}
	if n.privKey == nil {
		logger.LogWarningWithID(log, n.nodeID, "Matrix chat: skipped (privKey is nil)")
		return
	}

	var homeserverURL, serverName, dbPath, registrationSecret string

	if n.sharedManager != nil && n.sharedManager.NetConfig != nil {
		homeserverURL = n.sharedManager.NetConfig.MatrixInternalURL
		serverName = n.sharedManager.NetConfig.MatrixServerName
		registrationSecret = n.sharedManager.NetConfig.MatrixRegistrationSecret
	}

	if registrationSecret == "" {
		registrationSecret = os.Getenv("MATRIX_REGISTRATION_SECRET")
	}

	if homeserverURL == "" {
		homeserverURL = "https://matrix.mobazha.org"
	}
	if serverName == "" {
		serverName = "matrix.mobazha.org"
	}

	if n.repo != nil {
		dbPath = filepath.Join(n.repo.DataDir(), "mautrix_crypto.db")
	}

	cfg := MautrixChatServiceConfig{
		DB:                 n.db,
		PrivKey:            n.privKey,
		PeerID:             n.peerID,
		NodeCtx:            n.nodeCtx,
		HomeserverURL:      homeserverURL,
		ServerName:         serverName,
		DBPath:             dbPath,
		RegistrationSecret: registrationSecret,
	}

	if n.matrixCryptoStore != nil {
		cfg.CryptoStore = n.matrixCryptoStore
		cfg.CryptoDBAccountID = n.peerID.String()
		logger.LogInfoWithIDf(log, n.nodeID, "Matrix chat: creating service (homeserver=%s, server=%s, regSecret=%v, cryptoStore=shared-PG)",
			homeserverURL, serverName, registrationSecret != "")
	} else {
		logger.LogInfoWithIDf(log, n.nodeID, "Matrix chat: creating service (homeserver=%s, server=%s, regSecret=%v, cryptoStore=SQLite)",
			homeserverURL, serverName, registrationSecret != "")
	}

	svc, err := NewMautrixChatService(cfg)
	if err != nil {
		log.Errorf("Failed to create matrix chat service: %v", err)
		return
	}
	n.matrixChatService = svc
	logger.LogInfoWithIDf(log, n.nodeID, "Matrix chat: service created (userID=%s)", svc.matrixUserID)
}

// initPreferencesService creates the PreferencesAppService.
func (n *MobazhaNode) initPreferencesService() {
	if n.infrastructureOnly {
		return
	}
	n.preferencesService = NewPreferencesAppService(PreferencesAppServiceConfig{
		DB:         n.db,
		BanManager: n.banManager,
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
	if blobStore == nil && n.repo != nil {
		blobDir := filepath.Join(n.repo.DataDir(), "blobs")
		if bs, err := storage.NewLocalFSAdapter(blobDir); err != nil {
			log.Errorf("Failed to create local blob store at %s: %v", blobDir, err)
		} else {
			blobStore = bs
		}
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

// initChatService is a no-op placeholder. The old P2P ChatAppService was removed
// in the Phase Chat migration; all chat is now handled by mautrixChatService.
func (n *MobazhaNode) initChatService() {}

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
		Shutdown:       n.shutdown,

		Escrow:     n.paymentService,
		Listings:   n.listingService,
		Moderators: n.moderationService,
		Profiles:   n.profileService,

		DiscountResolver:           n.buildDiscountResolver(),
		DiscountRedemptionRecorder: n.buildDiscountRecorder(),
	})
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
//
// Note: FiatPaymentQuery is wired via setter in wireServiceSetters()
// to resolve late-init dependency.
func (n *MobazhaNode) initPaymentService() {
	if n.infrastructureOnly {
		return
	}

	var evmRelay EVMRelayService
	var solanaRelay SolanaRelayService
	if n.hostService != nil {
		evmRelay = n.hostService.GetEVMRelayService()
		solanaRelay = n.hostService.GetSolanaRelayService()
	}

	n.paymentService = NewPaymentAppService(PaymentAppServiceConfig{
		DB:          n.db,
		Multiwallet: n.multiwallet,
		EventBus:    n.eventBus,
		NodeID:      n.nodeID,
		Shutdown:    n.shutdown,

		Profiles:           n.profileService,
		EscrowMasterPubKey: n.escrowMasterKey.PubKey(),

		Keys:          n.keyProvider,
		ExchangeRates: n.exchangeRates,

		EVMRelayService:    evmRelay,
		SolanaRelayService: solanaRelay,
		RelayAPIURL:        n.relayAPIURL,
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
	})
}

// initPostsService creates the PostsAppService.
func (n *MobazhaNode) initPostsService() {
	if n.infrastructureOnly {
		return
	}

	n.postsService = NewPostsAppService(PostsAppServiceConfig{
		DB:      n.db,
		Signer:  n.signer,
		Keys:    n.keyProvider,
		PeerID:  n.peerID,
		Publish: n.Publish,
	})
}

// initFollowService creates the FollowAppService.
func (n *MobazhaNode) initFollowService() {
	if n.infrastructureOnly {
		return
	}

	n.followService = NewFollowAppService(FollowAppServiceConfig{
		DB:                 n.db,
		Messenger:          n.messenger,
		EventBus:           n.eventBus,
		NodeID:             n.nodeID,
		NetDB:              n.netDB,
		CoTenantPublicData: n.coTenantPublicDataDeferred(),
	})
}

// initModerationService creates the ModerationAppService.
func (n *MobazhaNode) initModerationService() {
	if n.infrastructureOnly {
		return
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
	})
}

func (n *MobazhaNode) initListingService() {
	if n.infrastructureOnly {
		return
	}

	var shippingStore contracts.ShippingStore
	if n.shippingService != nil {
		shippingStore = n.shippingService.Store()
	}

	n.listingService = NewListingAppService(ListingAppServiceConfig{
		DB:                 n.db,
		Signer:             n.signer,
		ContentStore:       n.contentStore,
		NetDB:              n.netDB,
		BanManager:         n.banManager,
		Keys:               n.keyProvider,
		FeatureManager:     n.featureManager,
		LocalListingCrypto: n.localListingCrypto,
		NodeID:             n.Identity(),
		Testnet:            n.testnet,
		Publish:            n.Publish,
		CoTenantPublicData: n.coTenantPublicDataDeferred(),
		ShippingStore:      shippingStore,
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
