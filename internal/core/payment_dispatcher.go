//go:build !private_distribution

package core

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	evmchain "github.com/mobazha/mobazha3.0/internal/chains/evm"
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
	evmRelay := n.distributionEVMRelayService()
	n.evmRelay = evmRelay
	guestRuntimeBinder := &distributionManagedEscrowGuestRuntimeBinder{node: n, source: guestSettlementSource}
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
		WatchSource:      distributionManagedEscrowWatchSource{node: n},
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

// registerManagedEscrowAdapterShadow constructs ManagedEscrowAdapter instances and
// activates them as the canonical V2 escrow path for chains enabled by the
// distribution's payment capability provider.
//
// Routing semantics (EVM_HYBRID_STRATEGY.md §5.5 D-Hybrid-23):
//   - Ready EVM chains: ForCoinV2 returns ManagedEscrowAdapter when relayer is
//     configured; legacy ClientSigned V1 registration is retired.
//   - Without relayer (no RelayAPIURL on standalone, no hosting relay on
//     SaaS), ManagedEscrowAdapter is skipped and EVM coins have no V2 strategy.
//   - UTXO/Solana register as V2-monitored in registerPaymentStrategies.
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
//     ManagedEscrowAdapter V2 is NOT registered when no real relayer is wired — matching
//     hosting gateway (relay required before enabling the managed EVM escrow capability). SetupPayment without
//     relay would only strand buyer funds in a ManagedEscrow that cannot settle.
//     Note: settlement.IsEVMRelayAvailable() is a separate check for legacy
//     ClientSigned relay HTTP paths; ManagedEscrow uses RelayerIsConfigured(managed_escrowRelayer).
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

	store, recorder = n.newSettlementActionStore("ManagedEscrowAdapter V2")

	activeChainsEarly := payment.EnabledChains(
		n.paymentCapabilities,
		payment.CapabilityManagedEVMEscrowV2,
		managed_escrow.ReadyEVMChainTypes(),
	)
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

	if len(activeChainsEarly) > 0 && !managed_escrow.RelayerIsConfigured(relayer) {
		logger.LogWarningWithIDf(log, n.nodeID,
			"ManagedEscrowAdapter V2: relayer not configured — skipping %d ManagedEscrow-enabled chain(s). "+
				"Configure RelayAPIURL (standalone) or enable hosting relay (SaaS) before EVM ManagedEscrow checkout.",
			len(activeChainsEarly))
		n.managed_escrowRelayer = relayer
		for _, chain := range managed_escrow.ReadyEVMChainTypes() {
			managed_escrow.SetManagedEscrowRoutingDecision(string(chain), false)
		}
		n.configureGuestEVMManagedEscrowClosureRuntime(nil)
		return
	}

	codeAndNonce := &paymentManagedEscrowNonceProvider{readers: distributionEVMReaderProvider{wallets: n.multiwallet}}
	deps := adapters.ManagedEscrowAdapterDeps{
		Relayer:       relayer,
		ActionStore:   store,
		AutoConfirmer: managed_escrowCancelableAutoConfirmer{settlement: n.settlementService},
		OwnerSigner:   &paymentManagedEscrowOwnerSigner{signer: distributionManagedEVMSigner{keys: n.keyProvider}},
		NonceProvider: codeAndNonce,
		CodeProvider:  codeAndNonce,
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
	runtimeChainIDs := make(map[iwallet.ChainType]uint64, len(activeChainsEarly))
	for _, chain := range activeChainsEarly {
		if n.paymentRegistry.HasChain(chain) {
			logger.LogInfoWithIDf(log, n.nodeID,
				"ManagedEscrowAdapter transition fallback skipped for chain %s: distribution module already registered a strategy", chain)
			continue
		}
		if n.db == nil {
			continue
		}

		runtimeChainID := n.runtimeManagedEscrowChainID(chain)
		chainDeps := deps
		chainDeps.ChainIDOverride = runtimeChainID

		// Skip chains without a runtime chainID mapping (expected for
		// testnet where only a subset of chains have deployed contracts).
		if runtimeChainID == 0 {
			logger.LogInfoWithIDf(log, n.nodeID,
				"ManagedEscrowAdapter V2 chain %s skipped: no runtime chainID (testnet subset)", chain)
			continue
		}
		monitor, err := n.buildManagedEscrowMonitor(chain)
		if err != nil {
			logger.LogErrorWithIDf(log, n.nodeID,
				"ManagedEscrowAdapter V2 activation FAILED for chain %s: monitor wiring: %v", chain, err)
			continue
		}
		chainDeps.Monitor = monitor

		adapter, err := adapters.NewManagedEscrowAdapter(chain, chainDeps)
		if err != nil {
			if errors.Is(err, adapters.ErrManagedEscrowChainNotReady) {
				logger.LogInfoWithIDf(log, n.nodeID,
					"ManagedEscrowAdapter V2 activation skipped for chain %s — not in Ready matrix yet (%v)", chain, err)
			} else {
				logger.LogErrorWithIDf(log, n.nodeID,
					"ManagedEscrowAdapter V2 activation FAILED for chain %s: %v", chain, err)
			}
			continue
		}

		monitors[chain] = monitor
		n.paymentRegistry.RegisterV2(chain, adapter)
		if nativeCoin, coinErr := iwallet.RequireCanonicalNativeCoinType(chain); coinErr == nil {
			registered, regErr := n.paymentRegistry.ForCoinV2(nativeCoin)
			if regErr != nil {
				logger.LogErrorWithIDf(log, n.nodeID,
					"ManagedEscrowAdapter V2 activation FAILED self-check for chain %s coin %s: %v", chain, nativeCoin, regErr)
				continue
			}
			if registered != adapter {
				logger.LogErrorWithIDf(log, n.nodeID,
					"ManagedEscrowAdapter V2 activation FAILED self-check for chain %s coin %s: registry returned %T, want %T",
					chain, nativeCoin, registered, adapter)
				continue
			}
		}
		shadow[chain] = adapter
		runtimeChainIDs[chain] = runtimeChainID
	}

	n.managed_escrowActionStore = store
	n.managed_escrowRelayer = relayer
	n.managedEscrowAdapters = shadow
	n.managed_escrowMonitors = monitors
	n.rewatchPendingManagedEscrowPayments(monitors)
	n.rewatchGuestEVMManagedEscrowOrders(monitors)
	n.wireGuestEVMManagedEscrowObservation(monitors)
	n.wireGuestEVMManagedEscrowSettlement()
	n.configureGuestEVMManagedEscrowClosureRuntime(monitors)

	// Publish grayscale routing decisions as startup metrics. This provides
	// an audit trail in Grafana for "which chains were live on V2 at node start".
	for chain, adapter := range shadow {
		managed_escrow.SetManagedEscrowRoutingDecision(string(chain), adapter != nil)
	}
	// Chains without an active ManagedEscrowAdapter also get routing=0 for observability.
	for _, chain := range managed_escrow.ReadyEVMChainTypes() {
		if _, ok := shadow[chain]; !ok {
			managed_escrow.SetManagedEscrowRoutingDecision(string(chain), false)
		}
	}

	if len(activeChainsEarly) == 0 {
		logger.LogInfoWithIDf(log, n.nodeID,
			"ManagedEscrowAdapter: no chains activated (no Ready EVM chains or monitor wiring failed)")
	} else {
		logger.LogInfoWithIDf(log, n.nodeID,
			"ManagedEscrowAdapter V2 activated for %d/%d EVM chains: %v (runtime chainIDs: %v)", len(shadow), len(activeChainsEarly), shadow, runtimeChainIDs)
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
				&managed_escrowOrderTenantResolver{db: n.db},
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

func (n *MobazhaNode) rewatchPendingManagedEscrowPayments(monitors map[iwallet.ChainType]*managed_escrow.LiveMonitor) {
	if n.db == nil || len(monitors) == 0 {
		return
	}

	var orders []models.Order
	if err := n.db.View(func(tx database.Tx) error {
		return tx.Read().
			Where("payment_address <> ''").
			Where("payment_verification_status <> ?", models.PaymentVerificationStatusVerified).
			Find(&orders).Error
	}); err != nil {
		logger.LogWarningWithIDf(log, n.nodeID, "ManagedEscrowAdapter: failed to load pending ManagedEscrow watches: %v", err)
		return
	}

	for i := range orders {
		n.rewatchPendingManagedEscrowPayment(&orders[i], monitors)
	}
}

func (n *MobazhaNode) rewatchPendingManagedEscrowPayment(order *models.Order, monitors map[iwallet.ChainType]*managed_escrow.LiveMonitor) {
	if order == nil {
		return
	}
	info, err := order.GetPendingManagedEscrowPaymentInfo()
	if err != nil {
		logger.LogWarningWithIDf(log, n.nodeID, "ManagedEscrowAdapter: pending ManagedEscrow info invalid for order %s: %v", order.ID, err)
		return
	}
	if info == nil || strings.TrimSpace(info.Address) == "" || strings.TrimSpace(info.Coin) == "" {
		return
	}
	expected := new(big.Int).SetUint64(info.Amount)
	if expected.Sign() == 0 {
		logger.LogWarningWithIDf(log, n.nodeID, "ManagedEscrowAdapter: cannot restore watch for order %s without pending ManagedEscrow amount", order.ID)
		return
	}

	coinInfo, err := iwallet.CoinInfoFromCoinType(iwallet.CoinType(info.Coin))
	if err != nil || !coinInfo.IsEthTypeChain() {
		return
	}
	monitor := monitors[coinInfo.Chain]
	if monitor == nil {
		return
	}

	asset := managed_escrow.AssetID{Type: managed_escrow.AssetNative}
	if !coinInfo.IsNative {
		contract := common.HexToAddress(coinInfo.ContractAddress(n.managed_escrowRuntimeUsesTestnet(coinInfo.Chain)))
		if contract == (common.Address{}) {
			logger.LogWarningWithIDf(log, n.nodeID, "ManagedEscrowAdapter: cannot restore ERC-20 watch for order %s without token contract", order.ID)
			return
		}
		asset = managed_escrow.AssetID{Type: managed_escrow.AssetERC20, Contract: contract}
	}

	err = monitor.Watch(context.Background(), managed_escrow.WatchRequest{
		OrderID:  order.ID.String(),
		ManagedEscrow:     common.HexToAddress(info.Address),
		ChainID:  n.runtimeManagedEscrowChainID(coinInfo.Chain),
		Asset:    asset,
		Expected: expected,
		Deadline: time.Now().Add(defaultManagedEscrowRewatchFundingTimeout),
	})
	if err != nil && !errors.Is(err, managed_escrow.ErrOrderAlreadyWatched) {
		logger.LogWarningWithIDf(log, n.nodeID, "ManagedEscrowAdapter: failed to restore watch for order %s: %v", order.ID, err)
	}
}

const defaultManagedEscrowRewatchFundingTimeout = 48 * time.Hour

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

	obsRepo := NewGormPaymentObservationRepo(tenantDB, tenantDB.RawDB())
	orderAggregator := corepayment.NewAggregatingVerifier(n.db, n.eventBus)
	if n.orderService != nil {
		orderAggregator.SetPaymentVerifiedHandler(n.handleCryptoPaymentVerified)
	}
	aggregator := corepayment.PaymentAggregator(orderAggregator)
	if n.guestOrderService != nil {
		aggregator = corepayment.NewRoutingPaymentAggregator(
			corepayment.NewGuestManagedEscrowPaymentAggregator(n.db, n.guestOrderService, obsRepo),
			orderAggregator,
		)
	}
	dispatcher := corepayment.NewObservationDispatcher(
		obsRepo,
		aggregator,
		&managed_escrowOrderTenantResolver{db: n.db},
		n.nodeID,
	)
	handler := corepayment.NewManagedEscrowEventHandler(dispatcher)

	chainID := n.runtimeManagedEscrowChainID(chain)
	if chainID == 0 {
		return nil, fmt.Errorf("missing ManagedEscrow chain id for %s", chain)
	}

	return managed_escrow.NewLiveMonitor(logClient, handler, managed_escrow.LiveMonitorConfig{
		ChainID:           chainID,
		ConfirmationDepth: 0,
	}), nil
}

func (n *MobazhaNode) runtimeManagedEscrowChainID(chain iwallet.ChainType) uint64 {
	chainID, ok := managed_escrow.ChainIDForNetwork(chain, n.managed_escrowRuntimeUsesTestnet(chain))
	if !ok {
		return 0
	}
	return chainID
}

func (n *MobazhaNode) managed_escrowRuntimeUsesTestnet(chain iwallet.ChainType) bool {
	if n.walletTestnet {
		return true
	}
	if n.multiwallet == nil {
		return false
	}
	wallet, ok := n.multiwallet.WalletForChain(chain)
	if !ok || wallet == nil {
		return false
	}
	if wallet.IsTestnet() {
		return true
	}

	evmWallet, ok := wallet.(*evmchain.ETHWallet)
	if !ok || evmWallet == nil {
		return false
	}
	client, ok := evmWallet.ChainClient.(*evmchain.EthClient)
	return ok && client != nil && client.Testnet
}

type managed_escrowOrderTenantResolver struct {
	db database.Database
}

func (r *managed_escrowOrderTenantResolver) ResolveTenant(_ context.Context, orderID string) (string, error) {
	if strings.HasPrefix(orderID, corepayment.GuestOrderTokenPrefix) {
		var guest models.GuestOrder
		err := r.db.View(func(tx database.Tx) error {
			return tx.Read().Where("order_token = ?", orderID).First(&guest).Error
		})
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return "", corepayment.ErrUnknownOrder
			}
			return "", err
		}
		if guest.TenantID == "" {
			return database.StandaloneTenantID, nil
		}
		return guest.TenantID, nil
	}

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

