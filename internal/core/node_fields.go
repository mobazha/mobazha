package core

import (
	"context"
	"sync"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/gagliardetto/solana-go"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	peer "github.com/libp2p/go-libp2p/core/peer"
	aipkg "github.com/mobazha/mobazha/internal/ai"
	tronchain "github.com/mobazha/mobazha/internal/chains/tron"
	"github.com/mobazha/mobazha/internal/config"
	"github.com/mobazha/mobazha/internal/net"
	"github.com/mobazha/mobazha/internal/notifier"
	"github.com/mobazha/mobazha/internal/orders"
	"github.com/mobazha/mobazha/internal/repo"
	"github.com/mobazha/mobazha/internal/wallet"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/core/coreiface"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/database/netdb"
	"github.com/mobazha/mobazha/pkg/distribution"
	"github.com/mobazha/mobazha/pkg/encryption"
	"github.com/mobazha/mobazha/pkg/events"
	"github.com/mobazha/mobazha/pkg/evm"
	"github.com/mobazha/mobazha/pkg/payment"
	"github.com/mobazha/mobazha/pkg/relay"
	"github.com/mobazha/mobazha/pkg/utxo"
	wh "github.com/mobazha/mobazha/pkg/webhook"
)

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
	p2pInfra     *P2PInfra
	contentStore contracts.ContentStore
	db           database.Database
	repo         *repo.Repo
}

// cryptoFields groups cryptographic key material and signing.
type cryptoFields struct {
	signer          contracts.Signer
	ethMasterKey    *btcec.PrivateKey
	escrowMasterKey *btcec.PrivateKey
	solPrivKey      *solana.PrivateKey
	ratingMasterKey *btcec.PrivateKey
	tronMasterKey   *btcec.PrivateKey
	keyProvider     contracts.KeyProvider
	credentialKeys  contracts.ProviderCredentialKeyProvider
	bip44Key        *hdkeychain.ExtendedKey
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
//
// paymentCapabilities is supplied by the distribution composition root.
// Optional rails fail closed when the provider is nil.
type walletFields struct {
	multiwallet                   contracts.WalletOperator
	orderProcessor                *orders.OrderProcessor
	exchangeRates                 *wallet.ExchangeRateProvider
	paymentRegistry               *payment.Registry
	relayAPIURL                   string
	relayAPIBearer                string
	evmRelay                      relay.EVMRelayService
	managedEscrowReceiptValidator payment.ManagedEscrowReceiptValidator
	paymentCapabilities           payment.ChainCapabilityProvider
	paymentModules                []distribution.PaymentModule
	paymentModuleManager          *distribution.TrustedPaymentModuleManager
}

// chainFields groups blockchain client configuration.
type chainFields struct {
	evmChainConfigs []evm.EVMClientConfig
	tronChainConfig *TronChainConfig
	tronClient      *tronchain.TronClient
	monitorService  utxo.UTXOMonitorService

	// Per-chain Electrum server configs (UTXO RPC endpoints). Parsed from
	// Config.ElectrumServers / ElectrumTLSFingerprints. Same architectural
	// layer as evmChainConfigs / solanaChainConfig — chain-specific RPC
	// endpoint configuration consumed by the lifecycle wiring.
	// Key: chain code (e.g. "ltc", "btc"). Value: host:port or fingerprint hex.
	electrumEndpoints    map[string]string
	electrumFingerprints map[string]string

	// Local-first compositions inject a narrow payment runtime and product
	// policy. Chain protocol and wallet administration stay in the private
	// distribution module.
	externalPaymentMu sync.Mutex
	externalPayments  *distribution.ExternalPaymentRuntimeCatalog
	sovereignPolicy   distribution.SovereignNodePolicy
}

// ipnsFields groups NetDB configuration (IPNS resolution retired).
type ipnsFields struct {
	netDB     *netdb.NetDB
	netConfig *config.NetConfig
}

// platformFields groups platform-enhanced capabilities: SaaS hosting,
// webhook/notification, AI services, encryption, and fiat integration.
type platformFields struct {
	webhookStore  wh.EndpointStore
	webhookEngine *wh.Engine

	eventDispatcher *events.Dispatcher
	notifierSink    *notifier.ChannelNotificationSink

	coTenantPublicData      contracts.CoTenantPublicDataFn
	coTenantDigitalAssets   contracts.CoTenantDigitalAssetsFn
	coTenantVerifiedPayment contracts.CoTenantVerifiedPaymentFn

	aiProxy             *aipkg.Proxy
	platformAIProfileMu sync.RWMutex
	platformAIProfile   aipkg.PlatformProfile
	aiRateLimiter       *aipkg.DailyRateLimiter

	stripeAccountID string

	keyManager         *encryption.KeyManager
	localListingCrypto *encryption.LocalListingCrypto

	hostService coreiface.HostService
}
