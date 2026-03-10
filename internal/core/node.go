package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/gagliardetto/solana-go"
	"github.com/ipfs/kubo/core"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	peer "github.com/libp2p/go-libp2p/core/peer"
	corecontracts "github.com/mobazha/mobazha-core/contracts"
	aipkg "github.com/mobazha/mobazha3.0/internal/ai"
	"github.com/mobazha/mobazha3.0/internal/config"
	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/internal/multiwallet/utxo"
	"github.com/mobazha/mobazha3.0/internal/net"
	"github.com/mobazha/mobazha3.0/internal/notifier"
	"github.com/mobazha/mobazha3.0/internal/orders"
	"github.com/mobazha/mobazha3.0/internal/repo"
	"github.com/mobazha/mobazha3.0/internal/wallet"
	pkgconfig "github.com/mobazha/mobazha3.0/pkg/config"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	pkgdb "github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/database/netdb"
	"github.com/mobazha/mobazha3.0/pkg/encryption"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/evm"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	wh "github.com/mobazha/mobazha3.0/pkg/webhook"
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

	// Webhook subsystem
	webhookStore  wh.EndpointStore
	webhookEngine *wh.Engine

	// Event subsystem
	eventDispatcher *events.Dispatcher
	notifierSink    *notifier.ChannelNotificationSink

	// SaaS co-tenant fast path (nil in standalone mode)
	coTenantPublicData contracts.CoTenantPublicDataFn

	// AI proxy
	aiProxy *aipkg.Proxy

	// Stripe account
	stripeAccountID string

	// Phase 2 encryption
	keyManager         *encryption.KeyManager
	localListingCrypto *encryption.LocalListingCrypto

	// Hosting interface
	hostService coreiface.HostService
}

// identityFields groups node identity and lifecycle context.
type identityFields struct {
	nodeID     string
	peerID     peer.ID
	privKey    crypto.PrivKey
	peerHost   host.Host
	nodeCtx    context.Context
	nodeCancel context.CancelFunc
}

// storageFields groups data storage dependencies.
type storageFields struct {
	ipfsNode     *core.IpfsNode
	contentStore contracts.ContentStore
	db           database.Database
	repo         *repo.Repo
}

// cryptoFields groups cryptographic key material and signing.
type cryptoFields struct {
	signer          corecontracts.Signer
	ethMasterKey    *btcec.PrivateKey
	escrowMasterKey *btcec.PrivateKey
	solPrivKey      *solana.PrivateKey
	ratingMasterKey *btcec.PrivateKey
	keyProvider     contracts.KeyProvider
}

// networkFields groups P2P networking components.
type networkFields struct {
	messenger              contracts.Messenger
	networkService         contracts.NetworkService
	banManager             *net.BanManager
	eventBus               events.Bus
	followerTracker        *FollowerTracker
	storeAndForwardServers []string
	boostrapPeers          []peer.ID
}

// walletFields groups wallet and payment processing.
type walletFields struct {
	multiwallet     contracts.WalletOperator
	orderProcessor  *orders.OrderProcessor
	exchangeRates   *wallet.ExchangeRateProvider
	paymentRegistry *payment.Registry
	relayAPIURL     string
}

// chainFields groups blockchain client configuration.
type chainFields struct {
	evmChainConfigs   []evm.EVMClientConfig
	solanaChainConfig *SolanaChainConfig
	monitorService    utxo.UTXOMonitorService
}

// ipnsFields groups IPNS resolution and NetDB configuration.
type ipnsFields struct {
	ipnsQuorum   uint
	ipnsResolver string
	netDB        *netdb.NetDB
	netConfig    *config.NetConfig
}

// modeFlags groups boolean mode switches.
type modeFlags struct {
	testnet       bool
	walletTestnet bool
	torOnly       bool
	ipfsOnlyMode  bool
}

// lifecycleFields groups runtime lifecycle state.
type lifecycleFields struct {
	publishActive        int32
	publishChan          chan pubCloser
	featureManager       *pkgconfig.FeatureManager
	shutdownTorFunc      func() error
	initialBootstrapChan chan struct{}
	shutdown             chan struct{}
}

// appServices groups all extracted App Service dependencies.
type appServices struct {
	paymentService      *PaymentAppService
	orderService        *OrderAppService
	chatService         *ChatAppService
	matrixService       *MatrixAppService
	preferencesService  *PreferencesAppService
	mediaService        *MediaAppService
	ratingsService      *RatingsAppService
	profileService      *ProfileAppService
	followService       *FollowAppService
	postsService        *PostsAppService
	moderationService   *ModerationAppService
	channelsService     *ChannelsAppService
	listingService      *ListingAppService
	notificationService *NotificationAppService
	shoppingCartService *ShoppingCartAppService
	wishlistService     *WishlistAppService
	discountService     *DiscountAppService
	collectionService   *CollectionAppService
	fiatRegistry        contracts.FiatProviderRegistry
	fiatPaymentService  *FiatPaymentAppService
	shippingService     *ShippingAppService
	analyticsService    *AnalyticsAppService
}

