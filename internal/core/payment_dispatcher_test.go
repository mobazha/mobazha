package core

import (
	"context"
	"errors"
	"testing"
	"time"

	corepayment "github.com/mobazha/mobazha3.0/internal/core/payment"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/distribution"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"google.golang.org/protobuf/encoding/protojson"
)

// ── Chain categorization ────────────────────────────────────────────────
//
// These tests verify coin → chain classification and registry-driven dispatch.
//
// Public UTXO chains and explicitly composed distribution modules register in
// the payment registry. The legacy fallback switch has been eliminated.

// chainCategory classifies the dispatch target for a given coin.
// This mirrors the dispatch logic: registry first, then legacy fallback.
type chainCategory string

const (
	categoryUTXO    chainCategory = "utxo"
	categoryEVM     chainCategory = "evm"
	categorySolana  chainCategory = "solana"
	categoryUnknown chainCategory = "unknown"
)

var (
	testBTCNativeCoin   = mustNativeCoin(iwallet.ChainBitcoin)
	testBCHNativeCoin   = mustNativeCoin(iwallet.ChainBitcoinCash)
	testLTCNativeCoin   = mustNativeCoin(iwallet.ChainLitecoin)
	testZECNativeCoin   = mustNativeCoin(iwallet.ChainZCash)
	testETHNativeCoin   = mustNativeCoin(iwallet.ChainEthereum)
	testBNBNativeCoin   = mustNativeCoin(iwallet.ChainBSC)
	testMATICNativeCoin = mustNativeCoin(iwallet.ChainPolygon)
	testBASENativeCoin  = mustNativeCoin(iwallet.ChainBase)
	testSOLNativeCoin   = mustNativeCoin(iwallet.ChainSolana)
)

func mustNativeCoin(chain iwallet.ChainType) iwallet.CoinType {
	coin, err := iwallet.RequireCanonicalNativeCoinType(chain)
	if err != nil {
		panic(err)
	}
	return coin
}

func TestManagedEscrowOrderTenantResolver_ResolveTenants_GuestOrder(t *testing.T) {
	db := newTestDatabase(t)
	if err := db.gormDB.AutoMigrate(&models.GuestOrder{}); err != nil {
		t.Fatalf("AutoMigrate GuestOrder: %v", err)
	}
	orderToken := corepayment.GuestOrderTokenPrefix + "tenant_resolver"
	if err := db.gormDB.Create(&models.GuestOrder{
		TenantMixin: models.TenantMixin{TenantID: "tenant-guest"},
		OrderToken:  orderToken,
		State:       models.GuestOrderAwaitingPayment,
	}).Error; err != nil {
		t.Fatalf("create guest order: %v", err)
	}

	tenants, err := (&paymentOrderTenantResolver{db: db}).ResolveTenants(context.Background(), orderToken)
	if err != nil {
		t.Fatalf("ResolveTenants: %v", err)
	}
	if len(tenants) != 1 || tenants[0] != "tenant-guest" {
		t.Fatalf("ResolveTenants = %#v, want [tenant-guest]", tenants)
	}
}

func TestManagedEscrowOrderTenantResolver_ResolveTenants_GuestOrderStandaloneDefault(t *testing.T) {
	db := newTestDatabase(t)
	if err := db.gormDB.AutoMigrate(&models.GuestOrder{}); err != nil {
		t.Fatalf("AutoMigrate GuestOrder: %v", err)
	}
	orderToken := corepayment.GuestOrderTokenPrefix + "tenant_default"
	if err := db.gormDB.Create(&models.GuestOrder{
		OrderToken: orderToken,
		State:      models.GuestOrderAwaitingPayment,
	}).Error; err != nil {
		t.Fatalf("create guest order: %v", err)
	}

	tenants, err := (&paymentOrderTenantResolver{db: db}).ResolveTenants(context.Background(), orderToken)
	if err != nil {
		t.Fatalf("ResolveTenants: %v", err)
	}
	if len(tenants) != 1 || tenants[0] != database.StandaloneTenantID {
		t.Fatalf("ResolveTenants = %#v, want [%s]", tenants, database.StandaloneTenantID)
	}
}

