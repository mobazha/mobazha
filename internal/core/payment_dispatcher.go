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
	solLegacy := adapters.NewClientSignedAdapter(solOps, n.paymentService.BuildInitEscrowInstructions, n.orderService.GetEscrowReleaseInstructions)
	n.paymentRegistry.Register(iwallet.ChainSolana, solLegacy)
	solActionStore := adapters.NewMemoryActionStore()
	var solRelayer adapters.SolanaInstructionRelayer
	if n.settlementService != nil {
		solRelayer = adapters.SolanaInstructionRelayerFunc(n.settlementService.RelaySolanaTransactionWithSigners)
	}
	n.paymentRegistry.RegisterV2(iwallet.ChainSolana, adapters.NewSolanaAnchorAdapter(adapters.SolanaAnchorAdapterDeps{
		Legacy:   solLegacy,
		Relayer:  solRelayer,
		Store:    solActionStore,
		Recorder: solActionStore,
		Keys:     n.keyProvider,
		Wallets:  n.multiwallet,
	}))

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

	codeAndNonce := &paymentManagedEscrowNonceProvider{multiwallet: n.multiwallet}
	deps := adapters.ManagedEscrowAdapterDeps{
		Relayer:       relayer,
		Keys:          n.keyProvider,
		Multiwallet:   n.multiwallet,
		ActionStore:   store,
		AutoConfirmer: managed_escrowCancelableAutoConfirmer{settlement: n.settlementService},
		OwnerSigner:   &paymentManagedEscrowOwnerSigner{keys: n.keyProvider},
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
	// Chains on V1 legacy path also get a routing=0 gauge for completeness.
	for _, chain := range managed_escrow.LegacyChains(n.managed_escrowCapConfig) {
		managed_escrow.SetManagedEscrowRoutingDecision(string(chain), false)
	}

	if len(activeChainsEarly) == 0 {
		logger.LogInfoWithIDf(log, n.nodeID,
			"ManagedEscrowAdapter: no chains activated (managed_escrow_chains config empty — all EVM on V1 legacy path)")
	} else {
		logger.LogInfoWithIDf(log, n.nodeID,
			"ManagedEscrowAdapter V2 activated for %d/%d EVM chains: %v (runtime chainIDs: %v)", len(shadow), len(activeChainsEarly), shadow, runtimeChainIDs)
	}

	// Wire ObservationDispatcher into PaymentAppService for UTXO audit path.
	// Aggregator is nil (audit-only): observations are inserted for unified
	// multi-chain audit, but the legacy UTXO verification pipeline remains
	// the source of truth for order state transitions. This avoids competing
	// with UTXOPaymentDetected → ProcessOrderPayment and hardcoding DIRECT.
	if n.paymentService != nil && n.db != nil {
		if tenantDB, ok := n.db.(*dbstore.TenantDB); ok {
			utxoDispatcher := corepayment.NewObservationDispatcher(
				NewGormPaymentObservationRepo(tenantDB, tenantDB.RawDB()),
				nil, // audit-only: no aggregator
				&managed_escrowOrderTenantResolver{db: n.db},
				n.nodeID,
			)
			n.paymentService.SetObservationDispatcher(utxoDispatcher)
		}
	}
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
	codeAndNonce := &paymentManagedEscrowNonceProvider{multiwallet: n.multiwallet}
	svc := guest.NewEVMManagedEscrowSettlementService(guest.EVMManagedEscrowSettlementConfig{
		DB:            n.db,
		Relayer:       n.managed_escrowRelayer,
		OwnerSigner:   &paymentManagedEscrowOwnerSigner{keys: n.keyProvider},
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
