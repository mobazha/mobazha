package core

import (
	"encoding/json"
	"time"

	"github.com/mobazha/mobazha/internal/core/digital"
	"github.com/mobazha/mobazha/internal/core/guest"
	"github.com/mobazha/mobazha/internal/core/order"
	"github.com/mobazha/mobazha/internal/core/payment"
	"github.com/mobazha/mobazha/internal/core/settlement"
	"github.com/mobazha/mobazha/internal/repo"
	pkgcollateral "github.com/mobazha/mobazha/pkg/collateral"
	pkgconfig "github.com/mobazha/mobazha/pkg/config"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/events"
	"github.com/mobazha/mobazha/pkg/models"
)

// MobazhaNode holds all the components that make up a network node
// on the Mobazha network. It also exposes an exported API which can
// be used to control the node.
//
// Fields are organized into logical groups via anonymous embedding.
// Access remains flat: n.peerID, n.testnet, etc. Only struct literal
// construction in builder.go needs nested syntax.
type MobazhaNode struct {
	sharedManager *SharedManager

	identityFields
	storageFields
	cryptoFields
	networkFields
	walletFields
	chainFields
	ipnsFields
	modeFlags
	lifecycleFields
	appServices
	platformFields
	orderExtensionFields
}

// identityFields, storageFields, cryptoFields, networkFields, walletFields,
// chainFields, ipnsFields, and platformFields are defined in node_fields.go.

// modeFlags groups boolean mode switches.
type modeFlags struct {
	testnet            bool
	walletTestnet      bool
	torOnly            bool
	infrastructureOnly bool
	sovereign          bool
}

// lifecycleFields groups runtime lifecycle state.
type lifecycleFields struct {
	publishActive        int32
	publishChan          chan pubCloser
	featureManager       *pkgconfig.FeatureManager
	shutdownTorFunc      func() error
	initialBootstrapChan chan struct{}
	shutdown             chan struct{}
	stopped              int32
	orderLockManager     *order.OrderLockManager
}

// appServices groups all extracted App Service dependencies.
type appServices struct {
	paymentService             *payment.PaymentAppService
	settlementService          *settlement.SettlementService
	orderService               *order.OrderAppService
	matrixChatService          contracts.MatrixChatService
	matrixCryptoStore          interface{} // shared *dbutil.Database for SaaS multi-tenant; nil = SQLite
	preferencesService         *PreferencesAppService
	mediaService               *MediaAppService
	ratingsService             *RatingsAppService
	profileService             *ProfileAppService
	followService              *FollowAppService
	postsService               *PostsAppService
	moderationService          *ModerationAppService
	listingService             *ListingAppService
	notificationService        *NotificationAppService
	shoppingCartService        *ShoppingCartAppService
	wishlistService            *WishlistAppService
	discountService            *DiscountAppService
	collectionService          *CollectionAppService
	storePolicyService         *StorePolicyAppService
	sellerAffiliateService     *SellerAffiliateAppService
	fiatRegistry               contracts.FiatProviderRegistry
	fiatPaymentService         *FiatPaymentAppService
	embeddedWalletRegistry     contracts.EmbeddedWalletProviderRegistry
	onrampRegistry             contracts.OnrampProviderRegistry
	onrampFundingService       *payment.OnrampFundingAppService
	supplyChainRegistry        contracts.FulfillmentProviderRegistry
	supplyChainService         *SupplyChainAppService
	supplyAvailabilityService  contracts.SupplyAvailabilityService
	shippingService            *ShippingAppService
	analyticsService           *AnalyticsAppService
	paymentVerificationService *payment.PaymentVerificationService
	netDBSyncService           *NetDBSyncService
	guestOrderService          *guest.GuestOrderAppService
	directPaymentService       *guest.DirectPaymentService
	walletAccountService       contracts.WalletAccountService
	receivingAccountService    *receivingAccountService
	guestPaymentMonitor        *guest.GuestPaymentMonitor
	unifiedOrderView           *UnifiedOrderView
	digitalAssetService        *digital.DigitalAssetAppService
	digitalEntitlementService  *digital.DigitalEntitlementAppService
	paymentSessionService      contracts.PaymentSessionService
	collateralRail             pkgcollateral.Rail

	// Feature flag resolver infrastructure (Phase FF-3).
	// featureResolver is the SSOT for `isEnabled(ctx, key)` queries; it
	// combines the three providers below under the registry's AllowedScopes.
	// Providers remain injectable so SaaS hosting can swap in cross-tenant
	// adapters (platform-global config from app.yaml, proxying tenant store,
	// etc.) without forking core.
	featureResolver         pkgconfig.ResolverInterface
	platformFeatureProvider pkgconfig.PlatformGlobalProvider
	tenantFeatureStore      pkgconfig.TenantFeatureStore
	nodeFeatureProvider     pkgconfig.NodeFeatureProvider

	// featureAuditLogger persists feature-flag write events to the
	// feature_flag_audit_logs table. Initialised in initFeatureResolver
	// once a database is available; remains nil on infrastructure-only /
	// mock nodes, in which case handlers fall back to log-and-continue.
	// See pkg/contracts/features.go FeatureAuditProvider.
	featureAuditLogger contracts.FeatureAuditLogger
}

