//go:build !private_distribution

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
	"github.com/mobazha/mobazha3.0/internal/chains"
	"github.com/mobazha/mobazha3.0/internal/config"
	"github.com/mobazha/mobazha3.0/internal/net"
	"github.com/mobazha/mobazha3.0/internal/orders"
	"github.com/mobazha/mobazha3.0/internal/orders/utils"
	adapters "github.com/mobazha/mobazha3.0/internal/payment/adapters"
	"github.com/mobazha/mobazha3.0/internal/repo"
	"github.com/mobazha/mobazha3.0/internal/wallet"
	pkgconfig "github.com/mobazha/mobazha3.0/pkg/config"
	pkgcontracts "github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	postsPb "github.com/mobazha/mobazha3.0/pkg/posts/pb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	ma "github.com/multiformats/go-multiaddr"
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
	signer, err := pkgcontracts.NewKeyPairSignerFromMarshaledKey(dbIdentityKey.Value)
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
	initStorePolicySubsystem(node)
	initShippingSubsystem(node)
	seedMockShippingProfile(node)
	node.initListingService()
	node.initPaymentService()
	node.initSettlementService()
	node.initPaymentVerificationService()
	node.initOrderService()
	node.wireServiceSetters()
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
	nodes  []*MobazhaNode
	p2pNet mocknet.Mocknet
	wn     *wallet.MockWalletNetwork
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
		signer, err := pkgcontracts.NewKeyPairSignerFromMarshaledKey(dbIdentityKey.Value)
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
		initStorePolicySubsystem(node)
		initShippingSubsystem(node)
		seedMockShippingProfile(node)
		node.initListingService()
		node.initPaymentService()
		node.initSettlementService()
		node.initPaymentVerificationService()
		node.initOrderService()
		node.wireServiceSetters()
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

	// Wire coTenantPublicData so mocknet nodes can resolve each other's
	// public data (profiles, ratings, listings, etc.). This replaces the
	// IPFS content-exchange path that was retired in Phase Media.
	peerToNode := make(map[peer.ID]*MobazhaNode, len(nodes))
	for _, n := range nodes {
		peerToNode[n.Identity()] = n
	}
	allPeerIDs := make([]peer.ID, 0, len(nodes))
	for _, n := range nodes {
		allPeerIDs = append(allPeerIDs, n.Identity())
	}

	for _, n := range nodes {
		localPeerID := n.Identity()
		n.SetCoTenantPublicData(func(targetPeer peer.ID) (database.PublicData, error) {
			if targetPeer == localPeerID {
				return nil, fmt.Errorf("co-tenant: self")
			}
			target, ok := peerToNode[targetPeer]
			if !ok {
				return nil, fmt.Errorf("co-tenant: unknown peer %s", targetPeer)
			}
			return &dbViewPublicData{db: target.repo.DB()}, nil
		})

		if n.listingService != nil {
			localID := localPeerID
			n.listingService.SetCoTenantAllPeers(func() []peer.ID {
				others := make([]peer.ID, 0, len(allPeerIDs)-1)
				for _, pid := range allPeerIDs {
					if pid != localID {
						others = append(others, pid)
					}
				}
				return others
			})
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

// dbViewPublicData adapts a database.Database into a database.PublicData for
// the mocknet's coTenantPublicData wiring. Read methods open a View tx;
// write methods are stubs (coTenantPublicData callers only read).
type dbViewPublicData struct{ db database.Database }

var _ database.PublicData = (*dbViewPublicData)(nil)

func (d *dbViewPublicData) GetProfile() (*models.Profile, error) {
	var r *models.Profile
	err := d.db.View(func(tx database.Tx) error { var e error; r, e = tx.GetProfile(); return e })
	return r, err
}
func (d *dbViewPublicData) GetFollowers() (models.Followers, error) {
	var r models.Followers
	err := d.db.View(func(tx database.Tx) error { var e error; r, e = tx.GetFollowers(); return e })
	return r, err
}
func (d *dbViewPublicData) GetFollowing() (models.Following, error) {
	var r models.Following
	err := d.db.View(func(tx database.Tx) error { var e error; r, e = tx.GetFollowing(); return e })
	return r, err
}
func (d *dbViewPublicData) GetListingIndex() (models.ListingIndex, error) {
	var r models.ListingIndex
	err := d.db.View(func(tx database.Tx) error { var e error; r, e = tx.GetListingIndex(); return e })
	return r, err
}
func (d *dbViewPublicData) GetListing(slug string) (*pb.SignedListing, error) {
	var r *pb.SignedListing
	err := d.db.View(func(tx database.Tx) error { var e error; r, e = tx.GetListing(slug); return e })
	return r, err
}
func (d *dbViewPublicData) GetEncryptedListing(slug string) ([]byte, error) {
	var r []byte
	err := d.db.View(func(tx database.Tx) error { var e error; r, e = tx.GetEncryptedListing(slug); return e })
	return r, err
}
func (d *dbViewPublicData) GetRatingIndex() (models.RatingIndex, error) {
	var r models.RatingIndex
	err := d.db.View(func(tx database.Tx) error { var e error; r, e = tx.GetRatingIndex(); return e })
	return r, err
}
func (d *dbViewPublicData) GetPostIndex() ([]models.PostData, error) {
	var r []models.PostData
	err := d.db.View(func(tx database.Tx) error { var e error; r, e = tx.GetPostIndex(); return e })
	return r, err
}
func (d *dbViewPublicData) GetPost(slug string) (*postsPb.SignedPost, error) {
	var r *postsPb.SignedPost
	err := d.db.View(func(tx database.Tx) error { var e error; r, e = tx.GetPost(slug); return e })
	return r, err
}
func (d *dbViewPublicData) PostExist(slug string) bool {
	var exists bool
	_ = d.db.View(func(tx database.Tx) error { exists = tx.PostExist(slug); return nil })
	return exists
}
func (d *dbViewPublicData) GetImageByName(size models.ImageSize, name string) ([]byte, error) {
	var r []byte
	err := d.db.View(func(tx database.Tx) error { var e error; r, e = tx.GetImageByName(size, name); return e })
	return r, err
}
func (d *dbViewPublicData) GetMediaByCID(cidHash string) ([]byte, string, error) {
	var data []byte
	var ct string
	err := d.db.View(func(tx database.Tx) error { var e error; data, ct, e = tx.GetMediaByCID(cidHash); return e })
	return data, ct, err
}

// Write stubs — coTenantPublicData callers only read.
func (d *dbViewPublicData) SetProfile(*models.Profile) error          { return nil }
func (d *dbViewPublicData) SetFollowers(models.Followers) error       { return nil }
func (d *dbViewPublicData) SetFollowing(models.Following) error       { return nil }
func (d *dbViewPublicData) SetListingIndex(models.ListingIndex) error { return nil }
func (d *dbViewPublicData) SetListing(*pb.SignedListing) error        { return nil }
func (d *dbViewPublicData) SetEncryptedListing(string, []byte) error  { return nil }
func (d *dbViewPublicData) DeleteListing(string) error                { return nil }
func (d *dbViewPublicData) SetRatingIndex(models.RatingIndex) error   { return nil }
func (d *dbViewPublicData) SetRating(*pb.Rating) error                { return nil }
func (d *dbViewPublicData) SetPostIndex([]models.PostData) error      { return nil }
func (d *dbViewPublicData) AddPost(*postsPb.SignedPost) error         { return nil }
func (d *dbViewPublicData) DeletePost(string) error                   { return nil }
func (d *dbViewPublicData) SetImage(models.Image) error               { return nil }
func (d *dbViewPublicData) SetUploadedFile(models.UploadedFile) error { return nil }
func (d *dbViewPublicData) SetIntroVideo(models.IntroVideo) error     { return nil }
func (d *dbViewPublicData) IndexMediaCID(string, string, string, string, string) error {
	return nil
}
