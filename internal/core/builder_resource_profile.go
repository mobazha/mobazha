package core

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"strings"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/gagliardetto/solana-go"
	"github.com/libp2p/go-libp2p/core/peer"
	aipkg "github.com/mobazha/mobazha/internal/ai"
	"github.com/mobazha/mobazha/internal/core/checkoutsupply"
	"github.com/mobazha/mobazha/internal/core/guest"
	"github.com/mobazha/mobazha/internal/logger"
	"github.com/mobazha/mobazha/internal/notifications"
	"github.com/mobazha/mobazha/internal/repo"
	pkgconfig "github.com/mobazha/mobazha/pkg/config"
	pkgcontracts "github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/core/coreiface"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/distribution"
	"github.com/mobazha/mobazha/pkg/events"
	"github.com/mobazha/mobazha/pkg/models"
)

func newResourceProfileNode(ctx context.Context, cfg *repo.Config, nodeID string,
	hs coreiface.HostService, composition distribution.SovereignNodeConfig,
	options []NodeOption,
) (*MobazhaNode, error) {
	if hs != nil {
		return nil, errors.New("sovereign node cannot be composed with a hosting service")
	}

	if err := repo.CheckAndMigrateRepo(cfg.DataDir); err != nil {
		return nil, fmt.Errorf("repo migration: %w", err)
	}
	if nodeID == "" {
		nodeID = repo.DefaultNodeID
	}

	nodeRepoPath := path.Join(cfg.DataDir, "nodes", nodeID)
	if err := os.MkdirAll(nodeRepoPath, 0755); err != nil {
		return nil, fmt.Errorf("create node dir: %w", err)
	}

	r, err := repo.NewRepo(nodeID, nodeRepoPath, cfg.Testnet)
	if err != nil {
		return nil, fmt.Errorf("repo init: %w", err)
	}
	succeeded := false
	var nodeCancel context.CancelFunc
	var sm *SharedManager
	defer func() {
		if succeeded {
			return
		}
		if nodeCancel != nil {
			nodeCancel()
		}
		r.Close()
		if sm != nil {
			sm.Stop()
		}
	}()

	db := r.DB()
	if err := MigrateNodeSettings(db); err != nil {
		log.Warningf("Failed to migrate node settings: %v", err)
	}
	peerID, signer, bip44Key, keyProvider, err := loadResourceProfileIdentityAndKeys(db)
	if err != nil {
		return nil, err
	}

	nodeCtx, cancelNode := context.WithCancel(ctx)
	nodeCancel = cancelNode
	bus := events.NewBus()

	fm := pkgconfig.GetGlobalFeatureManager()

	n := &MobazhaNode{
		identityFields: identityFields{
			nodeID:     nodeID,
			peerID:     peerID,
			nodeCtx:    nodeCtx,
			nodeCancel: nodeCancel,
		},
		storageFields: storageFields{
			contentStore: composition.ContentStore,
			db:           db,
			repo:         r,
		},
		cryptoFields: cryptoFields{
			signer:      signer,
			keyProvider: keyProvider,
			bip44Key:    bip44Key,
		},
		networkFields: networkFields{
			eventBus: bus,
		},
		chainFields: chainFields{sovereignPolicy: composition.Policy},
		modeFlags: modeFlags{
			testnet:   cfg.Testnet,
			sovereign: true,
		},
		lifecycleFields: lifecycleFields{
			shutdown:       make(chan struct{}),
			featureManager: fm,
			publishChan:    make(chan pubCloser),
		},
	}
	applyNodeOptions(n, options)

	if n.contentStore == nil {
		n.contentStore = &cidContentStore{}
	}

	sm, err = NewResourceProfileSharedManager(ctx, cfg, composition.TrustedHumaModules, composition.Policy)
	if err != nil {
		return nil, fmt.Errorf("shared manager: %w", err)
	}
	n.sharedManager = sm

	n.initResourceProfileServices(cfg)
	if err := n.registerSovereignPaymentModules(); err != nil {
		return nil, fmt.Errorf("register sovereign payment modules: %w", err)
	}
	sm.AddNode(nodeID, n)

	succeeded = true
	return n, nil
}