func (r *managed_escrowOrderTenantResolver) ResolveTenants(ctx context.Context, orderID string) ([]string, error) {
	if strings.HasPrefix(orderID, corepayment.GuestOrderTokenPrefix) {
		tenantID, err := r.ResolveTenant(ctx, orderID)
		if err != nil {
			return nil, err
		}
		return []string{tenantID}, nil
	}

	var tenantIDs []string
	if rawProvider, ok := r.db.(interface{ RawDB() *gorm.DB }); ok {
		raw := rawProvider.RawDB()
		if raw == nil {
			return nil, fmt.Errorf("raw DB unavailable")
		}
		if err := raw.
			Model(&models.Order{}).
			Where("id = ? AND tenant_id <> ''", orderID).
			Distinct("tenant_id").
			Pluck("tenant_id", &tenantIDs).Error; err != nil {
			return nil, err
		}
		if len(tenantIDs) == 0 {
			return nil, corepayment.ErrUnknownOrder
		}
		return tenantIDs, nil
	}

	err := r.db.View(func(tx database.Tx) error {
		return tx.Read().
			Model(&models.Order{}).
			Where("id = ? AND tenant_id <> ''", orderID).
			Distinct("tenant_id").
			Pluck("tenant_id", &tenantIDs).Error
	})
	if err != nil {
		return nil, err
	}
	if len(tenantIDs) == 0 {
		return nil, corepayment.ErrUnknownOrder
	}
	return tenantIDs, nil
}

