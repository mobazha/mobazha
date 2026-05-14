//go:build !private_distribution

package core

import (
	"context"

	internalutxo "github.com/mobazha/mobazha3.0/internal/chains/utxo"
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/utxo"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// ── Infrastructure lifecycle (MobazhaNode owns these) ───────────────────

// startUTXOPaymentMonitor starts the UTXO payment monitoring service.
// Two modes: shared (hosting) or standalone (dedicated Electrum/Mempool connections).
// Business logic is delegated to PaymentAppService.
func (n *MobazhaNode) startUTXOPaymentMonitor() {
	if n.hostService != nil {
		if sharedMonitor := n.hostService.GetUTXOMonitor(); sharedMonitor != nil {
			n.monitorService = sharedMonitor
			logger.LogInfoWithIDf(log, n.nodeID, "Using shared UTXO monitor from HostService")
		}
	}

	if n.monitorService == nil {
		monitor, err := internalutxo.CreateMonitor(context.Background(), n.UsingWalletTestnet())
		if err != nil {
			logger.LogErrorWithIDf(log, n.nodeID, "Failed to create UTXO monitor: %v", err)
			return
		}
		n.monitorService = monitor
		n.monitorService.Start()
		logger.LogInfoWithIDf(log, n.nodeID, "Created standalone UTXO monitor")
	}

	// Inject monitor into PaymentAppService and SettlementService
	if n.paymentService != nil {
		n.paymentService.SetMonitorService(n.monitorService)
	}
	if n.settlementService != nil {
		n.settlementService.SetMonitorService(n.monitorService)
	}

	// Register callback — routes to PaymentAppService
	callback := n.handleUTXOPayment
	if err := n.monitorService.RegisterNodeCallback(n.nodeID, callback); err != nil {
		logger.LogErrorWithIDf(log, n.nodeID, "Failed to register node callback: %v", err)
		return
	}

	n.configureUTXOWallets(n.monitorService)

	// Wire the guest checkout subsystem to the running monitor.
	//
	//   1. UTXO monitor → guestPaymentMonitor (so WatchAddress works).
	//   2. ChainOperations (monitor) → autoSweepService (confirmation polling).
	//   3. Wire UTXOSweepBroadcaster for sweepable chains.
	//   4. EnableUTXOChain for every UTXO chain whose wallet is loaded and
	//      whose sweeper is supported (P2WPKH today).
	if n.guestPaymentMonitor != nil {
		if mon, ok := n.monitorService.(*utxo.Monitor); ok {
			n.guestPaymentMonitor.SetUTXOMonitor(mon)
		}
	}
	if n.autoSweepService != nil {
		n.autoSweepService.SetChainOps(n.monitorService)
		n.autoSweepService.SetMultiwallet(n.multiwallet)
	}
	if n.guestOrderService != nil && n.multiwallet != nil {
		for _, chain := range n.multiwallet.SupportedChains() {
			if !chain.IsUTXOChain() || !isSweepableP2WPKHChain(chain) {
				continue
			}
			n.guestOrderService.EnableUTXOChain(chain)
		}
	}

	// Delegate startup recovery to PaymentAppService (escrow orders)
	if n.paymentService != nil {
		n.paymentService.CheckPendingPaymentsOnStartup()
	}

	// Restore guest checkout watches and pending sweeps after restart.
	if n.guestPaymentMonitor != nil {
		if err := n.guestPaymentMonitor.RestoreWatches(context.Background()); err != nil {
			logger.LogErrorWithIDf(log, n.nodeID, "restore guest payment watches: %v", err)
		}
	}
	if n.autoSweepService != nil {
		n.autoSweepService.RestorePendingSweeps(context.Background())
	}
}

// StopUTXOPaymentMonitor stops the UTXO payment monitor.
func (n *MobazhaNode) StopUTXOPaymentMonitor() {
	if n.monitorService != nil {
		n.monitorService.UnregisterNode(n.nodeID)
		n.monitorService.Stop()
		n.monitorService = nil
		logger.LogInfoWithIDf(log, n.nodeID, "Stopped UTXO monitor")
	}
	if n.paymentService != nil {
		n.paymentService.SetMonitorService(nil)
	}
	if n.settlementService != nil {
		n.settlementService.SetMonitorService(nil)
	}
}

// SetUTXOMonitor sets a custom UTXO monitor (primarily for testing).
func (n *MobazhaNode) SetUTXOMonitor(monitor *utxo.Monitor) {
	n.monitorService = monitor
	if n.paymentService != nil {
		n.paymentService.SetMonitorService(monitor)
	}
	if n.settlementService != nil {
		n.settlementService.SetMonitorService(monitor)
	}
}

// GetMonitorService returns the monitor service (primarily for testing).
func (n *MobazhaNode) GetMonitorService() utxo.UTXOMonitorService {
	return n.monitorService
}

// ── Thin delegates to PaymentAppService ─────────────────────────────────

func (n *MobazhaNode) handleCancelablePaymentForUTXO(event *events.CancelablePaymentReady) {
	n.settlementService.HandleCancelablePaymentForUTXO(event)
}

func (n *MobazhaNode) handleUTXOPayment(tx iwallet.Transaction, wa *utxo.WatchedAddress) {
	n.paymentService.HandleUTXOPayment(tx, wa)
}

func (n *MobazhaNode) WatchPaymentAddress(orderID string, address string, chainType iwallet.ChainType, scriptPubKey []byte) error {
	return n.paymentService.WatchPaymentAddress(orderID, address, chainType, scriptPubKey)
}

func (n *MobazhaNode) StopWatchingPayment(orderID string) error {
	return n.paymentService.StopWatchingPayment(orderID)
}

// ── Payment verification & event monitors ───────────────────────────────

func (n *MobazhaNode) verifyPendingPayments() {
	if n.paymentService != nil {
		n.paymentService.VerifyPendingPayments()
	}
}

// startPaymentEventMonitors starts all event-driven monitors for payment→order decoupling.
// OrderAppService subscribes to payment events (auto-confirm, UTXO detection, RWA completion).
func (n *MobazhaNode) startPaymentEventMonitors() {
	if n.orderService != nil {
		n.orderService.StartPaymentEventMonitor()
	}
}
