package core

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/mobazha/mobazha/internal/core/guest"
	"github.com/mobazha/mobazha/internal/core/order"
	corepayment "github.com/mobazha/mobazha/internal/core/payment"
	"github.com/mobazha/mobazha/internal/core/settlement"
	dbgorm "github.com/mobazha/mobazha/internal/database"
	"github.com/mobazha/mobazha/internal/database/dbstore"
	"github.com/mobazha/mobazha/internal/logger"
	"github.com/mobazha/mobazha/internal/orderextensions"
	adapters "github.com/mobazha/mobazha/internal/payment/adapters"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/distribution"
	"github.com/mobazha/mobazha/pkg/events"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/mobazha/mobazha/pkg/payment"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

// ── Payment Strategy Registration ───────────────────────────────────────

// registerPaymentStrategies initializes the payment registry and registers
// strategies for all supported chains. Called once from MobazhaNode.Start()
// before the cancelable payment monitor begins dispatching events.
//
// All chains are registered here — the dispatcher uses registry-only lookup
// with no legacy fallback.
//
// UTXO registers as the public V2-monitored adapter. Managed EVM and Solana
// chains are supplied only by trusted distribution modules. TRON is retired.
//
// Dependencies are injected into adapters via explicit fields / callbacks,
// not via a *MobazhaNode reference (hexagonal architecture Phase A).
func (n *MobazhaNode) registerPaymentStrategies() error {
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

	if err := n.registerDistributionPaymentModules(); err != nil {
		return fmt.Errorf("register trusted payment modules: %w", err)
	}

	// ── EVM ─────────────────────────────────────────────────────
	// Legacy V1 ClientSigned registration is retired. Managed EVM escrow is
	// registered only by a trusted distribution module.

	// ── Solana ──────────────────────────────────────────────────
	// Solana has no Open Core fallback. A distribution that does not compose
	// the private module does not advertise or register the chain.

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
	ctx := n.nodeCtx
	if ctx == nil {
		ctx = context.Background()
	}
	if err := n.runDistributionPaymentModules(ctx); err != nil {
		return err
	}

	n.registerUTXOObservationDispatcher()
	return nil
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
	modules := make([]distribution.PaymentModule, 0, len(n.paymentModules)+2)
	modules = append(modules, newCoreFiatPaymentModule(), newCoreNativeUTXOPaymentModule())
	modules = append(modules, n.paymentModules...)
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
	evmSigner := distributionManagedEVMSigner{keys: n.keyProvider, settlement: n.settlementSigner}
	managedEVM := distribution.ManagedEVMRuntime{
		SettlementSigner: evmSigner,
		EVMReaders:       distributionEVMReaderProvider{wallets: n.multiwallet},
		EVMLogs:          distributionEVMReaderProvider{wallets: n.multiwallet},
		EVMRelay:         evmRelay,
		FundingSink:      n.newDistributionFundingSink(),
		AutoConfirmer:    distributionManagedEscrowAutoConfirmer{settlement: n.settlementService},
		Actions:          actionStore,
		ActionRecorder:   actionRecorder,
	}
	managedSolana := distribution.ManagedSolanaRuntime{
		Signer:         distributionManagedSolanaSigner{keys: n.keyProvider, settlement: n.settlementSigner},
		Setup:          distributionManagedSolanaSetupService{service: n.paymentService},
		Orders:         distributionManagedSolanaOrderSource{db: n.db},
		FundingSink:    n.newDistributionFundingSink(),
		Actions:        actionStore,
		ActionRecorder: actionRecorder,
	}
	if n.paymentService != nil {
		managedEVM.EscrowOwners = &paymentEscrowOwnerProvider{svc: n.paymentService}
	}
	guestRuntime := distribution.ManagedEscrowGuestRuntimePorts{
		WatchSource:      guestWatchSource,
		GuestSettlements: guestSettlementSource,
		GuestRuntime:     guestRuntimeBinder,
	}
	directObserved := distribution.DirectObservedRuntimePorts{
		Binder: &distributionDirectObservedRuntimeBinder{node: n},
	}
	authority := distribution.NewPaymentRuntimeAuthority(managedEVM, managedSolana, guestRuntime, directObserved)
	manager, err := distribution.NewTrustedPaymentModuleManager(
		authority,
		distributionPaymentRegistry{registry: n.paymentRegistry},
		modules...,
	)
	if err != nil {
		return err
	}
	if err := manager.Register(ctx); err != nil {
		return err
	}
	n.paymentModuleManager = manager
	return nil
}

// registerSovereignPaymentModules registers local-first payment modules after
// guest services exist but before the node starts. Direct-observed modules do
// not contribute escrow strategies, so an otherwise empty registry is valid.
func (n *MobazhaNode) registerSovereignPaymentModules() error {
	n.paymentRegistry = payment.NewRegistry()
	modules := make([]distribution.PaymentModule, 0, len(n.paymentModules)+1)
	modules = append(modules, newCoreNativeUTXOPaymentModule())
	modules = append(modules, n.paymentModules...)
	authority := distribution.NewPaymentRuntimeAuthority(
		distribution.ManagedEVMRuntime{},
		distribution.ManagedSolanaRuntime{},
		distribution.ManagedEscrowGuestRuntimePorts{},
		distribution.DirectObservedRuntimePorts{
			Binder: &distributionDirectObservedRuntimeBinder{node: n},
		},
	)
	manager, err := distribution.NewTrustedPaymentModuleManager(
		authority,
		distributionPaymentRegistry{registry: n.paymentRegistry},
		modules...,
	)
	if err != nil {
		return err
	}
	ctx := n.nodeCtx
	if ctx == nil {
		ctx = context.Background()
	}
	if err := manager.Register(ctx); err != nil {
		return err
	}
	n.paymentModuleManager = manager
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
	return c.settlement.AutoConfirmManagedEscrow(ctx, event, chain)
}

