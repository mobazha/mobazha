package core

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/payment"
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
	testCFXNativeCoin   = mustNativeCoin(iwallet.ChainConflux)
	testSOLNativeCoin   = mustNativeCoin(iwallet.ChainSolana)
)

func mustNativeCoin(chain iwallet.ChainType) iwallet.CoinType {
	coin, err := iwallet.RequireCanonicalNativeCoinType(chain)
	if err != nil {
		panic(err)
	}
	return coin
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
		{"CFX→EVM", testCFXNativeCoin, categoryEVM},

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
		testETHNativeCoin, testBNBNativeCoin, testMATICNativeCoin, testBASENativeCoin, testCFXNativeCoin,
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
		strategy, err := n.paymentRegistry.ForCoin(tc.coin)
		if err != nil {
			t.Errorf("ForCoin(%s): unexpected error: %v", tc.coin, err)
			continue
		}
		if strategy.Model() != payment.PaymentModelMonitored {
			t.Errorf("ForCoin(%s).Model() = %s, want %s", tc.coin, strategy.Model(), payment.PaymentModelMonitored)
		}
	}
}

func TestRegistryDispatch_EVMChainsRegistered(t *testing.T) {
	n := &MobazhaNode{identityFields: identityFields{nodeID: "test-registry"}}
	n.registerPaymentStrategies()

	evmCoins := []iwallet.CoinType{
		testETHNativeCoin, testBNBNativeCoin, testMATICNativeCoin,
		testBASENativeCoin, testCFXNativeCoin,
	}

	for _, coin := range evmCoins {
		strategy, err := n.paymentRegistry.ForCoin(coin)
		if err != nil {
			t.Errorf("ForCoin(%s): unexpected error: %v", coin, err)
			continue
		}
		if strategy.Model() != payment.PaymentModelClientSigned {
			t.Errorf("ForCoin(%s).Model() = %s, want %s", coin, strategy.Model(), payment.PaymentModelClientSigned)
		}
	}
}

func TestRegistryDispatch_SolanaRegistered(t *testing.T) {
	n := &MobazhaNode{identityFields: identityFields{nodeID: "test-registry"}}
	n.registerPaymentStrategies()

	strategy, err := n.paymentRegistry.ForCoin(testSOLNativeCoin)
	if err != nil {
		t.Fatalf("ForCoin(SOL): unexpected error: %v", err)
	}
	if strategy.Model() != payment.PaymentModelClientSigned {
		t.Errorf("solana strategy.Model() = %s, want %s", strategy.Model(), payment.PaymentModelClientSigned)
	}
}

func TestRegistryDispatch_ChainCount(t *testing.T) {
	n := &MobazhaNode{identityFields: identityFields{nodeID: "test-registry"}}
	n.registerPaymentStrategies()

	chains := n.paymentRegistry.Chains()
	// Expected: UTXO (4) + EVM (5) + Solana (1) + TRON (1) = 11
	if len(chains) != 11 {
		t.Errorf("registry has %d chains, want 11 (4 UTXO + 5 EVM + 1 Solana + 1 TRON)", len(chains))
	}
}