// IsDefaultNode returns whether this node is the default node.
func (n *MobazhaNode) IsDefaultNode() bool {
	return n.nodeID == repo.DefaultNodeID
}

// Lifecycle and cross-tenant methods are shared by every distribution. The
// selected runtime profile controls which workers they start.

func (n *MobazhaNode) checkRepoMigration() error {
	version, err := n.repo.ReadVersion()
	if err != nil {
		return err
	}

	if version != repo.DefaultRepoVersion {
		if err := n.repo.WriteVersion(repo.DefaultRepoVersion); err != nil {
			return err
		}
	}
	return nil
}

// UsingTestnet returns whether or not this node is running on
// the test network.
func (n *MobazhaNode) UsingTestnet() bool {
	return n.testnet
}

// UsingWalletTestnet returns whether or not this node is using
// testnet for wallet transactions (coins and chains).
func (n *MobazhaNode) UsingWalletTestnet() bool {
	return n.walletTestnet
}

// UsingTorMode returns whether or not this node is using the tor
// network exclusively. Dual stack returns false for this.
func (n *MobazhaNode) UsingTorMode() bool {
	return n.torOnly
}

// DestroyNode shutsdown the node and deletes the entire data directory.
// This should only be used during testing as destroying a live node will
// result in data loss.
func (n *MobazhaNode) DestroyNode() {
	n.Stop(true)
	n.repo.DestroyRepo()
}

// DB returns the node's database.
func (n *MobazhaNode) DB() database.Database {
	return n.db
}

// GetNodeID returns the user ID for this node.
func (n *MobazhaNode) GetNodeID() string {
	return n.nodeID
}

func (n *MobazhaNode) SharedManager() *SharedManager {
	return n.sharedManager
}

// SubscribeEvent returns a subscription to the provided event. The event argument
// may be an interface slice.
func (n *MobazhaNode) SubscribeEvent(event interface{}) (events.Subscription, error) {
	return n.eventBus.Subscribe(event)
}

// EventBus returns the node's event bus.
func (n *MobazhaNode) EventBus() events.Bus {
	return n.eventBus
}

// StoreConfig reads the storefront branding config from the database.
func (n *MobazhaNode) StoreConfig() (json.RawMessage, error) {
	val, err := n.getSetting(models.SettingsKeyStoreConfig)
	if err != nil {
		return nil, err
	}
	if val == "" {
		return nil, nil
	}
	return json.RawMessage(val), nil
}

// SaveStoreConfig persists the storefront branding config.
func (n *MobazhaNode) SaveStoreConfig(cfg json.RawMessage) error {
	if err := n.saveSetting(models.SettingsKeyStoreConfig, string(cfg)); err != nil {
		return err
	}
	if n.eventBus != nil {
		n.eventBus.Emit(&events.StorefrontChanged{Config: cfg})
	}
	return nil
}

// StoreDraftConfig reads the unpublished storefront draft, if any.
// Read errors propagate — folding them into "no draft" would let the editor
// silently overwrite a stored draft after a transient DB failure.
func (n *MobazhaNode) StoreDraftConfig() (json.RawMessage, error) {
	val, err := n.getSetting(models.SettingsKeyStoreConfigDraft)
	if err != nil {
		return nil, err
	}
	if val == "" {
		return nil, nil
	}
	return json.RawMessage(val), nil
}

// SaveStoreDraftConfig persists the storefront draft. Drafts never emit
// StorefrontChanged — they must not propagate to public caches.
func (n *MobazhaNode) SaveStoreDraftConfig(cfg json.RawMessage) error {
	return n.saveSetting(models.SettingsKeyStoreConfigDraft, string(cfg))
}

// DeleteStoreDraftConfig discards the storefront draft.
func (n *MobazhaNode) DeleteStoreDraftConfig() error {
	return n.saveSetting(models.SettingsKeyStoreConfigDraft, "")
}

// maxStoreConfigHistory bounds the published-revision archive. Ten covers a
// season of layout changes at ~100KB each without the settings row growing
// unbounded.
const maxStoreConfigHistory = 10

