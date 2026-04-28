package core

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"path"
	"strings"
	"time"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	solana "github.com/gagliardetto/solana-go"
	"github.com/libp2p/go-libp2p"
	inet "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	corecontracts "github.com/mobazha/mobazha-core/contracts"
	coreorders "github.com/mobazha/mobazha-core/orders"
	aipkg "github.com/mobazha/mobazha3.0/internal/ai"
	"github.com/mobazha/mobazha3.0/internal/chains"
	solanaWal "github.com/mobazha/mobazha3.0/internal/chains/solana"
	"github.com/mobazha/mobazha3.0/internal/contracts"
	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/logger"
	obnet "github.com/mobazha/mobazha3.0/internal/net"
	"github.com/mobazha/mobazha3.0/internal/notifications"
	"github.com/mobazha/mobazha3.0/internal/notifier"
	"github.com/mobazha/mobazha3.0/internal/orders"
	"github.com/mobazha/mobazha3.0/internal/orders/utils"
	fiat "github.com/mobazha/mobazha3.0/internal/payment/fiat"
	"github.com/mobazha/mobazha3.0/internal/repo"
	nodeVersion "github.com/mobazha/mobazha3.0/internal/version"
	webhookinternal "github.com/mobazha/mobazha3.0/internal/webhook"
	oniontransport "github.com/mobazha/mobazha3.0/libs/onion-transport"
	"github.com/mobazha/mobazha3.0/libs/proxyclient"
	storeandforward "github.com/mobazha/mobazha3.0/libs/store-and-forward"
	pkgconfig "github.com/mobazha/mobazha3.0/pkg/config"
	pkgcontracts "github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/fulfillment"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/database/netdb"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/logging"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	"github.com/mobazha/mobazha3.0/pkg/request"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	wh "github.com/mobazha/mobazha3.0/pkg/webhook"
	madns "github.com/multiformats/go-multiaddr-dns"
	"github.com/tyler-smith/go-bip39"
	"golang.org/x/net/proxy"
	"gorm.io/gorm"
)

const (
	// maxRecordAge is the maximum amount of time to keep a record in the DHT before deleting it.
	maxRecordAge = time.Hour * 24 * 7

	// netConfigEndpoint is the endpoint to get the node configuration.
	netConfigEndpoint = "https://app.mobazha.org/search/v1/config"
)

var (
	log         = logging.MustGetLogger("CORE")
	ProtocolDHT protocol.ID
)