var _ distribution.ManagedEscrowAutoConfirmer = distributionManagedEscrowAutoConfirmer{}

func (n *MobazhaNode) runDistributionPaymentModules(ctx context.Context) error {
	if n.paymentModuleManager == nil {
		return nil
	}
	if err := n.paymentModuleManager.Start(ctx, func(health distribution.PaymentModuleHealth) {
		switch health.State {
		case distribution.PaymentModuleNeedsSetup:
			logger.LogInfoWithIDf(log, n.nodeID,
				"Trusted payment module %s is awaiting setup: %s",
				health.Descriptor.ID, health.Error)
		case distribution.PaymentModuleDegraded:
			availability := "its owned rails were disabled"
			if health.Active {
				availability = "it remains active for recovery and diagnostics"
			}
			logger.LogErrorWithIDf(log, n.nodeID,
				"Trusted payment module %s degraded; %s: %s",
				health.Descriptor.ID, availability, health.Error)
			if health.Descriptor.Activation == distribution.PaymentModuleRequired && !health.Active {
				go func() {
					if stopErr := n.Stop(true); stopErr != nil {
						logger.LogErrorWithIDf(log, n.nodeID,
							"stop node after required payment module %s failed: %v", health.Descriptor.ID, stopErr)
					}
				}()
			}
		}
	}); err != nil {
		return fmt.Errorf("start trusted payment modules: %w", err)
	}
	return nil
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

	requiresAttestation, err := n.orderRequiresAttestedSettlement(event.OrderID)
	if err != nil {
		logger.LogErrorWithIDf(log, n.nodeID,
			"Cannot establish extension settlement policy for order %s; auto-confirm denied: %v", event.OrderID, err)
		return
	}
	if requiresAttestation {
		logger.LogInfoWithIDf(log, n.nodeID,
			"Skipping auto-confirm for extension-attested order %s; awaiting validated module evidence", event.OrderID)
		return
	}

	supplyChainManaged, err := n.isSupplyChainManagedOrder(event.OrderID)
	if err != nil {
		logger.LogErrorWithIDf(log, n.nodeID,
			"Cannot establish supply-chain payment policy for order %s; auto-confirm denied: %v", event.OrderID, err)
		return
	}
	if supplyChainManaged {
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
				"Skipping managed Solana auto-confirm for order %s; awaiting seller confirm or seller_decline_refund", event.OrderID)
			return
		}
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := n.prepareSellerAffiliateAutoConfirm(ctx, event.OrderID); err != nil {
			logger.LogErrorWithIDf(log, n.nodeID,
				"AutoConfirm denied until affiliate settlement facts are ready for order %s: %v", event.OrderID, err)
			return
		}
		if err := strategyV2.AutoConfirm(context.Background(), event); err != nil {
			logger.LogErrorWithIDf(log, n.nodeID, "AutoConfirm failed for order %s (coin=%s): %v", event.OrderID, event.Coin, err)
		}
	}()
}

func (n *MobazhaNode) prepareSellerAffiliateAutoConfirm(ctx context.Context, orderID string) error {
	if n == nil || n.orderService == nil {
		return nil
	}
	for {
		err := n.orderService.PrepareSellerAffiliateSettlement(ctx, models.OrderID(orderID))
		if !errors.Is(err, order.ErrSellerAffiliateSettlementNotReady) {
			return err
		}
		timer := time.NewTimer(100 * time.Millisecond)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return fmt.Errorf("%w: %v", order.ErrSellerAffiliateSettlementNotReady, ctx.Err())
		case <-timer.C:
		}
	}
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

func (n *MobazhaNode) orderRequiresAttestedSettlement(orderID string) (bool, error) {
	if n == nil || n.db == nil {
		return false, fmt.Errorf("order database is unavailable")
	}
	var order models.Order
	var requiresAttestation bool
	if err := n.db.View(func(tx database.Tx) error {
		if err := tx.Read().Where("id = ?", orderID).First(&order).Error; err != nil {
			return err
		}
		var err error
		requiresAttestation, err = orderextensions.RequiresAttestedSettlementTx(tx, orderID)
		return err
	}); err != nil {
		return false, fmt.Errorf("load order settlement policy: %w", err)
	}
	if _, err := order.OrderOpenMessage(); err != nil {
		return false, fmt.Errorf("decode order open: %w", err)
	}
	return requiresAttestation, nil
}

func (n *MobazhaNode) isSupplyChainManagedOrder(orderID string) (bool, error) {
	if n.supplyChainService == nil {
		return false, nil
	}
	if n.repo == nil || n.repo.DB() == nil {
		return false, fmt.Errorf("order repository is unavailable")
	}

	var order models.Order
	err := n.repo.DB().View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID).First(&order).Error
	})
	if err != nil {
		return false, fmt.Errorf("load order: %w", err)
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

func (n *MobazhaNode) isOrderManagedBySupplier(order *models.Order) (bool, error) {
	if n.supplyChainService == nil {
		return false, nil
	}
	if order == nil {
		return false, fmt.Errorf("order is required")
	}

	oo, err := order.OrderOpenMessage()
	if err != nil {
		return false, fmt.Errorf("decode order open: %w", err)
	}
	if oo == nil {
		return false, fmt.Errorf("order open is unavailable")
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
	return len(slugs) > 0 && n.supplyChainService.IsOrderAutoFulfillable(slugs), nil
}
