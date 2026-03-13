package core

import (
	"sync"

	"github.com/mobazha/mobazha3.0/internal/logger"
	adapters "github.com/mobazha/mobazha3.0/internal/payment/adapters"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// cancelableAutoConfirmInProgress tracks orders currently being auto-confirmed to prevent concurrent processing.
// Shared across all chain types (UTXO, EVM, Solana). Keys are "nodeID:orderID" to prevent
// cross-tenant collisions in multi-tenant SaaS mode.
var cancelableAutoConfirmInProgress sync.Map

// ── Payment Strategy Registration ───────────────────────────────────────

// registerPaymentStrategies initializes the payment registry and registers
// strategies for all supported chains. Called once from MobazhaNode.Start()
// before the cancelable payment monitor begins dispatching events.
//
// All chains are registered here — the dispatcher uses registry-only lookup
// with no legacy fallback.
//
// UTXO chains use utxoAutoConfirmAdapter directly.
// EVM and Solana chains use clientSignedAdapter with chain-specific chainOps.
//
// Dependencies are injected into adapters via explicit fields / callbacks,
// not via a *MobazhaNode reference (hexagonal architecture Phase A).
func (n *MobazhaNode) registerPaymentStrategies() {
	n.paymentRegistry = payment.NewRegistry()

	// ── UTXO ────────────────────────────────────────────────────
	utxoStrategy := &adapters.UTXOAutoConfirmAdapter{
		Multiwallet:    n.multiwallet,
		Keys:           n.keyProvider,
		OnAutoConfirm:  n.handleCancelablePaymentForUTXO,
		GetPaymentInfo: n.paymentService.GetUTXOPaymentInfo,
	}
	for _, chain := range []iwallet.ChainType{
		iwallet.ChainBitcoin, iwallet.ChainBitcoinCash,
		iwallet.ChainLitecoin, iwallet.ChainZCash,
	} {
		n.paymentRegistry.Register(chain, utxoStrategy)
	}

	// ── EVM ─────────────────────────────────────────────────────
	evmOps := &adapters.EVMChainOps{
		Keys:            n.keyProvider,
		Multiwallet:     n.multiwallet,
		BuildReleaseTxn: n.orderService.buildDisputeReleaseTransaction,
		OnAutoConfirm:   n.handleCancelablePaymentForEVM,
	}
	evmStrategy := adapters.NewClientSignedAdapter(evmOps, n.paymentService.BuildInitEscrowInstructions, n.orderService.GetEscrowReleaseInstructions)
	for _, chain := range evmChains {
		n.paymentRegistry.Register(chain, evmStrategy)
	}

	// ── Solana ──────────────────────────────────────────────────
	solOps := &adapters.SolanaChainOps{
		Keys:            n.keyProvider,
		Multiwallet:     n.multiwallet,
		BuildReleaseTxn: n.orderService.buildDisputeReleaseTransaction,
		OnAutoConfirm:   n.handleCancelablePaymentForSolana,
		NodeID:          n.nodeID,
	}
	n.paymentRegistry.Register(iwallet.ChainSolana, adapters.NewClientSignedAdapter(solOps, n.paymentService.BuildInitEscrowInstructions, n.orderService.GetEscrowReleaseInstructions))

	logger.LogInfoWithIDf(log, n.nodeID, "Registered payment strategies for %d chains", len(n.paymentRegistry.Chains()))

	// Wire the registry and receipt verifier to App Services
	if n.paymentService != nil {
		n.paymentService.SetRegistry(n.paymentRegistry)
		n.paymentService.SetReceiptVerifier(adapters.NewEVMReceiptVerifier(n.multiwallet))
	}
	if n.orderService != nil {
		n.orderService.SetRegistry(n.paymentRegistry)
		n.orderService.SetReceiptVerifier(adapters.NewEVMReceiptVerifier(n.multiwallet))
	}

}

// ── Thin delegates for strategy callbacks ────────────────────────────────

func (n *MobazhaNode) handleCancelablePaymentForEVM(event *events.CancelablePaymentReady, chainType string) {
	n.paymentService.HandleCancelablePaymentForEVM(event, chainType)
}

func (n *MobazhaNode) handleCancelablePaymentForSolana(event *events.CancelablePaymentReady) {
	n.paymentService.HandleCancelablePaymentForSolana(event)
}
