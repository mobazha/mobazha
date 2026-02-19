package core

import (
	"context"
	"fmt"
	"io"

	ipfsfiles "github.com/ipfs/boxo/files"
	ipath "github.com/ipfs/boxo/path"
	"github.com/ipfs/kubo/core/coreapi"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
)

// NodeOption configures optional dependencies on a MobazhaNode after
// construction. Used by NewNodeWithOptions to allow hosting (SaaS) to
// inject custom adapters (e.g. KeyVaultProvider) without modifying the
// core construction flow.
type NodeOption func(*MobazhaNode)

// WithKeyProvider overrides the default fileKeyProvider.
// SaaS mode uses this to inject a KeyVault-backed implementation.
func WithKeyProvider(kp contracts.KeyProvider) NodeOption {
	return func(n *MobazhaNode) {
		n.keyProvider = kp
	}
}

// WithHostService sets the HostService for SaaS integration.
// This is extracted from the variadic parameter of NewNode for cleaner API.
func WithHostService(hs coreiface.HostService) NodeOption {
	return func(n *MobazhaNode) {
		n.hostService = hs
	}
}

// applyOptions applies NodeOption functions and sets defaults for
// fields that weren't explicitly overridden.
func (n *MobazhaNode) applyOptions(opts []NodeOption) {
	for _, opt := range opts {
		opt(n)
	}
	if n.keyProvider == nil {
		n.keyProvider = newFileKeyProvider(
			n.ethMasterKey,
			n.escrowMasterKey,
			n.ratingMasterKey,
			n.solPrivKey,
		)
	}
	n.initPaymentService()
	n.initOrderService()
	n.initChatService()
	n.initMatrixService()
	n.initPreferencesService()
	n.initMediaService()
	n.initRatingsService()
	n.initNotificationService()
	n.initShoppingCartService()
}

// initMatrixService creates the MatrixAppService.
func (n *MobazhaNode) initMatrixService() {
	if n.ipfsOnlyMode {
		return
	}
	if n.privKey == nil {
		return
	}
	n.matrixService = NewMatrixAppService(MatrixAppServiceConfig{
		DB:      n.db,
		PrivKey: n.privKey,
		PeerID:  n.peerID,
	})
}

// initPreferencesService creates the PreferencesAppService.
func (n *MobazhaNode) initPreferencesService() {
	if n.ipfsOnlyMode {
		return
	}
	n.preferencesService = NewPreferencesAppService(PreferencesAppServiceConfig{
		DB:                    n.db,
		BanManager:            n.banManager,
		UpdateAllListingsFunc: n.UpdateAllListings,
		GetMyListingsFunc:     n.GetMyListings,
	})
}

// initMediaService creates the MediaAppService with IPFS infrastructure callbacks.
func (n *MobazhaNode) initMediaService() {
	if n.ipfsOnlyMode {
		return
	}

	var getIPFSFile GetIPFSFileFunc
	if n.sharedManager != nil {
		getIPFSFile = func(ctx context.Context, path ipath.Path) (io.ReadSeeker, error) {
			api, err := coreapi.NewCoreAPI(n.sharedManager.GetIPFSNode())
			if err != nil {
				return nil, err
			}
			nd, err := api.Unixfs().Get(ctx, path)
			if err != nil {
				return nil, err
			}
			f, ok := nd.(ipfsfiles.File)
			if !ok {
				return nil, fmt.Errorf("error asserting ipfs file type")
			}
			return f, nil
		}
	}

	n.mediaService = NewMediaAppService(MediaAppServiceConfig{
		DB:              n.db,
		ContentStore:    n.contentStore,
		NodeID:          n.nodeID,
		GetIPFSFile:     getIPFSFile,
		FetchIPNSRecord: n.fetchIPNSRecord,
		Publish:         n.Publish,
		PublishFile:     n.PublishFile,
	})
}

