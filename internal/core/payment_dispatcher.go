//go:build !private_distribution

package core

import (
	"context"
	"errors"
	"fmt"
	"strings"

	corepayment "github.com/mobazha/mobazha3.0/internal/core/payment"
	dbgorm "github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/database/dbstore"
	"github.com/mobazha/mobazha3.0/internal/logger"
	adapters "github.com/mobazha/mobazha3.0/internal/payment/adapters"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	"github.com/mobazha/mobazha3.0/pkg/relay"
	"github.com/mobazha/mobazha3.0/pkg/managedescrow"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"gorm.io/gorm"
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
	// ManagedEscrowEnabled chains are activated via the V2 RegisterV2 path in
	// registerManagedEscrowAdapterShadow; they are intentionally excluded from
	// the V1 Register loop so that ForCoin(managed_escrowEnabledChain) returns
	// an error — callers must migrate to ForCoinV2 to use those chains.
	// This makes the migration explicit and prevents silent fallback to
	// the legacy ClientSigned path on ManagedEscrow-activated chains.
	evmOps := &adapters.EVMChainOps{
		Keys:            n.keyProvider,
		Multiwallet:     n.multiwallet,
		BuildReleaseTxn: n.orderService.BuildDisputeReleaseTransaction,
		OnAutoConfirm:   n.handleCancelablePaymentForEVM,
	}
	evmStrategy := adapters.NewClientSignedAdapter(evmOps, n.paymentService.BuildInitEscrowInstructions, n.orderService.GetEscrowReleaseInstructions)
	for _, chain := range managed_escrow.LegacyChains(n.managed_escrowCapConfig) {
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

// registerManagedEscrowAdapterShadow constructs ManagedEscrowAdapter instances and
// activates them as the canonical V2 escrow path for chains listed in
// n.managed_escrowCapConfig.ManagedEscrowChains (Phase EVM-ManagedEscrow v0.3.0 — Sprint 3
// grayscale routing).
//
// Routing semantics (EVM_HYBRID_STRATEGY.md §5.5 D-Hybrid-23):
//   - ManagedEscrow-enabled chains: ForCoinV2 returns ManagedEscrowAdapter; ForCoin (V1)
//     has no entry for these chains (they were skipped in
//     registerPaymentStrategies). Callers MUST use ForCoinV2.
//   - Legacy chains (not in ManagedEscrowChains): ForCoin (V1) returns
//     ClientSignedAdapter; ForCoinV2 is empty for them.
//
// Provider wiring lands in incremental commits:
//
//   - D18a — OwnerProvider (paymentManagedEscrowOwnerProvider) so SetupPayment
//     can predict ManagedEscrow addresses end-to-end. Wired when paymentService
//     is non-nil.
//   - D18b — NonceProvider (paymentManagedEscrowNonceProvider) so Confirm /
//     Cancel / Complete / DisputeRelease can build deterministic
//     execTransaction envelopes against the live ManagedEscrow nonce. Wired
//     unconditionally because multiwallet is already required for
//     shadow registration to begin.
//   - D18c — Relayer. Hosted/SaaS: HostService.GetEVMRelayService() via
//     adapters.NewRelayBridgeWithRecorder. Standalone: RelayAPIURL + Bearer from
//     RelayAPIBearer, else pkg/relay.EnvPlatformRelayToken (same as Settlement HTTP).
//
// Test-only paths that build a stripped MobazhaNode (no paymentService)
// continue to leave OwnerProvider nil, so SetupPayment short-circuits
// with errManagedEscrowStubNotImplemented as documented. NonceProvider is
// wired in those tests too — a missing OwnerProvider trips the action
// path before the nonce read is reached.
//
// Skipped entirely when keyProvider or multiwallet is nil (unit tests
// that build a stripped-down MobazhaNode); production builds always
// have both.
func (n *MobazhaNode) registerManagedEscrowAdapterShadow() {
	if n.keyProvider == nil || n.multiwallet == nil {
		logger.LogInfoWithIDf(log, n.nodeID,
			"ManagedEscrowAdapter shadow registration skipped (deps unavailable: keyProvider=%v multiwallet=%v)",
			n.keyProvider != nil, n.multiwallet != nil)
		return
	}

	var store adapters.ActionStore
	var recorder adapters.ActionRecorder

	if n.db != nil {
		if err := dbgorm.MigrateManagedEscrowRelayActionModels(n.db); err != nil {
			logger.LogErrorWithIDf(log, n.nodeID, "ManagedEscrow relay actions: migrate failed (non-fatal): %v", err)
		}
		if sqlStore := NewManagedEscrowRelayActionStore(n.db); sqlStore != nil {
			store = sqlStore
			recorder = sqlStore
		}
	}
	if store == nil {
		mem := adapters.NewMemoryActionStore()
		store = mem
		recorder = mem
	}

	activeChainsEarly := managed_escrow.ManagedEscrowEnabledChains(n.managed_escrowCapConfig)
	if len(activeChainsEarly) > 0 && n.db == nil {
		logger.LogWarningWithIDf(log, n.nodeID,
			"ManagedEscrowAdapter V2: database unavailable — skipping ManagedEscrow-enabled chains (on-chain observation requires persistence)")
	}

	var relaySvc relay.EVMRelayService
	if n.hostService != nil {
		relaySvc = n.hostService.GetEVMRelayService()
	} else if u := strings.TrimSpace(n.relayAPIURL); u != "" {
		tok := relay.BearerFromConfigOrEnv(n.relayAPIBearer)
		relaySvc = relay.NewHTTPPlatformRelay(u, tok)
		logger.LogInfoWithIDf(log, n.nodeID, "ManagedEscrow relay: using HTTP platform relay (RelayAPIURL)")
	}

	var relayer managed_escrow.Relayer = managed_escrow.NoopRelayer()
	if relaySvc != nil {
		relayer = adapters.NewRelayBridgeWithRecorder(relaySvc, recorder)
	}

	deps := adapters.ManagedEscrowAdapterDeps{
		Relayer:       relayer,
		Keys:          n.keyProvider,
		Multiwallet:   n.multiwallet,
		ActionStore:   store,
		OwnerSigner:   &paymentManagedEscrowOwnerSigner{keys: n.keyProvider},
		NonceProvider: &paymentManagedEscrowNonceProvider{multiwallet: n.multiwallet},
		WalletTestnet: n.walletTestnet,
	}
	if n.paymentService != nil {
		deps.OwnerProvider = &paymentManagedEscrowOwnerProvider{svc: n.paymentService}
	} else {
		// Test-only path: nodeWithManagedEscrowShadowDeps builds a stripped
		// MobazhaNode without paymentService. Leaving OwnerProvider
		// nil keeps SetupPayment short-circuiting to the documented
		// errManagedEscrowStubNotImplemented path.
		logger.LogInfoWithIDf(log, n.nodeID,
			"ManagedEscrowAdapter shadow registration: paymentService unavailable; OwnerProvider left nil (SetupPayment will stub)")
	}

	shadow := make(map[iwallet.ChainType]*adapters.ManagedEscrowAdapter, len(activeChainsEarly))
	monitors := make(map[iwallet.ChainType]*managed_escrow.LiveMonitor, len(activeChainsEarly))
	for _, chain := range activeChainsEarly {
		if n.db == nil {
			continue
		}
		monitor, err := n.buildManagedEscrowMonitor(chain)
		if err != nil {
			logger.LogErrorWithIDf(log, n.nodeID,
				"ManagedEscrowAdapter V2 activation FAILED for chain %s: monitor wiring: %v", chain, err)
			continue
		}
		deps.Monitor = monitor
		adapter, err := adapters.NewManagedEscrowAdapter(chain, deps)
		if err != nil {
			// Distinguish "chain not yet promoted to Ready" (expected
			// while the matrix is phasing in) from "wiring bug" (action
			// required) so operators can triage at a glance.
			if errors.Is(err, adapters.ErrManagedEscrowChainNotReady) {
				logger.LogInfoWithIDf(log, n.nodeID,
					"ManagedEscrowAdapter V2 activation skipped for chain %s — not in Ready matrix yet (%v)", chain, err)
			} else {
				logger.LogErrorWithIDf(log, n.nodeID,
					"ManagedEscrowAdapter V2 activation FAILED for chain %s: %v", chain, err)
			}
			continue
		}

		n.paymentRegistry.RegisterV2(chain, adapter)
		shadow[chain] = adapter
		monitors[chain] = monitor
	}

	n.managed_escrowActionStore = store
	n.managedEscrowAdapters = shadow
	n.managed_escrowMonitors = monitors

	// Publish grayscale routing decisions as startup metrics. This provides
	// an audit trail in Grafana for "which chains were live on V2 at node start".
	for chain, adapter := range shadow {
		managed_escrow.SetManagedEscrowRoutingDecision(string(chain), adapter != nil)
	}
	// Chains on V1 legacy path also get a routing=0 gauge for completeness.
	for _, chain := range managed_escrow.LegacyChains(n.managed_escrowCapConfig) {
		managed_escrow.SetManagedEscrowRoutingDecision(string(chain), false)
	}

	if len(activeChainsEarly) == 0 {
		logger.LogInfoWithIDf(log, n.nodeID,
			"ManagedEscrowAdapter: no chains activated (managed_escrow_chains config empty — all EVM on V1 legacy path)")
	} else {
		logger.LogInfoWithIDf(log, n.nodeID,
			"ManagedEscrowAdapter V2 activated for %d/%d EVM chains: %v", len(shadow), len(activeChainsEarly), shadow)
	}
}

func (n *MobazhaNode) buildManagedEscrowMonitor(chain iwallet.ChainType) (*managed_escrow.LiveMonitor, error) {
	if n.db == nil {
		return nil, errors.New("database unavailable")
	}
	tenantDB, ok := n.db.(*dbstore.TenantDB)
	if !ok {
		return nil, fmt.Errorf("unsupported database type %T", n.db)
	}
	if n.eventBus == nil {
		return nil, errors.New("event bus unavailable")
	}

	wallet, ok := n.multiwallet.WalletForChain(chain)
	if !ok {
		return nil, fmt.Errorf("wallet unavailable for chain %s", chain)
	}
	getter, ok := wallet.(interface{ GetChainClient() iwallet.ChainClient })
	if !ok {
		return nil, fmt.Errorf("wallet %T does not expose chain client", wallet)
	}
	client := getter.GetChainClient()
	if client == nil {
		return nil, fmt.Errorf("chain client not configured for %s", chain)
	}
	logClient, ok := client.(managed_escrow.LogSubscriber)
	if !ok {
		return nil, fmt.Errorf("chain client %T does not implement managed_escrow.LogSubscriber", client)
	}

	dispatcher := corepayment.NewObservationDispatcher(
		NewGormPaymentObservationRepo(tenantDB, tenantDB.RawDB()),
		corepayment.NewAggregatingVerifier(n.db, n.eventBus),
		&managed_escrowOrderTenantResolver{db: n.db},
		n.nodeID,
	)
	handler := corepayment.NewManagedEscrowEventHandler(dispatcher)

	chainID, ok := managed_escrow.ChainIDFor(chain)
	if !ok {
		return nil, fmt.Errorf("missing ManagedEscrow chain id for %s", chain)
	}

	return managed_escrow.NewLiveMonitor(logClient, handler, managed_escrow.LiveMonitorConfig{
		ChainID:           chainID,
		ConfirmationDepth: 0,
	}), nil
}

type managed_escrowOrderTenantResolver struct {
	db database.Database
}

func (r *managed_escrowOrderTenantResolver) ResolveTenant(_ context.Context, orderID string) (string, error) {
	var order models.Order
	err := r.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID).First(&order).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", corepayment.ErrUnknownOrder
		}
		return "", err
	}
	if order.TenantID == "" {
		return "", corepayment.ErrUnknownOrder
	}
	return order.TenantID, nil
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

	strategyV2, err := n.paymentRegistry.ForCoinV2(coinType)
	if err != nil {
		logger.LogWarningWithIDf(log, n.nodeID, "No chain escrow for coin %s (order %s): %v", event.Coin, event.OrderID, err)
		return
	}

	go func() {
		if err := strategyV2.AutoConfirm(context.Background(), event); err != nil {
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