func TestRegistryDispatch_AllSupportedCoinsInRegistry(t *testing.T) {
	n := &MobazhaNode{identityFields: identityFields{nodeID: "test-registry"}}
	n.registerPaymentStrategies()

	// Every supported coin should resolve to a registered strategy
	supportedCoins := []iwallet.CoinType{
		testBTCNativeCoin, testBCHNativeCoin, testLTCNativeCoin, testZECNativeCoin,
		testETHNativeCoin, testBNBNativeCoin, testMATICNativeCoin, testBASENativeCoin, testCFXNativeCoin,
		testSOLNativeCoin,
	}

	for _, coin := range supportedCoins {
		if _, err := n.paymentRegistry.ForCoin(coin); err != nil {
			t.Errorf("ForCoin(%s): expected success, got error: %v", coin, err)
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

// ── tryLockAutoConfirm ──────────────────────────────────────────────────

func TestTryLockAutoConfirm_SingleOrder(t *testing.T) {
	svc := &PaymentAppService{}

	// First lock should succeed
	unlock := svc.TryLockAutoConfirm("order-1")
	if unlock == nil {
		t.Fatal("first tryLockAutoConfirm should succeed")
	}

	// Second lock for same order should fail
	unlock2 := svc.TryLockAutoConfirm("order-1")
	if unlock2 != nil {
		t.Fatal("second tryLockAutoConfirm for same order should return nil")
	}

	// After unlock, should be able to lock again
	unlock()
	unlock3 := svc.TryLockAutoConfirm("order-1")
	if unlock3 == nil {
		t.Fatal("tryLockAutoConfirm should succeed after unlock")
	}
	unlock3()
}

func TestTryLockAutoConfirm_DifferentOrders(t *testing.T) {
	svc := &PaymentAppService{}

	unlock1 := svc.TryLockAutoConfirm("order-1")
	if unlock1 == nil {
		t.Fatal("lock for order-1 should succeed")
	}
	defer unlock1()

	// Different order should also succeed
	unlock2 := svc.TryLockAutoConfirm("order-2")
	if unlock2 == nil {
		t.Fatal("lock for order-2 should succeed while order-1 is locked")
	}
	defer unlock2()
}

func TestTryLockAutoConfirm_ConcurrentSafety(t *testing.T) {
	svc := &PaymentAppService{}
	const orderID = "concurrent-order"

	var lockCount int32
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			unlock := svc.TryLockAutoConfirm(orderID)
			if unlock != nil {
				atomic.AddInt32(&lockCount, 1)
				// Hold lock briefly
				unlock()
			}
		}()
	}
	wg.Wait()

	// At least 1 goroutine must have acquired the lock, and all should eventually succeed
	// (since they release quickly). The exact count depends on scheduling.
	if lockCount == 0 {
		t.Fatal("no goroutine was able to acquire the lock")
	}
	t.Logf("concurrent test: %d out of 100 goroutines acquired the lock", lockCount)
}

// ── dispatchCancelablePayment safety ────────────────────────────────────

func TestDispatchCancelablePayment_NilRegistrySafety(t *testing.T) {
	svc := NewPaymentAppService(PaymentAppServiceConfig{
		NodeID: "test-dispatch-nil",
	})
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("dispatchCancelablePayment panicked with nil registry: %v", r)
		}
	}()
	svc.dispatchCancelablePayment(&events.CancelablePaymentReady{
		OrderID:       "test-nil-registry",
		TransactionID: "test-tx",
		Coin:          string(testBTCNativeCoin),
		Amount:        1000,
	})
}

func TestDispatchCancelablePayment_UnknownCoinSafety(t *testing.T) {
	n := &MobazhaNode{identityFields: identityFields{nodeID: "test-dispatch"}}
	n.registerPaymentStrategies()

	svc := NewPaymentAppService(PaymentAppServiceConfig{
		NodeID:          "test-dispatch",
		PaymentRegistry: n.paymentRegistry,
	})

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
			svc.dispatchCancelablePayment(&events.CancelablePaymentReady{
				OrderID:       "test-order-" + tc.name,
				TransactionID: "test-tx",
				Coin:          tc.coin,
				Amount:        1000,
			})
		})
	}
}

// ── Stripe: should NOT reach cancelable dispatch ────────────────────────
// Stripe payments use webhook-based confirmation, not the cancelable payment
// pipeline. This test documents this intentional design.

func TestDispatchCancelablePayment_StripeIsNotDispatched(t *testing.T) {
	cat := classifyCoin(iwallet.CtStripe)
	if cat != categoryUnknown {
		t.Errorf("Stripe should be 'unknown' in cancelable dispatch (uses webhook flow), got %s", cat)
	}
}