// initRatingsService creates the RatingsAppService.
func (n *MobazhaNode) initRatingsService() {
	if n.ipfsOnlyMode {
		return
	}

	var getRatingIndex GetRatingIndexFromNetDBFunc
	if n.netDB != nil {
		getRatingIndex = n.netDB.GetRatingIndex
	}

	n.ratingsService = NewRatingsAppService(RatingsAppServiceConfig{
		DB:              n.db,
		ContentStore:    n.contentStore,
		FetchIPNSRecord: n.fetchIPNSRecord,
		GetRatingIndex:  getRatingIndex,
	})
}

// initNotificationService creates the NotificationAppService.
func (n *MobazhaNode) initNotificationService() {
	if n.ipfsOnlyMode {
		return
	}
	n.notificationService = NewNotificationAppService(NotificationAppServiceConfig{
		DB: n.db,
	})
}

// initShoppingCartService creates the ShoppingCartAppService.
func (n *MobazhaNode) initShoppingCartService() {
	if n.ipfsOnlyMode {
		return
	}
	n.shoppingCartService = NewShoppingCartAppService(ShoppingCartAppServiceConfig{
		DB:       n.db,
		EventBus: n.eventBus,
		NodeID:   n.nodeID,
	})
}

// initChatService creates the ChatAppService if the necessary
// dependencies are available. IPFSOnly nodes skip this.
func (n *MobazhaNode) initChatService() {
	if n.ipfsOnlyMode {
		return
	}

	n.chatService = NewChatAppService(ChatAppServiceConfig{
		DB:             n.db,
		Messenger:      n.messenger,
		NetworkService: n.networkService,
		EventBus:       n.eventBus,
		NodeID:         n.nodeID,
		PeerID:         n.peerID,
	})
}

// initOrderService creates the OrderAppService if the necessary
// dependencies are available. IPFSOnly nodes skip this.
func (n *MobazhaNode) initOrderService() {
	if n.ipfsOnlyMode {
		return
	}

	n.orderService = NewOrderAppService(OrderAppServiceConfig{
		DB:              n.db,
		Multiwallet:     n.multiwallet,
		Signer:          n.signer,
		OrderProcessor:  n.orderProcessor,
		Messenger:       n.messenger,
		NodeID:          n.nodeID,

		GetPayoutAddr:               n.GetPayoutAddress,
		ReleaseCancelableWithParams: n.releaseFromCancelableAddressWithParams,
	})
}

// initPaymentService creates the PaymentAppService if the necessary
// dependencies are available. IPFSOnly nodes skip this.
func (n *MobazhaNode) initPaymentService() {
	if n.ipfsOnlyMode {
		return
	}

	var evmRelay EVMRelayService
	if n.hostService != nil {
		evmRelay = n.hostService.GetEVMRelayService()
	}

	var getStripeConfigFromHost GetStripeConfigFromHostFunc
	var registerStripeAccountFn RegisterStripeAccountFunc
	if n.hostService != nil {
		getStripeConfigFromHost = n.hostService.GetStripeConfig
		registerStripeAccountFn = n.hostService.RegisterStripeAccount
	}

	var getStripeAccountIDFn GetStripeAccountIDFunc
	if n.netDB != nil {
		getStripeAccountIDFn = func(peerID string) (string, error) {
			return n.netDB.GetStripeAccountID(peerID, nil)
		}
	}

	n.paymentService = NewPaymentAppService(PaymentAppServiceConfig{
		DB:          n.db,
		Multiwallet: n.multiwallet,
		EventBus:    n.eventBus,
		NodeID:      n.nodeID,
		Shutdown:    n.shutdown,

		GetProfile:              n.GetProfile,
		ConfirmOrder:            n.ConfirmOrder,
		FulfillOrder:            n.FulfillOrder,
		GetStripeConfigFromHost: getStripeConfigFromHost,
		RegisterStripeAccount:   registerStripeAccountFn,
		GetStripeAccountID:      getStripeAccountIDFn,
		StripeConfigCache:       n.stripeConfigCache,
		ReleaseCancelable:       n.releaseFromCancelableAddress,

		EVMRelayService: evmRelay,
		RelayAPIURL:     n.relayAPIURL,
	})
}