func TestManagedEscrowOrderTenantResolver_ResolveTenants_OrderMirrors(t *testing.T) {
	db := newTestDatabase(t)
	orderID := "QmSharedManagedEscrowOrder"
	for _, tenantID := range []string{"tenant-buyer", "tenant-vendor"} {
		if err := db.gormDB.Create(&models.Order{
			TenantMixin: models.TenantMixin{TenantID: tenantID},
			ID:          models.OrderID(orderID),
		}).Error; err != nil {
			t.Fatalf("create mirrored order for %s: %v", tenantID, err)
		}
	}

	tenants, err := (&paymentOrderTenantResolver{db: db}).ResolveTenants(context.Background(), orderID)
	if err != nil {
		t.Fatalf("ResolveTenants: %v", err)
	}
	got := map[string]bool{}
	for _, tenantID := range tenants {
		got[tenantID] = true
	}
	if len(got) != 2 || !got["tenant-buyer"] || !got["tenant-vendor"] {
		t.Fatalf("ResolveTenants = %#v, want both mirrored tenants", tenants)
	}
}

func classifyCoin(coin iwallet.CoinType) chainCategory {
	coinInfo, err := coin.CoinInfo()
	if err != nil {
		return categoryUnknown
	}
	switch {
	case coinInfo.Chain.IsUTXOChain():
		return categoryUTXO
	case coinInfo.IsEthTypeChain():
		return categoryEVM
	case coinInfo.Chain == iwallet.ChainSolana:
		return categorySolana
	default:
		return categoryUnknown
	}
}

func TestDispatchCancelablePayment_ChainCategorization(t *testing.T) {
	tests := []struct {
		name     string
		coin     iwallet.CoinType
		expected chainCategory
	}{
		// UTXO chains
		{"BTC→UTXO", testBTCNativeCoin, categoryUTXO},
		{"BCH→UTXO", testBCHNativeCoin, categoryUTXO},
		{"LTC→UTXO", testLTCNativeCoin, categoryUTXO},
		{"ZEC→UTXO", testZECNativeCoin, categoryUTXO},

		// EVM chains
		{"ETH→EVM", testETHNativeCoin, categoryEVM},
		{"BNB→EVM", testBNBNativeCoin, categoryEVM},
		{"MATIC→EVM", testMATICNativeCoin, categoryEVM},
		{"BASE→EVM", testBASENativeCoin, categoryEVM},
		// Solana
		{"SOL→Solana", testSOLNativeCoin, categorySolana},

		// Unknown
		{"INVALID→Unknown", iwallet.CoinType("INVALID_COIN"), categoryUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyCoin(tt.coin)
			if got != tt.expected {
				t.Errorf("classifyCoin(%s) = %s, want %s", tt.coin, got, tt.expected)
			}
		})
	}
}

// TestDispatchCancelablePayment_AllSupportedCoins verifies that every registered
// CoinType can be classified (none fall through to "unknown" except truly invalid ones).
func TestDispatchCancelablePayment_AllSupportedCoins(t *testing.T) {
	// All supported coins should have a known category
	supportedCoins := []iwallet.CoinType{
		testBTCNativeCoin, testBCHNativeCoin, testLTCNativeCoin, testZECNativeCoin,
		testETHNativeCoin, testBNBNativeCoin, testMATICNativeCoin, testBASENativeCoin,
		testSOLNativeCoin,
	}

	for _, coin := range supportedCoins {
		cat := classifyCoin(coin)
		if cat == categoryUnknown {
			t.Errorf("supported coin %s classified as unknown — dispatch would log warning instead of handling", coin)
		}
	}
}

// ── Payment Registry integration ────────────────────────────────────────
//
// These tests validate that the payment registry is correctly populated
// by registerPaymentStrategies() and that ForCoin resolves as expected.

