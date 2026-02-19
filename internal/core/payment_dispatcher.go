package core

import (
	"sync"

	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// cancelableAutoConfirmInProgress tracks orders currently being auto-confirmed to prevent concurrent processing.
// Shared across all chain types (UTXO, EVM, Solana).
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
	utxoStrategy := &utxoAutoConfirmAdapter{
		multiwallet:    n.multiwallet,
		keys:           n.keyProvider,
		onAutoConfirm:  n.handleCancelablePaymentForUTXO,
		getPaymentInfo: n.paymentService.GetUTXOPaymentInfo,
	}
	for _, chain := range []iwallet.ChainType{
		iwallet.ChainBitcoin, iwallet.ChainBitcoinCash,
		iwallet.ChainLitecoin, iwallet.ChainZCash,
	} {
		n.paymentRegistry.Register(chain, utxoStrategy)
	}

	// ── EVM ─────────────────────────────────────────────────────
	evmOps := &evmChainOps{
		keys:            n.keyProvider,
		multiwallet:     n.multiwallet,
		buildReleaseTxn: n.orderService.buildDisputeReleaseTransaction,
		onAutoConfirm:   n.handleCancelablePaymentForEVM,
	}
	evmStrategy := newClientSignedAdapter(evmOps, n.paymentService.BuildInitEscrowInstructions, n.orderService.GetEscrowReleaseInstructions)
	for _, chain := range evmChains {
		n.paymentRegistry.Register(chain, evmStrategy)
	}

	// ── Solana ──────────────────────────────────────────────────
	solOps := &solanaChainOps{
		keys:            n.keyProvider,
		multiwallet:     n.multiwallet,
		buildReleaseTxn: n.orderService.buildDisputeReleaseTransaction,
		nodeID:          n.nodeID,
	}
	n.paymentRegistry.Register(iwallet.ChainSolana, newClientSignedAdapter(solOps, n.paymentService.BuildInitEscrowInstructions, n.orderService.GetEscrowReleaseInstructions))

	logger.LogInfoWithIDf(log, n.nodeID, "Registered payment strategies for %d chains", len(n.paymentRegistry.Chains()))

	// Wire the registry to App Services
	if n.paymentService != nil {
		n.paymentService.SetRegistry(n.paymentRegistry)
	}
	if n.orderService != nil {
		n.orderService.SetRegistry(n.paymentRegistry)
	}
}

