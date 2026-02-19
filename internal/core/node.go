package core

import (
	"context"
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
	"github.com/mobazha/mobazha3.0/internal/channels"
	"github.com/mobazha/mobazha3.0/internal/config"
	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/internal/multiwallet/utxo"
	"github.com/mobazha/mobazha3.0/internal/net"
	"github.com/mobazha/mobazha3.0/internal/notifications"
	"github.com/mobazha/mobazha3.0/internal/orders"
	"github.com/mobazha/mobazha3.0/internal/repo"
	"github.com/mobazha/mobazha3.0/internal/wallet"
	pkgconfig "github.com/mobazha/mobazha3.0/pkg/config"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/database/netdb"
	"github.com/mobazha/mobazha3.0/pkg/encryption"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/evm"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// MobazhaNode holds all the components that make up a network node
// on the Mobazha network. It also exposes an exported API which can
// be used to control the node.
type MobazhaNode struct {
	nodeID string

	sharedManager *SharedManager

	// Identity fields — always available, independent of ipfsNode.
	// For full nodes these are populated from ipfsNode; for lightweight
	// nodes they come from a minimal libp2p Host.
	peerID     peer.ID
	privKey    crypto.PrivKey
	peerHost   host.Host
	nodeCtx    context.Context
	nodeCancel context.CancelFunc

	// ipfsNode is the IPFS instance that powers this node.
	// May be nil for lightweight (non-default) nodes.
	ipfsNode *core.IpfsNode

	// signer is the contracts.Signer for signing order messages and other data.
	signer corecontracts.Signer

	// contentStore abstracts content-addressed storage (IPFS).
	// Standalone: backed by a local/shared IPFS node.
	// SaaS: backed by a shared IPFS gateway or HTTP API.
	contentStore contracts.ContentStore

	// db is the injectable database interface. Both standalone (FFSqliteDB) and
	// SaaS (TenantDB) modes provide an implementation. Business methods should
	// use n.db instead of n.repo.DB() so that the dependency is explicit and
	// injectable.
	db database.Database

	// repo holds the database and public data directory.
	repo *repo.Repo

	// ethMasterKey represents an secp256k1 private key, the
	// public key of which is advertised by the node in its profile
	// and in listings to be used when building eth escrow transactions.
	ethMasterKey *btcec.PrivateKey

	// escrowMasterKey represents an secp256k1 private key, the
	// public key of which is advertised by the node in its profile
	// and in listings to be used when building escrow transactions.
	escrowMasterKey *btcec.PrivateKey

	// solPrivKey represents an ed25519 private key, the
	// public key of which is advertised by the node in its profile
	// and in listings to be used when building solana escrow transactions.
	solPrivKey *solana.PrivateKey

	// ratingMasterKey represents an secp256k1 private key that
	// we used to generate rating keys to sign ratings with.
	ratingMasterKey *btcec.PrivateKey

	// keyProvider abstracts access to cryptographic master keys.
	// Standalone: fileKeyProvider (reads from the fields above).
	// SaaS: injected keyVaultProvider (reads from centralized KeyVault).
	keyProvider contracts.KeyProvider

	// paymentService is the extracted App Service for payment operations.
	// Owns escrow instruction generation, cancelable payment dispatching, etc.
	paymentService *PaymentAppService

	// orderService encapsulates order lifecycle business logic (reject, refund, cancel).
	orderService *OrderAppService

	// chatService encapsulates chat and chat-group business logic.
	chatService *ChatAppService

	// stripeAccountID represents the stripe account id of the node.
	stripeAccountID string

	// ipnsQuorum is the size of the IPNS quorum to use. Smaller quorums
	// resolve faster but run the risk of getting back older records.
	ipnsQuorum uint

	// ipnsResolver is the URL of a resolver that can be queried to resolve
	// IPNS records. If this is empty we will use the p2p network.
	ipnsResolver string

	// netDB is the endpoint that can be queried for profile
	// and listing data if it is not found in the p2p network.
	netDB *netdb.NetDB

	netConfig *config.NetConfig

	// messenger is the primary object used to send messages to other peers.
	// It ensures reliable delivery by persisting messages and retrying them.
	// Generally you should always send messages using this and not the
	// NetworkService as the later will only attempt to send direct messages
	// and will not retry.
	messenger contracts.Messenger

	// networkService manages the sending and receiving of messages
	// on the Mobazha protocol.
	networkService contracts.NetworkService

	// banManager holds a list of peers that have been banned by this node.
	banManager *net.BanManager

	// eventBus allows a subscriber to receive event notifications from the node.
	eventBus events.Bus

	// followerTracker tries to maintain connections to a minimum number of our
	// followers so that we can use them to push data for redundancy.
	followerTracker *FollowerTracker

	// multiwallet abstracts multi-currency wallet operations.
	// Standalone: backed by a real Multiwallet.
	// SaaS: backed by KeyVault + shared chain services.
	multiwallet contracts.WalletOperator

	// orderProcessor is the engine we use for processing all orders.
	orderProcessor *orders.OrderProcessor

	// exchangeRates is a provider of exchange rate data for various currencies.
	exchangeRates *wallet.ExchangeRateProvider

	// notifier listens to events coming off the bus, marshals them to notifications
	// and sends them off to the websocket.
	notifier *notifications.Notifier

	// testnet is whether the this node is configured to use the test network (IPFS bootstrap).
	testnet bool

	// walletTestnet is whether the this node is configured to use testnet for wallet transactions.
	walletTestnet bool

	// torOnly is whether the node is running in tor only mode.
	torOnly bool

	// publishActive is an atomic integer that represents the number of inflight
	// publishes.
	publishActive int32

	// publishChan is used to signal to the republish loop that a publish
	// has just completed and it should update it's last published time.
	publishChan chan pubCloser

	// ipfsOnlyMode signals that the node is running in IPFS only mode.
	ipfsOnlyMode bool

	// storeAndForwardServers is a list of string peerIDs of servers we use
	// as our store and forward nodes.
	storeAndForwardServers []string

	// boostrapPeers holds the peers we use to bootstrap the node.
	boostrapPeers []peer.ID

	// channels holds active chat channels
	channels map[string]*channels.Channel

	featureManager *pkgconfig.FeatureManager

	// shutdownTorFunc is used to shutdown the embedded Tor client.
	shutdownTorFunc func() error

	// initialBootstrapChan is closed after the initial IPFS bootstrap completes.
	initialBootstrapChan chan struct{}

	// shutdown is closed when the node is stopped. Any listening
	// goroutines can use this to terminate.
	shutdown chan struct{}

	// monitorService provides unified UTXO monitoring operations
	monitorService utxo.UTXOMonitorService

	// Stripe 配置缓存
	stripeConfigCache *netdb.StripeConfigCache

	hostService coreiface.HostService

	// Phase 2 加密相关服务
	// keyManager 管理商品和产品组的加密密钥（HKDF 派生）
	keyManager *encryption.KeyManager

	// localListingCrypto 提供本地商品加密的核心服务（含加解密功能）
	localListingCrypto *encryption.LocalListingCrypto

	// relayAPIURL is the platform relay API URL for gas fee payment (EVM CANCELABLE payments)
	// Note: Solana support requires additional relay service implementation
	relayAPIURL string

	// evmChainConfigs holds per-chain EVM client configs derived from the node's
	// multiwallet ChainAPIs config. Used by startEVMChainClients() in standalone mode
	// to create chain clients that respect user-configured RPC URLs (instead of
	// compiled-in factory defaults). In SaaS mode this is nil (clients come from HostService).
	evmChainConfigs []evm.EVMClientConfig

	// solanaChainConfig holds Solana chain config derived from multiwallet ChainAPIs.
	// Used by startSolanaChainClients() in standalone mode to create the SolanaClient
	// and resolve the escrow program ID. In SaaS mode this is nil (clients come from HostService).
	solanaChainConfig *SolanaChainConfig

	// paymentRegistry maps ChainType to PaymentStrategy for dispatching
	// chain-specific payment operations. Initialized in registerPaymentStrategies().
	paymentRegistry *payment.Registry
}

// IsDefaultNode returns whether this node is the default node.
func (n *MobazhaNode) IsDefaultNode() bool {
	return n.nodeID == repo.DefaultNodeID
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
		if err := n.CheckAndMigrateShippingProfiles(); err != nil {
			logger.LogErrorWithIDf(log, n.nodeID, "CheckAndMigrateShippingProfiles failed, %v", err)
		}
		if err := n.SyncShippingProfilesToListings(); err != nil {
			logger.LogErrorWithIDf(log, n.nodeID, "SyncShippingProfilesToListings failed, %v", err)
		}
	}()

	go n.bootstrapIPFS()
	if !n.ipfsOnlyMode {
		n.publishHandler()
		go n.messenger.Start()
		go n.followerTracker.Start()

		go n.orderProcessor.Start()
		go n.syncMessages()
		go func() {
			n.multiwallet.Start()

			if n.IsDefaultNode() {
				go n.SharedManager().Start()
			}
		}()

		go n.notifier.Start()
		go n.OpenSavedChannels()

		if err := n.updateSNFServers(); err != nil {
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
		n.startCancelablePaymentMonitor()

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

	if version == 0 {
		logger.LogInfoWithIDf(log, n.nodeID, "Migrate repo from version 0")
		err = n.migrateRepoFromVersion0()
		if err != nil {
			logger.LogErrorWithIDf(log, n.nodeID, "Migration error: %v", err)
		}

		logger.LogInfoWithIDf(log, n.nodeID, "Migrate repo from version 1")
		err = n.migrateRepoFromVersion1()
		if err != nil {
			logger.LogErrorWithIDf(log, n.nodeID, "Migration error: %v", err)
		}
	} else if version == 1 {
		logger.LogInfoWithIDf(log, n.nodeID, "Migrate repo from version 1")
		err = n.migrateRepoFromVersion1()
		if err != nil {
			logger.LogErrorWithIDf(log, n.nodeID, "Migration error: %v", err)
		}
	} else if version == 2 || version == 3 {
		logger.LogInfoWithIDf(log, n.nodeID, "Migrate repo from version 2")
		err = n.migrateRepoWithListingsUpdate()
		if err != nil {
			logger.LogErrorWithIDf(log, n.nodeID, "Migration error: %v", err)
		}
	} else if version == 5 {
		logger.LogInfoWithIDf(log, n.nodeID, "Migrate repo from version 5")
		err = n.migrateRepoFromVersion5()
		if err != nil {
			logger.LogErrorWithIDf(log, n.nodeID, "Migration error: %v", err)
		}
	}

	if version != repo.DefaultRepoVersion {
		if err := n.repo.WriteVersion(repo.DefaultRepoVersion); err != nil {
			return err
		}
	}
	return nil
}

// Do profile and listings migration with ETH pubKey adding
func (n *MobazhaNode) migrateRepoFromVersion0() error {
	done1 := make(chan struct{})
	myProfile, err := n.GetMyProfile()
	if err != nil {
		return fmt.Errorf("get my profile failed, %v", err)
	}
	err = n.SetProfile(myProfile, done1)
	if err != nil {
		return fmt.Errorf("update profile failed, %v", err)
	}

	select {
	case <-done1:
		break
	case <-time.After(time.Second * 300):
		return errors.New("timeout waiting on profile update")
	}

	done2 := make(chan struct{})
	err = n.UpdateAllListings(func(listing *pb.Listing) (bool, error) {
		listing.VendorID.Pubkeys.Eth = n.ethMasterKey.PubKey().SerializeCompressed()
		return true, nil
	}, done2)
	if err != nil {
		return fmt.Errorf("update listings failed, %v", err)
	}
	select {
	case <-done2:
		return nil
	case <-time.After(time.Second * 300):
		return errors.New("timeout waiting on listing update")
	}
}

// Do profile and listings migration with MATIC currencies update
func (n *MobazhaNode) migrateRepoFromVersion1() error {
	done := make(chan struct{})
	myProfile, err := n.GetMyProfile()
	if err != nil {
		return fmt.Errorf("get my profile failed, %v", err)
	}
	myProfile.Currencies = []string{"MATIC", "MATICUSDT", "MATICUSDC"}
	err = n.SetProfile(myProfile, done)
	if err != nil {
		return fmt.Errorf("update profile failed, %v", err)
	}

	select {
	case <-done:
		return nil
	case <-time.After(time.Second * 300):
		return errors.New("timeout waiting on profile update")
	}
}

// Do listings migration about signature due to new fields added
func (n *MobazhaNode) migrateRepoWithListingsUpdate() error {
	done := make(chan struct{})
	err := n.UpdateAllListings(func(listing *pb.Listing) (bool, error) {
		// do nothing, just update the signature using existing flow
		return true, nil
	}, done)
	if err != nil {
		return fmt.Errorf("update listings failed, %v", err)
	}

	select {
	case <-done:
		return nil
	case <-time.After(time.Second * 300):
		return errors.New("timeout waiting on listing update")
	}
}

// Do listings migration about signature due to new fields added
func (n *MobazhaNode) migrateRepoFromVersion5() error {
	// Add Solana pubkey to profile
	done1 := make(chan struct{})
	myProfile, err := n.GetMyProfile()
	if err != nil {
		return fmt.Errorf("get my profile failed, %v", err)
	}
	err = n.SetProfile(myProfile, done1)
	if err != nil {
		return fmt.Errorf("update profile failed, %v", err)
	}

	select {
	case <-done1:
		break
	case <-time.After(time.Second * 300):
		return errors.New("timeout waiting on profile update")
	}

	// Add Solana pubkey to listings
	done2 := make(chan struct{})
	err = n.UpdateAllListings(func(listing *pb.Listing) (bool, error) {
		listing.VendorID.Pubkeys.Solana = n.solPrivKey.PublicKey().Bytes()
		return true, nil
	}, done2)
	if err != nil {
		return fmt.Errorf("update listings failed, %v", err)
	}
	select {
	case <-done2:
		return nil
	case <-time.After(time.Second * 300):
		return errors.New("timeout waiting on listing update")
	}
}

// Stop cleanly shutsdown the MobazhaNode and signals to any
// listening goroutines that it's time to stop.
func (n *MobazhaNode) Stop(force bool) error {
	if atomic.LoadInt32(&n.publishActive) > 0 && !force {
		return coreiface.ErrPublishingActive
	}

	if !n.ipfsOnlyMode {
		n.messenger.Stop()
		n.networkService.Close()
		n.orderProcessor.Stop()
		n.followerTracker.Close()
		n.multiwallet.Close()
		if n.IsDefaultNode() {
			n.SharedManager().Stop()
		}
		if n.notifier != nil {
			n.notifier.Stop()
		}
		for _, channel := range n.channels {
			channel.Close()
		}
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

// GetAllRates implements contracts.ExchangeRateService.
// Delegates to the internal ExchangeRateProvider.
func (n *MobazhaNode) GetAllRates(base models.CurrencyCode, breakCache bool) (map[models.CurrencyCode]iwallet.Amount, error) {
	if n.exchangeRates == nil {
		return nil, fmt.Errorf("exchange rate provider not available")
	}
	return n.exchangeRates.GetAllRates(base, breakCache)
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
