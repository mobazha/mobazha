//go:build !private_distribution

package core

import (
	"context"
	"fmt"
	"time"

	"github.com/mobazha/mobazha3.0/internal/core/guest"
	corepayment "github.com/mobazha/mobazha3.0/internal/core/payment"
	"github.com/mobazha/mobazha3.0/internal/core/settlement"
	dbgorm "github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/database/dbstore"
	"github.com/mobazha/mobazha3.0/internal/logger"
	adapters "github.com/mobazha/mobazha3.0/internal/payment/adapters"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/distribution"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/mobazha/mobazha3.0/pkg/payment"
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
// UTXO and Solana register as V2-monitored adapters; EVM chains are supplied
// only by trusted distribution modules. TRON is retired.
//
// Dependencies are injected into adapters via explicit fields / callbacks,
// not via a *MobazhaNode reference (hexagonal architecture Phase A).
func (n *MobazhaNode) registerPaymentStrategies() {
	n.paymentRegistry = payment.NewRegistry()

	// ── UTXO ────────────────────────────────────────────────────
	// Legacy V1 registration is retired. UTXO chains register as V2-native
	// (Monitored + payment-session observation); V1AsV2 forwards shared ops.
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
		n.paymentRegistry.RegisterV2(chain, payment.NewV1AsV2(utxoStrategy))
	}

	commercialModulesHealthy := true
	if err := n.registerDistributionPaymentModules(); err != nil {
		commercialModulesHealthy = false
		logger.LogErrorWithIDf(log, n.nodeID,
			"Trusted payment module registration failed; optional commercial rails remain disabled: %v", err)
	}

	// ── EVM ─────────────────────────────────────────────────────
	// Legacy V1 ClientSigned registration is retired. Managed EVM escrow is
	// registered only by a trusted distribution module.

	// ── Solana ──────────────────────────────────────────────────
	// This is the CN-3 transition fallback. A successfully registered external
	// module takes precedence. If module registration fails, optional commercial
	// rails stay disabled instead of silently falling back to bundled code.
	if commercialModulesHealthy && !n.paymentRegistry.HasChain(iwallet.ChainSolana) {
		solOps := &adapters.SolanaChainOps{
			Keys:            n.keyProvider,
			Multiwallet:     n.multiwallet,
			BuildReleaseTxn: n.orderService.BuildDisputeReleaseTransaction,
			NodeID:          n.nodeID,
		}
		solCompat := adapters.NewSolanaLifecycleCompatAdapter(solOps, n.paymentService.BuildInitEscrowInstructions, n.orderService.GetEscrowReleaseInstructions)
		solActionStore, solActionRecorder := n.newSettlementActionStore("Solana Anchor")
		if solActionStore == nil || solActionRecorder == nil {
			logger.LogErrorWithIDf(log, n.nodeID, "Solana Anchor disabled: durable settlement action store is required")
		} else {
			var solRelayer adapters.SolanaInstructionRelayer
			if n.settlementService != nil {
				solRelayer = adapters.NewSolanaInstructionRelayer(
					n.settlementService.RelaySolanaTransactionWithSigners,
					n.settlementService.SolanaRelayAuthorityAddress,
				)
			}
			n.paymentRegistry.RegisterV2(iwallet.ChainSolana, adapters.NewSolanaAnchorAdapter(adapters.SolanaAnchorAdapterDeps{
				BuildInitEscrow: n.paymentService.BuildInitEscrowInstructions,
				Compat:          solCompat,
				Relayer:         solRelayer,
				Store:           solActionStore,
				Recorder:        solActionRecorder,
				Keys:            n.keyProvider,
				Wallets:         n.multiwallet,
			}))
		}
	}

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
	if commercialModulesHealthy {
		ctx := n.nodeCtx
		if ctx == nil {
			ctx = context.Background()
		}
		n.runDistributionPaymentModules(ctx)
	}

	n.registerUTXOObservationDispatcher()
}

type distributionPaymentRegistry struct {
	registry *payment.Registry
}

func (r distributionPaymentRegistry) RegisterV2BatchExclusive(strategies map[iwallet.ChainType]payment.ChainEscrowV2) error {
	return r.registry.RegisterV2BatchExclusive(strategies)
}

func (r distributionPaymentRegistry) UnregisterV2Batch(chains []iwallet.ChainType) {
	r.registry.UnregisterV2Batch(chains)
}

