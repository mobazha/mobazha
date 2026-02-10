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
	config "github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core"
	nlibp2p "github.com/ipfs/kubo/core/node/libp2p"
	"github.com/ipfs/kubo/repo/fsrepo"
	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p-kad-dht/dual"
	lcfg "github.com/libp2p/go-libp2p/config"
	"github.com/libp2p/go-libp2p/core/host"
	inet "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/core/routing"
	"github.com/mobazha/mobazha3.0/internal/channels"
	"github.com/mobazha/mobazha3.0/internal/contracts"
	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/internal/multiwallet"
	solanaWal "github.com/mobazha/mobazha3.0/internal/multiwallet/coins/solana"
	obnet "github.com/mobazha/mobazha3.0/internal/net"
	"github.com/mobazha/mobazha3.0/internal/notifications"
	"github.com/mobazha/mobazha3.0/internal/orders"
	"github.com/mobazha/mobazha3.0/internal/orders/utils"
	"github.com/mobazha/mobazha3.0/internal/repo"
	corecontracts "github.com/mobazha/mobazha-core/contracts"
	coreorders "github.com/mobazha/mobazha-core/orders"
	oniontransport "github.com/mobazha/mobazha3.0/libs/onion-transport"
	"github.com/mobazha/mobazha3.0/libs/proxyclient"
	storeandforward "github.com/mobazha/mobazha3.0/libs/store-and-forward"
	pkgconfig "github.com/mobazha/mobazha3.0/pkg/config"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/database/netdb"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	madns "github.com/multiformats/go-multiaddr-dns"
	"github.com/op/go-logging"
	"github.com/tyler-smith/go-bip39"
	"go.uber.org/fx"
	"gorm.io/gorm"
)

const (
	// maxRecordAge is the maximum amount of time to keep a record in the DHT before deleting it.
	maxRecordAge = time.Hour * 24 * 7

	// netConfigEndpoint is the endpoint to get the node configuration.
	netConfigEndpoint = "https://mobazha.info/api/nodeConfig"
)

var (
	log             = logging.MustGetLogger("CORE")
	stdoutLogFormat = logging.MustStringFormatter(`%{color:reset}%{color}%{time:15:04:05.000} [%{level}] [%{module}/%{shortfunc}] %{message}`)
	fileLogFormat   = logging.MustStringFormatter(`%{time:15:04:05.000} [%{level}] [%{module}/%{shortfunc}] %{message}`)
	ProtocolDHT     protocol.ID
)