// IsDefaultNode returns whether this node is the default node.
func (n *MobazhaNode) IsDefaultNode() bool {
	return n.nodeID == repo.DefaultNodeID
}

// SetCoTenantPublicData injects a resolver for co-located tenant data on the
// same SaaS host. Called after construction; the deferred wrappers created
// during applyOptions will pick up the injected fn at call time.
func (n *MobazhaNode) SetCoTenantPublicData(fn contracts.CoTenantPublicDataFn) {
	n.coTenantPublicData = fn
}

// coTenantPublicDataDeferred returns a closure that forwards to
// n.coTenantPublicData at call time. This allows App Services to be
// initialized before SetCoTenantPublicData is called by hosting.
func (n *MobazhaNode) coTenantPublicDataDeferred() contracts.CoTenantPublicDataFn {
	return func(peerID peer.ID) (pkgdb.PublicData, error) {
		fn := n.coTenantPublicData
		if fn == nil {
			return nil, fmt.Errorf("co-tenant resolver not configured")
		}
		return fn(peerID)
	}
}

// Start gets the node up and running and listens for a signal interrupt.
func (n *MobazhaNode) Start() {
	// Check repo migration
	go func() {
		if err := n.checkRepoMigration(); err != nil {
			logger.LogErrorWithIDf(log, n.nodeID, "checkRepoMigration failed, %v", err)
		}
	}()

	// Migrate and sync shipping profiles at startup
	// 1. CheckAndMigrateShippingProfiles: For users who haven't migrated yet (have shippingOptions but no shippingProfiles)
	// 2. SyncShippingProfilesToListings: For users who migrated via frontend but listings weren't updated
	go func() {
		if n.preferencesService == nil {
			return
		}
		if err := n.preferencesService.CheckAndMigrateShippingProfiles(); err != nil {
			logger.LogErrorWithIDf(log, n.nodeID, "CheckAndMigrateShippingProfiles failed, %v", err)
		}
		if err := n.preferencesService.SyncShippingProfilesToListings(); err != nil {
			logger.LogErrorWithIDf(log, n.nodeID, "SyncShippingProfilesToListings failed, %v", err)
		}
	}()

	go n.bootstrapIPFS()

	// Default node always starts the SharedManager (HTTP gateway) regardless of mode,
	// because hosting proxies /v1/* requests to the internal API on port 5102.
	if n.IsDefaultNode() {
		go n.SharedManager().Start()
	}

	if !n.ipfsOnlyMode {
		n.publishHandler()
		go n.messenger.Start()
		go n.followerTracker.Start()

		go n.orderProcessor.Start()
		go n.syncMessages()
		go func() {
			n.multiwallet.Start()
		}()

		if n.eventDispatcher != nil {
			if err := n.eventDispatcher.Start(); err != nil {
				logger.LogErrorWithIDf(log, n.nodeID, "Failed to start event dispatcher: %v", err)
			}
		}
		go n.channelsService.OpenSavedChannels()

		if err := n.profileService.UpdateSNFServers(); err != nil {
			logger.LogErrorWithIDf(log, n.nodeID, "Error updating store and forward servers in profile: %s", err)
		}

		go n.listenPubsub()

		// Start UTXO payment monitor for external wallet payments
		go n.startUTXOPaymentMonitor()

		// ADR-7: Verify unconfirmed PaymentSent transactions on-chain
		go n.startPaymentVerificationLoop()

		// Inject EVM chain clients into wallets (symmetric with UTXO monitor above)
		// SaaS: shared clients from HostService; Standalone: per-node clients via factory
		n.startEVMChainClients()

		// Inject Solana chain client into wallet (symmetric with EVM and UTXO above)
		// SaaS: shared client from HostService; Standalone: per-node client + escrow resolution
		n.startSolanaChainClients()

		// Register payment strategies for all supported chains.
		// Must be called before startCancelablePaymentMonitor which uses the registry.
		n.registerPaymentStrategies()

		// Start unified cancelable payment monitor for auto-confirmation
		// This handles UTXO, EVM, and (future) Solana chains via event dispatch
		n.paymentService.StartCancelablePaymentMonitor()

		// Start RWA instant buy monitor for auto-confirmation
		// This handles RWA instant buy (atomic swap) orders that complete on-chain
		n.startRwaInstantBuyMonitor()
	}

	// Add log to verify connection reuse
	go func() {
		if n.peerHost == nil {
			return
		}
		conns := n.peerHost.Network().Conns()
		for _, conn := range conns {
			streams := conn.GetStreams()
			logger.LogDebugWithIDf(log, n.nodeID, "Connection to %s has %d streams",
				conn.RemotePeer(), len(streams))
		}
	}()
}

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