func (n *MobazhaNode) registerDistributionPaymentModules() error {
	if len(n.paymentModules) == 0 {
		return nil
	}
	ctx := n.nodeCtx
	if ctx == nil {
		ctx = context.Background()
	}
	actionStore, actionRecorder := n.newDurableSettlementActionStore("Distribution payment module")
	guestSettlementSource := guest.NewManagedEscrowGuestSettlementSource(n.db)
	guestWatchSource := &distributionManagedEscrowWatchSource{node: n}
	evmRelay := n.distributionEVMRelayService()
	n.evmRelay = evmRelay
	guestRuntimeBinder := &distributionManagedEscrowGuestRuntimeBinder{node: n, source: guestSettlementSource, watchSource: guestWatchSource}
	managedEVM := distribution.ManagedEVMRuntime{
		EVMSigner:      distributionManagedEVMSigner{keys: n.keyProvider},
		EVMReaders:     distributionEVMReaderProvider{wallets: n.multiwallet},
		EVMLogs:        distributionEVMReaderProvider{wallets: n.multiwallet},
		EVMRelay:       evmRelay,
		FundingSink:    n.newDistributionFundingSink(),
		AutoConfirmer:  distributionManagedEscrowAutoConfirmer{settlement: n.settlementService},
		Actions:        actionStore,
		ActionRecorder: actionRecorder,
	}
	if n.paymentService != nil {
		managedEVM.EscrowOwners = &paymentManagedEscrowOwnerProvider{svc: n.paymentService}
	}
	guestRuntime := distribution.ManagedEscrowGuestRuntimePorts{
		WatchSource:      guestWatchSource,
		GuestSettlements: guestSettlementSource,
		GuestRuntime:     guestRuntimeBinder,
	}
	authority := distribution.NewPaymentRuntimeAuthority(managedEVM, guestRuntime)
	if err := distribution.RegisterPaymentModules(
		ctx,
		authority,
		distributionPaymentRegistry{registry: n.paymentRegistry},
		n.paymentModules...,
	); err != nil {
		return err
	}
	return nil
}

type distributionManagedEscrowAutoConfirmer struct {
	settlement *settlement.SettlementService
}

func (c distributionManagedEscrowAutoConfirmer) AutoConfirmManagedEscrow(
	ctx context.Context,
	event *events.CancelablePaymentReady,
	chain iwallet.ChainType,
) error {
	if c.settlement == nil {
		return fmt.Errorf("managed escrow auto confirmer: settlement service unavailable")
	}
	return c.settlement.AutoConfirmManagedEscrowCancelable(ctx, event, chain)
}

var _ distribution.ManagedEscrowAutoConfirmer = distributionManagedEscrowAutoConfirmer{}

func (n *MobazhaNode) runDistributionPaymentModules(ctx context.Context) {
	for _, module := range n.paymentModules {
		runner, ok := module.(distribution.PaymentModuleRunner)
		if !ok {
			continue
		}
		moduleID := module.Descriptor().ID
		go func() {
			if err := runner.Start(ctx); err != nil && ctx.Err() == nil {
				logger.LogErrorWithIDf(log, n.nodeID, "Trusted payment module %s stopped: %v", moduleID, err)
			}
			stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := runner.Stop(stopCtx); err != nil {
				logger.LogErrorWithIDf(log, n.nodeID, "Trusted payment module %s cleanup failed: %v", moduleID, err)
			}
		}()
	}
}

// registerUTXOObservationDispatcher wires address-monitored UTXO payments into
// the unified observation path used by payment-session checkout.
func (n *MobazhaNode) registerUTXOObservationDispatcher() {
	if n.paymentService != nil && n.db != nil {
		if tenantDB, ok := n.db.(*dbstore.TenantDB); ok {
			aggregator := corepayment.NewAggregatingVerifier(n.db, n.eventBus)
			if n.orderService != nil {
				aggregator.SetPaymentVerifiedHandler(n.handleCryptoPaymentVerified)
			}
			utxoDispatcher := corepayment.NewObservationDispatcher(
				NewGormPaymentObservationRepo(tenantDB, tenantDB.RawDB()),
				aggregator,
				&paymentOrderTenantResolver{db: n.db},
				n.nodeID,
			)
			n.paymentService.SetObservationDispatcher(utxoDispatcher)
		}
	}
}