func loadResourceProfileIdentityAndKeys(db database.Database) (peer.ID, pkgcontracts.Signer, *hdkeychain.ExtendedKey, pkgcontracts.KeyProvider, error) {
	var (
		dbIdentityKey models.Key
		dbEscrowKey   models.Key
		dbBip44Key    models.Key
		dbSolKey      models.Key
		dbRatingKey   models.Key
	)
	if err := db.View(func(tx database.Tx) error {
		if err := tx.Read().Where("name = ?", "identity").First(&dbIdentityKey).Error; err != nil {
			return fmt.Errorf("load identity key: %w", err)
		}
		var err error
		dbEscrowKey, dbBip44Key, dbSolKey, dbRatingKey, err = repo.GetKeysFromDB(tx)
		return err
	}); err != nil {
		return "", nil, nil, nil, err
	}

	signer, err := pkgcontracts.NewKeyPairSignerFromMarshaledKey(dbIdentityKey.Value)
	if err != nil {
		return "", nil, nil, nil, fmt.Errorf("identity signer: %w", err)
	}
	peerID, err := peer.Decode(string(signer.PeerID()))
	if err != nil {
		return "", nil, nil, nil, fmt.Errorf("decode identity peer ID: %w", err)
	}

	bip44Key, err := hdkeychain.NewKeyFromString(string(dbBip44Key.Value))
	if err != nil {
		return "", nil, nil, nil, fmt.Errorf("parse BIP44 key: %w", err)
	}

	escrowKey, _ := btcec.PrivKeyFromBytes(dbEscrowKey.Value)
	ratingKey, _ := btcec.PrivKeyFromBytes(dbRatingKey.Value)

	ethChild, err := bip44Key.Derive(hdkeychain.HardenedKeyStart + 60)
	if err != nil {
		return "", nil, nil, nil, fmt.Errorf("derive ETH key: %w", err)
	}
	ethPrivKey, err := ethChild.ECPrivKey()
	if err != nil {
		return "", nil, nil, nil, fmt.Errorf("parse ETH privkey: %w", err)
	}

	solPrivKey := solana.PrivateKey(dbSolKey.Value)

	kp := newFileKeyProvider(ethPrivKey, escrowKey, ratingKey, &solPrivKey, ethPrivKey)

	return peerID, signer, bip44Key, kp, nil
}

