//go:build !private_distribution

package core

import (
	"context"
	"errors"
	"testing"
	"time"

	corepayment "github.com/mobazha/mobazha3.0/internal/core/payment"
	"github.com/mobazha/mobazha3.0/internal/database/dbstore"
	adapters "github.com/mobazha/mobazha3.0/internal/payment/adapters"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	"github.com/mobazha/mobazha3.0/pkg/managedescrow"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// ── Chain categorization ────────────────────────────────────────────────
//
// These tests verify coin → chain classification and registry-driven dispatch.
//
// All chains (UTXO, EVM, Solana) are registered in the payment registry.
// The legacy fallback switch has been eliminated — dispatch is registry-only.

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

	tenants, err := (&managed_escrowOrderTenantResolver{db: db}).ResolveTenants(context.Background(), orderToken)
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

	tenants, err := (&managed_escrowOrderTenantResolver{db: db}).ResolveTenants(context.Background(), orderToken)
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

	tenants, err := (&managed_escrowOrderTenantResolver{db: db}).ResolveTenants(context.Background(), orderID)
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

func TestRegistryDispatch_SolanaForCoinV2UsesAnchorAdapter(t *testing.T) {
	n := &MobazhaNode{identityFields: identityFields{nodeID: "test-registry"}}
	n.registerPaymentStrategies()

	strategy, err := n.paymentRegistry.ForCoinV2(testSOLNativeCoin)
	if err != nil {
		t.Fatalf("ForCoinV2(SOL): unexpected error: %v", err)
	}
	if _, ok := strategy.(*adapters.SolanaAnchorAdapter); !ok {
		t.Fatalf("ForCoinV2(SOL) returned %T, want *adapters.SolanaAnchorAdapter", strategy)
	}
	if strategy.Model() != payment.PaymentModelMonitored {
		t.Errorf("solana V2 strategy.Model() = %s, want %s", strategy.Model(), payment.PaymentModelMonitored)
	}
	if strategy.Capabilities().HasClientSignedEscrow {
		t.Error("solana V2 strategy must not advertise client-signed escrow")
	}
}

func TestRegistryDispatch_ChainCount(t *testing.T) {
	n := &MobazhaNode{identityFields: identityFields{nodeID: "test-registry"}}
	n.registerPaymentStrategies()

	chains := n.paymentRegistry.Chains()
	// Legacy V1 registration retired for UTXO/Solana/EVM/TRON. Expected: UTXO (4) + Solana (1) = 5.
	if len(chains) != 5 {
		t.Errorf("registry has %d chains, want 5 (4 UTXO + 1 Solana)", len(chains))
	}
}

func TestRegistryDispatch_AllSupportedCoinsInRegistry(t *testing.T) {
	n := &MobazhaNode{identityFields: identityFields{nodeID: "test-registry"}}
	n.registerPaymentStrategies()

	utxoCoins := []iwallet.CoinType{
		testBTCNativeCoin, testBCHNativeCoin, testLTCNativeCoin, testZECNativeCoin,
		testSOLNativeCoin,
	}
	for _, coin := range utxoCoins {
		if _, err := n.paymentRegistry.ForCoin(coin); err == nil {
			t.Errorf("ForCoin(%s): legacy V1 registration is retired, expected error", coin)
		}
		if _, err := n.paymentRegistry.ForCoinV2(coin); err != nil {
			t.Errorf("ForCoinV2(%s): expected success, got error: %v", coin, err)
		}
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

// ── ManagedEscrowAdapter shadow registration (Sprint 2 D17) ──────────────────────
//
// Pin down the contract registerManagedEscrowAdapterShadow upholds while V1
// EVMChainOps remains canonical: V1 ForCoin lookups untouched, V2
// ForChainV2 returns the ManagedEscrowAdapter verbatim, GetActionStatus is wired
// against an in-memory store, and missing deps skip registration cleanly.

// nodeWithManagedEscrowShadowDeps builds a MobazhaNode with just the deps
// registerManagedEscrowAdapterShadow needs (keyProvider, multiwallet).
func nodeWithManagedEscrowShadowDeps() *MobazhaNode {
	n := &MobazhaNode{identityFields: identityFields{nodeID: "test-shadow"}}
	n.keyProvider = &mockKeyProvider{}
	n.multiwallet = &mockWalletOperator{}
	return n
}

func nodeWithManagedEscrowShadowMonitorDeps(t *testing.T) *MobazhaNode {
	t.Helper()

	db, err := dbstore.NewMemoryDB(t.TempDir())
	if err != nil {
		t.Fatalf("NewMemoryDB: %v", err)
	}

	wallets := make(map[iwallet.ChainType]iwallet.Wallet)
	for _, chain := range readyEVMChains(t) {
		wallets[chain] = newMockEVMWallet(chain, &mockManagedEscrowLogSubscriber{})
	}

	n := &MobazhaNode{
		identityFields: identityFields{
			nodeID:  "test-shadow-monitor",
			nodeCtx: context.Background(),
		},
		storageFields: storageFields{db: db},
		networkFields: networkFields{eventBus: events.NewBus()},
		walletFields: walletFields{
			managed_escrowCapConfig: readyManagedEscrowCapConfig(t),
			relayAPIURL:   "http://relay.test", // ManagedEscrowAdapter V2 requires a real relayer
		},
	}
	n.keyProvider = &mockKeyProvider{}
	n.multiwallet = &mockWalletOperatorWithChainWallets{wallets: wallets}
	return n
}

// readyEVMChains returns the subset of evmChains whose ManagedEscrowAdapter
// shadow registration is expected to succeed. zkSync Era stays in
// evmChains for the multiwallet/EVM-client wiring but its
// chainMatrix Ready flag is false, so registerManagedEscrowAdapterShadow MUST
// skip it (the V1 ClientSigned path remains canonical for it too;
// the higher-level guard is exercised separately below).
func readyEVMChains(t *testing.T) []iwallet.ChainType {
	t.Helper()
	out := make([]iwallet.ChainType, 0, len(evmChains))
	for _, c := range evmChains {
		if _, ok := managed_escrow.PolicyFor(c); ok {
			out = append(out, c)
		}
	}
	return out
}

// shadowRegisteredEVMChains returns Ready EVM chains whose ManagedEscrowAdapter shadow
// registration succeeds for n — mirrors runtimeManagedEscrowChainID fail-closed behavior
// (testnet mock wallets only expose explicit public testnet chain ids).
func shadowRegisteredEVMChains(t *testing.T, n *MobazhaNode) []iwallet.ChainType {
	t.Helper()
	out := make([]iwallet.ChainType, 0, len(evmChains))
	for _, chain := range readyEVMChains(t) {
		if n.runtimeManagedEscrowChainID(chain) != 0 {
			out = append(out, chain)
		}
	}
	return out
}

func readyManagedEscrowCapConfig(t *testing.T) *managed_escrow.ChainCapabilityConfig {
	t.Helper()
	chains := readyEVMChains(t)
	managed_escrowChains := make([]string, 0, len(chains))
	for _, chain := range chains {
		managed_escrowChains = append(managed_escrowChains, string(chain))
	}
	return &managed_escrow.ChainCapabilityConfig{ManagedEscrowChains: managed_escrowChains}
}

func TestManagedEscrowAdapterShadow_SkipsRegistrationWithoutRelayer(t *testing.T) {
	n := nodeWithManagedEscrowShadowMonitorDeps(t)
	n.relayAPIURL = "" // simulate standalone without RelayAPIURL
	n.registerPaymentStrategies()

	if len(n.managedEscrowAdapters) != 0 {
		t.Fatalf("managedEscrowAdapters = %d entries, want 0 without relayer", len(n.managedEscrowAdapters))
	}
	if len(n.managed_escrowMonitors) != 0 {
		t.Fatalf("managed_escrowMonitors = %d entries, want 0 without relayer", len(n.managed_escrowMonitors))
	}
	if !managed_escrow.RelayerIsConfigured(n.managed_escrowRelayer) {
		return
	}
	t.Fatal("managed_escrowRelayer should be NoopRelayer when relay is not configured")
}

func TestManagedEscrowAdapterShadow_ForCoinV2UnavailableWithoutRelayer(t *testing.T) {
	n := nodeWithManagedEscrowShadowMonitorDeps(t)
	n.relayAPIURL = ""
	n.registerPaymentStrategies()

	_, err := n.paymentRegistry.ForCoinV2(testETHNativeCoin)
	if err == nil {
		t.Fatal("ForCoinV2(ETH) should fail when ManagedEscrowAdapter is skipped due to missing relayer")
	}
}

func TestManagedEscrowAdapterShadow_RegistersForReadyEVMChains(t *testing.T) {
	n := nodeWithManagedEscrowShadowMonitorDeps(t)
	n.registerPaymentStrategies()

	if n.managed_escrowActionStore == nil {
		t.Fatal("registerManagedEscrowAdapterShadow did not populate managed_escrowActionStore")
	}
	want := shadowRegisteredEVMChains(t, n)
	if got := len(n.managedEscrowAdapters); got != len(want) {
		t.Fatalf("managedEscrowAdapters has %d entries, want %d (runtime-supported Ready EVM chains — zkSync Era is Ready=false; testnet mocks only register explicit testnet mappings)", got, len(want))
	}
	for _, chain := range want {
		adapter, ok := n.managedEscrowAdapters[chain]
		if !ok {
			t.Errorf("managedEscrowAdapters missing chain %s", chain)
			continue
		}
		if adapter == nil {
			t.Errorf("managedEscrowAdapters[%s] is nil", chain)
			continue
		}
		if adapter.Chain() != chain {
			t.Errorf("managedEscrowAdapters[%s].Chain() = %s, want %s", chain, adapter.Chain(), chain)
		}
	}
	// And the converse: not-Ready EVM chains MUST be absent so the
	// ManagedEscrowAdapter cannot quietly serve a chain whose deployments row
	// is missing.
	for _, chain := range evmChains {
		if _, ok := managed_escrow.PolicyFor(chain); ok {
			continue
		}
		if _, registered := n.managedEscrowAdapters[chain]; registered {
			t.Errorf("managedEscrowAdapters has Ready=false chain %s registered — shadow registration ignored chainMatrix gate", chain)
		}
	}
}

func TestManagedEscrowAdapterShadow_V1PathRetired(t *testing.T) {
	n := nodeWithManagedEscrowShadowDeps()
	n.registerPaymentStrategies()

	for _, coin := range []iwallet.CoinType{
		testETHNativeCoin, testBNBNativeCoin, testMATICNativeCoin, testBASENativeCoin,
	} {
		if _, err := n.paymentRegistry.ForCoin(coin); err == nil {
			t.Errorf("ForCoin(%s): legacy V1 EVM registration is retired, expected error", coin)
		}
	}
}

func TestManagedEscrowAdapterShadow_V2LookupReturnsManagedEscrowAdapter(t *testing.T) {
	n := nodeWithManagedEscrowShadowMonitorDeps(t)
	n.registerPaymentStrategies()

	for chain, want := range n.managedEscrowAdapters {
		got, err := n.paymentRegistry.ForChainV2(chain)
		if err != nil {
			t.Errorf("ForChainV2(%s): %v", chain, err)
			continue
		}
		gotManagedEscrow, ok := got.(*adapters.ManagedEscrowAdapter)
		if !ok {
			t.Errorf("ForChainV2(%s) returned %T; expected *adapters.ManagedEscrowAdapter", chain, got)
			continue
		}
		if gotManagedEscrow != want {
			t.Errorf("ForChainV2(%s) returned different ManagedEscrowAdapter pointer than the one in managedEscrowAdapters map", chain)
		}
	}
}

func TestManagedEscrowAdapterShadow_RegistersManagedEscrowMonitorsWhenMonitorDepsAvailable(t *testing.T) {
	n := nodeWithManagedEscrowShadowMonitorDeps(t)
	n.registerPaymentStrategies()

	want := shadowRegisteredEVMChains(t, n)
	if got := len(n.managed_escrowMonitors); got != len(want) {
		t.Fatalf("managed_escrowMonitors has %d entries, want %d", got, len(want))
	}
	for _, chain := range want {
		if n.managed_escrowMonitors[chain] == nil {
			t.Fatalf("managed_escrowMonitors[%s] is nil", chain)
		}
		if _, ok := n.managedEscrowAdapters[chain]; !ok {
			t.Fatalf("managedEscrowAdapters[%s] missing despite monitor deps being available", chain)
		}
	}
}

func TestRuntimeManagedEscrowChainID_UsesPublicTestnetIDs(t *testing.T) {
	n := &MobazhaNode{modeFlags: modeFlags{walletTestnet: true}}

	cases := []struct {
		chain iwallet.ChainType
		want  uint64
	}{
		{iwallet.ChainEthereum, 11155111},
		{iwallet.ChainBSC, 97},
		{iwallet.ChainPolygon, 80002},
		{iwallet.ChainBase, 84532},
	}
	for _, tc := range cases {
		t.Run(string(tc.chain), func(t *testing.T) {
			if got := n.runtimeManagedEscrowChainID(tc.chain); got != tc.want {
				t.Fatalf("runtimeManagedEscrowChainID(%s) = %d, want %d", tc.chain, got, tc.want)
			}
		})
	}
}

func TestRuntimeManagedEscrowChainID_TestnetFailsClosedWithoutExplicitMapping(t *testing.T) {
	n := &MobazhaNode{modeFlags: modeFlags{walletTestnet: true}}

	if got := n.runtimeManagedEscrowChainID(iwallet.ChainOptimism); got != 0 {
		t.Fatalf("runtimeManagedEscrowChainID(Optimism testnet) = %d, want 0 until explicit ManagedEscrow testnet support lands", got)
	}
}

func TestRuntimeManagedEscrowChainID_UsesWalletRuntimeNetwork(t *testing.T) {
	n := &MobazhaNode{
		walletFields: walletFields{
			multiwallet: &mockWalletOperatorWithChainWallets{wallets: map[iwallet.ChainType]iwallet.Wallet{
				iwallet.ChainEthereum: newMockEVMWalletWithTestnet(iwallet.ChainEthereum, nil, true),
			}},
		},
	}

	if got := n.runtimeManagedEscrowChainID(iwallet.ChainEthereum); got != 11155111 {
		t.Fatalf("runtimeManagedEscrowChainID(ETH) = %d, want Sepolia chain id", got)
	}
}

func TestRuntimeManagedEscrowChainID_DefaultsToMainnetWithoutRuntimeTestnet(t *testing.T) {
	n := &MobazhaNode{
		walletFields: walletFields{
			multiwallet: &mockWalletOperatorWithChainWallets{wallets: map[iwallet.ChainType]iwallet.Wallet{
				iwallet.ChainEthereum: &mockEVMWallet{
					chain:   iwallet.ChainEthereum,
					coin:    testETHNativeCoin,
					testnet: false,
				},
			}},
		},
	}

	if got := n.runtimeManagedEscrowChainID(iwallet.ChainEthereum); got != 1 {
		t.Fatalf("runtimeManagedEscrowChainID(ETH) = %d, want mainnet chain id", got)
	}
}

func TestManagedEscrowAdapterShadow_GetActionStatusReportsActionNotFound(t *testing.T) {
	n := nodeWithManagedEscrowShadowMonitorDeps(t)
	n.registerPaymentStrategies()

	// Loop every Ready=true EVM chain so a per-chain wiring regression
	// cannot slip past on a chain we did not name explicitly. zkSync
	// Era is intentionally skipped — see TestManagedEscrowAdapterShadow_RegistersForReadyEVMChains.
	for _, chain := range shadowRegisteredEVMChains(t, n) {
		t.Run(string(chain), func(t *testing.T) {
			adapter, ok := n.managedEscrowAdapters[chain]
			if !ok {
				t.Fatalf("expected ManagedEscrowAdapter for %s after shadow registration", chain)
			}
			_, err := adapter.GetActionStatus(context.Background(), "unknown-action-id")
			if !errors.Is(err, payment.ErrActionNotFound) {
				t.Fatalf("GetActionStatus(unknown) on %s = %v, want payment.ErrActionNotFound", chain, err)
			}
		})
	}
}

func TestManagedEscrowAdapterShadow_OwnerProviderInjectedWhenPaymentServiceAvailable(t *testing.T) {
	// D18a contract: when paymentService is wired, every ManagedEscrowAdapter
	// gets a real OwnerProvider so SetupPayment no longer short-circuits
	// to errManagedEscrowStubNotImplemented. We use HasOwnerProvider() — the
	// dispatcher-facing predicate — instead of reaching into unexported
	// state, so the test stays decoupled from ManagedEscrowAdapter's internals.
	//
	// D18b additionally pins NonceProvider injection: it is wired
	// unconditionally (multiwallet is the only required dep, already
	// provided by nodeWithManagedEscrowShadowDeps), so it must be present here
	// alongside the OwnerProvider.
	n := nodeWithManagedEscrowShadowDeps()
	n = nodeWithManagedEscrowShadowMonitorDeps(t)
	// PaymentAppService is non-nil; its inner deps stay zero-valued
	// because registerManagedEscrowAdapterShadow only checks pointer presence.
	n.paymentService = corepayment.NewPaymentAppService(corepayment.PaymentAppServiceConfig{NodeID: "test-shadow"})
	n.registerPaymentStrategies()

	if len(n.managedEscrowAdapters) == 0 {
		t.Fatal("managedEscrowAdapters empty; shadow registration did not run with paymentService present")
	}
	for chain, adapter := range n.managedEscrowAdapters {
		if !adapter.HasOwnerProvider() {
			t.Errorf("managedEscrowAdapters[%s] has no OwnerProvider; D18a injection regressed", chain)
		}
		if !adapter.HasNonceProvider() {
			t.Errorf("managedEscrowAdapters[%s] has no NonceProvider; D18b injection regressed", chain)
		}
	}
}

func TestManagedEscrowAdapterShadow_OwnerProviderNilWhenPaymentServiceMissing(t *testing.T) {
	// Mirror image of the injection test: with paymentService nil
	// (test-only path), OwnerProvider stays nil and SetupPayment
	// continues to short-circuit. Document this explicitly so the
	// test suite encodes the boundary on both sides.
	//
	// NonceProvider is independent of paymentService — D18b wires it
	// from multiwallet, which IS present in nodeWithManagedEscrowShadowDeps —
	// so it must still be wired even on this branch. Asserting both
	// directions here keeps the two providers' wiring contracts
	// distinct and pinned.
	n := nodeWithManagedEscrowShadowMonitorDeps(t)
	n.registerPaymentStrategies()

	if len(n.managedEscrowAdapters) == 0 {
		t.Fatal("managedEscrowAdapters empty; deps-present shadow registration unexpectedly skipped")
	}
	for chain, adapter := range n.managedEscrowAdapters {
		if adapter.HasOwnerProvider() {
			t.Errorf("managedEscrowAdapters[%s] has OwnerProvider despite paymentService=nil; D18a guard broken", chain)
		}
		if !adapter.HasNonceProvider() {
			t.Errorf("managedEscrowAdapters[%s] has no NonceProvider despite multiwallet present; D18b wiring should not depend on paymentService", chain)
		}
	}
}

func TestManagedEscrowAdapterShadow_SkippedWhenDepsMissing(t *testing.T) {
	// Existing tests exercise the deps-missing path implicitly (bare
	// MobazhaNode); make the invariant explicit so a future refactor
	// cannot silently start requiring keyProvider/multiwallet here.
	n := &MobazhaNode{identityFields: identityFields{nodeID: "test-shadow-skipped"}}
	n.registerPaymentStrategies()

	if n.managed_escrowActionStore != nil {
		t.Error("managed_escrowActionStore should be nil when shadow registration is skipped")
	}
	if n.managedEscrowAdapters != nil {
		t.Errorf("managedEscrowAdapters should be nil when shadow registration is skipped, got %d entries", len(n.managedEscrowAdapters))
	}

	// Legacy V1 EVM strategies must not be registered when shadow is skipped.
	for _, coin := range []iwallet.CoinType{
		testETHNativeCoin, testBNBNativeCoin, testMATICNativeCoin, testBASENativeCoin,
	} {
		if _, err := n.paymentRegistry.ForCoin(coin); err == nil {
			t.Errorf("ForCoin(%s) after shadow skip: legacy V1 EVM registration is retired, expected error", coin)
		}
	}
}
