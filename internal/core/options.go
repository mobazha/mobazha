//go:build !private_distribution

package core

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/internal/core/guest"
	coreorder "github.com/mobazha/mobazha3.0/internal/core/order"
	corepayment "github.com/mobazha/mobazha3.0/internal/core/payment"
	coresettlement "github.com/mobazha/mobazha3.0/internal/core/settlement"
	"github.com/mobazha/mobazha3.0/internal/logger"
	obnet "github.com/mobazha/mobazha3.0/internal/net"
	"github.com/mobazha/mobazha3.0/internal/storage"
	pkgconfig "github.com/mobazha/mobazha3.0/pkg/config"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/deploy"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/pkg/relay"
	"github.com/mobazha/mobazha3.0/pkg/managedescrow"
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

// WithManagedEscrowCapConfig sets the grayscale routing configuration for EVM-ManagedEscrow.
// When cfg is non-nil and lists chains in ManagedEscrowChains, those chains activate
// the V2 ManagedEscrowAdapter path (ForCoinV2) and are excluded from V1 ClientSigned
// registration. Pass nil (default) to keep all EVM chains on the legacy V1
// path — this is the safe production default until the operator is ready.
//
// Injection point: hosting builder calls this after reading HostingConfig
// from app.yaml. Standalone nodes omit this option (nil default).
func WithManagedEscrowCapConfig(cfg *managed_escrow.ChainCapabilityConfig) NodeOption {
	return func(n *MobazhaNode) {
		n.managed_escrowCapConfig = cfg
	}
}

// WithPlatformFeatureProvider overrides the default PlatformGlobalProvider
// (which allows every feature). SaaS hosting injects an adapter that
// reads app.yaml / runtime admin API to honor platform-wide kill switches.
func WithPlatformFeatureProvider(p pkgconfig.PlatformGlobalProvider) NodeOption {
	return func(n *MobazhaNode) {
		n.platformFeatureProvider = p
	}
}

// WithTenantFeatureStore overrides the default FeatureOverrideStore.
// Hosting SaaS can inject a proxying adapter that fans reads out to a
// central multi-tenant store; standalone nodes use the local
// feature_overrides GORM table.
func WithTenantFeatureStore(s pkgconfig.TenantFeatureStore) NodeOption {
	return func(n *MobazhaNode) {
		n.tenantFeatureStore = s
	}
}