func TestRegistryDispatch_UTXOChainsRegistered(t *testing.T) {
	n := &MobazhaNode{identityFields: identityFields{nodeID: "test-registry"}}
	n.registerPaymentStrategies()

	utxoChains := []struct {
		coin  iwallet.CoinType
		chain iwallet.ChainType
	}{
		{testBTCNativeCoin, iwallet.ChainBitcoin},
		{testBCHNativeCoin, iwallet.ChainBitcoinCash},
		{testLTCNativeCoin, iwallet.ChainLitecoin},
		{testZECNativeCoin, iwallet.ChainZCash},
	}

	for _, tc := range utxoChains {
		if _, err := n.paymentRegistry.ForCoin(tc.coin); err == nil {
			t.Errorf("ForCoin(%s): legacy V1 UTXO registration is retired, expected error", tc.coin)
		}
		strategy, err := n.paymentRegistry.ForCoinV2(tc.coin)
		if err != nil {
			t.Errorf("ForCoinV2(%s): unexpected error: %v", tc.coin, err)
			continue
		}
		if strategy.Model() != payment.PaymentModelMonitored {
			t.Errorf("ForCoinV2(%s).Model() = %s, want %s", tc.coin, strategy.Model(), payment.PaymentModelMonitored)
		}
	}
}

func TestRegistryDispatch_EVMLegacyV1Retired(t *testing.T) {
	n := &MobazhaNode{identityFields: identityFields{nodeID: "test-registry"}}
	n.registerPaymentStrategies()

	evmCoins := []iwallet.CoinType{
		testETHNativeCoin, testBNBNativeCoin, testMATICNativeCoin,
		testBASENativeCoin,
	}

	for _, coin := range evmCoins {
		if _, err := n.paymentRegistry.ForCoin(coin); err == nil {
			t.Errorf("ForCoin(%s): legacy V1 EVM registration is retired, expected error", coin)
		}
		if _, err := n.paymentRegistry.ForCoinV2(coin); err == nil {
			t.Errorf("ForCoinV2(%s): expected error without ManagedEscrowAdapter registration", coin)
		}
	}
}

func TestRegistryDispatch_SolanaRegistered(t *testing.T) {
	n := &MobazhaNode{identityFields: identityFields{nodeID: "test-registry"}}
	n.registerPaymentStrategies()

	if _, err := n.paymentRegistry.ForCoin(testSOLNativeCoin); err == nil {
		t.Fatal("ForCoin(SOL): legacy V1 Solana registration is retired, expected error")
	}
}

func TestRegistryDispatch_SolanaRequiresDistributionModule(t *testing.T) {
	n := &MobazhaNode{identityFields: identityFields{nodeID: "test-registry"}}
	n.registerPaymentStrategies()

	if _, err := n.paymentRegistry.ForCoinV2(testSOLNativeCoin); err == nil {
		t.Fatal("ForCoinV2(SOL): expected error without a private distribution module")
	}
}

func TestRegistryDispatch_ChainCount(t *testing.T) {
	n := &MobazhaNode{identityFields: identityFields{nodeID: "test-registry"}}
	n.registerPaymentStrategies()

	chains := n.paymentRegistry.Chains()
	// Open Core registers only the public UTXO allowlist. Commercial rails are
	// contributed by trusted distribution modules.
	if len(chains) != 4 {
		t.Errorf("registry has %d chains, want 4 public UTXO chains", len(chains))
	}
}

func TestRegistryDispatch_AllSupportedCoinsInRegistry(t *testing.T) {
	n := &MobazhaNode{identityFields: identityFields{nodeID: "test-registry"}}
	n.registerPaymentStrategies()

	utxoCoins := []iwallet.CoinType{
		testBTCNativeCoin, testBCHNativeCoin, testLTCNativeCoin, testZECNativeCoin,
	}
	for _, coin := range utxoCoins {
		if _, err := n.paymentRegistry.ForCoin(coin); err == nil {
			t.Errorf("ForCoin(%s): legacy V1 registration is retired, expected error", coin)
		}
		if _, err := n.paymentRegistry.ForCoinV2(coin); err != nil {
			t.Errorf("ForCoinV2(%s): expected success, got error: %v", coin, err)
		}
	}
	if _, err := n.paymentRegistry.ForCoinV2(testSOLNativeCoin); err == nil {
		t.Error("ForCoinV2(SOL): expected error without a private distribution module")
	}

	evmCoins := []iwallet.CoinType{
		testETHNativeCoin, testBNBNativeCoin, testMATICNativeCoin, testBASENativeCoin,
	}
	for _, coin := range evmCoins {
		if _, err := n.paymentRegistry.ForCoin(coin); err == nil {
			t.Errorf("ForCoin(%s): legacy V1 EVM registration is retired, expected error", coin)
		}
		if _, err := n.paymentRegistry.ForCoinV2(coin); err == nil {
			t.Errorf("ForCoinV2(%s): expected error without ManagedEscrowAdapter registration", coin)
		}
	}
}