func (n *MobazhaNode) wireGuestEVMManagedEscrowObservation(monitors map[iwallet.ChainType]*managed_escrow.LiveMonitor) {
	if n.guestPaymentMonitor == nil || len(monitors) == 0 {
		return
	}
	n.guestPaymentMonitor.SetEVMManagedEscrowWatch(guest.NewEVMManagedEscrowWatchRegistrarFromLive(monitors, n.walletTestnet))
}

func (n *MobazhaNode) wireGuestEVMManagedEscrowSettlement() {
	if n.db == nil || n.guestOrderService == nil || n.keyProvider == nil || n.multiwallet == nil || !managed_escrow.RelayerIsConfigured(n.managed_escrowRelayer) {
		return
	}
	codeAndNonce := &paymentManagedEscrowNonceProvider{readers: distributionEVMReaderProvider{wallets: n.multiwallet}}
	svc := guest.NewEVMManagedEscrowSettlementService(guest.EVMManagedEscrowSettlementConfig{
		DB:            n.db,
		Relayer:       n.managed_escrowRelayer,
		OwnerSigner:   &paymentManagedEscrowOwnerSigner{signer: distributionManagedEVMSigner{keys: n.keyProvider}},
		NonceProvider: codeAndNonce,
		CodeProvider:  codeAndNonce,
		WalletTestnet: n.walletTestnet,
	})
	n.guestOrderService.SetEVMManagedEscrowSettlement(svc)
	go func() {
		ctx := context.Background()
		svc.RecoverPendingSettlements(ctx)
		svc.RecoverConfirmedEntitlements(ctx)
	}()
}