// WithNodeFeatureProvider overrides the default ConfigNodeFeatureProvider
// (which reads repo.Config CLI flags). Tests and SaaS tenant nodes can
// substitute a NoopAllowAllNodeProvider or an in-memory implementation.
func WithNodeFeatureProvider(p pkgconfig.NodeFeatureProvider) NodeOption {
	return func(n *MobazhaNode) {
		n.nodeFeatureProvider = p
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
//	 4   │ paymentService       │ profileService (PeerProfileReader)       │ fiatPaymentService, settlement
//	4.5  │ settlementService    │ paymentService (UTXOKeyDeriver)          │ paymentRegistry, receiptVerifier, supplyChainChecker, monitorService
//	 5   │ orderService         │ settlementService (EscrowOperations)     │
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
	n.initMediaService()
	n.initListingService()
	n.initReceivingAccountService()
	n.initPaymentService()
	n.initSettlementService()
	n.initPaymentVerificationService()
	n.initOrderService()
	n.wireServiceSetters()
	n.initMatrixChatService()
	n.initPreferencesService()
	n.initRatingsService()
	n.initNotificationService()
	n.initShoppingCartService()
	n.initWishlistService()
	n.initFollowService()
	n.initPostsService()
	n.initAnalyticsService()
	n.initNetDBSyncService()
	n.initFeatureResolver()
	n.initGuestOrderService()
}

// initFeatureResolver composes the three-layer feature-flag resolver from
// whatever providers have been installed on the node. Any provider that is
// still nil is replaced with a safe default:
//
//   - platform  → NoopPlatformProvider (standalone nodes have no platform
//     kill switch; hosting injects its own via WithPlatformFeatureProvider).
//     The Noop provider returns configured=false for every key, which makes
//     the resolver fall back to feature.DefaultValue — i.e. the value
//     declared in pkg/config/features_defined.go is the source of truth on
//     independent nodes.
//   - tenant    → FeatureOverrideStore backed by the node's GORM database.
//   - node      → ConfigNodeFeatureProvider reading repo.Config CLI flags.
//     When nothing is available (tests/mocks without config/db), the
//     resolver falls back to the allow-all stubs from pkg/config.
//
// The resolver is idempotent and safe to call multiple times; it is a
// no-op once featureResolver is set. infrastructure-only nodes also get
// a resolver so shared handlers can query capability flags without
// nil-checking `n.featureResolver`.
func (n *MobazhaNode) initFeatureResolver() {
	if n == nil || n.featureResolver != nil {
		return
	}
	platform := n.platformFeatureProvider
	// Fall back to the HostService before the allow-all default so that SaaS
	// tenant nodes — which call core.NewNode (not NewNodeWithOptions) — still
	// pick up platform kill switches without every call site having to thread
	// WithPlatformFeatureProvider explicitly.
	if platform == nil && n.hostService != nil {
		if hp := n.hostService.GetPlatformFeatureProvider(); hp != nil {
			platform = hp
		}
	}
	if platform == nil {
		platform = pkgconfig.NoopPlatformProvider{}
	}
	n.platformFeatureProvider = platform

	tenant := n.tenantFeatureStore
	if tenant == nil && n.db != nil {
		store := NewFeatureOverrideStore(n.db)
		if err := store.Migrate(); err != nil {
			// Migrate failure is non-fatal: the resolver degrades to
			// feature defaults because Get() will surface the error.
			logger.LogErrorWithIDf(log, n.nodeID, "feature_override: migrate failed: %v", err)
		}
		tenant = store
		n.tenantFeatureStore = tenant
	}
	if tenant == nil {
		tenant = pkgconfig.NoopTenantStore{}
		n.tenantFeatureStore = tenant
	}

	node := n.nodeFeatureProvider
	if node == nil {
		node = pkgconfig.AllowAllNodeProvider{}
		n.nodeFeatureProvider = node
	}

	n.featureResolver = pkgconfig.NewResolver(
		pkgconfig.WithPlatformProvider(platform),
		pkgconfig.WithTenantStore(tenant),
		pkgconfig.WithNodeProvider(node),
	)

	// Initialise the audit log store when a DB is available. Migrate
	// failures are non-fatal: handlers log-and-continue on audit errors
	// so the underlying feature-flag mutation still succeeds.
	if n.featureAuditLogger == nil && n.db != nil {
		auditStore := NewFeatureAuditLogStore(n.db)
		if err := auditStore.Migrate(); err != nil {
			logger.LogErrorWithIDf(log, n.nodeID, "feature_audit: migrate failed: %v", err)
		}
		n.featureAuditLogger = auditStore
	}
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
	n.paymentVerificationService = corepayment.NewPaymentVerificationService(
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
// For standalone nodes without a registration secret, the function attempts to
// provision a Matrix user via the SaaS proxy API. If provisioning succeeds (or the
// node was previously provisioned), the service is created in login-only mode.
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

	var homeserverURL, serverName, dbPath, registrationSecret, matrixSDKLogLevel string

	if n.sharedManager != nil && n.sharedManager.NetConfig != nil {
		homeserverURL = n.sharedManager.NetConfig.MatrixInternalURL
		serverName = n.sharedManager.NetConfig.MatrixServerName
		registrationSecret = n.sharedManager.NetConfig.MatrixRegistrationSecret
		matrixSDKLogLevel = n.sharedManager.NetConfig.MatrixSDKLogLevel
	}

	if registrationSecret == "" {
		registrationSecret = os.Getenv("MATRIX_REGISTRATION_SECRET")
	}

	if registrationSecret == "" && !deploy.IsSaaS() {
		homeserverURL, serverName = n.tryStandaloneMatrixProvision(homeserverURL, serverName)
	}

	if registrationSecret == "" && homeserverURL == "" {
		logger.LogInfoWithID(log, n.nodeID, "Matrix chat: skipped (no registration secret and not provisioned)")
		return
	}

	if matrixSDKLogLevel == "" {
		matrixSDKLogLevel = "off"
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
		SDKLogLevel:        matrixSDKLogLevel,
	}

	if n.matrixCryptoStore != nil {
		cfg.CryptoStore = n.matrixCryptoStore
		cfg.CryptoDBAccountID = n.peerID.String()
		logger.LogInfoWithIDf(log, n.nodeID, "Matrix chat: creating service (homeserver=%s, server=%s, regSecret=%v, sdkLog=%s, cryptoStore=shared-PG)",
			homeserverURL, serverName, registrationSecret != "", matrixSDKLogLevel)
	} else {
		mode := "full"
		if registrationSecret == "" {
			mode = "login-only"
		}
		logger.LogInfoWithIDf(log, n.nodeID, "Matrix chat: creating service (homeserver=%s, server=%s, mode=%s, sdkLog=%s, cryptoStore=SQLite)",
			homeserverURL, serverName, mode, matrixSDKLogLevel)
	}

	svc, err := NewMautrixChatService(cfg)
	if err != nil {
		log.Errorf("Failed to create matrix chat service: %v", err)
		return
	}
	n.matrixChatService = svc
	logger.LogInfoWithIDf(log, n.nodeID, "Matrix chat: service created (userID=%s)", svc.matrixUserID)
}

// tryStandaloneMatrixProvision checks local disk for a previous provision result,
// or calls the SaaS proxy API to provision a Matrix user for this standalone node.
// If no API key is available yet, it auto-registers with SaaS first.
// Returns (homeserverURL, serverName) on success, or ("", "") if not available.
func (n *MobazhaNode) tryStandaloneMatrixProvision(currentURL, currentName string) (string, string) {
	if n.repo == nil {
		return "", ""
	}
	dataDir := n.repo.DataDir()

	state, err := loadMatrixProvisionState(dataDir)
	if err == nil && state.Provisioned && state.HomeserverURL != "" {
		logger.LogInfoWithID(log, n.nodeID, "Matrix chat: using previously provisioned config")
		return state.HomeserverURL, state.ServerName
	}

	sm := n.sharedManager
	if sm == nil || sm.saasAPIURL == "" {
		return currentURL, currentName
	}

	// Auto-register with SaaS to obtain an API key if we don't have one yet.
	if sm.standaloneAPIKey == "" {
		logger.LogInfoWithID(log, n.nodeID, "Matrix chat: auto-registering with SaaS to get API key...")
		ctx, cancel := context.WithTimeout(n.nodeCtx, 30*time.Second)
		defer cancel()

		apiKey, regErr := obnet.RegisterWithSaaS(ctx, sm.saasAPIURL, n.peerID.String(), "", "nat")
		if regErr != nil {
			logger.LogWarningWithIDf(log, n.nodeID, "SaaS auto-registration failed: %v", regErr)
			return currentURL, currentName
		}

		sm.standaloneAPIKey = apiKey
		// Persist to the top-level app data dir (where loadPersistedAPIKey reads),
		// not the per-node dir, so the key survives restart.
		persistDir := sm.appDataDir
		if persistDir == "" {
			persistDir = dataDir
		}
		if persistErr := PersistAPIKey(persistDir, apiKey); persistErr != nil {
			logger.LogErrorWithIDf(log, n.nodeID, "Failed to persist SaaS API key: %v", persistErr)
		} else {
			logger.LogInfoWithIDf(log, n.nodeID, "SaaS API key obtained and persisted to %s", persistDir)
		}
	}

	logger.LogInfoWithID(log, n.nodeID, "Matrix chat: requesting provision from SaaS...")

	result, err := requestMatrixProvision(sm.saasAPIURL, sm.standaloneAPIKey, n.peerID.String(), n.privKey)
	if err != nil {
		logger.LogWarningWithIDf(log, n.nodeID, "Matrix provision failed: %v (chat unavailable until next restart)", err)
		return "", ""
	}

	if saveErr := saveMatrixProvisionState(dataDir, &matrixProvisionState{
		HomeserverURL: result.HomeserverURL,
		ServerName:    result.ServerName,
		Provisioned:   true,
	}); saveErr != nil {
		logger.LogErrorWithIDf(log, n.nodeID, "Failed to persist Matrix provision state: %v", saveErr)
	}

	logger.LogInfoWithIDf(log, n.nodeID, "Matrix chat: provisioned via SaaS (homeserver=%s, server=%s)",
		result.HomeserverURL, result.ServerName)

	return result.HomeserverURL, result.ServerName
}

// initPreferencesService creates the PreferencesAppService.
func (n *MobazhaNode) initPreferencesService() {
	if n.infrastructureOnly {
		return
	}
	n.preferencesService = NewPreferencesAppService(PreferencesAppServiceConfig{
		DB:         n.db,
		BanChecker: n.banManager,
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
		EventBus:     n.eventBus,
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

// initOrderService creates the OrderAppService if the necessary
// dependencies are available. Infrastructure-only nodes skip this.
func (n *MobazhaNode) initOrderService() {
	if n.infrastructureOnly {
		return
	}

	n.orderService = coreorder.NewOrderAppService(coreorder.OrderAppServiceConfig{
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

		Escrow:     n.settlementService,
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
func (n *MobazhaNode) buildDiscountResolver() coreorder.DiscountResolverFunc {
	if n.hostService != nil {
		return func(ctx context.Context, vendorPeerID string, dc models.DiscountContext) (*models.DiscountResult, error) {
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
		return func(ctx context.Context, vendorPeerID string, dc models.DiscountContext) (*models.DiscountResult, error) {
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
func (n *MobazhaNode) buildDiscountRecorder() coreorder.DiscountRedemptionRecorderFunc {
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

	n.paymentService = corepayment.NewPaymentAppService(corepayment.PaymentAppServiceConfig{
		DB:          n.db,
		Multiwallet: n.multiwallet,
		EventBus:    n.eventBus,
		NodeID:      n.nodeID,
		Shutdown:    n.shutdown,

		Profiles:           n.profileService,
		EscrowMasterPubKey: n.escrowMasterKey.PubKey(),

		Keys:          n.keyProvider,
		ExchangeRates: n.exchangeRates,
	})
}

// initSettlementService creates the SettlementService and wires it
// into PaymentAppService. Called after initPaymentService.
//
// Note: paymentRegistry is nil at construction time; it is wired later
// by registerPaymentStrategies() which also calls SetRegistry on settlement.
func (n *MobazhaNode) initSettlementService() {
	if n.infrastructureOnly {
		return
	}

	var evmRelay relay.EVMRelayService
	var solanaRelay relay.SolanaRelayService
	if n.hostService != nil {
		evmRelay = n.hostService.GetEVMRelayService()
		solanaRelay = n.hostService.GetSolanaRelayService()
	}

	n.settlementService = coresettlement.NewSettlementService(coresettlement.SettlementServiceConfig{
		DB:                 n.db,
		Multiwallet:        n.multiwallet,
		Keys:               n.keyProvider,
		EventBus:           n.eventBus,
		NodeID:             n.nodeID,
		MonitorService:     n.monitorService,
		EscrowMasterPubKey: n.escrowMasterKey.PubKey(),
		UTXOKeyDeriver:     n.paymentService,
		EVMRelayService:    evmRelay,
		SolanaRelayService: solanaRelay,
		RelayAPIURL:        n.relayAPIURL,
		RelayAPIBearer:     n.relayAPIBearer,
	})

	if n.paymentService != nil {
		n.paymentService.SetEscrowOps(n.settlementService)
	}
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
		EventBus:               n.eventBus,
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
		EventBus:           n.eventBus,
		BanChecker:         n.banManager,
		Keys:               n.keyProvider,
		FeatureManager:     n.featureManager,
		LocalListingCrypto: n.localListingCrypto,
		NodeID:             n.Identity(),
		Testnet:            n.testnet,
		Publish:            n.Publish,
		CoTenantPublicData: n.coTenantPublicDataDeferred(),
		ShippingStore:      shippingStore,
	})

	n.listingService.onDeleteCleanup = func(slug string) {
		if n.collectionService != nil {
			if err := n.collectionService.RemoveProductFromAllCollections(context.Background(), slug); err != nil {
				log.Errorf("Collection: failed to remove product %s from collections: %v", slug, err)
			}
		}
		if n.supplyChainService != nil {
			n.supplyChainService.ClearMappingForListing(slug)
		}
	}

	if n.supplyChainService != nil {
		n.supplyChainService.SetListingOps(n.listingService)
		if n.mediaService != nil {
			n.supplyChainService.SetMediaOps(n.mediaService)
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

func (n *MobazhaNode) initNetDBSyncService() {
	if n.netDB == nil || n.eventBus == nil {
		return
	}
	n.netDBSyncService = NewNetDBSyncService(NetDBSyncServiceConfig{
		NetDB:             n.netDB,
		DB:                n.db,
		EventBus:          n.eventBus,
		NodeID:            n.nodeID,
		ListingService:    n.listingService,
		RatingsService:    n.ratingsService,
		CollectionService: n.collectionService,
		DiscountService:   n.discountService,
	})
}

// initGuestOrderService creates the Guest Checkout subsystem:
// DirectPaymentService → AutoSweepService → GuestOrderAppService.
// Requires: db, eventBus, bip44MasterKey (from cryptoFields).
func (n *MobazhaNode) initGuestOrderService() {
	if n.infrastructureOnly {
		return
	}
	if n.db == nil || n.eventBus == nil {
		return
	}
	if n.bip44Key == nil {
		return
	}

	keyDeriver := guest.NewNodeKeyDeriver(n.bip44Key, n.multiwallet)
	n.directPaymentService = guest.NewDirectPaymentService(n.db, keyDeriver)
	n.autoSweepService = guest.NewAutoSweepService(n.db, keyDeriver, n.eventBus)

	// Capability gating: GuestOrderAppService advertises and accepts only
	// coins whose anonymous checkout closure path is ready. UTXO chains are
	// first narrowed to wallet-loaded, sweepable chains here; runtime health
	// (monitor, sweep service, seller receiving account) is checked later by
	// the service capability evaluator. EVM/Solana stay hidden until their
	// settlement paths are implemented.
	supportedUTXO := n.detectGuestUTXOChains()

	// EVM/Solana guest checkout is disabled until concrete
	// ChainBalanceChecker / SolanaReferenceChecker adapters are
	// implemented and wired via SetCheckers in the lifecycle.
	// Without working checkers, payments would be accepted but
	// never detected — risking buyer fund loss.
	guestEvmAvailable := false
	guestSolanaAvailable := false

	n.guestOrderService = guest.NewGuestOrderAppService(guest.GuestOrderAppServiceConfig{
		DB:                     n.db,
		DirectPayment:          n.directPaymentService,
		SweepService:           n.autoSweepService,
		EventBus:               n.eventBus,
		NodeID:                 n.nodeID,
		Shutdown:               n.shutdown,
		Listings:               n.listingService,
		ExchangeRates:          n.exchangeRates,
		Resolver:               n.featureResolver,
		SupportedUTXOChains:    supportedUTXO,
		EVMMonitorAvailable:    guestEvmAvailable,
		SolanaMonitorAvailable: guestSolanaAvailable,
	})
	if n.multiwallet != nil {
		n.guestOrderService.SetMultiwallet(n.multiwallet)
	}

	n.guestPaymentMonitor = guest.NewGuestPaymentMonitor(n.db, n.guestOrderService, nil, nil)
	n.guestPaymentMonitor.SetMultiwallet(n.multiwallet)
	n.guestOrderService.SetPaymentWatcher(n.guestPaymentMonitor)

	n.unifiedOrderView = NewUnifiedOrderView(n.orderService, n.guestOrderService, n.db)
}

// detectGuestUTXOChains returns the UTXO chains that should be enabled for
// guest checkout in the full build, based on which UTXO wallets the
// multiwallet has loaded AND for which a sweeper exists today. BCH/ZEC are
// excluded because the sweep path only signs P2WPKH (BIP-143) — see
// node_lifecycle_private_distribution.go for the matching gate. When a P2PKH-capable
// sweeper is added, drop the isSweepableP2WPKHChain filter here.
func (n *MobazhaNode) detectGuestUTXOChains() []iwallet.ChainType {
	if n.multiwallet == nil {
		return nil
	}
	out := make([]iwallet.ChainType, 0, 4)
	for _, chain := range n.multiwallet.SupportedChains() {
		if !chain.IsUTXOChain() {
			continue
		}
		if !isSweepableP2WPKHChain(chain) {
			continue
		}
		out = append(out, chain)
	}
	return out
}
