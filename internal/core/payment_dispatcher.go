//go:build !private_distribution

package core

import (
	"context"
	"errors"

	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/logger"
	adapters "github.com/mobazha/mobazha3.0/internal/payment/adapters"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	"github.com/mobazha/mobazha3.0/pkg/managedescrow"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

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
		BuildReleaseTxn: n.orderService.BuildDisputeReleaseTransaction,
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
		BuildReleaseTxn: n.orderService.BuildDisputeReleaseTransaction,
		OnAutoConfirm:   n.handleCancelablePaymentForSolana,
		NodeID:          n.nodeID,
	}
	n.paymentRegistry.Register(iwallet.ChainSolana, adapters.NewClientSignedAdapter(solOps, n.paymentService.BuildInitEscrowInstructions, n.orderService.GetEscrowReleaseInstructions))

	// ── TRON ──────────────────────────────────────────────────
	tronOps := &adapters.TRONChainOps{
		Keys:            n.keyProvider,
		Multiwallet:     n.multiwallet,
		BuildReleaseTxn: n.orderService.BuildDisputeReleaseTransaction,
		OnAutoConfirm:   n.handleCancelablePaymentForTRON,
		TronClient:      n.tronClient,
		NodeID:          n.nodeID,
	}
	n.paymentRegistry.Register(iwallet.ChainTRON, adapters.NewClientSignedAdapter(tronOps, n.paymentService.BuildInitEscrowInstructions, n.orderService.GetEscrowReleaseInstructions))

	logger.LogInfoWithIDf(log, n.nodeID, "Registered payment strategies for %d chains", len(n.paymentRegistry.Chains()))

	// Wire the registry and receipt verifier to App Services and PVS
	compositeVerifier := adapters.NewCompositeReceiptVerifier(n.multiwallet)
	if n.paymentVerificationService != nil {
		n.paymentVerificationService.SetRegistry(n.paymentRegistry)
	}
	if n.paymentService != nil {
		n.paymentService.SetRegistry(n.paymentRegistry)
		n.paymentService.SetReceiptVerifier(compositeVerifier)
	}
	if n.settlementService != nil {
		n.settlementService.SetRegistry(n.paymentRegistry)
		n.settlementService.SetReceiptVerifier(compositeVerifier)
	}
	if n.orderService != nil {
		n.orderService.SetRegistry(n.paymentRegistry)
		n.orderService.SetReceiptVerifier(compositeVerifier)
	}

	n.registerManagedEscrowAdapterShadow()
}

// registerManagedEscrowAdapterShadow constructs ManagedEscrowAdapter instances for the
// Ready EVM chains and registers them via Registry.RegisterV2 alongside
// the canonical V1 EVMChainOps entries (Phase EVM-ManagedEscrow v0.3.0 — Sprint
// 2 D17 shadow stage).
//
// Shadow registration is intentionally non-functional for live payment
// paths: V1 ForCoin lookups remain canonical and the V2 lookup path has
// no production caller today. Real Relayer / OwnerProvider /
// NonceProvider land in a follow-up commit. Until then, accidental V2
// invocations surface ErrRelayerNotConfigured (Submit/GasWallet) or
// errManagedEscrowStubNotImplemented (SetupPayment/Confirm/Cancel/Complete/
// DisputeRelease) instead of silently broadcasting; GetActionStatus is
// the only surface fully wired here, against an in-memory store.
//
// Skipped when keyProvider or multiwallet is nil (unit tests that build
// a stripped-down MobazhaNode); production builds always have both.
func (n *MobazhaNode) registerManagedEscrowAdapterShadow() {
	if n.keyProvider == nil || n.multiwallet == nil {
		logger.LogInfoWithIDf(log, n.nodeID,
			"ManagedEscrowAdapter shadow registration skipped (deps unavailable: keyProvider=%v multiwallet=%v)",
			n.keyProvider != nil, n.multiwallet != nil)
		return
	}

	store := adapters.NewMemoryActionStore()
	deps := adapters.ManagedEscrowAdapterDeps{
		Relayer:     managed_escrow.NoopRelayer(),
		Keys:        n.keyProvider,
		Multiwallet: n.multiwallet,
		ActionStore: store,
	}

	shadow := make(map[iwallet.ChainType]*adapters.ManagedEscrowAdapter, len(evmChains))
	for _, chain := range evmChains {
		adapter, err := adapters.NewManagedEscrowAdapter(chain, deps)
		if err != nil {
			// Distinguish "chain not yet promoted to Ready" (expected
			// while the matrix is phasing in) from "wiring bug" (action
			// required) so operators can triage at a glance.
			if errors.Is(err, adapters.ErrManagedEscrowChainNotReady) {
				logger.LogInfoWithIDf(log, n.nodeID,
					"ManagedEscrowAdapter shadow registration skipped for chain %s — not in Ready matrix yet (%v)", chain, err)
			} else {
				logger.LogErrorWithIDf(log, n.nodeID,
					"ManagedEscrowAdapter shadow registration FAILED for chain %s: %v", chain, err)
			}
			continue
		}
		n.paymentRegistry.RegisterV2(chain, adapter)
		shadow[chain] = adapter
	}

	n.managed_escrowActionStore = store
	n.managedEscrowAdapters = shadow
	logger.LogInfoWithIDf(log, n.nodeID,
		"ManagedEscrowAdapter shadow registered for %d EVM chains (V1 path canonical)", len(shadow))
}

