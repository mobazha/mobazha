package core

import (
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

	n.paymentService = NewPaymentAppService(PaymentAppServiceConfig{
		DB:          n.db,
		Multiwallet: n.multiwallet,
		EventBus:    n.eventBus,
		NodeID:      n.nodeID,
		Shutdown:    n.shutdown,

		GetProfile:        n.GetProfile,
		GetPayoutAddr:     n.GetPayoutAddress,
		ConfirmOrder:      n.ConfirmOrder,
		ReleaseCancelable: n.releaseFromCancelableAddress,

		EVMRelayService: evmRelay,
		RelayAPIURL:     n.relayAPIURL,
	})
}
