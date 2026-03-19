package core

import (
	"context"
	"encoding/json"
	"fmt"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	solana "github.com/gagliardetto/solana-go"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	ma "github.com/multiformats/go-multiaddr"
	corecontracts "github.com/mobazha/mobazha-core/contracts"
	"github.com/mobazha/mobazha3.0/internal/config"
	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/chains"
	"github.com/mobazha/mobazha3.0/internal/net"
	"github.com/mobazha/mobazha3.0/internal/orders"
	"github.com/mobazha/mobazha3.0/internal/orders/utils"
	adapters "github.com/mobazha/mobazha3.0/internal/payment/adapters"
	"github.com/mobazha/mobazha3.0/internal/repo"
	"github.com/mobazha/mobazha3.0/internal/wallet"
	pkgconfig "github.com/mobazha/mobazha3.0/pkg/config"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// MockNode builds a mock node with a temp data directory,
// in-memory database, mock libp2p host, and mock network
// service.
func MockNode() (*MobazhaNode, error) {
	r, err := repo.MockRepo()
	if err != nil {
		return nil, err
	}

	var dbIdentityKey models.Key
	err = r.DB().View(func(tx database.Tx) error {
		return tx.Read().Where("name = ?", "identity").First(&dbIdentityKey).Error
	})
	if err != nil {
		return nil, err
	}

	privKey, _, err := repo.PrivKeyAndPeerIDFromKey(dbIdentityKey.Value)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()

	mn := mocknet.New()
	h, err := mn.AddPeer(privKey, nil)
	if err != nil {
		return nil, err
	}
	mockListenAddr, _ := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/10000")
	h.Peerstore().AddAddrs(h.ID(), []ma.Multiaddr{mockListenAddr}, peerstore.PermanentAddrTTL)

	banManager := net.NewBanManager(nil, nil)
	service := net.NewNetworkService("", h, banManager, true)

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
	tracker := NewFollowerTracker(r, bus, h)

	w := wallet.NewMockWallet()
	w.SetEventBus(bus)

	mw := chains.Multiwallet{
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
		sharedManager: sharedManager,
		identityFields: identityFields{
			nodeID:   repo.DefaultNodeID,
			peerID:   h.ID(),
			privKey:  privKey,
			peerHost: h,
			nodeCtx:  ctx,
		},
		storageFields: storageFields{
			db:   r.DB(),
			repo: r,
		},
		cryptoFields: cryptoFields{
			ethMasterKey:    ethMasterKey,
			escrowMasterKey: escrowKey,
			ratingMasterKey: ratingKey,
			solPrivKey:      &mockSolPrivKey,
		},
		networkFields: networkFields{
			networkService:  service,
			eventBus:        bus,
			banManager:      banManager,
			followerTracker: tracker,
		},
		walletFields: walletFields{
			multiwallet:   &mw,
			exchangeRates: erp,
		},
		ipnsFields: ipnsFields{
			netConfig: config.DefaultNetConfig(),
		},
		lifecycleFields: lifecycleFields{
			shutdown:             make(chan struct{}),
			initialBootstrapChan: make(chan struct{}),
			publishChan:          make(chan pubCloser),
			featureManager:       pkgconfig.GetGlobalFeatureManager(),
		},
	}

	node.contentStore = &cidContentStore{}

	sharedManager.AddNode(repo.DefaultNodeID, node)

	node.messenger, err = net.NewMessenger(&net.MessengerConfig{
		Privkey: privKey,
		Service: service,
		DB:      r.DB(),
		Context: ctx,
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
		Identity:             h.ID(),
		Signer:               signer,
		Db:                   r.DB(),
		Multiwallet:          node.multiwallet,
		Messenger:            node.messenger,
		ExchangeRateProvider: erp,
		EventBus:             bus,
		CalcCIDFunc:          node.contentStore.ComputeCID,
		FeatureManager:       node.featureManager,
	})

	node.keyProvider = newFileKeyProvider(node.ethMasterKey, node.escrowMasterKey, node.ratingMasterKey, node.solPrivKey, node.tronMasterKey)

	node.initProfileService()
	node.initModerationService()
	initShippingSubsystem(node)
	seedMockShippingProfile(node)
	node.initListingService()
	node.initPaymentService()
	node.initOrderService()
	node.wireServiceSetters()
	node.initChatService()
	node.initMatrixService()
	node.initPreferencesService()
	node.initMediaService()
	node.initRatingsService()
	node.initNotificationService()
	node.initFollowService()
	node.initPostsService()
	node.initShoppingCartService()
	node.registerPaymentStrategies()
	node.paymentRegistry.Register(iwallet.ChainMock, &adapters.UTXOAutoConfirmAdapter{
		Multiwallet:    node.multiwallet,
		Keys:           node.keyProvider,
		OnAutoConfirm:  node.handleCancelablePaymentForUTXO,
		GetPaymentInfo: node.paymentService.GetUTXOPaymentInfo,
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
	p2pNet mocknet.Mocknet
	wn      *wallet.MockWalletNetwork
}

// NewMocknet returns a new MockNet with all nodes linked and connected.
// LinkAll and ConnectAllButSelf are called during construction.
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

		var dbIdentityKey models.Key
		err = r.DB().View(func(tx database.Tx) error {
			return tx.Read().Where("name = ?", "identity").First(&dbIdentityKey).Error
		})
		if err != nil {
			return nil, err
		}

		privKey, _, err := repo.PrivKeyAndPeerIDFromKey(dbIdentityKey.Value)
		if err != nil {
			return nil, err
		}

		h, err := mn.AddPeer(privKey, nil)
		if err != nil {
			return nil, err
		}
		mockAddr, _ := ma.NewMultiaddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", 10000+i))
		h.Peerstore().AddAddrs(h.ID(), []ma.Multiaddr{mockAddr}, peerstore.PermanentAddrTTL)

		banManager := net.NewBanManager(nil, nil)
		service := net.NewNetworkService("", h, banManager, true)

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

		mockSolPrivKey := solana.NewWallet().PrivateKey

		bus := events.NewBus()
		tracker := NewFollowerTracker(r, bus, h)

		w := wn.Wallets()[i]
		w.SetEventBus(bus)

		mw := chains.Multiwallet{
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
			sharedManager: sharedManager,
			identityFields: identityFields{
				nodeID:   nodeID,
				peerID:   h.ID(),
				privKey:  privKey,
				peerHost: h,
				nodeCtx:  ctx,
			},
			storageFields: storageFields{
				db:   r.DB(),
				repo: r,
			},
			cryptoFields: cryptoFields{
				ethMasterKey:    ethMasterKey,
				escrowMasterKey: escrowKey,
				ratingMasterKey: ratingKey,
				solPrivKey:      &mockSolPrivKey,
			},
			networkFields: networkFields{
				networkService:  service,
				eventBus:        bus,
				banManager:      banManager,
				followerTracker: tracker,
			},
			walletFields: walletFields{
				multiwallet:   &mw,
				exchangeRates: erp,
			},
			ipnsFields: ipnsFields{
				netConfig: config.DefaultNetConfig(),
			},
			lifecycleFields: lifecycleFields{
				shutdown:             make(chan struct{}),
				initialBootstrapChan: make(chan struct{}),
				publishChan:          make(chan pubCloser),
				featureManager:       pkgconfig.GetGlobalFeatureManager(),
			},
		}
		node.contentStore = &cidContentStore{}

		sharedManager.AddNode(nodeID, node)

		node.messenger, err = net.NewMessenger(&net.MessengerConfig{
			Privkey: privKey,
			Service: service,
			DB:      r.DB(),
			Context: ctx,
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
			Identity:             h.ID(),
			Signer:               signer,
			Db:                   r.DB(),
			Messenger:            node.messenger,
			Multiwallet:          node.multiwallet,
			ExchangeRateProvider: erp,
			EventBus:             bus,
			CalcCIDFunc:          node.contentStore.ComputeCID,
			FeatureManager:       node.featureManager,
		})

		node.keyProvider = newFileKeyProvider(node.ethMasterKey, node.escrowMasterKey, node.ratingMasterKey, node.solPrivKey, node.tronMasterKey)

		node.initProfileService()
		node.initModerationService()
		initShippingSubsystem(node)
		seedMockShippingProfile(node)
		node.initListingService()
		node.initPaymentService()
		node.initOrderService()
		node.wireServiceSetters()
		node.initChatService()
		node.initMatrixService()
		node.initPreferencesService()
		node.initMediaService()
		node.initRatingsService()
		node.initNotificationService()
		node.initFollowService()
		node.initPostsService()
		node.initShoppingCartService()
		node.registerPaymentStrategies()
		node.paymentRegistry.Register(iwallet.ChainMock, &adapters.UTXOAutoConfirmAdapter{
			Multiwallet:    node.multiwallet,
			Keys:           node.keyProvider,
			OnAutoConfirm:  node.handleCancelablePaymentForUTXO,
			GetPaymentInfo: node.paymentService.GetUTXOPaymentInfo,
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
	if err := mn.ConnectAllButSelf(); err != nil {
		return nil, err
	}

	firstPeerInfo := peer.AddrInfo{
		ID:    nodes[0].Identity(),
		Addrs: nodes[0].peerHost.Addrs(),
	}
	for _, n := range nodes[1:] {
		n.peerHost.Peerstore().AddAddrs(firstPeerInfo.ID, firstPeerInfo.Addrs, peerstore.PermanentAddrTTL)
	}

	return &Mocknet{nodes, mn, wn}, nil
}

// Nodes returns the Mobazha nodes in this network.
func (mn *Mocknet) Nodes() []*MobazhaNode {
	return mn.nodes
}

// Peers returns the peer IDs of the nodes in the network.
func (mn *Mocknet) Peers() []peer.ID {
	return mn.p2pNet.Peers()
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

// seedMockShippingProfile creates the default shipping profile that
// factory.NewPhysicalListing references (ProfileID = "factory-default-profile").
// Without this, listing tests fail with "shipping profile not found".
func seedMockShippingProfile(node *MobazhaNode) {
	if node.shippingService == nil {
		return
	}
	groups := []*models.LocationGroup{{
		ID: "default-lg",
		Zones: []*models.ShippingZone{{
			ID:      "zone-all",
			Name:    "Worldwide",
			Regions: []string{"ALL"},
			Rates: []*models.ShippingRate{{
				ID:       "rate-std",
				Name:     "Standard",
				Price:    "500",
				Currency: "USD",
			}},
		}},
	}}
	groupsJSON, _ := json.Marshal(groups)
	_ = node.shippingService.CreateProfile(context.Background(), &models.ShippingProfileEntity{
		ID:                 "factory-default-profile",
		Name:               "Default Shipping",
		IsDefault:          true,
		LocationGroupsJSON: string(groupsJSON),
	})
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