func TestRegistryDispatch_UnknownCoinNotInRegistry(t *testing.T) {
	n := &MobazhaNode{identityFields: identityFields{nodeID: "test-registry"}}
	n.registerPaymentStrategies()

	_, err := n.paymentRegistry.ForCoin(iwallet.CoinType("INVALID_COIN"))
	if err == nil {
		t.Error("ForCoin(INVALID_COIN): expected error, got nil")
	}
}

// ── dispatchCancelablePayment safety (MobazhaNode level) ────────────────

func TestDispatchCancelablePayment_NilRegistrySafety(t *testing.T) {
	n := &MobazhaNode{identityFields: identityFields{nodeID: "test-dispatch-nil"}}
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("dispatchCancelablePayment panicked with nil registry: %v", r)
		}
	}()
	n.dispatchCancelablePayment(&events.CancelablePaymentReady{
		OrderID:       "test-nil-registry",
		TransactionID: "test-tx",
		Coin:          string(testBTCNativeCoin),
		Amount:        "1000",
	})
}

func TestDispatchCancelablePayment_UnknownCoinSafety(t *testing.T) {
	n := &MobazhaNode{identityFields: identityFields{nodeID: "test-dispatch"}}
	n.registerPaymentStrategies()

	testCases := []struct {
		name string
		coin string
	}{
		{"Unknown-coin", "NONEXISTENT"},
		{"Empty-coin", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("dispatchCancelablePayment panicked for coin %q: %v", tc.coin, r)
				}
			}()
			n.dispatchCancelablePayment(&events.CancelablePaymentReady{
				OrderID:       "test-order-" + tc.name,
				TransactionID: "test-tx",
				Coin:          tc.coin,
				Amount:        "1000",
			})
		})
	}
}