// NewNode constructs and returns an MobazhaNode using the given cfg.
func NewNode(ctx context.Context, cfg *repo.Config, nodeID string, hostService ...coreiface.HostService) (*MobazhaNode, error) {
	var hs coreiface.HostService
	if len(hostService) > 0 {
		hs = hostService[0]
	}

	if err := repo.CheckAndMigrateRepo(cfg.DataDir); err != nil {
		return nil, fmt.Errorf("repo migration failed: %v", err)
	}

	sharedManager, err := NewSharedManager(ctx, cfg)
	if err != nil {
		return nil, err
	}

	if nodeID == "" {
		nodeID = repo.DefaultNodeID
	}
	isDefaultNode := nodeID == repo.DefaultNodeID

	repoPath := path.Join(cfg.DataDir, "nodes", nodeID)

	var obRepo *repo.Repo
	if cfg.SharedDB != nil {
		// Multi-tenant shared DB mode: use TenantDB wrapper
		sharedGormDB, ok := cfg.SharedDB.(*gorm.DB)
		if !ok {
			return nil, fmt.Errorf("SharedDB must be a *gorm.DB, got %T", cfg.SharedDB)
		}
		obRepo, err = repo.NewRepoWithSharedDB(nodeID, repoPath, sharedGormDB, cfg.IdentityKey, cfg.Testnet)
	} else if len(cfg.IdentityKey) > 0 {
		obRepo, err = repo.NewRepoWithIdentityKey(nodeID, repoPath, cfg.IdentityKey, cfg.Testnet)
	} else {
		obRepo, err = repo.NewRepo(nodeID, repoPath, cfg.Testnet)
	}
	if err != nil {
		return nil, err
	}

	if err := obRepo.WriteUserAgent(cfg.UserAgentComment); err != nil {
		return nil, err
	}

	netConfig := sharedManager.NetConfig

	var (
		transportOpt    libp2p.Option
		onionID         string
		shutdownTorFunc func() error
	)
	if cfg.Tor || cfg.DualStack {
		logger.LogNoticeWithID(log, nodeID, "Starting embedded Tor client")

		var torKey models.Key
		err = obRepo.DB().View(func(tx database.Tx) error {
			return tx.Read().Where("name = ?", "tor").First(&torKey).Error
		})
		if err != nil {
			return nil, err
		}

		key := ed25519.NewKeyFromSeed(torKey.Value)

		onion, dialer, transport, closeTor, err := obnet.SetupTor(ctx, key, repoPath, cfg.DualStack)
		if err != nil {
			return nil, err
		}
		onionID = onion
		transportOpt = transport
		shutdownTorFunc = closeTor

		if cfg.Tor {
			// Very important to set the proxy on the http client as well as the DNSResover.
			proxyclient.SetProxy(dialer)
			madns.DefaultResolver = oniontransport.NewTorResover(obnet.TorDNSResover)
		}
	}

	// ── External SOCKS5 proxy (--socksproxy / --privacy-mode) ────────
	// When an external proxy is configured and embedded Tor is NOT active,
	// route all outbound HTTP through the SOCKS5 dialer.
	if cfg.SocksProxy != "" && !cfg.Tor {
		socksDialer, dialErr := proxy.SOCKS5("tcp", cfg.SocksProxy, nil, proxy.Direct)
		if dialErr != nil {
			return nil, fmt.Errorf("SOCKS5 proxy %s: %w", cfg.SocksProxy, dialErr)
		}
		proxyclient.SetProxy(socksDialer)
		logger.LogInfoWithIDf(log, nodeID, "Outbound HTTP routed through SOCKS5 proxy %s", cfg.SocksProxy)
	}

	// ── SaaS / lightweight node path ──────────────────────────────────
	// When SaaSMode is enabled, skip full P2P infrastructure creation.
	// Instead create a minimal libp2p Host (identity only, no listen addrs)
	// and share the default node's infrastructure for content ops.
	if cfg.SaaSMode {
		return newLightweightNode(ctx, cfg, nodeID, obRepo, sharedManager, shutdownTorFunc, hs)
	}

	// ── Full node path ─────────────────────────────────────────────────
	// Load identity key: use external key from config if provided, otherwise load from DB.
	var identityKeyBytes []byte
	if len(cfg.IdentityKey) > 0 {
		identityKeyBytes = cfg.IdentityKey
	} else {
		var dbIdentityKey models.Key
		err = obRepo.DB().View(func(tx database.Tx) error {
			return tx.Read().Where("name = ?", "identity").First(&dbIdentityKey).Error
		})
		if err != nil {
			return nil, fmt.Errorf("failed to load identity key from DB: %w", err)
		}
		identityKeyBytes = dbIdentityKey.Value
	}

	privKey, _, err := repo.PrivKeyAndPeerIDFromKey(identityKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse identity key: %w", err)
	}

	// Set DHT protocol prefix for Mobazha network segregation.
	if !cfg.Testnet {
		ProtocolDHT = obnet.ProtocolPrefixMainnet
	} else {
		ProtocolDHT = obnet.ProtocolPrefixTestnet
	}

	// Merge bootstrap addresses from all sources (net config, CLI, defaults).
	bootstraps := append(netConfig.BootstrapAddrs, cfg.BoostrapAddrs...)
	if cfg.Testnet {
		bootstraps = append(bootstraps, repo.DefaultTestnetBootstrapAddrs...)
	} else {
		bootstraps = append(bootstraps, repo.DefaultMainnetBootstrapAddrs...)
	}

	// Resolve swarm listen addresses.
	swarmAddrs := cfg.SwarmAddrs
	if len(swarmAddrs) == 0 {
		swarmAddrs = []string{"/ip4/0.0.0.0/tcp/4001", "/ip6/::/tcp/4001"}
	}
	if cfg.Tor {
		swarmAddrs = []string{fmt.Sprintf("/onion3/%s:9003", onionID)}
	} else if cfg.DualStack {
		swarmAddrs = append(swarmAddrs, fmt.Sprintf("/onion3/%s:9003", onionID))
	}

	// Create the P2P infrastructure (libp2p Host + DHT + datastores).
	infra, err := createP2PInfra(ctx, &P2PConfig{
		PrivKey:           privKey,
		SwarmAddrs:        swarmAddrs,
		AnnounceAddrs:     cfg.AnnounceAddrs,
		BootstrapAddrs:    bootstraps,
		StaticRelayPeers:  cfg.StaticRelayPeers,
		DataDir:           repoPath,
		Testnet:           cfg.Testnet,
		DHTClientOnly:     cfg.DHTClientOnly,
		IsDefaultNode:     isDefaultNode,
		DisableNATPortMap: cfg.DisableNATPortMap,
		DisableReuseport:  cfg.DisableReuseport,
		EnableSNFServer:   cfg.EnableSNFServer,
		EnableRelayServer: cfg.EnableRelayServer,
		Tor:               cfg.Tor,
		DualStack:         cfg.DualStack,
		TransportOpt:      transportOpt,
		NATConnectivity:   cfg.StandaloneConnectivity,
	})
	if err != nil {
		return nil, err
	}
	infraOwned := false
	defer func() {
		if !infraOwned {
			infra.Close()
		}
	}()

	snfServerProtocol := obnet.ProtocolStoreAndForwardMainnet_Server
	snfClientProtocol := obnet.ProtocolStoreAndForwardMainnet_Client
	if cfg.Testnet {
		snfServerProtocol = obnet.ProtocolStoreAndForwardTestnet_Server
		snfClientProtocol = obnet.ProtocolStoreAndForwardTestnet_Client
	}

	if cfg.EnableSNFServer {
		snfReplicationPeers := make([]peer.ID, 0, len(cfg.SNFServerPeers))
		for _, serverStr := range cfg.SNFServerPeers {
			server, err := peer.Decode(serverStr)
			if err != nil {
				return nil, err
			}
			snfReplicationPeers = append(snfReplicationPeers, server)
		}
		snfDS := infra.SNFStore
		if snfDS == nil {
			snfDS = infra.DHTStore
		}
		serverOpts := []storeandforward.Option{
			storeandforward.ServerProtocols(protocol.ID(snfServerProtocol)),
			storeandforward.ClientProtocols(protocol.ID(snfClientProtocol)),
			storeandforward.ReplicationPeers(snfReplicationPeers...),
			storeandforward.Datastore(snfDS),
		}
		_, err := storeandforward.NewServer(infra.Ctx, infra.Host, serverOpts...)
		if err != nil {
			return nil, err
		}
	}

	var netDB *netdb.NetDB
	standaloneNetDBEndpoint := netConfig.GetNetDBEndpoint()
	if cfg.NetDBEndpoint != "" {
		standaloneNetDBEndpoint = cfg.NetDBEndpoint
	}
	if len(standaloneNetDBEndpoint) > 0 {
		netDB, _ = netdb.NewNetDB(standaloneNetDBEndpoint, infra.PeerID.String(), infra.PrivKey)
	}

	// 使用 WalletTestnet（如果设置），否则回退到 Testnet
	walletTestnet := cfg.Testnet
	if cfg.WalletTestnet {
		walletTestnet = cfg.WalletTestnet
	}

	if cfg.InfrastructureOnly {
		infraOnlyCtx, infraOnlyCancel := context.WithCancel(infra.Ctx)
		obNode := &MobazhaNode{
			sharedManager: sharedManager,
			identityFields: identityFields{
				nodeID:     nodeID,
				peerID:     infra.PeerID,
				privKey:    infra.PrivKey,
				peerHost:   infra.Host,
				nodeCtx:    infraOnlyCtx,
				nodeCancel: infraOnlyCancel,
			},
			storageFields: storageFields{
				p2pInfra: infra,
				db:       obRepo.DB(),
				repo:     obRepo,
			},
			networkFields: networkFields{
				eventBus: events.NewBus(),
			},
			walletFields: walletFields{
				exchangeRates: sharedManager.ExchangeRateProvider,
			},
			ipnsFields: ipnsFields{
				netDB: netDB,
			},
			modeFlags: modeFlags{
				infrastructureOnly: true,
				testnet:            cfg.Testnet,
				walletTestnet:      walletTestnet,
				torOnly:            cfg.Tor,
			},
			lifecycleFields: lifecycleFields{
				shutdownTorFunc:      shutdownTorFunc,
				featureManager:       pkgconfig.GetGlobalFeatureManager(),
				initialBootstrapChan: make(chan struct{}),
				shutdown:             make(chan struct{}),
			},
			hostService: hs,
		}
		sharedManager.AddNode(nodeID, obNode)

		if isDefaultNode {
			if _, err := sharedManager.initHTTPGateway(cfg); err != nil {
				log.Warningf("Failed to init HTTP gateway for infrastructure-only default node: %v", err)
			}

			// Initialize SNF Proxy so lightweight tenant nodes can relay
			// messages through the default node's P2P host (which has
			// SNF server addresses from bootstrap).
			if err := sharedManager.InitSNFProxy(obNode.peerHost); err != nil {
				log.Warningf("Failed to init SNF Proxy for infrastructure-only default node: %v", err)
			}
		}

		initWebhookSubsystem(obNode)
		initDiscountSubsystem(obNode)
		initCollectionSubsystem(obNode)
		initFiatSubsystem(obNode)
		initSupplyChainSubsystem(obNode)
		initShippingSubsystem(obNode)
		obNode.applyOptions([]NodeOption{
			WithNodeFeatureProvider(NewConfigNodeFeatureProvider(cfg)),
		})
		infraOwned = true
		return obNode, nil
	}

	// Load the keys from the db
	var (
		dbBip44Key   models.Key
		dbEscrowKey  models.Key
		dbRatingKey  models.Key
		dbSolKey     models.Key
		prefs        models.UserPreferences
		needDBUpdate bool
	)

	err = obRepo.DB().View(func(tx database.Tx) error {
		if err := tx.Read().First(&prefs).Error; err != nil {
			return fmt.Errorf("failed to get user preferences: %v", err)
		}
		dbEscrowKey, dbBip44Key, dbSolKey, dbRatingKey, err = repo.GetKeysFromDB(tx)
		if err != nil {
			logger.LogInfoWithID(log, nodeID, "Keys missing from DB, update required")
			needDBUpdate = true
			return nil
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if needDBUpdate {
		logger.LogInfoWithID(log, nodeID, "Updating keys in DB")
		err = obRepo.DB().Update(func(tx database.Tx) error {
			var dbMnemonic models.Key
			err = tx.Read().Where("name = ?", "mnemonic").First(&dbMnemonic).Error
			if err != nil {
				return fmt.Errorf("failed to get mnemonic: %v", err)
			}

			// 从助记词生成种子
			hdSeed := bip39.NewSeed(string(dbMnemonic.Value), "")
			escrowKey, ratingKey, bip44Key, solKey, err := repo.CreateHDKeys(hdSeed)
			if err != nil {
				return fmt.Errorf("failed to generate HD keys: %v", err)
			}

			// 保存新生成的密钥
			if err := repo.SaveKeysToDB(tx, escrowKey, bip44Key, solKey, ratingKey); err != nil {
				return fmt.Errorf("failed to save keys: %v", err)
			}

			dbEscrowKey, dbBip44Key, dbSolKey, dbRatingKey, err = repo.GetKeysFromDB(tx)
			if err != nil {
				return fmt.Errorf("failed to get keys: %v", err)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	bip44Key, err := hdkeychain.NewKeyFromString(string(dbBip44Key.Value))
	if err != nil {
		return nil, fmt.Errorf("parse bip44 key: %w", err)
	}
	ethMasterKey, err := utils.GenerateEthPrivateKey(bip44Key)
	if err != nil {
		return nil, fmt.Errorf("derive eth master key: %w", err)
	}

	tronMasterKey, err := utils.GenerateTRONPrivateKey(bip44Key)
	if err != nil {
		return nil, fmt.Errorf("derive tron master key: %w", err)
	}

	escrowKey, _ := btcec.PrivKeyFromBytes(dbEscrowKey.Value)
	ratingKey, _ := btcec.PrivKeyFromBytes(dbRatingKey.Value)

	solPrivKey := solana.PrivateKey(dbSolKey.Value)

	enabledChains := iwallet.GetAllSupportedChainTypes()

	erp := sharedManager.ExchangeRateProvider

	opts := []chains.Option{
		chains.NodeID(nodeID),
		chains.DataDir(repoPath),
		chains.LogDir(cfg.LogDir),
		chains.Chains(enabledChains),
		chains.LogLevel(repo.LogLevelMap[strings.ToLower(cfg.LogLevel)]),
		chains.NetConfig(netConfig),
		chains.Testnet(walletTestnet),
		chains.Regtest(cfg.Regtest),
	}
	mw, err := chains.NewMultiwallet(opts...)
	if err != nil {
		return nil, err
	}

	if err := InitializeMultiwallet(mw, obRepo.DB(), time.Now()); err != nil {
		return nil, err
	}

	// Extract EVM chain configs from multiwallet ChainAPIs for standalone client creation.
	// This is done here (not in Start()) so the node stores user-configured RPC URLs.
	// Must prepend Defaults (same as NewMultiwallet does internally) to populate ChainAPIs.
	var mwCfg chains.Config
	_ = mwCfg.Apply(append([]chains.Option{chains.Defaults}, opts...)...)
	evmConfigs := extractEVMConfigs(mwCfg.ChainAPIs, walletTestnet)
	solanaConfig := extractSolanaConfig(mwCfg.ChainAPIs, walletTestnet)
	tronConfig := extractTronConfig(mwCfg.ChainAPIs, walletTestnet)

	globalBlockedIds := []peer.ID{}
	contracts, err := contracts.NewContracts(opts...)
	if err == nil {
		globalBlockedIds, err = contracts.GetBlockedIds()
		if err != nil {
			logger.LogErrorWithIDf(log, nodeID, "Failed to get global blocked nodes: %v", err)
		}
	} else {
		logger.LogErrorWithIDf(log, nodeID, "Failed to create contracts util: %v", err)
	}

	// Share global blocked IDs with HostService for SaaS tenant nodes to reuse,
	// avoiding per-tenant contract queries.
	if hs != nil && len(globalBlockedIds) > 0 {
		hs.SetGlobalBlockedIDs(globalBlockedIds)
	}

	blocked, err := prefs.BlockedNodes()
	if err != nil {
		return nil, err
	}
	bm := obnet.NewBanManager(globalBlockedIds, blocked)
	service := obnet.NewNetworkService(nodeID, infra.Host, bm, cfg.Testnet)
	if hs != nil {
		if ld, ok := any(hs).(obnet.LocalDeliverer); ok {
			service.SetLocalDeliverer(ld)
		}
	}

	bus := events.NewBus()
	tracker := NewFollowerTracker(obRepo, bus, infra.Host)

	for _, server := range cfg.StoreAndForwardServers {
		_, err := peer.Decode(server)
		if err != nil {
			return nil, errors.New("invalid store and forward peer ID in config")
		}
	}

	// Construct our Mobazha node.repo object
	saasCtx, saasCancel := context.WithCancel(infra.Ctx)
	obNode := &MobazhaNode{
		sharedManager: sharedManager,
		identityFields: identityFields{
			nodeID:     nodeID,
			peerID:     infra.PeerID,
			privKey:    infra.PrivKey,
			peerHost:   infra.Host,
			nodeCtx:    saasCtx,
			nodeCancel: saasCancel,
		},
		storageFields: storageFields{
			p2pInfra: infra,
			db:       obRepo.DB(),
			repo:     obRepo,
		},
		cryptoFields: cryptoFields{
			ethMasterKey:    ethMasterKey,
			escrowMasterKey: escrowKey,
			ratingMasterKey: ratingKey,
			solPrivKey:      &solPrivKey,
			tronMasterKey:   tronMasterKey,
			bip44Key:        bip44Key,
		},
		networkFields: networkFields{
			networkService:         service,
			banManager:             bm,
			eventBus:               bus,
			followerTracker:        tracker,
			storeAndForwardServers: cfg.StoreAndForwardServers,
		},
		walletFields: walletFields{
			multiwallet:   &mw,
			exchangeRates: erp,
			relayAPIURL:   cfg.RelayAPIURL,
		},
		chainFields: chainFields{
			evmChainConfigs:   evmConfigs,
			solanaChainConfig: solanaConfig,
			tronChainConfig:   tronConfig,
		},
		ipnsFields: ipnsFields{
			netDB:     netDB,
			netConfig: netConfig,
		},
		modeFlags: modeFlags{
			testnet:       cfg.Testnet,
			walletTestnet: walletTestnet,
			torOnly:       cfg.Tor,
		},
		lifecycleFields: lifecycleFields{
			shutdownTorFunc:      shutdownTorFunc,
			publishChan:          make(chan pubCloser),
			featureManager:       pkgconfig.GetGlobalFeatureManager(),
			initialBootstrapChan: make(chan struct{}),
			shutdown:             make(chan struct{}),
		},
		hostService: hs,
	}
	obNode.contentStore = &cidContentStore{}
	infraOwned = true

	sharedManager.AddNode(nodeID, obNode)

	// If this is the default node, we need to create the HTTP gateway and initialize SNF Proxy
	if isDefaultNode {
		_, err = sharedManager.initHTTPGateway(cfg)
		if err != nil {
			return nil, err
		}

		// Initialize SNF Proxy using the default node's host as transport
		if err := sharedManager.InitSNFProxy(obNode.peerHost); err != nil {
			logger.LogErrorWithIDf(log, nodeID, "Failed to initialize SNF Proxy: %v", err)
			// Continue without proxy - will use direct connections
		}
	} else {
		sharedManager.GetHTTPGateway().EnsureHubForUser(nodeID)
	}

	if err := MigrateNodeSettings(obNode.db); err != nil {
		logger.LogErrorWithIDf(log, nodeID, "Failed to migrate node_settings: %v", err)
	}
	if err := aipkg.MigrateChatModels(obNode.db); err != nil {
		logger.LogErrorWithIDf(log, nodeID, "Failed to migrate ai_chat_sessions: %v", err)
	}

	initWebhookSubsystem(obNode)
	initDiscountSubsystem(obNode)
	initCollectionSubsystem(obNode)
	initFiatSubsystem(obNode)
	initSupplyChainSubsystem(obNode)
	initShippingSubsystem(obNode)

	notifyWsFn := sharedManager.GetHTTPGateway().NotifyWebsockets(nodeID)
	initEventDispatcher(obNode, notifyWsFn)
	initPlatformAIConfig(obNode, cfg)

	// Create messenger with appropriate SNF client
	messengerCfg := &obnet.MessengerConfig{
		NodeID:  nodeID,
		Service: service,
		Privkey: obNode.privKey,
		Context: obNode.nodeCtx,
		DB:      obRepo.DB(),
		Testnet: cfg.Testnet,
		GetProfileFunc: func(ctx context.Context, peerID peer.ID, reqCtx *request.Context, useCache bool) (*models.Profile, error) {
			return obNode.profileService.GetProfile(ctx, peerID, reqCtx, useCache)
		},
	}

	// Always set SNFServers so the messenger has a proper fallback list
	// (used when the target peer's profile can't be loaded).
	messengerCfg.SNFServers = sharedManager.SNFServers

	// Use the SNF Proxy's LocalClient if available (for both default and tenant nodes).
	// This is important: the proxy's stream handler must not be overridden by
	// a standalone Client's stream handler on the same transport host.
	if sharedManager.HasSNFProxy() {
		proxy := sharedManager.GetSNFProxy()
		localClient, err := proxy.RegisterNode(obNode.peerID, obNode.privKey)
		if err != nil {
			logger.LogErrorWithIDf(log, nodeID, "Failed to register with SNF Proxy: %v, falling back to direct connection", err)
		} else {
			messengerCfg.SNFClient = localClient
			logger.LogInfoWithIDf(log, nodeID, "Using SNF Proxy for store-and-forward messaging")
		}
	}

	obNode.messenger, err = obnet.NewMessenger(messengerCfg)
	if err != nil {
		return nil, err
	}

	// Create a Signer from the node's identity key (external or DB-loaded).
	// This is the standard contracts.Signer implementation from mobazha-core.
	signer, err := corecontracts.NewKeyPairSignerFromMarshaledKey(identityKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to create signer: %w", err)
	}
	obNode.signer = signer
	obNode.orderLockManager = NewOrderLockManager()

	obNode.orderProcessor = orders.NewOrderProcessor(&orders.Config{
		NodeID:               nodeID,
		Identity:             obNode.peerID,
		Signer:               signer,
		Db:                   obRepo.DB(),
		Multiwallet:          obNode.multiwallet,
		Messenger:            obNode.messenger,
		ExchangeRateProvider: erp,
		EventBus:             bus,
		CalcCIDFunc:          obNode.contentStore.ComputeCID,
		FeatureManager:       obNode.featureManager,
		StateValidator:       &coreStateBridge{},
	})
	// Register libp2p HTTP proxy handler for standalone nodes so that the
	// SaaS proxy can forward management requests via libp2p streams.
	if !cfg.SaaSMode && len(cfg.HTTPProxyTrustedPeers) > 0 {
		trustedPeers := make([]peer.ID, 0, len(cfg.HTTPProxyTrustedPeers))
		for _, ps := range cfg.HTTPProxyTrustedPeers {
			pid, err := peer.Decode(ps)
			if err != nil {
				logger.LogErrorWithIDf(log, nodeID, "Invalid HTTP proxy trusted peer %q: %v", ps, err)
				continue
			}
			trustedPeers = append(trustedPeers, pid)
		}
		if len(trustedPeers) > 0 {
			localAddr := cfg.HTTPProxyLocalAddr
			if localAddr == "" {
				localAddr = gatewayLocalAddr(cfg)
			}
			proxyProto := protocol.ID(obnet.ProtocolHTTPProxyMainnet)
			if cfg.Testnet {
				proxyProto = protocol.ID(obnet.ProtocolHTTPProxyTestnet)
			}
			obnet.RegisterHTTPProxyOnHost(infra.Host, proxyProto, trustedPeers, localAddr)
			logger.LogInfoWithID(log, nodeID, "LibP2P HTTP proxy handler registered")
		}
	}

	obNode.applyOptions([]NodeOption{
		WithNodeFeatureProvider(NewConfigNodeFeatureProvider(cfg)),
	})
	obNode.registerHandlers()
	obNode.listenNetworkEvents()

	// Start heartbeat sender for standalone stores registered with SaaS.
	// Runs after applyOptions which may auto-register and obtain an API key.
	hbSaaSURL := cfg.SaaSAPIURL
	hbAPIKey := cfg.StandaloneAPIKey
	if sharedManager != nil {
		if hbSaaSURL == "" {
			hbSaaSURL = sharedManager.saasAPIURL
		}
		if hbAPIKey == "" {
			hbAPIKey = sharedManager.standaloneAPIKey
		}
	}
	if !cfg.SaaSMode && hbSaaSURL != "" && hbAPIKey != "" {
		hbCfg := obnet.StoreHeartbeatConfig{
			SaaSURL: hbSaaSURL,
			PeerID:  obNode.peerID.String(),
			APIKey:  hbAPIKey,
			Version: nodeVersion.String(),
		}
		if cfg.StandaloneConnectivity == "public" || cfg.StandaloneConnectivity == "tunnel" {
			if len(cfg.SwarmAddrs) > 0 {
				hbCfg.EndpointURL = cfg.SwarmAddrs[0]
			}
		}
		heartbeat := obnet.NewStoreHeartbeatSender(hbCfg)
		heartbeat.Start(ctx)
		logger.LogInfoWithID(log, nodeID, "Store heartbeat sender started")
	}

	return obNode, nil
}

// NewNodeWithOptions creates a MobazhaNode with explicit HostService and
// functional options. This allows hosting (SaaS) to inject custom adapters
// such as KeyVaultProvider without modifying the core construction flow.
//
// Usage:
//
//	node, err := core.NewNodeWithOptions(ctx, cfg, userID, hostService,
//	    core.WithKeyProvider(keyVaultProvider),
//	)
func NewNodeWithOptions(ctx context.Context, cfg *repo.Config, nodeID string,
	hs coreiface.HostService, opts ...NodeOption) (*MobazhaNode, error) {
	node, err := NewNode(ctx, cfg, nodeID, hs)
	if err != nil {
		return nil, err
	}
	for _, opt := range opts {
		opt(node)
	}
	return node, nil
}

// coreStateBridge wraps mobazha-core order state machine for transition validation.
// This is defined here (instead of using pkg/core.OrderStateBridge) to avoid import cycles.
type coreStateBridge struct{}

func (b *coreStateBridge) ValidateTransition(currentState, event int) (int, bool) {
	result := coreorders.Transition(coreorders.OrderState(currentState), coreorders.OrderEvent(event))
	return int(result.NewState), result.Valid
}

func (b *coreStateBridge) GetAllowedEvents(state int) []int {
	allowed := coreorders.AllowedEvents(coreorders.OrderState(state))
	result := make([]int, len(allowed))
	for i, e := range allowed {
		result[i] = int(e)
	}
	return result
}

type dummyListener struct {
	addr net.Addr
}

func (d *dummyListener) Addr() net.Addr {
	return d.addr
}
func (d *dummyListener) Accept() (net.Conn, error) {
	conn, _ := net.FileConn(nil)
	return conn, nil
}
func (d *dummyListener) Close() error {
	return nil
}

func InitializeMultiwallet(mw chains.Multiwallet, db database.Database, creationDate time.Time) error {
	var bip44ModelKey models.Key
	err := db.View(func(tx database.Tx) error {
		return tx.Read().Where("name = ?", "bip44").First(&bip44ModelKey).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("can not initialize wallet: seed does not exist in database")
	} else if err != nil {
		return err
	}

	bip44Key, err := hdkeychain.NewKeyFromString(string(bip44ModelKey.Value))
	if err != nil {
		return fmt.Errorf("cannot decode key, %v", err)
	}

	// 获取 SOL 私钥
	var dbSolKey models.Key
	err = db.View(func(tx database.Tx) error {
		return tx.Read().Where("name = ?", "solana").First(&dbSolKey).Error
	})
	if err != nil {
		return fmt.Errorf("can not initialize solana wallet: solana key does not exist in database")
	}

	solPrivKey := solana.PrivateKey(dbSolKey.Value)

	for chain, wallet := range mw {
		if chain == iwallet.ChainSolana {
			// 对于 SOL 钱包，使用 solPrivKey
			solWallet, ok := wallet.(*solanaWal.SolanaWallet)
			if !ok {
				return fmt.Errorf("wallet is not a SolanaWallet")
			}
			// 直接使用 solPrivKey 初始化钱包
			if err := solWallet.InitializeWithKey(solPrivKey, creationDate); err != nil {
				return err
			}
		} else if chain == iwallet.ChainFiat {
			// Do nothing
		} else {
			// 其他钱包使用 bip44Key
			if !wallet.WalletExists() {
				canonicalNative := iwallet.CoinType("")
				if chain == iwallet.ChainMock {
					// ChainMock is test-only and intentionally outside canonical asset registry.
					canonicalNative = iwallet.CtMock
				} else {
					canonicalNative, err = iwallet.RequireCanonicalNativeCoinType(chain)
					if err != nil {
						return err
					}
				}
				pricingCode, err := canonicalNative.PricingCurrencyCode()
				if err != nil {
					return err
				}
				def, err := models.CurrencyDefinitions.Lookup(pricingCode)
				if err != nil {
					return err
				}

				coinTypeKey, err := bip44Key.Derive(hdkeychain.HardenedKeyStart + uint32(def.Bip44Code))
				if err != nil {
					return err
				}

				accountKey, err := coinTypeKey.Derive(hdkeychain.HardenedKeyStart + 0)
				if err != nil {
					return err
				}

				if err := wallet.CreateWallet(*accountKey, nil, creationDate); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (n *MobazhaNode) registerHandlers() {
	// P2P chat message handlers removed; chat is Matrix-based (mautrix-go).

	n.networkService.RegisterHandler(pb.Message_ACK, func(from peer.ID, message *pb.Message) error {
		return n.handleAckMessage(from, message)
	})
	n.networkService.RegisterHandler(pb.Message_FOLLOW, func(from peer.ID, message *pb.Message) error {
		if n.followService != nil {
			return n.followService.HandleFollowMessage(from, message)
		}
		return fmt.Errorf("follow service not initialized")
	})
	n.networkService.RegisterHandler(pb.Message_UNFOLLOW, func(from peer.ID, message *pb.Message) error {
		if n.followService != nil {
			return n.followService.HandleUnFollowMessage(from, message)
		}
		return fmt.Errorf("follow service not initialized")
	})
	n.networkService.RegisterHandler(pb.Message_STORE, func(from peer.ID, message *pb.Message) error {
		return n.handleStoreMessage(from, message)
	})
	n.networkService.RegisterHandler(pb.Message_ORDER, func(from peer.ID, message *pb.Message) error {
		return n.handleOrderMessage(from, message)
	})
	n.networkService.RegisterHandler(pb.Message_ADDRESS_REQUEST, func(from peer.ID, message *pb.Message) error {
		return n.orderService.handleAddressRequest(from, message)
	})
	n.networkService.RegisterHandler(pb.Message_ADDRESS_RESPONSE, func(from peer.ID, message *pb.Message) error {
		return n.orderService.handleAddressResponse(from, message)
	})
	n.networkService.RegisterHandler(pb.Message_PING, func(from peer.ID, message *pb.Message) error {
		return n.handlePingMessage(from, message)
	})
	n.networkService.RegisterHandler(pb.Message_PONG, func(from peer.ID, message *pb.Message) error {
		return n.handlePongMessage(from, message)
	})
	n.networkService.RegisterHandler(pb.Message_DISPUTE, func(from peer.ID, message *pb.Message) error {
		return n.orderService.handleDisputeMessage(from, message, n.isDuplicate, n.sendAckMessage)
	})
}

func (n *MobazhaNode) listenNetworkEvents() {
	serverMap := make(map[string]bool)
	for _, server := range n.storeAndForwardServers {
		serverMap[server] = true
	}

	connected := func(_ inet.Network, conn inet.Conn) {
		if serverMap[conn.RemotePeer().String()] {
			logger.LogDebugWithIDf(log, n.nodeID, "Established connection to store and forward server %s", conn.RemotePeer())
		}
		n.eventBus.Emit(&events.PeerConnected{Peer: conn.RemotePeer()})
	}
	disConnected := func(_ inet.Network, conn inet.Conn) {
		if serverMap[conn.RemotePeer().String()] {
			logger.LogDebugWithIDf(log, n.nodeID, "Disconnected from store and forward server %s", conn.RemotePeer())
		}
		n.eventBus.Emit(&events.PeerDisconnected{Peer: conn.RemotePeer()})
	}

	notifier := &inet.NotifyBundle{
		ConnectedF:    connected,
		DisconnectedF: disConnected,
	}

	n.peerHost.Network().Notify(notifier)
}

// newMessageWithID returns a new *pb.Message with a random
// message ID.
func newMessageWithID() *pb.Message {
	messageID := make([]byte, 20)
	rand.Read(messageID)
	return &pb.Message{
		MessageID: hex.EncodeToString(messageID),
	}
}

// newLightweightNode creates a non-default node without its own P2P infrastructure.
// It creates a minimal libp2p Host for identity and messaging, and shares
// the default node's infrastructure for content operations.
//
// Skipped (compared to full node):
//   - P2P infrastructure (Host, DHT, datastores)
//   - Swarm/Gateway port allocation
//   - SNF Server (only default node runs it)
//   - bootstrapDHT()
//
// Retained:
//   - Mobazha repo (DB, keys) — already created by caller
//   - Key derivation (escrow, bip44, sol, rating)
//   - NetworkService (uses minimal Host)
//   - Messenger (via SNF Proxy)
//   - OrderProcessor
//   - Multiwallet
//   - FollowerTracker (uses minimal Host)
func newLightweightNode(
	ctx context.Context,
	cfg *repo.Config,
	nodeID string,
	obRepo *repo.Repo,
	sharedManager *SharedManager,
	shutdownTorFunc func() error,
	hs coreiface.HostService,
) (*MobazhaNode, error) {
	netConfig := sharedManager.NetConfig

	// ── 1. Load identity key and create minimal libp2p Host ──────────
	var identityKeyBytes []byte
	if len(cfg.IdentityKey) > 0 {
		identityKeyBytes = cfg.IdentityKey
	} else {
		var dbIdentityKey models.Key
		err := obRepo.DB().View(func(tx database.Tx) error {
			return tx.Read().Where("name = ?", "identity").First(&dbIdentityKey).Error
		})
		if err != nil {
			return nil, fmt.Errorf("lightweight: failed to load identity key from DB: %w", err)
		}
		identityKeyBytes = dbIdentityKey.Value
	}

	privKey, nodePeerID, err := repo.PrivKeyAndPeerIDFromKey(identityKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("lightweight: failed to parse identity key: %w", err)
	}

	// Create a minimal libp2p host — identity only, no listen addresses.
	// This host is used for NetworkService protocol handling and peer identity.
	nodeCtx, nodeCancel := context.WithCancel(ctx)
	minimalHost, err := libp2p.New(
		libp2p.Identity(privKey),
		libp2p.NoListenAddrs,
	)
	if err != nil {
		nodeCancel()
		return nil, fmt.Errorf("lightweight: failed to create minimal host: %w", err)
	}

	// Cleanup on failure — released on success path at the end.
	success := false
	defer func() {
		if !success {
			minimalHost.Close()
			nodeCancel()
		}
	}()

	// ── 2. NetDB (optional) ──────────────────────────────────────────
	var netDB *netdb.NetDB
	netDBEndpoint := netConfig.GetNetDBEndpoint()
	if cfg.NetDBEndpoint != "" {
		netDBEndpoint = cfg.NetDBEndpoint
	}
	if len(netDBEndpoint) > 0 {
		netDB, _ = netdb.NewNetDB(netDBEndpoint, nodePeerID.String(), privKey)
	}

	walletTestnet := cfg.Testnet
	if cfg.WalletTestnet {
		walletTestnet = cfg.WalletTestnet
	}

	// ── 3. Load wallet keys ──────────────────────────────────────────
	var (
		dbBip44Key   models.Key
		dbEscrowKey  models.Key
		dbRatingKey  models.Key
		dbSolKey     models.Key
		prefs        models.UserPreferences
		needDBUpdate bool
	)

	err = obRepo.DB().View(func(tx database.Tx) error {
		if err := tx.Read().First(&prefs).Error; err != nil {
			return fmt.Errorf("failed to load user preferences: %v", err)
		}
		dbEscrowKey, dbBip44Key, dbSolKey, dbRatingKey, err = repo.GetKeysFromDB(tx)
		if err != nil {
			logger.LogInfoWithID(log, nodeID, "Keys missing from DB, need derivation")
			needDBUpdate = true
			return nil
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if needDBUpdate {
		logger.LogInfoWithID(log, nodeID, "Deriving wallet keys from mnemonic")
		err = obRepo.DB().Update(func(tx database.Tx) error {
			var dbMnemonic models.Key
			err = tx.Read().Where("name = ?", "mnemonic").First(&dbMnemonic).Error
			if err != nil {
				return fmt.Errorf("failed to load mnemonic: %v", err)
			}
			hdSeed := bip39.NewSeed(string(dbMnemonic.Value), "")
			escrowKey, ratingKey, bip44Key, solKey, err := repo.CreateHDKeys(hdSeed)
			if err != nil {
				return fmt.Errorf("failed to derive HD keys: %v", err)
			}
			if err := repo.SaveKeysToDB(tx, escrowKey, bip44Key, solKey, ratingKey); err != nil {
				return fmt.Errorf("failed to save keys: %v", err)
			}
			dbEscrowKey, dbBip44Key, dbSolKey, dbRatingKey, err = repo.GetKeysFromDB(tx)
			if err != nil {
				return fmt.Errorf("failed to reload keys: %v", err)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	bip44Key, err := hdkeychain.NewKeyFromString(string(dbBip44Key.Value))
	if err != nil {
		return nil, fmt.Errorf("parse bip44 key: %w", err)
	}
	ethMasterKey, err := utils.GenerateEthPrivateKey(bip44Key)
	if err != nil {
		return nil, fmt.Errorf("derive eth master key: %w", err)
	}
	tronMasterKey, err := utils.GenerateTRONPrivateKey(bip44Key)
	if err != nil {
		return nil, fmt.Errorf("derive tron master key: %w", err)
	}
	escrowKey, _ := btcec.PrivKeyFromBytes(dbEscrowKey.Value)
	ratingKey, _ := btcec.PrivKeyFromBytes(dbRatingKey.Value)
	solPrivKey := solana.PrivateKey(dbSolKey.Value)

	// ── 4. Multiwallet ───────────────────────────────────────────────
	erp := sharedManager.ExchangeRateProvider

	// All wallets are constructed with nil ChainClient — chain clients are
	// injected during MobazhaNode.Start() based on mode:
	//   - SaaS: shared clients from HostService (one connection per chain)
	//   - Standalone: per-node clients from ChainAPIs config or defaults
	// This eliminates 5+ RPC connections per tenant while preserving signing.
	enabledChains := iwallet.GetAllSupportedChainTypes()
	opts := []chains.Option{
		chains.NodeID(nodeID),
		chains.DataDir(path.Join(cfg.DataDir, "nodes", nodeID)),
		chains.LogDir(cfg.LogDir),
		chains.Chains(enabledChains),
		chains.LogLevel(repo.LogLevelMap[strings.ToLower(cfg.LogLevel)]),
		chains.NetConfig(netConfig),
		chains.Testnet(walletTestnet),
		chains.Regtest(cfg.Regtest),
	}

	mw, err := chains.NewMultiwallet(opts...)
	if err != nil {
		return nil, err
	}
	if err := InitializeMultiwallet(mw, obRepo.DB(), time.Now()); err != nil {
		return nil, err
	}
	var walletOp pkgcontracts.WalletOperator = &mw

	// Chain client injection is deferred to MobazhaNode.Start():
	//   - EVM: startEVMChainClients()
	//   - Solana: startSolanaChainClients()
	//   - UTXO: startUTXOPaymentMonitor()
	// Both SaaS and standalone modes inject during Start(), not at construction time.

	// ── 5. NetworkService & FollowerTracker ───────────────────────────
	// Get global blocked IDs from HostService (cached by default node),
	// avoiding per-tenant contract query connections.
	globalBlockedIds := []peer.ID{}
	if hs != nil {
		if ids := hs.GetGlobalBlockedIDs(); len(ids) > 0 {
			globalBlockedIds = ids
			logger.LogInfoWithIDf(log, nodeID, "Using %d global blocked IDs from HostService", len(ids))
		}
	}

	blocked, err := prefs.BlockedNodes()
	if err != nil {
		return nil, err
	}
	bm := obnet.NewBanManager(globalBlockedIds, blocked)
	service := obnet.NewNetworkService(nodeID, minimalHost, bm, cfg.Testnet)
	if hs != nil {
		if ld, ok := any(hs).(obnet.LocalDeliverer); ok {
			service.SetLocalDeliverer(ld)
		}
	}

	bus := events.NewBus()
	tracker := NewFollowerTracker(obRepo, bus, minimalHost)

	// ── 6. Construct the MobazhaNode ─────────────────────────────────
	obNode := &MobazhaNode{
		sharedManager: sharedManager,
		identityFields: identityFields{
			nodeID:     nodeID,
			peerID:     nodePeerID,
			privKey:    privKey,
			peerHost:   minimalHost,
			nodeCtx:    nodeCtx,
			nodeCancel: nodeCancel,
		},
		storageFields: storageFields{
			db:   obRepo.DB(),
			repo: obRepo,
		},
		cryptoFields: cryptoFields{
			ethMasterKey:    ethMasterKey,
			escrowMasterKey: escrowKey,
			ratingMasterKey: ratingKey,
			solPrivKey:      &solPrivKey,
			tronMasterKey:   tronMasterKey,
			bip44Key:        bip44Key,
		},
		networkFields: networkFields{
			networkService:         service,
			banManager:             bm,
			eventBus:               bus,
			followerTracker:        tracker,
			storeAndForwardServers: cfg.StoreAndForwardServers,
		},
		walletFields: walletFields{
			multiwallet:   walletOp,
			exchangeRates: erp,
			relayAPIURL:   cfg.RelayAPIURL,
		},
		ipnsFields: ipnsFields{
			netDB:     netDB,
			netConfig: netConfig,
		},
		modeFlags: modeFlags{
			testnet:       cfg.Testnet,
			walletTestnet: walletTestnet,
			torOnly:       cfg.Tor,
		},
		lifecycleFields: lifecycleFields{
			shutdownTorFunc:      shutdownTorFunc,
			publishChan:          make(chan pubCloser),
			featureManager:       pkgconfig.GetGlobalFeatureManager(),
			initialBootstrapChan: make(chan struct{}),
			shutdown:             make(chan struct{}),
		},
		hostService: hs,
	}

	obNode.contentStore = &cidContentStore{}

	// Pass shared CryptoStore (SaaS multi-tenant) for initMatrixChatService().
	if cfg.MatrixCryptoStore != nil {
		obNode.matrixCryptoStore = cfg.MatrixCryptoStore
	}

	sharedManager.AddNode(nodeID, obNode)

	// Lightweight nodes use the shared HTTP gateway for websocket hubs
	// when available. In hosting mode, httpGateway is nil because the
	// hosting project manages its own HTTP gateway and websocket hubs.
	var notifyWsFn func(any) error
	if gw := sharedManager.GetHTTPGateway(); gw != nil {
		gw.EnsureHubForUser(nodeID)
		notifyWsFn = gw.NotifyWebsockets(nodeID)
	}

	if err := MigrateNodeSettings(obNode.db); err != nil {
		logger.LogErrorWithIDf(log, nodeID, "Failed to migrate node_settings: %v", err)
	}
	if err := aipkg.MigrateChatModels(obNode.db); err != nil {
		logger.LogErrorWithIDf(log, nodeID, "Failed to migrate ai_chat_sessions: %v", err)
	}

	initWebhookSubsystem(obNode)
	initDiscountSubsystem(obNode)
	initCollectionSubsystem(obNode)
	initFiatSubsystem(obNode)
	initSupplyChainSubsystem(obNode)
	initShippingSubsystem(obNode)
	initEventDispatcher(obNode, notifyWsFn)
	initPlatformAIConfig(obNode, cfg)

	// ── 7. Messenger (via SNF Proxy) ─────────────────────────────────
	messengerCfg := &obnet.MessengerConfig{
		NodeID:  nodeID,
		Service: service,
		Privkey: privKey,
		Context: nodeCtx,
		DB:      obRepo.DB(),
		Testnet: cfg.Testnet,
		GetProfileFunc: func(ctx context.Context, peerID peer.ID, reqCtx *request.Context, useCache bool) (*models.Profile, error) {
			return obNode.profileService.GetProfile(ctx, peerID, reqCtx, useCache)
		},
	}

	// Always set SNFServers so the messenger has a proper fallback list
	// (used when the target peer's profile can't be loaded).
	messengerCfg.SNFServers = sharedManager.SNFServers

	if sharedManager.HasSNFProxy() {
		proxy := sharedManager.GetSNFProxy()
		localClient, err := proxy.RegisterNode(nodePeerID, privKey)
		if err != nil {
			logger.LogErrorWithIDf(log, nodeID, "Lightweight: Failed to register with SNF Proxy: %v, falling back to direct", err)
		} else {
			messengerCfg.SNFClient = localClient
			logger.LogInfoWithIDf(log, nodeID, "Lightweight: Using SNF Proxy for store-and-forward messaging")
		}
	}

	obNode.messenger, err = obnet.NewMessenger(messengerCfg)
	if err != nil {
		return nil, err
	}

	// ── 8. Signer & OrderProcessor ───────────────────────────────────
	signer, err := corecontracts.NewKeyPairSignerFromMarshaledKey(identityKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("lightweight: failed to create signer: %w", err)
	}
	obNode.signer = signer
	obNode.orderLockManager = NewOrderLockManager()

	obNode.orderProcessor = orders.NewOrderProcessor(&orders.Config{
		NodeID:               nodeID,
		Identity:             nodePeerID,
		Signer:               signer,
		Db:                   obRepo.DB(),
		Multiwallet:          obNode.multiwallet,
		Messenger:            obNode.messenger,
		ExchangeRateProvider: erp,
		EventBus:             bus,
		CalcCIDFunc:          obNode.contentStore.ComputeCID,
		FeatureManager:       obNode.featureManager,
		StateValidator:       &coreStateBridge{},
	})

	obNode.applyOptions([]NodeOption{
		WithNodeFeatureProvider(NewConfigNodeFeatureProvider(cfg)),
	})
	obNode.registerHandlers()
	obNode.listenNetworkEvents()

	success = true
	logger.LogInfoWithIDf(log, nodeID, "Lightweight node created: PeerID=%s", nodePeerID)
	return obNode, nil
}

// initWebhookSubsystem initializes the per-node webhook subsystem:
// migrates DB models, creates store + engine.
// The Engine is started here; the WebhookSink is plugged into the Dispatcher later.
func initWebhookSubsystem(obNode *MobazhaNode) {
	if err := webhookinternal.MigrateModels(obNode.db); err != nil {
		logger.LogErrorWithIDf(log, obNode.nodeID, "Webhook: failed to migrate models: %v", err)
		return
	}

	store := webhookinternal.NewSQLiteStore(obNode.db)
	engine := wh.NewEngine(store, wh.DefaultConfig())

	obNode.webhookStore = store
	obNode.webhookEngine = engine
	logger.LogInfoWithID(log, obNode.nodeID, "Webhook subsystem initialized")
}

// initCollectionSubsystem initializes the per-node collection subsystem:
// migrates DB models, creates CollectionStore, and wires up CollectionAppService.
func initCollectionSubsystem(obNode *MobazhaNode) {
	if err := database.MigrateCollectionModels(obNode.db); err != nil {
		logger.LogErrorWithIDf(log, obNode.nodeID, "Collection: failed to migrate models: %v", err)
		return
	}
	store := database.NewGormCollectionStore(obNode.db)
	obNode.collectionService = NewCollectionAppService(store, obNode.eventBus, obNode.nodeID)

	if obNode.discountService != nil {
		obNode.discountService.collectionStore = store
	}

	// NOTE: onDeleteCleanup wiring moved to options.go after listingService is created,
	// because initCollectionSubsystem runs before applyOptions() where listingService is initialized.

	logger.LogInfoWithID(log, obNode.nodeID, "Collection subsystem initialized")
}

// initSupplyChainSubsystem initializes the per-node supply chain subsystem:
// migrates DB models, creates FulfillmentProviderRegistry and SupplyChainAppService.
// Concrete providers are registered via ConnectProvider API (FF-1+).
func initSupplyChainSubsystem(obNode *MobazhaNode) {
	if err := database.MigrateFulfillmentModels(obNode.db); err != nil {
		logger.LogErrorWithIDf(log, obNode.nodeID, "SupplyChain: failed to migrate models: %v", err)
		return
	}
	obNode.supplyChainRegistry = fulfillment.NewRegistry()
	obNode.supplyChainService = NewSupplyChainAppService(
		obNode.supplyChainRegistry,
		obNode.db,
		obNode.nodeID,
	)
	obNode.supplyChainService.Start(context.Background())

	if obNode.paymentService != nil {
		obNode.paymentService.SetSupplyChainChecker(obNode.supplyChainService)
	}

	logger.LogInfoWithID(log, obNode.nodeID, "Supply chain subsystem initialized")
}

// initFiatSubsystem initializes the per-node fiat payment subsystem:
// migrates DB models, creates FiatProviderRegistry and FiatPaymentAppService.
// Providers are registered later by hosting (SaaS) or node config (standalone).
func initFiatSubsystem(obNode *MobazhaNode) {
	if err := database.MigrateFiatModels(obNode.db); err != nil {
		logger.LogErrorWithIDf(log, obNode.nodeID, "Fiat: failed to migrate models: %v", err)
		return
	}
	obNode.fiatRegistry = fiat.NewRegistry()
	obNode.fiatPaymentService = NewFiatPaymentAppService(obNode.fiatRegistry, obNode.db, obNode.nodeID, obNode.walletTestnet)
	obNode.fiatPaymentService.SetOrderRepo(NewGormOrderRepo(obNode.db))
	obNode.fiatPaymentService.LoadAndRegisterProviders()

	if obNode.orderService != nil {
		obNode.orderService.SetFiatOps(obNode.fiatPaymentService)
	}

	logger.LogInfoWithID(log, obNode.nodeID, "Fiat payment subsystem initialized")
}

// initDiscountSubsystem initializes the per-node discount subsystem:
// migrates DB models, creates DiscountStore, and wires up DiscountAppService.
func initDiscountSubsystem(obNode *MobazhaNode) {
	if err := database.MigrateDiscountModels(obNode.db); err != nil {
		logger.LogErrorWithIDf(log, obNode.nodeID, "Discount: failed to migrate models: %v", err)
		return
	}
	store := database.NewGormDiscountStore(obNode.db)
	obNode.discountService = NewDiscountAppService(store, nil, obNode.eventBus, obNode.nodeID)
	logger.LogInfoWithID(log, obNode.nodeID, "Discount subsystem initialized")
}

// initShippingSubsystem initializes the per-node shipping subsystem:
// migrates DB models, creates ShippingStore, and wires up ShippingAppService.
// ListingPublisher is injected via closure to avoid nil receiver capture
// (listingService may not be initialized yet at call time).
func initShippingSubsystem(obNode *MobazhaNode) {
	if err := database.MigrateShippingModels(obNode.db); err != nil {
		logger.LogErrorWithIDf(log, obNode.nodeID, "Shipping: failed to migrate models: %v", err)
		return
	}
	store := database.NewGormShippingStore(obNode.db)

	if err := MigrateShippingFromPreferences(obNode.db, store); err != nil {
		logger.LogErrorWithIDf(log, obNode.nodeID, "Shipping: data migration failed (non-fatal): %v", err)
	}

	publisher := &lazyListingPublisher{node: obNode}
	svc := NewShippingAppService(store, publisher)
	svc.SetEventBus(obNode.eventBus)
	obNode.shippingService = svc
	logger.LogInfoWithID(log, obNode.nodeID, "Shipping subsystem initialized")
}

// lazyListingPublisher wraps MobazhaNode to implement contracts.ListingPublisher
// using closure-style deferred evaluation. This avoids capturing a nil
// listingService during initialization (callback-safety-rules).
type lazyListingPublisher struct {
	node *MobazhaNode
}

func (p *lazyListingPublisher) RepublishListing(ctx context.Context, slug string) error {
	return p.node.listingService.RepublishListing(ctx, slug)
}

// initEventDispatcher creates the unified EventDispatcher with NotificationSink,
// WebhookSink, and ChannelNotificationSink. Provides error isolation between sinks.
func initEventDispatcher(obNode *MobazhaNode, notifyWsFn func(any) error) {
	notifSink := notifications.NewNotificationSink(obNode.db, notifyWsFn)
	sinks := []events.EventSink{notifSink}

	if obNode.webhookEngine != nil {
		whSink := webhookinternal.NewWebhookSink(obNode.webhookEngine, obNode.nodeID)
		sinks = append(sinks, whSink)
	}

	channels := obNode.loadNotificationChannels()
	obNode.notifierSink = notifier.NewChannelNotificationSink(channels, obNode.nodeID)
	obNode.notifierSink.SetOnChanged(func(chs []notifier.ChannelConfig) {
		if err := obNode.SaveNotificationChannels(chs); err != nil {
			log.Errorf("Failed to persist notification channels: %v", err)
		}
	})
	sinks = append(sinks, obNode.notifierSink)

	obNode.eventDispatcher = events.NewDispatcher(obNode.eventBus, sinks...)
	logger.LogInfoWithIDf(log, obNode.nodeID, "Event dispatcher initialized with %d sinks", len(sinks))

	obNode.aiProxy = aipkg.NewProxy(nil)
	obNode.aiRateLimiter = aipkg.NewDailyRateLimiter()
}

// initPlatformAIConfig sets up the platform-provided AI fallback config
// from repo.Config fields injected by hosting (SaaS) or standalone admin.
func initPlatformAIConfig(obNode *MobazhaNode, cfg *repo.Config) {
	if cfg.PlatformAIProvider == "" || cfg.PlatformAIAPIKey == "" {
		return
	}
	pCfg := &aipkg.Config{
		Provider:   cfg.PlatformAIProvider,
		APIKey:     cfg.PlatformAIAPIKey,
		Model:      cfg.PlatformAIModel,
		BaseURL:    cfg.PlatformAIBaseURL,
		Enabled:    true,
		IsPlatform: true,
		DailyLimit: cfg.PlatformAIDailyLimit,
	}
	if pCfg.IsValid() {
		obNode.platformAIConfig = pCfg
		logger.LogInfoWithIDf(log, obNode.nodeID, "Platform AI configured (provider=%s, limit=%d/day)", pCfg.Provider, pCfg.DailyLimit)
	}
}

// gatewayLocalAddr derives the local HTTP API address from cfg.GatewayAddr.
// Used by the LibP2P HTTP proxy to forward incoming streams to the local API.
func gatewayLocalAddr(cfg *repo.Config) string {
	gwAddr := cfg.GatewayAddr
	if gwAddr == "" {
		gwAddr = repo.DefaultGatewayMultiaddr
	}
	host, port := "127.0.0.1", repo.DefaultGatewayPort
	parts := strings.Split(gwAddr, "/")
	for i, p := range parts {
		switch p {
		case "ip4", "ip6":
			if i+1 < len(parts) {
				host = parts[i+1]
			}
		case "tcp":
			if i+1 < len(parts) {
				port = parts[i+1]
			}
		}
	}
	if host == "0.0.0.0" {
		host = "127.0.0.1"
	}
	return "http://" + net.JoinHostPort(host, port)
}