// initResourceProfileServices initialises the local-first App Service subset.
//
// Alignment with the standard profile's applyOptions service graph
// ====================================================
// Every service listed in applyOptions MUST appear below as either
// "covered" (initialised) or "excluded" (with reason). The arch guard
// architecture tests enforce this — adding a new
// initXxx call in applyOptions without updating this list will fail CI.
//
//	initProfileService          → covered (simplified, no escrow/NetDB keys)
//	initModerationService       → excluded: no P2P moderator network
//	initListingService          → covered (simplified, no LocalListingCrypto/NetDB)
//	initPaymentService          → excluded: Escrow — needs multiwallet/relay
//	  ReceivingAccountService split out (OP-1.3), separately initialized
//	initSettlementService       → excluded: money-out — needs multiwallet/relay (same as paymentService)
//	initPaymentVerificationService → excluded: Escrow verification
//	initOrderService            → excluded: Escrow orders — needs multiwallet/messenger
//	wireServiceSetters          → excluded: wires Escrow service graph
//	initMatrixChatService       → excluded: needs privKey/Matrix homeserver
//	initPreferencesService      → covered (noOpBanChecker)
//	initMediaService            → covered (no BlobStore/PublishFile)
//	initRatingsService          → excluded: needs NetDB.GetRatingIndex
//	initNotificationService     → covered (local SQLite query service;
//	                                outbound channel delivery — Telegram/Discord —
//	                                is omitted by the restricted API profile)
//	initShoppingCartService     → covered
//	initWishlistService         → excluded: no user accounts in sovereign
//	initFollowService           → excluded: needs Messenger/NetDB
//	initPostsService            → excluded: needs P2P Publish
//	initAnalyticsService        → excluded: deferred to AN-1+
//	initNetDBSyncService        → excluded: needs NetDB
//	initFeatureResolver         → covered (initSovereignFeatureResolver)
//	initGuestOrderService       → covered (OP-1.3)
func (n *MobazhaNode) initResourceProfileServices(cfg *repo.Config) {
	n.preferencesService = NewPreferencesAppService(PreferencesAppServiceConfig{
		DB:         n.db,
		BanChecker: noOpBanChecker{},
	})
	n.mediaService = NewMediaAppService(MediaAppServiceConfig{
		DB:           n.db,
		ContentStore: n.contentStore,
		NodeID:       n.nodeID,
		Publish:      n.Publish,
		EventBus:     n.eventBus,
	})
	n.profileService = NewProfileAppService(ProfileAppServiceConfig{
		DB:       n.db,
		Publish:  n.Publish,
		EventBus: n.eventBus,
		NodeID:   n.nodeID,
		PeerID:   n.peerID,
	})
	initDiscountSubsystem(n)
	initCollectionSubsystem(n)
	initStorePolicySubsystem(n)
	initSellerAffiliateSubsystem(n)
	initShippingSubsystem(n)
	n.listingService = NewListingAppService(ListingAppServiceConfig{
		DB:             n.db,
		Signer:         n.signer,
		ContentStore:   n.contentStore,
		EventBus:       n.eventBus,
		BanChecker:     noOpBanChecker{},
		Keys:           n.keyProvider,
		FeatureManager: n.featureManager,
		NodeID:         n.peerID,
		Testnet:        n.testnet,
		Publish:        n.Publish,
		ShippingStore:  n.shippingService.Store(),
		ListingPolicy:  n.sovereignPolicy,
	})
	if n.collectionService != nil {
		n.listingService.onDeleteCleanup = func(slug string) {
			if err := n.collectionService.RemoveProductFromAllCollections(context.Background(), slug); err != nil {
				log.Errorf("Collection: failed to remove product %s from collections: %v", slug, err)
			}
		}
	}
	n.shoppingCartService = NewShoppingCartAppService(ShoppingCartAppServiceConfig{
		DB:       n.db,
		EventBus: n.eventBus,
		NodeID:   n.nodeID,
	})
	// NotificationAppService is a pure local-SQLite query service with no
	// P2P / Matrix / outbound dependency. Sellers running a sovereign node rely on
	// the notification feed for new guest orders, payment events, auto-sweep
	// status, and system health alerts. Outbound channel delivery (Telegram /
	// Discord) is not registered by the restricted product surface, so only
	// the local feed is exposed.
	n.notificationService = NewNotificationAppService(NotificationAppServiceConfig{
		DB: n.db,
	})
	n.initReceivingAccountService()

	if n.bip44Key != nil {
		n.walletAccountService = NewWalletAccountService(n.db, n.bip44Key, n.multiwallet)
		n.directPaymentService = guest.NewDirectPaymentService(n.db)
		n.directPaymentService.SetWalletAccountService(n.walletAccountService)
	}

	initSovereignFeatureResolver(n)
	seedSovereignFeatureFlags(n, cfg)

	// Availability remains a per-request decision, but the private runtime
	// owns the layered health/provisioning/wallet-open policy. Core applies a
	// short deadline and consumes only the final capability verdict.
	if n.supplyAvailabilityService == nil {
		initSupplyAvailabilitySubsystem(n)
	}
	n.guestOrderService = guest.NewGuestOrderAppService(guest.GuestOrderAppServiceConfig{
		Context:            n.nodeCtx,
		DB:                 n.db,
		DirectPayment:      n.directPaymentService,
		EventBus:           n.eventBus,
		NodeID:             n.nodeID,
		PeerID:             n.peerID.String(),
		Shutdown:           n.shutdown,
		Listings:           n.listingService,
		ExchangeRates:      n.exchangeRates,
		Resolver:           n.featureResolver,
		SupplyAvailability: n.supplyAvailabilityService,
		GuestPaymentPolicy: n.sovereignPolicy,
		SellerAffiliate:    n.sellerAffiliateService,
		BillingHoldActive: func() bool {
			if n.preferencesService == nil {
				return false
			}
			prefs, err := n.preferencesService.GetPreferences()
			if err != nil || prefs == nil {
				return false
			}
			active, err := prefs.BillingHoldActive()
			return err == nil && active
		},
	})

	n.guestPaymentMonitor = guest.NewGuestPaymentMonitor(n.db, n.guestOrderService, nil)
	n.guestOrderService.SetPaymentWatcher(n.guestPaymentMonitor)

	checkoutSupplyQuoter := checkoutsupply.NewCheckoutSupplyQuoteService(checkoutsupply.CheckoutSupplyQuoteServiceConfig{
		DB:                 n.db,
		SupplyAvailability: n.supplyAvailabilityService,
		Resolver:           n.featureResolver,
		Listings:           n.listingService,
	})
	n.guestOrderService.SetCheckoutSupplyQuoter(checkoutSupplyQuoter)

	initDigitalSubsystem(n)
	initSovereignEventDispatcher(n, n.sharedManager)
}