func (n *MobazhaNode) newSettlementActionStore(component string) (adapters.ActionStore, adapters.ActionRecorder) {
	if n != nil && n.db != nil {
		if err := dbgorm.MigrateSettlementActionModels(n.db); err != nil {
			logger.LogErrorWithIDf(log, n.nodeID, "%s settlement actions: durable store unavailable: %v", component, err)
		} else if sqlStore := NewSettlementActionStore(n.db); sqlStore != nil {
			return sqlStore, sqlStore
		}
	}
	// Compatibility-only fallback for the bundled Solana transition path and
	// legacy tests. Trusted commercial modules must use
	// newDurableSettlementActionStore and never receive this store.
	mem := adapters.NewMemoryActionStore()
	return mem, mem
}

func (n *MobazhaNode) newDurableSettlementActionStore(component string) (adapters.ActionStore, adapters.ActionRecorder) {
	if n != nil && n.db != nil {
		if err := dbgorm.MigrateSettlementActionModels(n.db); err != nil {
			logger.LogErrorWithIDf(log, n.nodeID, "%s settlement actions: durable store unavailable: %v", component, err)
		} else if sqlStore := NewSettlementActionStore(n.db); sqlStore != nil {
			return sqlStore, sqlStore
		}
	}
	return nil, nil
}

// ── Thin delegates for strategy callbacks ────────────────────────────────
// These delegate to SettlementService for money-out operations.

func (n *MobazhaNode) handleCancelablePaymentForEVM(event *events.CancelablePaymentReady, chainType string) {
	n.settlementService.HandleCancelablePaymentForEVM(event, chainType)
}

func (n *MobazhaNode) handleCancelablePaymentForSolana(event *events.CancelablePaymentReady) {
	n.settlementService.HandleCancelablePaymentForSolana(event)
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

	if !cancelablePaymentEventTargetsNode(event.TenantID, n.nodeID, n.localTenantID()) {
		logger.LogDebugWithIDf(log, n.nodeID,
			"Skipping cancelable payment event for tenant %s order %s", event.TenantID, event.OrderID)
		return
	}

	if n.paymentRegistry == nil {
		logger.LogErrorWithIDf(log, n.nodeID, "Payment registry not initialized, cannot dispatch order %s", event.OrderID)
		return
	}

	if n.isSupplyChainManagedOrder(event.OrderID) {
		logger.LogInfoWithIDf(log, n.nodeID,
			"Skipping auto-confirm for supply-chain-managed order %s, waiting for supplier shipment", event.OrderID)
		return
	}

	strategyV2, err := n.paymentRegistry.ForCoinV2(coinType)
	if err != nil {
		logger.LogWarningWithIDf(log, n.nodeID, "No chain escrow for coin %s (order %s): %v", event.Coin, event.OrderID, err)
		return
	}
	chain, err := payment.SettlementChainForCoin(coinType)
	if err != nil {
		logger.LogWarningWithIDf(log, n.nodeID, "Unable to resolve chain for coin %s (order %s): %v", event.Coin, event.OrderID, err)
		return
	}
	if chain == iwallet.ChainSolana {
		if _, ok := strategyV2.(payment.SellerDeclineRefunder); ok {
			logger.LogInfoWithIDf(log, n.nodeID,
				"Skipping Solana Anchor auto-confirm for order %s; awaiting seller confirm or seller_decline_refund", event.OrderID)
			return
		}
	}

	go func() {
		if err := strategyV2.AutoConfirm(context.Background(), event); err != nil {
			logger.LogErrorWithIDf(log, n.nodeID, "AutoConfirm failed for order %s (coin=%s): %v", event.OrderID, event.Coin, err)
		}
	}()
}

func (n *MobazhaNode) localTenantID() string {
	type tenantIDGetter interface {
		TenantID() string
	}
	if n != nil {
		if db, ok := n.db.(tenantIDGetter); ok {
			return db.TenantID()
		}
	}
	return ""
}

func cancelablePaymentEventTargetsNode(eventTenantID, nodeID, localTenantID string) bool {
	switch eventTenantID {
	case "":
		return true
	case nodeID, localTenantID:
		return true
	default:
		return nodeID != "" && localTenantID == ""
	}
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