// Stop cleanly shutsdown the MobazhaNode and signals to any
// listening goroutines that it's time to stop.
func (n *MobazhaNode) Stop(force bool) error {
	if atomic.LoadInt32(&n.publishActive) > 0 && !force {
		return coreiface.ErrPublishingActive
	}

	if n.IsDefaultNode() {
		n.SharedManager().Stop()
	}

	if !n.ipfsOnlyMode {
		n.messenger.Stop()
		n.networkService.Close()
		n.orderProcessor.Stop()
		n.followerTracker.Close()
		n.multiwallet.Close()
		if n.channelsService != nil {
			n.channelsService.CloseAll()
		}
	}
	// Shutdown order matters: EventDispatcher must stop before WebhookEngine
	// so that WebhookSink stops emitting before the engine shuts down.
	if n.eventDispatcher != nil {
		n.eventDispatcher.Stop()
	}
	if n.webhookEngine != nil {
		n.webhookEngine.Stop()
	}
	if n.shutdownTorFunc != nil {
		n.shutdownTorFunc()
	}
	// Stop UTXO payment monitor (unregister from shared service if applicable)
	n.StopUTXOPaymentMonitor()
	close(n.shutdown)
	n.repo.Close()

	if n.ipfsNode != nil {
		// Full node: close the IPFS node
		stop := make(chan struct{})
		go func() {
			n.ipfsNode.Context().Done()
			n.ipfsNode.Close()
			time.AfterFunc(time.Second, func() {
				n.eventBus.Emit(&events.IPFSShutdown{})
			})
			close(stop)
		}()
		select {
		case <-time.After(time.Second * 2):
			return coreiface.ErrIPFSDelayedShutdown
		case <-stop:
		}
	} else {
		// Lightweight node: close the minimal libp2p host
		if n.peerHost != nil {
			n.peerHost.Close()
		}
		if n.nodeCancel != nil {
			n.nodeCancel()
		}
		n.eventBus.Emit(&events.IPFSShutdown{})
	}
	return nil
}

// UsingTestnet returns whether or not this node is running on
// the test network (IPFS bootstrap).
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

// IPFSNode returns the underlying IPFS node instance.
func (n *MobazhaNode) IPFSNode() *core.IpfsNode {
	return n.ipfsNode
}

// Multiwallet returns the WalletOperator interface.
// Internal callers that need concrete map access can type-assert to
// *multiwallet.Multiwallet.
func (n *MobazhaNode) Multiwallet() contracts.WalletOperator {
	return n.multiwallet
}

// DB returns the node's database.
func (n *MobazhaNode) DB() database.Database {
	return n.db
}

// ExchangeRates returns the node's exchange rate provider.
func (n *MobazhaNode) ExchangeRates() *wallet.ExchangeRateProvider {
	return n.exchangeRates
}

// GetNodeID returns the user ID for this node.
func (n *MobazhaNode) GetNodeID() string {
	return n.nodeID
}

func (n *MobazhaNode) SharedManager() *SharedManager {
	return n.sharedManager
}

// Identity returns the peer ID for this node.
func (n *MobazhaNode) Identity() peer.ID {
	return n.peerID
}

// PrivKey returns the libp2p private key for this node.
func (n *MobazhaNode) PrivKey() crypto.PrivKey {
	return n.privKey
}

// SignMessage signs a payload with the node's identity key via the injected Signer.
// Returns (signature, publicKeyBytes, error).
func (n *MobazhaNode) SignMessage(payload []byte) ([]byte, []byte, error) {
	if n.signer == nil {
		return nil, nil, fmt.Errorf("signer not available")
	}
	sig, err := n.signer.Sign(payload)
	if err != nil {
		return nil, nil, fmt.Errorf("signing payload: %w", err)
	}
	pubkey, err := n.signer.PublicKey()
	if err != nil {
		return nil, nil, fmt.Errorf("getting public key: %w", err)
	}
	return sig, pubkey, nil
}

// PeerHost returns the libp2p host for this node.
func (n *MobazhaNode) PeerHost() host.Host {
	return n.peerHost
}

