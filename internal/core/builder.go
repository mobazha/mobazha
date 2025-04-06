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
	"sync"
	"time"

	"github.com/bep/debounce"
	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
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
	"github.com/mobazha/mobazha3.0/internal/database/netdb"
	"github.com/mobazha/mobazha3.0/internal/events"
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/internal/models"
	"github.com/mobazha/mobazha3.0/internal/multiwallet"
	iwallet "github.com/mobazha/mobazha3.0/internal/multiwallet/wallet-interface"
	obnet "github.com/mobazha/mobazha3.0/internal/net"
	pb "github.com/mobazha/mobazha3.0/internal/net/mbzpb"
	"github.com/mobazha/mobazha3.0/internal/notifications"
	"github.com/mobazha/mobazha3.0/internal/orders"
	"github.com/mobazha/mobazha3.0/internal/orders/utils"
	"github.com/mobazha/mobazha3.0/internal/repo"
	oniontransport "github.com/mobazha/mobazha3.0/pkg/onion-transport"
	"github.com/mobazha/mobazha3.0/pkg/proxyclient"
	storeandforward "github.com/mobazha/mobazha3.0/pkg/store-and-forward"
	madns "github.com/multiformats/go-multiaddr-dns"
	"github.com/op/go-logging"
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

// NewNode constructs and returns an OpenBazaarNode using the given cfg.
func NewNode(ctx context.Context, cfg *repo.Config, nodeID string) (*OpenBazaarNode, error) {
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

	obRepo, err := repo.NewRepo(repoPath, cfg.Testnet)
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
		log.Notice("Starting embedded Tor client")

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

	// Load our identity key from the db and set it in the config.
	var dbIdentityKey models.Key
	err = obRepo.DB().View(func(tx database.Tx) error {
		return tx.Read().Where("name = ?", "identity").First(&dbIdentityKey).Error
	})

	ipfsConfig.Identity, err = repo.IdentityFromKey(dbIdentityKey.Value)
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
			if sharedDHT != nil {
				return sharedDHT, nil
			}

			dht, err := constructDHTRouting(dhtMode)(args)
			if err != nil {
				return nil, err
			}
			sharedDHT = dht
			return dht, nil
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

	if cfg.IPFSOnly {
		obNode := &OpenBazaarNode{
			sharedManager:        sharedManager,
			nodeID:               nodeID,
			repo:                 obRepo,
			ipfsNode:             ipfsNode,
			ipfsOnlyMode:         true,
			testnet:              cfg.Testnet,
			torOnly:              cfg.Tor,
			ipnsQuorum:           cfg.IPNSQuorum,
			ipnsResolver:         netConfig.GetIPNSResolver(),
			shutdownTorFunc:      shutdownTorFunc,
			eventBus:             events.NewBus(),
			initialBootstrapChan: make(chan struct{}),
			shutdown:             make(chan struct{}),
		}
		sharedManager.AddNode(nodeID, obNode)
		return obNode, nil
	}

	// Load the keys from the db
	var (
		dbBip44Key  models.Key
		dbEscrowKey models.Key
		dbRatingKey models.Key
		prefs       models.UserPreferences
	)
	err = obRepo.DB().View(func(tx database.Tx) error {
		if err := tx.Read().First(&prefs).Error; err != nil {
			return err
		}
		if err := tx.Read().Where("name = ?", "bip44").First(&dbBip44Key).Error; err != nil {
			return err
		}
		if err := tx.Read().Where("name = ?", "escrow").First(&dbEscrowKey).Error; err != nil {
			return err
		}
		return tx.Read().Where("name = ?", "ratings").First(&dbRatingKey).Error
	})
	if err != nil {
		return nil, err
	}

	bip44Key, _ := hdkeychain.NewKeyFromString(string(dbBip44Key.Value))
	ethMasterKey, _ := utils.GenerateEthPrivateKey(bip44Key)

	escrowKey, _ := btcec.PrivKeyFromBytes(dbEscrowKey.Value)
	ratingKey, _ := btcec.PrivKeyFromBytes(dbRatingKey.Value)

	enabledWallets := make([]iwallet.CoinType, len(cfg.EnabledWallets))
	for i, ew := range cfg.EnabledWallets {
		enabledWallets[i] = iwallet.CoinType(strings.ToUpper(ew))
	}

	erp := sharedManager.ExchangeRateProvider

	opts := []multiwallet.Option{
		multiwallet.NodeID(nodeID),
		multiwallet.DataDir(repoPath),
		multiwallet.LogDir(cfg.LogDir),
		multiwallet.Wallets(enabledWallets),
		multiwallet.LogLevel(repo.LogLevelMap[strings.ToLower(cfg.LogLevel)]),
		multiwallet.NetConfig(netConfig),
		multiwallet.Testnet(cfg.Testnet),
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
			log.Errorf("Failed to get global blocked nodes: %v", err)
		}
	} else {
		log.Errorf("Failed to create contracts util: %v", err)
	}

	blocked, err := prefs.BlockedNodes()
	if err != nil {
		return nil, err
	}
	bm := obnet.NewBanManager(globalBlockedIds, blocked)
	service := obnet.NewNetworkService(ipfsNode.PeerHost, bm, cfg.Testnet)

	bus := events.NewBus()
	tracker := NewFollowerTracker(obRepo, bus, ipfsNode.PeerHost)

	for _, server := range cfg.StoreAndForwardServers {
		_, err := peer.Decode(server)
		if err != nil {
			return nil, errors.New("invalid store and forward peer ID in config")
		}
	}

	var netDB *netdb.NetDB
	if len(netConfig.GetNetDBEndpoint()) > 0 {
		netDB, _ = netdb.NewNetDB(netConfig.GetNetDBEndpoint(), ipfsNode.Identity.String(), ipfsNode.PrivateKey)
	}

	// Construct our OpenBazaar node.repo object
	obNode := &OpenBazaarNode{
		sharedManager:          sharedManager,
		nodeID:                 nodeID,
		ipfsNode:               ipfsNode,
		repo:                   obRepo,
		ethMasterKey:           ethMasterKey,
		escrowMasterKey:        escrowKey,
		ratingMasterKey:        ratingKey,
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
		torOnly:                cfg.Tor,
		storeAndForwardServers: cfg.StoreAndForwardServers,
		channels:               make(map[string]*channels.Channel),
		shutdownTorFunc:        shutdownTorFunc,
		publishChan:            make(chan pubCloser),
		initialBootstrapChan:   make(chan struct{}),
		shutdown:               make(chan struct{}),
	}
	sharedManager.AddNode(nodeID, obNode)

	// If this is the default node, we need to create the HTTP gateway
	if isDefaultNode {
		_, err = sharedManager.initHTTPGateway(cfg)
		if err != nil {
			return nil, err
		}
	} else {
		sharedManager.GetHTTPGateway().EnsureHubForUser(nodeID)
	}

	obNode.notifier = notifications.NewNotifier(
		bus,
		obRepo.DB(),
		sharedManager.GetHTTPGateway().NotifyWebsockets(nodeID),
	)

	obNode.messenger, err = obnet.NewMessenger(&obnet.MessengerConfig{
		Service:        service,
		SNFServers:     sharedManager.SNFServers,
		Privkey:        ipfsNode.PrivateKey,
		Context:        ipfsNode.Context(),
		DB:             obRepo.DB(),
		Testnet:        cfg.Testnet,
		GetProfileFunc: obNode.GetProfile,
	})
	if err != nil {
		return nil, err
	}

	obNode.orderProcessor = orders.NewOrderProcessor(&orders.Config{
		Identity:             ipfsNode.Identity,
		IdentityPrivateKey:   ipfsNode.PrivateKey,
		Db:                   obRepo.DB(),
		Multiwallet:          mw,
		Messenger:            obNode.messenger,
		EscrowPrivateKey:     escrowKey,
		ExchangeRateProvider: erp,
		EventBus:             bus,
		CalcCIDFunc:          obNode.cid,
	})

	obNode.registerHandlers()
	obNode.listenNetworkEvents()

	return obNode, nil
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

	for ct, wallet := range mw {
		// Create wallet if not exists. This will fail if the bip44 key has been deleted
		// from the db, however we are not yet deleting keys or the mnemonic for encryption
		// purposes.
		if !wallet.WalletExists() {
			def, err := models.CurrencyDefinitions.Lookup(ct.CurrencyCode())
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
	return nil
}

// constructDHTRouting behaves exactly like the default constructDHTRouting function in the IPFS package
// but sets the ProtocolPrefix and MaxRecordAge.
func constructDHTRouting(mode dht.ModeOpt) func(args nlibp2p.RoutingOptionArgs) (routing.Routing, error) {
	return func(args nlibp2p.RoutingOptionArgs) (routing.Routing, error) {
		dhtOpts := []dht.Option{
			dht.Concurrency(10),
			dht.Mode(mode),
			dht.Datastore(args.Datastore),
			dht.Validator(args.Validator),
			dht.ProtocolPrefix(ProtocolDHT),
			dht.MaxRecordAge(maxRecordAge),
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

func (n *OpenBazaarNode) registerHandlers() {
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

func (n *OpenBazaarNode) listenNetworkEvents() {
	serverMap := make(map[string]bool)
	for _, server := range n.storeAndForwardServers {
		serverMap[server] = true
	}

	connected := func(_ inet.Network, conn inet.Conn) {
		if serverMap[conn.RemotePeer().String()] {
			logger.LogWithIDf(log, n.nodeID, logging.DEBUG, "Established connection to store and forward server %s", conn.RemotePeer())
		}
		n.eventBus.Emit(&events.PeerConnected{Peer: conn.RemotePeer()})
	}
	disConnected := func(_ inet.Network, conn inet.Conn) {
		if serverMap[conn.RemotePeer().String()] {
			logger.LogWithIDf(log, n.nodeID, logging.DEBUG, "Disconnected from store and forward server %s", conn.RemotePeer())
		}
		n.eventBus.Emit(&events.PeerDisconnected{Peer: conn.RemotePeer()})
	}

	notifier := &inet.NotifyBundle{
		ConnectedF:    connected,
		DisconnectedF: disConnected,
	}

	n.ipfsNode.PeerHost.Network().Notify(notifier)
}

func (n *OpenBazaarNode) listenWalletEvents() {
	blockChan := make(chan iwallet.CoinType)
	txChan := make(chan iwallet.CoinType)
	for ct, w := range n.multiwallet {
		go func(cointype iwallet.CoinType, wallet iwallet.Wallet) {
			blockSub := wallet.SubscribeBlocks()
			txSub := wallet.SubscribeTransactions()
			for {
				select {
				case <-n.shutdown:
					return
				case bi := <-blockSub:
					n.eventBus.Emit(&events.BlockReceived{
						BlockInfo:    bi,
						CurrencyCode: cointype.CurrencyCode(),
					})
					blockChan <- cointype
				case tx := <-txSub:
					n.eventBus.Emit(&events.TransactionReceived{
						Transaction:  tx,
						CurrencyCode: cointype.CurrencyCode(),
					})
					txChan <- cointype
				}
			}
		}(ct, w)
	}

	updateWalletInfo := func(ct iwallet.CoinType) {
		update := make(events.WalletUpdate)

		w := n.multiwallet[ct]

		bi, err := w.BlockchainInfo()
		if err != nil {
			logger.LogWithIDf(log, n.nodeID, logging.ERROR, "Error querying %s wallet for blockchain info: %s", ct.CurrencyCode(), err)
		}
		unconfirmed, confirmed, err := w.Balance()
		if err != nil {
			logger.LogWithIDf(log, n.nodeID, logging.ERROR, "Error querying %s wallet for balance: %s", ct.CurrencyCode(), err)
		}

		def, _ := models.CurrencyDefinitions.Lookup(ct.CurrencyCode())

		update[ct.CurrencyCode()] = events.WalletInfo{
			ChainHeight:        bi.Height,
			ConfirmedBalance:   confirmed,
			Currency:           *def,
			UnconfirmedBalance: unconfirmed,
		}
		n.eventBus.Emit(&update)
	}

	go func() {
		// To avoid multi wallet HTTP API calls when many txs come during rescan
		debounceMap := map[iwallet.CoinType]func(f func()){}
		mapMtx := sync.Mutex{}
		for {
			select {
			case <-n.shutdown:
				return
			case ct := <-blockChan:
				w := n.multiwallet[ct]
				if w.CoinCategory() == iwallet.CoinCategoryBitcoin {
					updateWalletInfo(ct)
				}
				// for ETH kind coin, we watch transactions from order and transfer events and update wallet info
			case ct := <-txChan:
				w := n.multiwallet[ct]
				if w.CoinCategory() == iwallet.CoinCategoryEthereum {
					mapMtx.Lock()
					debounced, ok := debounceMap[ct]
					if !ok {
						debounced = debounce.New(1 * time.Second)
						debounceMap[ct] = debounced
					}
					mapMtx.Unlock()
					debounced(func() {
						updateWalletInfo(ct)
					})
				}
			}
		}
	}()
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
