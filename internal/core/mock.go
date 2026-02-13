package core

import (
	"context"
	"fmt"
	"path"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	solana "github.com/gagliardetto/solana-go"
	"github.com/ipfs/boxo/bootstrap"
	"github.com/ipfs/boxo/namesys"
	"github.com/ipfs/kubo/core"
	coremock "github.com/ipfs/kubo/core/mock"
	nlibp2p "github.com/ipfs/kubo/core/node/libp2p"
	"github.com/ipfs/kubo/repo/fsrepo"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/routing"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/mobazha/mobazha3.0/internal/channels"
	"github.com/mobazha/mobazha3.0/internal/config"
	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/multiwallet"
	"github.com/mobazha/mobazha3.0/internal/net"
	"github.com/mobazha/mobazha3.0/internal/orders"
	"github.com/mobazha/mobazha3.0/internal/orders/utils"
	"github.com/mobazha/mobazha3.0/internal/repo"
	"github.com/mobazha/mobazha3.0/internal/wallet"
	corecontracts "github.com/mobazha/mobazha-core/contracts"
	pkgconfig "github.com/mobazha/mobazha3.0/pkg/config"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// MockNode builds a mock node with a temp data directory,
// in-memory database, mock IPFS node, and mock network
// service.
func MockNode() (*MobazhaNode, error) {
	r, err := repo.MockRepo()
	if err != nil {
		return nil, err
	}

	ipfsRepo, err := fsrepo.Open(path.Join(r.DataDir(), repo.IPFSDirName))
	if err != nil {
		return nil, err
	}

	ipfsConfig, err := ipfsRepo.Config()
	if err != nil {
		return nil, err
	}

	ipfsConfig.Bootstrap = nil

	var dbIdentityKey models.Key
	err = r.DB().View(func(tx database.Tx) error {
		return tx.Read().Where("name = ?", "identity").First(&dbIdentityKey).Error
	})
	if err != nil {
		return nil, err
	}

	ipfsConfig.Identity, err = repo.IdentityFromKey(dbIdentityKey.Value)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()

	mn := mocknet.New()

	ipfsNode, err := core.NewNode(ctx, &core.BuildCfg{
		Online: true,
		Repo:   ipfsRepo,
		Host:   coremock.MockHostOption(mn),
		ExtraOpts: map[string]bool{
			"pubsub": true,
		},
	})
	if err != nil {
		return nil, err
	}

	banManager := net.NewBanManager(nil, nil)
	service := net.NewNetworkService("", ipfsNode.PeerHost, banManager, true)

	// Load the keys from the db
	var (
		dbBip44Key  models.Key
		dbEscrowKey models.Key
		dbRatingKey models.Key
	)
	err = r.DB().View(func(tx database.Tx) error {
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

	// Generate a mock Solana private key for testing
	mockSolPrivKey := solana.NewWallet().PrivateKey

	bus := events.NewBus()
	tracker := NewFollowerTracker(r, bus, ipfsNode.PeerHost)

	w := wallet.NewMockWallet()
	w.SetEventBus(bus)

	mw := multiwallet.Multiwallet{
		iwallet.ChainMock: w,
	}

	erp, err := wallet.NewMockExchangeRates()
	if err != nil {
		return nil, err
	}

	// 创建共享管理器
	cfg := &repo.Config{
		// 添加必要的配置
	}

	sharedManager, err := NewSharedManager(ctx, cfg)
	if err != nil {
		return nil, err
	}

	node := &MobazhaNode{
		nodeID:               repo.DefaultNodeID,
		sharedManager:        sharedManager,
		peerID:               ipfsNode.Identity,
		privKey:              ipfsNode.PrivateKey,
		peerHost:             ipfsNode.PeerHost,
		nodeCtx:              ipfsNode.Context(),
		ipfsNode:             ipfsNode,
		db:                   r.DB(),
		repo:                 r,
		networkService:       service,
		eventBus:             bus,
		banManager:           banManager,
		ipnsQuorum:           1,
		netConfig:            config.DefaultNetConfig(),
		shutdown:             make(chan struct{}),
		ethMasterKey:         ethMasterKey,
		escrowMasterKey:      escrowKey,
		ratingMasterKey:      ratingKey,
		solPrivKey:           &mockSolPrivKey,
		multiwallet:          mw,
		followerTracker:      tracker,
		exchangeRates:        erp,
		channels:             make(map[string]*channels.Channel),
		initialBootstrapChan: make(chan struct{}),
		publishChan:          make(chan pubCloser),
		featureManager:       pkgconfig.GetGlobalFeatureManager(),
	}
	sharedManager.AddNode(repo.DefaultNodeID, node)

	node.messenger, err = net.NewMessenger(&net.MessengerConfig{
		Privkey: ipfsNode.PrivateKey,
		Service: service,
		DB:      r.DB(),
		Context: ipfsNode.Context(),
	})
	if err != nil {
		return nil, err
	}
	signer, err := corecontracts.NewKeyPairSignerFromMarshaledKey(dbIdentityKey.Value)
	if err != nil {
		return nil, err
	}
	node.signer = signer

	node.orderProcessor = orders.NewOrderProcessor(&orders.Config{
		Identity:             ipfsNode.Identity,
		Signer:              signer,
		Db:                   r.DB(),
		Multiwallet:          mw,
		Messenger:            node.messenger,
		EscrowPrivateKey:     escrowKey,
		ExchangeRateProvider: erp,
		EventBus:             bus,
		CalcCIDFunc:          node.cid,
		FeatureManager:       node.featureManager,
	})

	node.registerHandlers()
	node.listenNetworkEvents()
	node.publishHandler()
	close(node.initialBootstrapChan)
	return node, nil
}

// MockNet represents a network of connected mock nodes.
type Mocknet struct {
	nodes   []*MobazhaNode
	ipfsNet mocknet.Mocknet
	wn      *wallet.MockWalletNetwork
}

// NewMocknet returns a new MockNet without the
// nodes connected to each other.
func NewMocknet(numNodes int) (*Mocknet, error) {
	ctx := context.Background()

	// create network
	mn := mocknet.New()

	wn := wallet.NewMockWalletNetwork(numNodes)

	//bootstrap.DefaultBootstrapConfig.MinPeerThreshold = 1

	// 创建共享管理器
	cfg := &repo.Config{
		// 添加必要的配置
	}

	sharedManager, err := NewSharedManager(ctx, cfg)
	if err != nil {
		return nil, err
	}

	var nodes []*MobazhaNode
	for i := 0; i < numNodes; i++ {
		r, err := repo.MockRepo()
		if err != nil {
			return nil, err
		}

		ipfsRepo, err := fsrepo.Open(path.Join(r.DataDir(), repo.IPFSDirName))
		if err != nil {
			return nil, err
		}

		ipfsConfig, err := ipfsRepo.Config()
		if err != nil {
			return nil, err
		}

		ipfsConfig.Bootstrap = nil

		var dbIdentityKey models.Key
		err = r.DB().View(func(tx database.Tx) error {
			return tx.Read().Where("name = ?", "identity").First(&dbIdentityKey).Error
		})
		if err != nil {
			return nil, err
		}

		ipfsConfig.Identity, err = repo.IdentityFromKey(dbIdentityKey.Value)
		if err != nil {
			return nil, err
		}

		ipfsNode, err := core.NewNode(ctx, &core.BuildCfg{
			Online: true,
			Repo:   ipfsRepo,
			Host:   coremock.MockHostOption(mn),
			ExtraOpts: map[string]bool{
				"pubsub": true,
			},
			Routing: constructMockRouting,
		})
		if err != nil {
			return nil, err
		}

		ipfsNode.Namesys, err = namesys.NewNameSystem(ipfsNode.Routing, namesys.WithDatastore(ipfsNode.Repo.Datastore()))
		if err != nil {
			return nil, err
		}

		banManager := net.NewBanManager(nil, nil)
		service := net.NewNetworkService("", ipfsNode.PeerHost, banManager, true)

		// Load the keys from the db
		var (
			dbBip44Key  models.Key
			dbEscrowKey models.Key
			dbRatingKey models.Key
		)
		err = r.DB().View(func(tx database.Tx) error {
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

		// Generate a mock Solana private key for testing
		mockSolPrivKey := solana.NewWallet().PrivateKey

		bus := events.NewBus()
		tracker := NewFollowerTracker(r, bus, ipfsNode.PeerHost)

		w := wn.Wallets()[i]
		w.SetEventBus(bus)

		mw := multiwallet.Multiwallet{
			iwallet.ChainMock: w,
		}

		erp, err := wallet.NewMockExchangeRates()
		if err != nil {
			return nil, err
		}

		nodeID := fmt.Sprintf("node-%d", i)
		if i == 0 {
			nodeID = repo.DefaultNodeID
		}

		node := &MobazhaNode{
			nodeID:               nodeID,
			sharedManager:        sharedManager,
			peerID:               ipfsNode.Identity,
			privKey:              ipfsNode.PrivateKey,
			peerHost:             ipfsNode.PeerHost,
			nodeCtx:              ipfsNode.Context(),
			ipfsNode:             ipfsNode,
			db:                   r.DB(),
			repo:                 r,
			networkService:       service,
			eventBus:             bus,
			banManager:           banManager,
			ipnsQuorum:           1,
			netConfig:            config.DefaultNetConfig(),
			shutdown:             make(chan struct{}),
			ethMasterKey:         ethMasterKey,
			escrowMasterKey:      escrowKey,
			ratingMasterKey:      ratingKey,
			solPrivKey:           &mockSolPrivKey,
			multiwallet:          mw,
			followerTracker:      tracker,
			exchangeRates:        erp,
			channels:             make(map[string]*channels.Channel),
			initialBootstrapChan: make(chan struct{}),
			publishChan:          make(chan pubCloser),
			featureManager:       pkgconfig.GetGlobalFeatureManager(),
		}
		sharedManager.AddNode(nodeID, node)

		node.messenger, err = net.NewMessenger(&net.MessengerConfig{
			Privkey: ipfsNode.PrivateKey,
			Service: service,
			DB:      r.DB(),
			Context: ipfsNode.Context(),
		})
		if err != nil {
			return nil, err
		}
		signer, err := corecontracts.NewKeyPairSignerFromMarshaledKey(dbIdentityKey.Value)
		if err != nil {
			return nil, err
		}
		node.signer = signer

		node.orderProcessor = orders.NewOrderProcessor(&orders.Config{
			Identity:             ipfsNode.Identity,
			Signer:              signer,
			Db:                   r.DB(),
			Messenger:            node.messenger,
			Multiwallet:          mw,
			EscrowPrivateKey:     escrowKey,
			ExchangeRateProvider: erp,
			EventBus:             bus,
			CalcCIDFunc:          node.cid,
			FeatureManager:       node.featureManager,
		})

		node.registerHandlers()
		node.listenNetworkEvents()
		node.publishHandler()
		close(node.initialBootstrapChan)

		nodes = append(nodes, node)
	}

	if err := mn.LinkAll(); err != nil {
		return nil, err
	}

	bsinf := bootstrap.BootstrapConfigWithPeers(
		[]peer.AddrInfo{
			nodes[0].ipfsNode.Peerstore.PeerInfo(nodes[0].Identity()),
		},
	)

	for _, n := range nodes[1:] {
		if err := n.ipfsNode.Bootstrap(bsinf); err != nil {
			return nil, err
		}
	}

	return &Mocknet{nodes, mn, wn}, nil
}

// Nodes returns the Mobazha nodes in this network.
func (mn *Mocknet) Nodes() []*MobazhaNode {
	return mn.nodes
}

// Peers returns the peer IDs of the nodes in the network.
func (mn *Mocknet) Peers() []peer.ID {
	return mn.ipfsNet.Peers()
}

// StartAll starts all nodes in the network.
func (mn *Mocknet) StartAll() {
	for _, n := range mn.nodes {
		n.Start()
	}
}

func (mn *Mocknet) StartWalletNetwork() {
	mn.wn.Start()
}

// WalletNetwork returns the mock wallet network.
func (mn *Mocknet) WalletNetwork() *wallet.MockWalletNetwork {
	return mn.wn
}

// TearDown shutsdown the network and destroys the data directories.
func (mn *Mocknet) TearDown() error {
	for _, n := range mn.nodes {
		if n == nil {
			continue
		}
		n.Stop(true)
		if err := n.repo.DestroyRepo(); err != nil {
			return err
		}
	}
	return nil
}

func constructMockRouting(args nlibp2p.RoutingOptionArgs) (routing.Routing, error) {
	return dht.New(
		args.Ctx, args.Host,
		dht.Concurrency(10),
		dht.Mode(dht.ModeServer),
		dht.Datastore(args.Datastore),
		dht.Validator(args.Validator),
		dht.ProtocolPrefix(ProtocolDHT),
		dht.MaxRecordAge(maxRecordAge),
	)
}
