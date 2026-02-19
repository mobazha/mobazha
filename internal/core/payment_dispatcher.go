package core

import (
	"context"
	"sync"

	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// cancelableAutoConfirmInProgress tracks orders currently being auto-confirmed to prevent concurrent processing
// This is shared across all chain types (UTXO, EVM, Solana)
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
		escrowKey:      n.escrowMasterKey,
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
		ethKey:          n.ethMasterKey,
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
		solKey:          n.solPrivKey,
		multiwallet:     n.multiwallet,
		buildReleaseTxn: n.buildReleaseTransaction,
		nodeID:          n.nodeID,
	}
	n.paymentRegistry.Register(iwallet.ChainSolana, newClientSignedAdapter(solOps, n.BuildInitEscrowInstructions, n.GetEscrowReleaseInstructions))

	logger.LogInfoWithIDf(log, n.nodeID, "Registered payment strategies for %d chains", len(n.paymentRegistry.Chains()))
}

// ── Cancelable Payment Monitor ──────────────────────────────────────────

// startCancelablePaymentMonitor starts the unified cancelable payment monitor
// This subscribes to CancelablePaymentReady events and dispatches to chain-specific handlers
func (n *MobazhaNode) startCancelablePaymentMonitor() {
	go n.subscribeCancelablePayments()
}

// subscribeCancelablePayments subscribes to CancelablePaymentReady events
// and dispatches to the appropriate handler based on chain type
func (n *MobazhaNode) subscribeCancelablePayments() {
	sub, err := n.eventBus.Subscribe(&events.CancelablePaymentReady{})
	if err != nil {
		logger.LogErrorWithIDf(log, n.nodeID, "Failed to subscribe to CancelablePaymentReady events: %v", err)
		return
	}

	logger.LogInfoWithIDf(log, n.nodeID, "Cancelable payment monitor started")

	for {
		select {
		case event := <-sub.Out():
			if e, ok := event.(*events.CancelablePaymentReady); ok {
				n.dispatchCancelablePayment(e)
			}
		case <-n.shutdown:
			sub.Close()
			logger.LogInfoWithIDf(log, n.nodeID, "Cancelable payment monitor stopped")
			return
		}
	}
}

// dispatchCancelablePayment dispatches CancelablePaymentReady event to the
// appropriate chain strategy via the payment registry.
//
// All supported chains (UTXO, EVM, Solana) are registered in the registry.
// Unknown coins that fail registry lookup are logged and dropped.
func (n *MobazhaNode) dispatchCancelablePayment(event *events.CancelablePaymentReady) {
	coinType := iwallet.CoinType(event.Coin)

	if n.paymentRegistry == nil {
		logger.LogErrorWithIDf(log, n.nodeID, "Payment registry not initialized, cannot dispatch order %s", event.OrderID)
		return
	}

	strategy, err := n.paymentRegistry.ForCoin(coinType)
	if err != nil {
		logger.LogWarningWithIDf(log, n.nodeID, "No payment strategy for coin %s (order %s): %v", event.Coin, event.OrderID, err)
		return
	}

	go func() {
		if err := strategy.AutoConfirm(context.Background(), event); err != nil {
			logger.LogErrorWithIDf(log, n.nodeID, "AutoConfirm failed for order %s (coin=%s): %v", event.OrderID, event.Coin, err)
		}
	}()
}

// ── Shared Helpers ──────────────────────────────────────────────────────

// tryLockAutoConfirm attempts to acquire a lock for auto-confirming an order
// Returns an unlock function if successful, or nil if the order is already being processed
// This prevents concurrent processing of the same order across all chain types
func (n *MobazhaNode) tryLockAutoConfirm(orderID string) func() {
	if _, loaded := cancelableAutoConfirmInProgress.LoadOrStore(orderID, true); loaded {
		logger.LogInfoWithIDf(log, n.nodeID, "Order %s auto-confirm already in progress, skipping", orderID)
		return nil
	}
	return func() {
		cancelableAutoConfirmInProgress.Delete(orderID)
	}
}

// fetchOrderByID fetches an order by ID without marking it as read
// Use this for internal/system processes that shouldn't affect the user's "unread" status
// For user-facing operations, use GetOrder() which marks the order as read
func (n *MobazhaNode) fetchOrderByID(orderID string) (*models.Order, error) {
	var order models.Order
	err := n.db.View(func(dbtx database.Tx) error {
		return dbtx.Read().Where("id = ?", orderID).First(&order).Error
	})
	if err != nil {
		return nil, err
	}
	return &order, nil
}