// 创建共享的DHT实例
var sharedDHT routing.Routing

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
	if len(cfg.IdentityKey) > 0 {
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

	// Load the IPFS Repo
	ipfsRepo, err := fsrepo.Open(path.Join(repoPath, repo.IPFSDirName))
	if err != nil {
		return nil, err
	}

	ipfsConfig, err := ipfsRepo.Config()
	if err != nil {
		return nil, err
	}

	// Disable MDNS
	ipfsConfig.Swarm.DisableNatPortMap = cfg.DisableNATPortMap

	getBootstrapAddrs := func() []string {
		// Merge the bootstrap addresses from the config with the ones from the net config.
		bootstraps := append(netConfig.BootstrapAddrs, cfg.BoostrapAddrs...)
		if cfg.Testnet {
			bootstraps = append(bootstraps, repo.DefaultTestnetBootstrapAddrs...)
		} else {
			bootstraps = append(bootstraps, repo.DefaultMainnetBootstrapAddrs...)
		}
		// Add the default IPFS bootstrap addresses
		bootstraps = append(bootstraps, config.DefaultBootstrapAddresses...)

		bootstrapAddrMap := make(map[string]bool)
		for _, addr := range bootstraps {
			bootstrapAddrMap[addr] = true
		}

		addrs := []string{}
		for addr := range bootstrapAddrMap {
			addrs = append(addrs, addr)
		}
		return addrs
	}
	ipfsConfig.Bootstrap = getBootstrapAddrs()

	// If swarm addresses were provided in the config, override the IPFS defaults.
	if len(cfg.SwarmAddrs) > 0 {
		ipfsConfig.Addresses.Swarm = cfg.SwarmAddrs
	}
	if cfg.Tor {
		ipfsConfig.Addresses.Swarm = []string{fmt.Sprintf("/onion3/%s:9003", onionID)}
	} else if cfg.DualStack {
		ipfsConfig.Addresses.Swarm = append(ipfsConfig.Addresses.Swarm, fmt.Sprintf("/onion3/%s:9003", onionID))
	}

	// If a gateway address was provided in the config, override the IPFS default.
	if cfg.GatewayAddr != "" {
		ipfsConfig.Addresses.Gateway = config.Strings{cfg.GatewayAddr}
	}

	if cfg.Tor {
		ipfsConfig.Swarm.DisableNatPortMap = true
	}

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

	ipfsConfig.Identity, err = repo.IdentityFromKey(identityKeyBytes)
	if err != nil {
		return nil, err
	}

	// Update the protocol IDs for Bitswap and the Kad-DHT. This is used to segregate the
	// network from mainline IPFS.
	if !cfg.Testnet {
		ProtocolDHT = obnet.ProtocolPrefixMainnet
	} else {
		ProtocolDHT = obnet.ProtocolPrefixTestnet
	}

	constructPeerHost := func(id peer.ID, ps peerstore.Peerstore, options ...libp2p.Option) (host.Host, error) {
		pkey := ps.PrivKey(id)
		if pkey == nil {
			return nil, fmt.Errorf("missing private key for node ID: %s", id)
		}
		options = append([]libp2p.Option{libp2p.Identity(pkey), libp2p.Peerstore(ps)}, options...)

		config := &lcfg.Config{}
		if err := config.Apply(options...); err != nil {
			return nil, err
		}
		config.DisableMetrics = true

		if cfg.Tor {
			config.Transports = []fx.Option{}
			if err := transportOpt(config); err != nil {
				return nil, err
			}
		} else if cfg.DualStack {
			if err := transportOpt(config); err != nil {
				return nil, err
			}
		}
		return config.NewNode()
	}

	// New IPFS build config
	dhtMode := dht.ModeAuto
	if cfg.DHTClientOnly {
		dhtMode = dht.ModeClient
	} else if isDefaultNode {
		// 默认节点使用服务器模式
		dhtMode = dht.ModeServer
	} else {
		// 其他节点使用客户端模式，减少开销
		dhtMode = dht.ModeClient
	}

	ncfg := &core.BuildCfg{
		Repo:      ipfsRepo,
		Online:    true,
		Permanent: true,
		ExtraOpts: map[string]bool{
			"ipnsps": !cfg.NoIPNSPubsub,
			"pubsub": true,
		},
		// 使用共享的DHT
		Routing: func(args nlibp2p.RoutingOptionArgs) (routing.Routing, error) {
			// 只在服务器模式下共享DHT
			if dhtMode == dht.ModeServer && sharedDHT != nil {
				return sharedDHT, nil
			}

			// 客户端模式或首次创建：每个节点使用自己的DHT
			dhtInstance, err := constructDHTRouting(dhtMode)(args)
			if err != nil {
				return nil, err
			}

			// 只在服务器模式下保存共享引用
			if dhtMode == dht.ModeServer {
				sharedDHT = dhtInstance
			}

			return dhtInstance, nil
		},
		Host: constructPeerHost,
	}

	// Construct IPFS node.
	ipfsNode, err := core.NewNode(ctx, ncfg)
	if err != nil {
		return nil, err
	}

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
		serverOpts := []storeandforward.Option{
			storeandforward.ServerProtocols(protocol.ID(snfServerProtocol)),
			storeandforward.ClientProtocols(protocol.ID(snfClientProtocol)),
			storeandforward.ReplicationPeers(snfReplicationPeers...),
			storeandforward.Datastore(ipfsNode.Repo.Datastore()),
		}
		_, err := storeandforward.NewServer(ipfsNode.Context(), ipfsNode.PeerHost, serverOpts...)
		if err != nil {
			return nil, err
		}
	}

	var netDB *netdb.NetDB
	if len(netConfig.GetNetDBEndpoint()) > 0 {
		netDB, _ = netdb.NewNetDB(netConfig.GetNetDBEndpoint(), ipfsNode.Identity.String(), ipfsNode.PrivateKey)
	}

	// 使用 WalletTestnet（如果设置），否则回退到 Testnet
	walletTestnet := cfg.Testnet
	if cfg.WalletTestnet {
		walletTestnet = cfg.WalletTestnet
	}

	if cfg.IPFSOnly {
		obNode := &MobazhaNode{
			sharedManager:        sharedManager,
			nodeID:               nodeID,
			repo:                 obRepo,
			ipfsNode:             ipfsNode,
			ipfsOnlyMode:         true,
			testnet:              cfg.Testnet,
			walletTestnet:        walletTestnet,
			torOnly:              cfg.Tor,
			ipnsQuorum:           cfg.IPNSQuorum,
			ipnsResolver:         netConfig.GetIPNSResolver(),
			netDB:                netDB,
			shutdownTorFunc:      shutdownTorFunc,
			eventBus:             events.NewBus(),
			featureManager:       pkgconfig.GetGlobalFeatureManager(),
			initialBootstrapChan: make(chan struct{}),
			shutdown:             make(chan struct{}),
			hostService:          hs,
		}
		sharedManager.AddNode(nodeID, obNode)
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
			return fmt.Errorf("获取用户偏好失败: %v", err)
		}
		dbEscrowKey, dbBip44Key, dbSolKey, dbRatingKey, err = repo.GetKeysFromDB(tx)
		if err != nil {
			logger.LogInfoWithID(log, nodeID, "数据库中缺少密钥，需要更新")
			needDBUpdate = true
			return nil
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if needDBUpdate {
		logger.LogInfoWithID(log, nodeID, "更新数据库中的密钥")
		err = obRepo.DB().Update(func(tx database.Tx) error {
			var dbMnemonic models.Key
			err = tx.Read().Where("name = ?", "mnemonic").First(&dbMnemonic).Error
			if err != nil {
				return fmt.Errorf("获取助记词失败: %v", err)
			}

			// 从助记词生成种子
			hdSeed := bip39.NewSeed(string(dbMnemonic.Value), "")
			escrowKey, ratingKey, bip44Key, solKey, err := repo.CreateHDKeys(hdSeed)
			if err != nil {
				return fmt.Errorf("生成密钥失败: %v", err)
			}

			// 保存新生成的密钥
			if err := repo.SaveKeysToDB(tx, escrowKey, bip44Key, solKey, ratingKey); err != nil {
				return fmt.Errorf("保存密钥失败: %v", err)
			}

			dbEscrowKey, dbBip44Key, dbSolKey, dbRatingKey, err = repo.GetKeysFromDB(tx)
			if err != nil {
				return fmt.Errorf("获取密钥失败: %v", err)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	bip44Key, _ := hdkeychain.NewKeyFromString(string(dbBip44Key.Value))
	ethMasterKey, _ := utils.GenerateEthPrivateKey(bip44Key)

	escrowKey, _ := btcec.PrivKeyFromBytes(dbEscrowKey.Value)
	ratingKey, _ := btcec.PrivKeyFromBytes(dbRatingKey.Value)

	solPrivKey := solana.PrivateKey(dbSolKey.Value)

	enabledChains := iwallet.GetAllSupportedChainTypes()

	erp := sharedManager.ExchangeRateProvider

	opts := []multiwallet.Option{
		multiwallet.NodeID(nodeID),
		multiwallet.DataDir(repoPath),
		multiwallet.LogDir(cfg.LogDir),
		multiwallet.Chains(enabledChains),
		multiwallet.LogLevel(repo.LogLevelMap[strings.ToLower(cfg.LogLevel)]),
		multiwallet.NetConfig(netConfig),
		multiwallet.Testnet(walletTestnet),
	}
	mw, err := multiwallet.NewMultiwallet(opts...)
	if err != nil {
		return nil, err
	}

	if err := InitializeMultiwallet(mw, obRepo.DB(), time.Now()); err != nil {
		return nil, err
	}

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

	blocked, err := prefs.BlockedNodes()
	if err != nil {
		return nil, err
	}
	bm := obnet.NewBanManager(globalBlockedIds, blocked)
	service := obnet.NewNetworkService(nodeID, ipfsNode.PeerHost, bm, cfg.Testnet)
	if hs != nil {
		if ld, ok := any(hs).(obnet.LocalDeliverer); ok {
			service.SetLocalDeliverer(ld)
		}
	}

	bus := events.NewBus()
	tracker := NewFollowerTracker(obRepo, bus, ipfsNode.PeerHost)

	for _, server := range cfg.StoreAndForwardServers {
		_, err := peer.Decode(server)
		if err != nil {
			return nil, errors.New("invalid store and forward peer ID in config")
		}
	}

	// Construct our Mobazha node.repo object
	obNode := &MobazhaNode{
		sharedManager:          sharedManager,
		nodeID:                 nodeID,
		ipfsNode:               ipfsNode,
		repo:                   obRepo,
		ethMasterKey:           ethMasterKey,
		escrowMasterKey:        escrowKey,
		ratingMasterKey:        ratingKey,
		solPrivKey:             &solPrivKey,
		ipnsQuorum:             cfg.IPNSQuorum,
		ipnsResolver:           netConfig.GetIPNSResolver(),
		netDB:                  netDB,
		netConfig:              netConfig,
		networkService:         service,
		banManager:             bm,
		eventBus:               bus,
		followerTracker:        tracker,
		multiwallet:            mw,
		exchangeRates:          erp,
		testnet:                cfg.Testnet,
		walletTestnet:          walletTestnet,
		torOnly:                cfg.Tor,
		storeAndForwardServers: cfg.StoreAndForwardServers,
		channels:               make(map[string]*channels.Channel),
		shutdownTorFunc:        shutdownTorFunc,
		publishChan:            make(chan pubCloser),
		featureManager:         pkgconfig.GetGlobalFeatureManager(),
		initialBootstrapChan:   make(chan struct{}),
		shutdown:               make(chan struct{}),
		hostService:            hs,
		stripeConfigCache:      netdb.NewStripeConfigCache(),
		relayAPIURL:            cfg.RelayAPIURL,
	}
	sharedManager.AddNode(nodeID, obNode)

	// If this is the default node, we need to create the HTTP gateway and initialize SNF Proxy
	if isDefaultNode {
		_, err = sharedManager.initHTTPGateway(cfg)
		if err != nil {
			return nil, err
		}

		// Initialize SNF Proxy using the default node's host as transport
		if err := sharedManager.InitSNFProxy(ipfsNode.PeerHost); err != nil {
			logger.LogErrorWithIDf(log, nodeID, "Failed to initialize SNF Proxy: %v", err)
			// Continue without proxy - will use direct connections
		}
	} else {
		sharedManager.GetHTTPGateway().EnsureHubForUser(nodeID)
	}

	obNode.notifier = notifications.NewNotifier(
		bus,
		obRepo.DB(),
		sharedManager.GetHTTPGateway().NotifyWebsockets(nodeID),
	)

	// Create messenger with appropriate SNF client
	messengerCfg := &obnet.MessengerConfig{
		NodeID:         nodeID,
		Service:        service,
		Privkey:        ipfsNode.PrivateKey,
		Context:        ipfsNode.Context(),
		DB:             obRepo.DB(),
		Testnet:        cfg.Testnet,
		GetProfileFunc: obNode.GetProfile,
	}

	// For non-default nodes, use the SNF Proxy's LocalClient if available
	if !isDefaultNode && sharedManager.HasSNFProxy() {
		proxy := sharedManager.GetSNFProxy()
		localClient, err := proxy.RegisterNode(ipfsNode.Identity, ipfsNode.PrivateKey)
		if err != nil {
			logger.LogErrorWithIDf(log, nodeID, "Failed to register with SNF Proxy: %v, falling back to direct connection", err)
			// Fall back to direct connection
			messengerCfg.SNFServers = sharedManager.SNFServers
		} else {
			messengerCfg.SNFClient = localClient
			logger.LogInfoWithIDf(log, nodeID, "Using SNF Proxy for store-and-forward messaging")
		}
	} else {
		// Default node or no proxy available - use direct connection
		messengerCfg.SNFServers = sharedManager.SNFServers
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

	obNode.orderProcessor = orders.NewOrderProcessor(&orders.Config{
		NodeID:                   nodeID,
		Identity:                 ipfsNode.Identity,
		Signer:                   signer,
		Db:                       obRepo.DB(),
		Multiwallet:              mw,
		Messenger:                obNode.messenger,
		EscrowPrivateKey:         escrowKey,
		ExchangeRateProvider:     erp,
		EventBus:                 bus,
		CalcCIDFunc:              obNode.cid,
		FeatureManager:           obNode.featureManager,
		GetStripeTransactionFunc: obNode.GetStripeTransaction,
		StateValidator:           &coreStateBridge{},
	})

	obNode.registerHandlers()
	obNode.listenNetworkEvents()

	return obNode, nil
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

func InitializeMultiwallet(mw multiwallet.Multiwallet, db database.Database, creationDate time.Time) error {
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
		} else if chain == iwallet.ChainStripe {
			// Do nothing
		} else {
			// 其他钱包使用 bip44Key
			if !wallet.WalletExists() {
				def, err := models.CurrencyDefinitions.Lookup(chain.String())
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

// constructDHTRouting behaves exactly like the default constructDHTRouting function in the IPFS package
// but sets the ProtocolPrefix and MaxRecordAge.
func constructDHTRouting(mode dht.ModeOpt) func(args nlibp2p.RoutingOptionArgs) (routing.Routing, error) {
	return func(args nlibp2p.RoutingOptionArgs) (routing.Routing, error) {
		dhtOpts := []dht.Option{
			dht.Concurrency(20),
			dht.Mode(mode),
			dht.Datastore(args.Datastore),
			dht.Validator(args.Validator),
			dht.ProtocolPrefix(ProtocolDHT),
			dht.MaxRecordAge(maxRecordAge),
			// 允许本地地址，支持共享端口场景
			dht.AddressFilter(nil),
		}
		if args.OptimisticProvide {
			dhtOpts = append(dhtOpts, dht.EnableOptimisticProvide())
		}
		if args.OptimisticProvideJobsPoolSize != 0 {
			dhtOpts = append(dhtOpts, dht.OptimisticProvideJobsPoolSize(args.OptimisticProvideJobsPoolSize))
		}
		wanOptions := []dht.Option{
			dht.BootstrapPeers(args.BootstrapPeers...),
		}
		lanOptions := []dht.Option{}
		if args.LoopbackAddressesOnLanDHT {
			lanOptions = append(lanOptions, dht.AddressFilter(nil))
		}
		return dual.New(
			args.Ctx, args.Host,
			dual.DHTOption(dhtOpts...),
			dual.WanDHTOption(wanOptions...),
			dual.LanDHTOption(lanOptions...),
		)
	}
}

func (n *MobazhaNode) registerHandlers() {
	n.networkService.RegisterHandler(pb.Message_CHAT, n.handleChatMessage)
	n.networkService.RegisterHandler(pb.Message_CHAT_GROUP, n.handleChatGroupMessage)
	n.networkService.RegisterHandler(pb.Message_ACK, n.handleAckMessage)
	n.networkService.RegisterHandler(pb.Message_FOLLOW, n.handleFollowMessage)
	n.networkService.RegisterHandler(pb.Message_UNFOLLOW, n.handleUnFollowMessage)
	n.networkService.RegisterHandler(pb.Message_STORE, n.handleStoreMessage)
	n.networkService.RegisterHandler(pb.Message_ORDER, n.handleOrderMessage)
	n.networkService.RegisterHandler(pb.Message_ADDRESS_REQUEST, n.handleAddressRequest)
	n.networkService.RegisterHandler(pb.Message_ADDRESS_RESPONSE, n.handleAddressResponse)
	n.networkService.RegisterHandler(pb.Message_PING, n.handlePingMessage)
	n.networkService.RegisterHandler(pb.Message_PONG, n.handlePongMessage)
	n.networkService.RegisterHandler(pb.Message_DISPUTE, n.handleDisputeMessage)
	n.networkService.RegisterHandler(pb.Message_CHANNEL_REQUEST, n.handleChannelRequest)
	n.networkService.RegisterHandler(pb.Message_CHANNEL_RESPONSE, n.handleChannelResponse)
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

	n.ipfsNode.PeerHost.Network().Notify(notifier)
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