func initSovereignFeatureResolver(n *MobazhaNode) {
	if n.featureResolver != nil {
		return
	}

	platform := n.platformFeatureProvider
	if platform == nil {
		// Sovereign has no SaaS platform governance. Use a passthrough
		// provider that returns configured=false with no error so the
		// Resolver falls back to DefaultValue — EXCEPT we override with
		// an allow-all provider that never vetoes, letting the Tenant
		// and Node layers be the effective decision-makers.
		platform = sovereignPlatformProvider{}
		n.platformFeatureProvider = platform
	}

	tenant := n.tenantFeatureStore
	if tenant == nil && n.db != nil {
		store := NewFeatureOverrideStore(n.db)
		if err := store.Migrate(); err != nil {
			log.Errorf("feature_override: migrate failed: %v", err)
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

	if n.featureAuditLogger == nil && n.db != nil {
		auditStore := NewFeatureAuditLogStore(n.db)
		if err := auditStore.Migrate(); err != nil {
			log.Errorf("feature_audit: migrate failed: %v", err)
		}
		n.featureAuditLogger = auditStore
	}
}

// seedSovereignFeatureFlags persists CLI-flag-driven feature states into the
// tenant override store so the Tenant layer of the Resolver reflects CLI flags.
func seedSovereignFeatureFlags(n *MobazhaNode, cfg *repo.Config) {
	if cfg == nil || n.tenantFeatureStore == nil {
		return
	}
	ctx := context.Background()
	tenantID := database.StandaloneTenantID
	if cfg.GuestCheckout {
		if err := n.tenantFeatureStore.Set(ctx, tenantID, pkgconfig.FeatureGuestCheckoutEnabled.Key, true, "cli-flag"); err != nil {
			log.Errorf("seed feature flag guestCheckout: %v", err)
		}
	}
}

// initDiscountSubsystem / initCollectionSubsystem are in builder_shared.go
// (shared by standard and sovereign profiles).

// initShippingSubsystem / safeListingPublisher are in builder_shared.go
// (shared by standard and sovereign profiles).

// initSovereignEventDispatcher creates a minimal local EventDispatcher:
// only NotificationSink (DB + WS push) — no webhook engine, multi-channel
// notifier, or AI proxy. Ensures the frontend receives real-time order/payment
// notifications over WebSocket.
func initSovereignEventDispatcher(n *MobazhaNode, sm *SharedManager) {
	var notifyWsFn func(any) error
	if gw := sm.GetHTTPGateway(); gw != nil {
		gw.EnsureHubForUser(n.nodeID)
		notifyWsFn = gw.NotifyWebsockets(n.nodeID)
	}

	notifSink := notifications.NewNotificationSink(n.db, notifyWsFn)
	n.eventDispatcher = events.NewDispatcher(n.eventBus, notifSink)
	logger.LogInfoWithID(log, n.nodeID, "Sovereign event dispatcher initialized (NotificationSink only)")

	resolvedOllamaURL := resolveBundledOllamaURL()
	probeHTTPClient := aipkg.NewLocalhostOnlyClient()
	if aipkg.IsTrustedLocalLLMEndpoint(resolvedOllamaURL) && !aipkg.IsLocalEndpointURL(resolvedOllamaURL) {
		probeHTTPClient = aipkg.NewPlainHTTPOnlyClient()
	}

	// Auto-detect bundled Ollama first so we know which HTTP client to use.
	// autoConfigureBundledOllama seeds the config only when no provider is set.
	ollamaURL := autoConfigureBundledOllama(n, resolvedOllamaURL, probeHTTPClient)

	// Choose the AI HTTP client based on the Ollama URL:
	// - localhost/loopback → strict LocalhostOnlyClient (DNS-verified loopback)
	// - plain HTTP non-loopback (Docker-internal) → PlainHTTPOnlyClient (blocks TLS to external)
	// - no bundled Ollama → fall back to LocalhostOnlyClient (user must configure localhost)
	var aiHTTPClient *http.Client
	if ollamaURL != "" && !aipkg.IsLocalEndpointURL(ollamaURL) {
		aiHTTPClient = aipkg.NewPlainHTTPOnlyClient()
	} else {
		aiHTTPClient = aipkg.NewLocalhostOnlyClient()
	}
	n.aiProxy = aipkg.NewProxy(aiHTTPClient)
}

const bundledOllamaModel = "llama3.2"

// resolveBundledOllamaURL returns the base URL to probe for a bundled Ollama
// instance. It checks the OLLAMA_HOST environment variable first (set by
// Docker Compose), then falls back to the native-binary default.
func resolveBundledOllamaURL() string {
	if h := os.Getenv("OLLAMA_HOST"); h != "" {
		// OLLAMA_HOST may or may not include a path; normalise to /v1 suffix.
		h = strings.TrimRight(h, "/")
		if !strings.HasSuffix(h, "/v1") {
			h += "/v1"
		}
		return h
	}
	return "http://localhost:11434/v1"
}

// autoConfigureBundledOllama seeds a default AI config when Ollama is
// reachable and no provider has been configured yet.
// Returns the configured Ollama base URL (for caller to pick the right HTTP
// client), or "" when auto-config was skipped or failed.
func autoConfigureBundledOllama(n *MobazhaNode, ollamaURL string, probeHTTPClient *http.Client) string {
	mc := n.AIMultiConfig()
	if mc.ActiveProvider != "" && mc.Providers != nil {
		// Config exists. If it points at the auto-detected Ollama URL, verify the
		// saved model still exists; re-probe if the model looks like a stale
		// fallback (i.e. it matches bundledOllamaModel without a tag suffix while
		// Ollama reports a tagged variant).
		if cred, ok := mc.Providers[mc.ActiveProvider]; ok {
			if cred.BaseURL == ollamaURL && cred.Model == bundledOllamaModel {
				// Stale fallback: re-probe to pick up the actual installed tag.
				if ready, detectedModel := aipkg.ProbeOllamaReady(probeHTTPClient, ollamaURL); ready && detectedModel != "" && detectedModel != bundledOllamaModel {
					cred.Model = detectedModel
					mc.SetProvider(mc.ActiveProvider, cred)
					if err := n.SaveAIMultiConfig(mc); err == nil {
						log.Infof("Updated bundled Ollama model from %s to %s", bundledOllamaModel, detectedModel)
					}
					return ollamaURL
				}
			}
			if cred.BaseURL != "" {
				return cred.BaseURL
			}
		}
		return ""
	}

	ready, detectedModel := aipkg.ProbeOllamaReady(probeHTTPClient, ollamaURL)
	if !ready {
		log.Infof("Bundled Ollama not reachable at %s — AI auto-config skipped", ollamaURL)
		return ""
	}

	// Prefer the first model reported by Ollama over the hardcoded fallback.
	// This handles cases where the bundled model is "llama3.2:1b" instead of "llama3.2".
	model := detectedModel
	if model == "" {
		model = bundledOllamaModel
	}

	mc = aipkg.MultiConfig{
		Enabled:        true,
		ActiveProvider: "custom",
		Providers: map[string]aipkg.ProviderCredential{
			"custom": {
				BaseURL: ollamaURL,
				Model:   model,
			},
		},
	}
	if err := n.SaveAIMultiConfig(mc); err != nil {
		log.Warningf("Failed to seed bundled Ollama config: %v", err)
		return ""
	}
	log.Infof("Bundled Ollama detected at %s — AI auto-configured with model=%s", ollamaURL, model)
	return ollamaURL
}

type noOpBanChecker struct{}

func (noOpBanChecker) IsGlobalBanned(peer.ID) bool { return false }
func (noOpBanChecker) AddBlockedID(peer.ID)        {}
func (noOpBanChecker) RemoveBlockedID(peer.ID)     {}

// sovereignPlatformProvider reports every feature as configured=true at the
// PlatformGlobal layer. Sovereign nodes are fully sovereign — there is no
// SaaS platform to push overrides, and the platform layer is conceptually
// absent. The Resolver evaluates layers in (Platform AND Tenant AND Node)
// order; returning configured=false here would silently fall back to
// each feature's DefaultValue and short-circuit the AND chain (e.g.
// guestCheckout.DefaultValue=false would override the CLI-flag-driven
// tenant value seeded in seedSovereignFeatureFlags, making --guestcheckout
// a no-op).
//
// Side-effect risk is zero: PlatformGlobal-only features (multistore,
// storefronts, telegram bots, SaaS quotas, etc.) have their consumer code
// omitted from the restricted composition profile, so blanket approval here
// cannot expose Hosting-only surfaces by itself.
//
// Real activation decisions live in the Tenant layer (seeded from CLI
// flags / repo.Config) and the Node layer (process-local runtime state).
type sovereignPlatformProvider struct{}

func (sovereignPlatformProvider) Get(_ context.Context, _ string) (bool, bool, error) {
	return true, true, nil
}
