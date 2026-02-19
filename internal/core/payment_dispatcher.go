package core

import (
	"sync"

	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/pkg/models"
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
		getPaymentInfo: n.GetUTXOPaymentInfo,
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
		buildReleaseTxn: n.buildReleaseTransaction,
		onAutoConfirm:   n.handleCancelablePaymentForEVM,
	}
	evmStrategy := newClientSignedAdapter(evmOps, n.BuildInitEscrowInstructions, n.GetEscrowReleaseInstructions)
	for _, chain := range evmChains {
		n.paymentRegistry.Register(chain, evmStrategy)
	}

	// ── Solana ──────────────────────────────────────────────────
	solOps := &solanaChainOps{
		keys:            n.keyProvider,
		multiwallet:     n.multiwallet,
		buildReleaseTxn: n.buildReleaseTransaction,
		nodeID:          n.nodeID,
	}
	n.paymentRegistry.Register(iwallet.ChainSolana, newClientSignedAdapter(solOps, n.BuildInitEscrowInstructions, n.GetEscrowReleaseInstructions))

	logger.LogInfoWithIDf(log, n.nodeID, "Registered payment strategies for %d chains", len(n.paymentRegistry.Chains()))

	// Wire the registry to App Services
	if n.paymentService != nil {
		n.paymentService.SetRegistry(n.paymentRegistry)
	}
	if n.orderService != nil {
		n.orderService.SetRegistry(n.paymentRegistry)
	}
}

// startCancelablePaymentMonitor delegates to PaymentAppService.
func (n *MobazhaNode) startCancelablePaymentMonitor() {
	if n.paymentService != nil {
		n.paymentService.StartCancelablePaymentMonitor()
		return
	}
}

// tryLockAutoConfirm delegates to PaymentAppService.
func (n *MobazhaNode) tryLockAutoConfirm(orderID string) func() {
	if n.paymentService != nil {
		return n.paymentService.TryLockAutoConfirm(orderID)
	}
	if _, loaded := cancelableAutoConfirmInProgress.LoadOrStore(orderID, true); loaded {
		return nil
	}
	return func() {
		cancelableAutoConfirmInProgress.Delete(orderID)
	}
}

// fetchOrderByID delegates to PaymentAppService.
func (n *MobazhaNode) fetchOrderByID(orderID string) (*models.Order, error) {
	if n.paymentService != nil {
		return n.paymentService.FetchOrderByID(orderID)
	}
	var order models.Order
	err := n.db.View(func(dbtx database.Tx) error {
		return dbtx.Read().Where("id = ?", orderID).First(&order).Error
	})
	if err != nil {
		return nil, err
	}
	return &order, nil
}