// ── Thin delegates for strategy callbacks ────────────────────────────────
// These delegate to SettlementService for money-out operations.

func (n *MobazhaNode) handleCancelablePaymentForEVM(event *events.CancelablePaymentReady, chainType string) {
	n.settlementService.HandleCancelablePaymentForEVM(event, chainType)
}

func (n *MobazhaNode) handleCancelablePaymentForSolana(event *events.CancelablePaymentReady) {
	n.settlementService.HandleCancelablePaymentForSolana(event)
}

func (n *MobazhaNode) handleCancelablePaymentForTRON(event *events.CancelablePaymentReady) {
	n.settlementService.HandleCancelablePaymentForTRON(event)
}

// ── Cancelable payment event dispatching ─────────────────────────────────

// startCancelablePaymentMonitor subscribes to CancelablePaymentReady events
// synchronously (so the subscription is registered before any Emit calls),
// then spawns a goroutine to consume and dispatch events.
//
// IMPORTANT: Must be called BEFORE startUTXOPaymentMonitor to avoid a race
// where CheckPendingPaymentsOnStartup emits events before the subscriber
// is registered (basicBus.Emit silently drops events with no subscribers).
func (n *MobazhaNode) startCancelablePaymentMonitor() {
	sub, err := n.eventBus.Subscribe(&events.CancelablePaymentReady{})
	if err != nil {
		logger.LogErrorWithIDf(log, n.nodeID, "Failed to subscribe to CancelablePaymentReady events: %v", err)
		return
	}

	logger.LogInfoWithIDf(log, n.nodeID, "Cancelable payment monitor started")

	go func() {
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
	}()
}

func (n *MobazhaNode) dispatchCancelablePayment(event *events.CancelablePaymentReady) {
	coinType := iwallet.CoinType(event.Coin)

	if n.paymentRegistry == nil {
		logger.LogErrorWithIDf(log, n.nodeID, "Payment registry not initialized, cannot dispatch order %s", event.OrderID)
		return
	}

	if n.isSupplyChainManagedOrder(event.OrderID) {
		logger.LogInfoWithIDf(log, n.nodeID,
			"Skipping auto-confirm for supply-chain-managed order %s, waiting for supplier shipment", event.OrderID)
		return
	}

	strategy, err := n.paymentRegistry.ForCoin(coinType)
	if err != nil {
		logger.LogWarningWithIDf(log, n.nodeID, "No chain escrow for coin %s (order %s): %v", event.Coin, event.OrderID, err)
		return
	}

	go func() {
		if err := strategy.AutoConfirm(context.Background(), event); err != nil {
			logger.LogErrorWithIDf(log, n.nodeID, "AutoConfirm failed for order %s (coin=%s): %v", event.OrderID, event.Coin, err)
		}
	}()
}

func (n *MobazhaNode) isSupplyChainManagedOrder(orderID string) bool {
	if n.supplyChainService == nil {
		return false
	}

	var order models.Order
	err := n.repo.DB().View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID).First(&order).Error
	})
	if err != nil {
		logger.LogWarningWithIDf(log, n.nodeID,
			"SupplyChain check: cannot fetch order %s: %v", orderID, err)
		return false
	}

	return n.isOrderManagedBySupplier(&order)
}

// ── Fiat payment event dispatching ───────────────────────────────────────

// startFiatPaymentMonitor subscribes to FiatPaymentReady events and
// dispatches them to SettlementService for auto-confirmation.
func (n *MobazhaNode) startFiatPaymentMonitor() {
	if n.settlementService == nil {
		return
	}
	sub, err := n.eventBus.Subscribe(&events.FiatPaymentReady{})
	if err != nil {
		logger.LogErrorWithIDf(log, n.nodeID, "Failed to subscribe to FiatPaymentReady events: %v", err)
		return
	}

	logger.LogInfoWithIDf(log, n.nodeID, "Fiat payment monitor started")

	go func() {
		for {
			select {
			case event := <-sub.Out():
				if e, ok := event.(*events.FiatPaymentReady); ok {
					n.settlementService.HandleFiatPaymentReady(e)
				}
			case <-n.shutdown:
				sub.Close()
				logger.LogInfoWithIDf(log, n.nodeID, "Fiat payment monitor stopped")
				return
			}
		}
	}()
}

func (n *MobazhaNode) isOrderManagedBySupplier(order *models.Order) bool {
	if n.supplyChainService == nil {
		return false
	}

	oo, err := order.OrderOpenMessage()
	if err != nil || oo == nil {
		return false
	}

	var slugs []string
	for _, item := range oo.Listings {
		if item == nil || item.Listing == nil {
			continue
		}
		slug := item.Listing.GetSlug()
		if slug != "" {
			slugs = append(slugs, slug)
		}
	}
	return len(slugs) > 0 && n.supplyChainService.IsOrderAutoFulfillable(slugs)
}