func (n *MobazhaNode) configureGuestEVMManagedEscrowClosureRuntime(monitors map[iwallet.ChainType]*managed_escrow.LiveMonitor) {
	if n.guestOrderService == nil {
		return
	}
	chainSet := make(map[iwallet.ChainType]struct{}, len(monitors))
	for chain := range monitors {
		chainSet[chain] = struct{}{}
	}
	fundingReady := n.directPaymentService != nil && n.directPaymentService.HasEVMManagedEscrowFunding()
	obsReady := len(monitors) > 0
	relayReady := managed_escrow.RelayerIsConfigured(n.managed_escrowRelayer)
	settleReady := relayReady && n.keyProvider != nil && n.multiwallet != nil && n.guestOrderService.HasEVMManagedEscrowSettlement()
	relayGasHealthy, relayGasUnhealthy := n.probeGuestRelayGasHealthyChains(monitors)

	n.guestOrderService.SetEVMManagedEscrowClosureRuntime(guest.EVMManagedEscrowClosureRuntime{
		FundingReady:            fundingReady,
		ObservationReady:        obsReady,
		SettlementReady:         settleReady,
		RelayReady:              relayReady,
		ManagedEscrowMonitorChains:       chainSet,
		RelayGasHealthyChains:   relayGasHealthy,
		RelayGasUnhealthyReason: relayGasUnhealthy,
	})
}