// StoreConfigHistoryEntry is one previously-published storefront config.
type StoreConfigHistoryEntry struct {
	PublishedAt time.Time       `json:"publishedAt"`
	Config      json.RawMessage `json:"config"`
}

// StoreConfigHistory returns previously-published storefront configs,
// newest first. Empty (not nil) when nothing has been superseded yet.
func (n *MobazhaNode) StoreConfigHistory() (json.RawMessage, error) {
	val, err := n.getSetting(models.SettingsKeyStoreConfigHistory)
	if err != nil {
		return nil, err
	}
	if val == "" {
		return json.RawMessage("[]"), nil
	}
	return json.RawMessage(val), nil
}

// PublishStoreConfig replaces the live storefront config and clears the
// draft slot in ONE transaction, so a partial failure can never leave the
// live config replaced with a stale draft still pending. The superseded
// live config is archived into the history slot in the same transaction —
// history exists precisely for the publishes the seller regrets, so it
// cannot be allowed to miss one.
func (n *MobazhaNode) PublishStoreConfig(cfg json.RawMessage) error {
	err := n.db.Update(func(tx database.Tx) error {
		var prev models.NodeSettings
		result := tx.Read().Where("\"key\" = ?", models.SettingsKeyStoreConfig).Limit(1).Find(&prev)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected > 0 && prev.Value != "" {
			var history []StoreConfigHistoryEntry
			var histRow models.NodeSettings
			histResult := tx.Read().Where("\"key\" = ?", models.SettingsKeyStoreConfigHistory).Limit(1).Find(&histRow)
			if histResult.Error != nil {
				return histResult.Error
			}
			if histResult.RowsAffected > 0 && histRow.Value != "" {
				// A corrupt archive should not block publishing; start over.
				if err := json.Unmarshal([]byte(histRow.Value), &history); err != nil {
					history = nil
				}
			}
			history = append([]StoreConfigHistoryEntry{{
				PublishedAt: time.Now().UTC(),
				Config:      json.RawMessage(prev.Value),
			}}, history...)
			if len(history) > maxStoreConfigHistory {
				history = history[:maxStoreConfigHistory]
			}
			encoded, err := json.Marshal(history)
			if err != nil {
				return err
			}
			if err := tx.Save(&models.NodeSettings{Key: models.SettingsKeyStoreConfigHistory, Value: string(encoded)}); err != nil {
				return err
			}
		}
		if err := tx.Save(&models.NodeSettings{Key: models.SettingsKeyStoreConfig, Value: string(cfg)}); err != nil {
			return err
		}
		return tx.Save(&models.NodeSettings{Key: models.SettingsKeyStoreConfigDraft, Value: ""})
	})
	if err != nil {
		return err
	}
	if n.eventBus != nil {
		n.eventBus.Emit(&events.StorefrontChanged{Config: cfg})
	}
	return nil
}

// StorefrontPreviewToken reads the stored draft-preview token record
// (JSON {token, expiresAt}), or nil when none has been issued.
func (n *MobazhaNode) StorefrontPreviewToken() (json.RawMessage, error) {
	val, err := n.getSetting(models.SettingsKeyStorefrontPreviewToken)
	if err != nil {
		return nil, err
	}
	if val == "" {
		return nil, nil
	}
	return json.RawMessage(val), nil
}

// SaveStorefrontPreviewToken stores the draft-preview token record,
// replacing (and thereby revoking) any previous one.
func (n *MobazhaNode) SaveStorefrontPreviewToken(record json.RawMessage) error {
	return n.saveSetting(models.SettingsKeyStorefrontPreviewToken, string(record))
}

// getSetting reads a single key from the node_settings table.
func (n *MobazhaNode) getSetting(key string) (string, error) {
	var setting models.NodeSettings
	var found bool
	err := n.db.View(func(tx database.Tx) error {
		result := tx.Read().Where("\"key\" = ?", key).Limit(1).Find(&setting)
		if result.Error != nil {
			return result.Error
		}
		found = result.RowsAffected > 0
		return nil
	})
	if err != nil {
		return "", err
	}
	if !found {
		return "", nil
	}
	return setting.Value, nil
}

// saveSetting upserts a key-value pair in the node_settings table.
func (n *MobazhaNode) saveSetting(key, value string) error {
	return n.db.Update(func(tx database.Tx) error {
		return tx.Save(&models.NodeSettings{Key: key, Value: value})
	})
}

// MigrateNodeSettings creates the node_settings table if it doesn't exist.
func MigrateNodeSettings(db database.Database) error {
	return db.Update(func(tx database.Tx) error {
		return tx.Migrate(&models.NodeSettings{})
	})
}

// AgentStore, ProfileName, ProductCatalog, and SchedulerHooks are defined in
// node_methods.go.