// getIPFSNode returns the IPFS node for content operations.
// For full nodes, returns the node's own IPFS instance.
// For lightweight nodes, falls back to the shared IPFS node.
func (n *MobazhaNode) getIPFSNode() (*core.IpfsNode, error) {
	if n.ipfsNode != nil {
		return n.ipfsNode, nil
	}
	if shared := n.SharedManager().GetIPFSNode(); shared != nil {
		return shared, nil
	}
	return nil, errors.New("no IPFS node available")
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

// NetService returns the underlying NetworkService for this node.
func (n *MobazhaNode) NetService() contracts.NetworkService {
	return n.networkService
}

// NetConfig returns the network configuration.
func (n *MobazhaNode) NetConfig() *config.NetConfig {
	return n.netConfig
}

// NotifierSink returns the node's channel notification sink (may be nil).
func (n *MobazhaNode) NotifierSink() *notifier.ChannelNotificationSink {
	return n.notifierSink
}

// SaveNotificationChannels persists channel configs to the database.
func (n *MobazhaNode) SaveNotificationChannels(channels []notifier.ChannelConfig) error {
	data, err := json.Marshal(channels)
	if err != nil {
		return fmt.Errorf("marshal notification channels: %w", err)
	}
	return n.saveSetting(models.SettingsKeyNotificationChannels, string(data))
}

// loadNotificationChannels reads the persisted channel configs from the database.
func (n *MobazhaNode) loadNotificationChannels() []notifier.ChannelConfig {
	val, err := n.getSetting(models.SettingsKeyNotificationChannels)
	if err != nil || val == "" {
		return nil
	}
	var channels []notifier.ChannelConfig
	if err := json.Unmarshal([]byte(val), &channels); err != nil {
		return nil
	}
	return channels
}

// AIProxy returns the node's AI proxy (may be nil).
func (n *MobazhaNode) AIProxy() *aipkg.Proxy {
	return n.aiProxy
}

// AIConfig returns the flat Config for the currently active provider.
// Used by Generate/TestConnection handlers and proxy layer.
func (n *MobazhaNode) AIConfig() aipkg.Config {
	mc := n.AIMultiConfig()
	return mc.ActiveConfig()
}

// AIMultiConfig reads the full multi-provider config from the database.
// Automatically handles migration from legacy single-provider format
// via MultiConfig.UnmarshalJSON.
func (n *MobazhaNode) AIMultiConfig() aipkg.MultiConfig {
	val, err := n.getSetting(models.SettingsKeyAIConfig)
	if err != nil || val == "" {
		return aipkg.MultiConfig{}
	}
	var mc aipkg.MultiConfig
	if err := json.Unmarshal([]byte(val), &mc); err != nil {
		return aipkg.MultiConfig{}
	}
	return mc
}

// SaveAIMultiConfig persists the multi-provider AI config to the database.
func (n *MobazhaNode) SaveAIMultiConfig(mc aipkg.MultiConfig) error {
	data, err := json.Marshal(mc)
	if err != nil {
		return fmt.Errorf("marshal AI multi config: %w", err)
	}
	return n.saveSetting(models.SettingsKeyAIConfig, string(data))
}

// StoreConfig reads the storefront branding config from the database.
func (n *MobazhaNode) StoreConfig() (json.RawMessage, error) {
	val, err := n.getSetting(models.SettingsKeyStoreConfig)
	if err != nil || val == "" {
		return nil, nil
	}
	return json.RawMessage(val), nil
}

// SaveStoreConfig persists the storefront branding config.
func (n *MobazhaNode) SaveStoreConfig(cfg json.RawMessage) error {
	if err := n.saveSetting(models.SettingsKeyStoreConfig, string(cfg)); err != nil {
		return err
	}
	if n.netDB != nil {
		go func() {
			if err := n.netDB.SetOwnStoreMetadata("storefront", cfg); err != nil {
				log.Debugf("pushStorefrontToNetDB: %v", err)
			}
		}()
	}
	return nil
}

// getSetting reads a single key from the node_settings table.
func (n *MobazhaNode) getSetting(key string) (string, error) {
	var setting models.NodeSettings
	err := n.db.View(func(tx database.Tx) error {
		return tx.Read().Where("\"key\" = ?", key).First(&setting).Error
	})
	if err != nil {
		return "", err
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

// ChatStore returns a chat store backed by this node's database.
func (n *MobazhaNode) ChatStore() *aipkg.ChatStore {
	return aipkg.NewChatStore(n.db)
}

// ProfileName returns the display name of this node's store profile.
func (n *MobazhaNode) ProfileName() string {
	ps := n.Profile()
	if ps == nil {
		return ""
	}
	profile, err := ps.GetMyProfile()
	if err != nil || profile == nil {
		return ""
	}
	return profile.Name
}
