package core

import (
	"context"
	"strings"

	internalutxo "github.com/mobazha/mobazha/internal/chains/utxo"
	"github.com/mobazha/mobazha/internal/logger"
	"github.com/mobazha/mobazha/pkg/events"
	"github.com/mobazha/mobazha/pkg/utxo"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
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
		var (
			monitor *utxo.Monitor
			err     error
		)
		overrides := n.electrumOverrides()
		if len(overrides) > 0 {
			monitor, err = utxo.NewMonitorWithElectrumOverrides(
				context.Background(), n.UsingWalletTestnet(), overrides,
			)
		} else {
			monitor, err = internalutxo.CreateMonitor(context.Background(), n.UsingWalletTestnet())
		}
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
	if walletAccounts, ok := n.walletAccountService.(*walletAccountService); ok {
		walletAccounts.SetChainOperations(n.monitorService)
	}

	// Wire the guest checkout subsystem to the running monitor.
	//
	//   1. UTXO monitor → guestPaymentMonitor (so WatchAddress works).
	//   2. EnableUTXOChain for every UTXO chain whose loaded wallet implements
	//      the chain-specific UTXOSweeper contract.
	if n.guestPaymentMonitor != nil {
		if mon, ok := n.monitorService.(*utxo.Monitor); ok {
			n.guestPaymentMonitor.SetUTXOMonitor(mon)
			if n.guestOrderService != nil {
				n.guestOrderService.SetUTXOMonitor(mon)
			}
		}
	}
	if n.guestOrderService != nil {
		n.guestOrderService.LogGuestUTXOReadinessSummary(context.Background(), n.nodeID)
	}
	if n.guestOrderService != nil && n.multiwallet != nil {
		for _, chain := range n.multiwallet.SupportedChains() {
			if !chain.IsUTXOChain() {
				continue
			}
			wallet, loaded := n.multiwallet.WalletForChain(chain)
			if !loaded {
				continue
			}
			if _, sweepable := wallet.(iwallet.UTXOSweeper); !sweepable {
				continue
			}
			if _, transferable := wallet.(iwallet.UTXOTransferBuilder); !transferable {
				continue
			}
			n.guestOrderService.EnableUTXOChain(chain)
		}
	}

	// Delegate startup recovery to PaymentAppService (escrow orders)
	if n.paymentService != nil {
		n.paymentService.CheckPendingPaymentsOnStartup()
	}

	// Restore guest checkout watches and wallet transfers after restart.
	if n.guestPaymentMonitor != nil {
		if err := n.guestPaymentMonitor.RestoreWatches(context.Background()); err != nil {
			logger.LogErrorWithIDf(log, n.nodeID, "restore guest payment watches: %v", err)
		}
	}
	if n.walletAccountService != nil {
		if err := n.walletAccountService.ReconcileTransfers(context.Background()); err != nil {
			logger.LogErrorWithIDf(log, n.nodeID, "restore wallet transfers: %v", err)
		}
	}
	if n.guestOrderService != nil {
		n.guestOrderService.RecoverGuestWalletAffiliateTransfers(context.Background())
	}
}

func (n *MobazhaNode) electrumOverrides() map[iwallet.ChainType]utxo.ElectrumOverride {
	if n == nil || len(n.electrumEndpoints) == 0 {
		return nil
	}
	overrides := make(map[iwallet.ChainType]utxo.ElectrumOverride, len(n.electrumEndpoints))
	for code, endpoint := range n.electrumEndpoints {
		chain := iwallet.ChainType(strings.ToUpper(strings.TrimSpace(code)))
		endpoint = strings.TrimSpace(endpoint)
		if !chain.IsUTXOChain() || endpoint == "" {
			continue
		}
		fingerprint := strings.TrimSpace(n.electrumFingerprints[strings.ToLower(strings.TrimSpace(code))])
		overrides[chain] = utxo.ElectrumOverride{
			Servers:        []string{endpoint},
			UseTLS:         fingerprint != "",
			TLSFingerprint: fingerprint,
		}
	}
	return overrides
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