func TestDispatchCancelablePayment_SkipsSolanaSellerDeclineRefunderAutoConfirm(t *testing.T) {
	called := make(chan struct{})
	strategy := &autoConfirmRefunderProbeStrategy{
		autoConfirmProbeStrategy: autoConfirmProbeStrategy{called: called},
	}
	n := &MobazhaNode{
		identityFields: identityFields{nodeID: "seller-node"},
		walletFields:   walletFields{paymentRegistry: payment.NewRegistry()},
	}
	n.paymentRegistry.RegisterV2(iwallet.ChainSolana, strategy)

	n.dispatchCancelablePayment(&events.CancelablePaymentReady{
		OrderID:       "solana-order",
		TransactionID: "tx",
		Coin:          string(testSOLNativeCoin),
		Amount:        "1000",
		TenantID:      "seller-node",
	})

	select {
	case <-called:
		t.Fatal("Solana seller_decline_refund strategy should preserve seller decision window instead of auto-confirming")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestDispatchCancelablePayment_SkipsManagedCollectibleFirstSaleAutoConfirm(t *testing.T) {
	db := newTestDatabase(t)
	orderOpen := collectibleLifecycleOrderOpen("holder-wallet", []byte("buyer-identity-key"))
	serializedOrderOpen, err := protojson.Marshal(orderOpen)
	if err != nil {
		t.Fatal(err)
	}
	orderID := models.OrderID("managed-collectible-order")
	if err := db.Update(func(tx database.Tx) error {
		return tx.Save(&models.Order{
			ID:                  orderID,
			MyRole:              string(models.RoleVendor),
			SerializedOrderOpen: serializedOrderOpen,
		})
	}); err != nil {
		t.Fatal(err)
	}

	called := make(chan struct{})
	n := &MobazhaNode{
		identityFields: identityFields{nodeID: "seller-node"},
		storageFields:  storageFields{db: db},
		walletFields:   walletFields{paymentRegistry: payment.NewRegistry()},
	}
	n.paymentRegistry.RegisterV2(iwallet.ChainEthereum, &autoConfirmProbeStrategy{called: called})

	n.dispatchCancelablePayment(&events.CancelablePaymentReady{
		OrderID:       orderID.String(),
		TransactionID: "tx",
		Coin:          string(testETHNativeCoin),
		Amount:        "1000",
		TenantID:      "seller-node",
	})

	select {
	case <-called:
		t.Fatal("managed collectible first sale should preserve escrow until Hub settle or default")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestDispatchCancelablePayment_StillAutoConfirmsNonRefunderStrategy(t *testing.T) {
	called := make(chan struct{})
	n := &MobazhaNode{
		identityFields: identityFields{nodeID: "seller-node"},
		walletFields:   walletFields{paymentRegistry: payment.NewRegistry()},
	}
	n.paymentRegistry.RegisterV2(iwallet.ChainBitcoin, &autoConfirmProbeStrategy{called: called})

	n.dispatchCancelablePayment(&events.CancelablePaymentReady{
		OrderID:       "btc-order",
		TransactionID: "tx",
		Coin:          string(testBTCNativeCoin),
		Amount:        "1000",
		TenantID:      "seller-node",
	})

	select {
	case <-called:
	case <-time.After(time.Second):
		t.Fatal("expected non-refunder strategy to receive AutoConfirm")
	}
}

type autoConfirmProbeStrategy struct {
	payment.ChainEscrowV2
	called chan struct{}
}

func (s *autoConfirmProbeStrategy) AutoConfirm(context.Context, *events.CancelablePaymentReady) error {
	close(s.called)
	return nil
}

type autoConfirmRefunderProbeStrategy struct {
	autoConfirmProbeStrategy
}

func (s *autoConfirmRefunderProbeStrategy) SellerDeclineRefund(context.Context, payment.ActionParams) (*payment.ActionResult, error) {
	return nil, nil
}

type paymentModuleProbe struct {
	id       string
	chain    iwallet.ChainType
	strategy payment.ChainEscrowV2
	err      error
	runtime  distribution.PaymentRuntime
}

type paymentModuleRunnerProbe struct {
	*paymentModuleProbe
	startErr error
	stopped  chan struct{}
}

func (m *paymentModuleRunnerProbe) Start(context.Context) error { return m.startErr }

func (m *paymentModuleRunnerProbe) Stop(context.Context) error {
	select {
	case m.stopped <- struct{}{}:
	default:
	}
	return nil
}

func (m *paymentModuleProbe) Descriptor() distribution.PaymentModuleDescriptor {
	return distribution.PaymentModuleDescriptor{ID: m.id, Capabilities: []distribution.PaymentModuleCapability{
		distribution.CapabilityManagedEVMExecution,
	}}
}

func (m *paymentModuleProbe) RollbackRegistration(context.Context) error { return nil }

func (m *paymentModuleProbe) Register(_ context.Context, runtime distribution.PaymentRuntime, registrar distribution.PaymentRegistrar) error {
	m.runtime = runtime
	if m.err != nil {
		return m.err
	}
	return registrar.RegisterV2(m.chain, m.strategy)
}

func TestRegisterDistributionPaymentModules_ExternalStrategyIsRegistered(t *testing.T) {
	strategy := &autoConfirmProbeStrategy{called: make(chan struct{})}
	module := &paymentModuleProbe{id: "commercial.solana", chain: iwallet.ChainSolana, strategy: strategy}
	n := &MobazhaNode{
		identityFields: identityFields{nodeCtx: context.Background()},
		walletFields: walletFields{
			paymentRegistry: payment.NewRegistry(),
			paymentModules:  []distribution.PaymentModule{module},
		},
	}

	if err := n.registerDistributionPaymentModules(); err != nil {
		t.Fatalf("registerDistributionPaymentModules: %v", err)
	}
	got, err := n.paymentRegistry.ForChainV2(iwallet.ChainSolana)
	if err != nil {
		t.Fatalf("ForChainV2(Solana): %v", err)
	}
	if got != strategy {
		t.Fatalf("ForChainV2(Solana) = %T, want module strategy", got)
	}
	managedEVM, err := module.runtime.ManagedEVM()
	if err != nil {
		t.Fatal(err)
	}
	if managedEVM.EVMSigner == nil || managedEVM.EVMReaders == nil {
		t.Fatal("distribution module did not receive narrow EVM operation ports")
	}
	if managedEVM.Actions != nil || managedEVM.ActionRecorder != nil {
		t.Fatal("distribution module must not receive an in-memory settlement action fallback")
	}
}

func TestRegisterDistributionPaymentModules_CannotReplaceCoreStrategy(t *testing.T) {
	registry := payment.NewRegistry()
	coreStrategy := &autoConfirmProbeStrategy{called: make(chan struct{})}
	registry.RegisterV2(iwallet.ChainBitcoin, coreStrategy)
	n := &MobazhaNode{
		identityFields: identityFields{nodeCtx: context.Background()},
		walletFields: walletFields{
			paymentRegistry: registry,
			paymentModules: []distribution.PaymentModule{
				&paymentModuleProbe{
					id:       "invalid.override",
					chain:    iwallet.ChainBitcoin,
					strategy: &autoConfirmProbeStrategy{called: make(chan struct{})},
				},
			},
		},
	}

	err := n.registerDistributionPaymentModules()
	if err == nil {
		t.Fatal("expected core strategy replacement to be rejected")
	}
	got, lookupErr := registry.ForChainV2(iwallet.ChainBitcoin)
	if lookupErr != nil {
		t.Fatalf("ForChainV2(Bitcoin): %v", lookupErr)
	}
	if got != coreStrategy {
		t.Fatal("core strategy was replaced after rejected module registration")
	}
}

func TestRunDistributionPaymentModules_FailureUnregistersOnlyDistributionChains(t *testing.T) {
	registry := payment.NewRegistry()
	strategy := &autoConfirmProbeStrategy{called: make(chan struct{})}
	registry.RegisterV2(iwallet.ChainBitcoin, strategy)
	registry.RegisterV2(iwallet.ChainSolana, strategy)
	runner := &paymentModuleRunnerProbe{
		paymentModuleProbe: &paymentModuleProbe{id: "commercial.runner"},
		startErr:           errors.New("monitor failed"),
		stopped:            make(chan struct{}, 1),
	}
	n := &MobazhaNode{
		identityFields: identityFields{nodeID: "test-registry"},
		walletFields: walletFields{
			paymentRegistry: registry,
			paymentModules:  []distribution.PaymentModule{runner},
		},
	}

	n.runDistributionPaymentModules(context.Background(), []iwallet.ChainType{iwallet.ChainSolana})
	select {
	case <-runner.stopped:
	case <-time.After(time.Second):
		t.Fatal("failed module was not stopped")
	}
	if registry.HasChain(iwallet.ChainSolana) {
		t.Fatal("failed distribution chain remained registered")
	}
	if !registry.HasChain(iwallet.ChainBitcoin) {
		t.Fatal("Open Core chain was removed with failed distribution chains")
	}
}

func TestCancelablePaymentEventTargetsNode(t *testing.T) {
	tests := []struct {
		name          string
		eventTenantID string
		nodeID        string
		localTenantID string
		want          bool
	}{
		{name: "standalone-empty-tenant", eventTenantID: "", nodeID: "peer-node", want: true},
		{name: "matching-node-id", eventTenantID: "tenant-a", nodeID: "tenant-a", want: true},
		{name: "matching-local-tenant", eventTenantID: "tenant-a", nodeID: "peer-node", localTenantID: "tenant-a", want: true},
		{name: "foreign-tenant", eventTenantID: "tenant-b", nodeID: "tenant-a", localTenantID: "tenant-c", want: false},
		{name: "platform-runtime-handles-tenant-event", eventTenantID: "tenant-a", nodeID: "platform-node", localTenantID: "", want: true},
		{name: "scoped-event-requires-local-identity", eventTenantID: "tenant-a", nodeID: "", localTenantID: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cancelablePaymentEventTargetsNode(tt.eventTenantID, tt.nodeID, tt.localTenantID); got != tt.want {
				t.Fatalf("cancelablePaymentEventTargetsNode(%q, %q, %q) = %v, want %v",
					tt.eventTenantID, tt.nodeID, tt.localTenantID, got, tt.want)
			}
		})
	}
}

// ── Fiat: should NOT reach cancelable dispatch ──────────────────────────
// Fiat payments use webhook-based confirmation, not the cancelable payment
// pipeline. This test documents this intentional design.

func TestDispatchCancelablePayment_FiatIsNotDispatched(t *testing.T) {
	cat := classifyCoin(iwallet.CtFiat)
	if cat != categoryUnknown {
		t.Errorf("Fiat should be 'unknown' in cancelable dispatch (uses webhook flow), got %s", cat)
	}
}