func (n *MobazhaNode) probeGuestRelayGasHealthyChains(monitors map[iwallet.ChainType]*managed_escrow.LiveMonitor) (healthy map[iwallet.ChainType]struct{}, unhealthyReason map[iwallet.ChainType]string) {
	if !managed_escrow.RelayerIsConfigured(n.managed_escrowRelayer) || len(monitors) == 0 {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	healthy = make(map[iwallet.ChainType]struct{}, len(monitors))
	unhealthyReason = make(map[iwallet.ChainType]string, len(monitors))
	for chain := range monitors {
		status, err := n.managed_escrowRelayer.GasWalletStatus(ctx, n.runtimeManagedEscrowChainID(chain))
		if err != nil {
			unhealthyReason[chain] = err.Error()
			logger.LogWarningWithIDf(log, n.nodeID,
				"Guest EVM closure: relay gas wallet status for %s: %v", chain, err)
			continue
		}
		if status.Healthy {
			healthy[chain] = struct{}{}
			continue
		}
		reason := status.Reason
		if reason == "" {
			reason = "relay gas wallet unhealthy"
		}
		unhealthyReason[chain] = reason
		logger.LogWarningWithIDf(log, n.nodeID,
			"Guest EVM closure: relay gas wallet unhealthy on %s: %s", chain, reason)
	}
	return healthy, unhealthyReason
}

func (n *MobazhaNode) rewatchGuestEVMManagedEscrowOrders(monitors map[iwallet.ChainType]*managed_escrow.LiveMonitor) {
	if n.db == nil || len(monitors) == 0 {
		return
	}
	registrar := guest.NewEVMManagedEscrowWatchRegistrarFromLive(monitors, n.walletTestnet)
	var orders []models.GuestOrder
	if err := n.db.View(func(tx database.Tx) error {
		return tx.Read().
			Where("state IN ?", []int{
				int(models.GuestOrderAwaitingPayment),
				int(models.GuestOrderPaymentDetected),
			}).
			Where("evm_managed_escrow_metadata IS NOT NULL AND evm_managed_escrow_metadata <> ''").
			Find(&orders).Error
	}); err != nil {
		logger.LogWarningWithIDf(log, n.nodeID, "Guest EVM ManagedEscrow: failed to load pending watches: %v", err)
		return
	}
	for i := range orders {
		if !orders[i].HasEVMManagedEscrowFundingTarget() {
			continue
		}
		if time.Now().After(orders[i].ExpiresAt.Add(evmGuestManagedEscrowRewatchGrace)) {
			continue
		}
		if err := registrar.RegisterWatch(context.Background(), &orders[i]); err != nil {
			logger.LogWarningWithIDf(log, n.nodeID,
				"Guest EVM ManagedEscrow: failed to restore watch for %s: %v", orders[i].OrderToken, err)
		}
	}
}

const evmGuestManagedEscrowRewatchGrace = 1 * time.Hour

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

type managed_escrowCancelableAutoConfirmer struct {
	settlement *settlement.SettlementService
}

func (s managed_escrowCancelableAutoConfirmer) AutoConfirmManagedEscrowCancelable(ctx context.Context, event *events.CancelablePaymentReady, chain iwallet.ChainType) error {
	if s.settlement == nil {
		return nil
	}
	return s.settlement.AutoConfirmManagedEscrowCancelable(ctx, event, chain)
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
