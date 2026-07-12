package utxo

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

// mockPaymentSource is a mock implementation of PaymentSource for testing
type mockPaymentSource struct {
	healthy        bool
	chain          iwallet.ChainType
	txs            []*iwallet.Transaction
	txByID         map[string]*iwallet.Transaction
	feeRate        uint64
	feeRateSet     bool
	broadcastTx    string
	subscribeErr   error
	transactions   []*iwallet.Transaction
	transactionErr error
	subscribeCalls []string // tracks addresses passed to Subscribe
	mu             sync.RWMutex
}

func newMockPaymentSource(healthy bool) *mockPaymentSource {
	return &mockPaymentSource{
		healthy: healthy,
		chain:   iwallet.ChainBitcoin,
		txByID:  make(map[string]*iwallet.Transaction),
	}
}

func (m *mockPaymentSource) Subscribe(ctx context.Context, address string, scriptPubKey []byte, callback func(tx *iwallet.Transaction)) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.subscribeCalls = append(m.subscribeCalls, address)
	return m.subscribeErr
}

func (m *mockPaymentSource) Unsubscribe(ctx context.Context, address string) error {
	return nil
}

func (m *mockPaymentSource) GetTransactions(ctx context.Context, address string, scriptPubKey []byte) ([]*iwallet.Transaction, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.transactions != nil {
		return m.transactions, nil
	}
	return m.txs, nil
}

func (m *mockPaymentSource) GetTransaction(ctx context.Context, txid string) (*iwallet.Transaction, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if tx, ok := m.txByID[txid]; ok {
		return tx, nil
	}
	if m.transactionErr != nil {
		return nil, m.transactionErr
	}
	return nil, ErrTransactionNotFound
}

func (m *mockPaymentSource) IsHealthy() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.healthy
}

func (m *mockPaymentSource) setHealthy(h bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.healthy = h
}

func (m *mockPaymentSource) Chain() iwallet.ChainType {
	return m.chain
}

func (m *mockPaymentSource) Close() error {
	return nil
}

func (m *mockPaymentSource) EstimateFee(ctx context.Context, targetBlocks int) (uint64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.feeRateSet || m.feeRate > 0 {
		return m.feeRate, nil
	}
	return 10, nil
}

func (m *mockPaymentSource) BroadcastTransaction(ctx context.Context, txHex string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.broadcastTx = txHex
	return "mock_txid", nil
}

func (m *mockPaymentSource) ListUnspent(_ context.Context, _ []byte) ([]UnspentOutput, error) {
	return nil, nil
}

func (m *mockPaymentSource) GetTxConfirmations(_ context.Context, _ string) (int, error) {
	return 0, nil
}

// failingBroadcastSource fails N times then succeeds
type failingBroadcastSource struct {
	mockPaymentSource
	failCount int
	callCount int
	mu        sync.Mutex
}

func newFailingSource(failCount int) *failingBroadcastSource {
	return &failingBroadcastSource{
		mockPaymentSource: mockPaymentSource{
			healthy: true,
			chain:   iwallet.ChainBitcoin,
			txByID:  make(map[string]*iwallet.Transaction),
		},
		failCount: failCount,
	}
}

func (f *failingBroadcastSource) BroadcastTransaction(ctx context.Context, txHex string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.callCount++
	if f.callCount <= f.failCount {
		return "", errors.New("broadcast rejected by network")
	}
	return "success_txid", nil
}

func (f *failingBroadcastSource) IsHealthy() bool { return true }

func TestBroadcastRetry_FailThenSucceed(t *testing.T) {
	m := NewMonitor(&MonitorConfig{
		BroadcastMaxRetries: 3,
		BroadcastBaseDelay:  1 * time.Millisecond,
	})

	src := newFailingSource(2) // fail twice, succeed on 3rd
	m.AddSource(iwallet.ChainBitcoin, src)

	txid, err := m.BroadcastTransaction(iwallet.ChainBitcoin, "rawtx_hex")
	if err != nil {
		t.Fatalf("Expected success after retries, got: %v", err)
	}
	if txid != "success_txid" {
		t.Errorf("Expected success_txid, got %s", txid)
	}
	src.mu.Lock()
	calls := src.callCount
	src.mu.Unlock()
	if calls != 3 {
		t.Errorf("Expected 3 calls (2 fail + 1 success), got %d", calls)
	}
}

func TestBroadcastRetry_AllFail(t *testing.T) {
	m := NewMonitor(&MonitorConfig{
		BroadcastMaxRetries: 2,
		BroadcastBaseDelay:  1 * time.Millisecond,
	})

	src := newFailingSource(100) // always fail
	m.AddSource(iwallet.ChainBitcoin, src)

	_, err := m.BroadcastTransaction(iwallet.ChainBitcoin, "rawtx_hex")
	if err == nil {
		t.Fatal("Expected error when all retries fail")
	}
	src.mu.Lock()
	calls := src.callCount
	src.mu.Unlock()
	// 1 initial + 2 retries = 3 total attempts
	if calls != 3 {
		t.Errorf("Expected 3 total attempts (1 initial + 2 retries), got %d", calls)
	}
}

func TestBroadcastRetry_NoRetryOnFirstSuccess(t *testing.T) {
	m := NewMonitor(&MonitorConfig{
		BroadcastMaxRetries: 3,
		BroadcastBaseDelay:  1 * time.Millisecond,
	})

	src := newFailingSource(0) // succeed immediately
	m.AddSource(iwallet.ChainBitcoin, src)

	txid, err := m.BroadcastTransaction(iwallet.ChainBitcoin, "rawtx_hex")
	if err != nil {
		t.Fatalf("Expected success, got: %v", err)
	}
	if txid != "success_txid" {
		t.Errorf("Expected success_txid, got %s", txid)
	}
	src.mu.Lock()
	calls := src.callCount
	src.mu.Unlock()
	if calls != 1 {
		t.Errorf("Expected 1 call (no retry needed), got %d", calls)
	}
}

func TestDefaultMonitorConfig(t *testing.T) {
	config := DefaultMonitorConfig()

	if config.PollInterval != 30*time.Second {
		t.Errorf("Expected PollInterval=30s, got %v", config.PollInterval)
	}
	if config.GracePeriod != 2*time.Hour {
		t.Errorf("Expected GracePeriod=2h, got %v", config.GracePeriod)
	}
}

func TestNewMonitor(t *testing.T) {
	t.Run("with nil config", func(t *testing.T) {
		m := NewMonitor(nil)
		if m == nil {
			t.Fatal("NewMonitor returned nil")
		}
		if m.sources == nil {
			t.Error("sources map should be initialized")
		}
		if m.watching == nil {
			t.Error("watching map should be initialized")
		}
		if m.pollInterval != 30*time.Second {
			t.Errorf("Expected default pollInterval=30s, got %v", m.pollInterval)
		}
	})

	t.Run("with custom config", func(t *testing.T) {
		config := &MonitorConfig{
			PollInterval: 1 * time.Minute,
			GracePeriod:  1 * time.Hour,
		}
		m := NewMonitor(config)
		if m.pollInterval != 1*time.Minute {
			t.Errorf("Expected pollInterval=1m, got %v", m.pollInterval)
		}
		if m.gracePeriod != 1*time.Hour {
			t.Errorf("Expected gracePeriod=1h, got %v", m.gracePeriod)
		}
	})
}

func TestMonitorAddSource(t *testing.T) {
	m := NewMonitor(nil)
	mock := newMockPaymentSource(true)

	m.AddSource(iwallet.ChainBitcoin, mock)

	sources := m.sources[iwallet.ChainBitcoin]
	if len(sources) != 1 {
		t.Errorf("Expected 1 source, got %d", len(sources))
	}

	// Add another source
	mock2 := newMockPaymentSource(true)
	m.AddSource(iwallet.ChainBitcoin, mock2)

	sources = m.sources[iwallet.ChainBitcoin]
	if len(sources) != 2 {
		t.Errorf("Expected 2 sources, got %d", len(sources))
	}
}

func TestMonitorStartStop(t *testing.T) {
	m := NewMonitor(&MonitorConfig{
		PollInterval: 100 * time.Millisecond,
	})

	m.Start()

	// Give it a moment to start
	time.Sleep(50 * time.Millisecond)

	// Stop should complete without hanging
	done := make(chan struct{})
	go func() {
		m.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Error("Stop timed out")
	}
}

func TestMonitorSubscribeTransactions(t *testing.T) {
	m := NewMonitor(nil)

	ch := m.SubscribeTransactions()
	if ch == nil {
		t.Fatal("SubscribeTransactions returned nil channel")
	}

	// Verify subscriber was added
	m.subscribersMu.RLock()
	count := len(m.subscribers)
	m.subscribersMu.RUnlock()

	if count != 1 {
		t.Errorf("Expected 1 subscriber, got %d", count)
	}
}

func TestMonitorBroadcast(t *testing.T) {
	m := NewMonitor(nil)

	ch1 := m.SubscribeTransactions()
	ch2 := m.SubscribeTransactions()

	tx := iwallet.Transaction{
		ID: "test_txid",
	}

	// Broadcast in goroutine
	go m.broadcast(tx)

	// Both channels should receive the transaction
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		select {
		case received := <-ch1:
			if received.ID != tx.ID {
				t.Errorf("ch1: Expected ID=%s, got %s", tx.ID, received.ID)
			}
		case <-time.After(1 * time.Second):
			t.Error("ch1: Timeout waiting for transaction")
		}
	}()

	go func() {
		defer wg.Done()
		select {
		case received := <-ch2:
			if received.ID != tx.ID {
				t.Errorf("ch2: Expected ID=%s, got %s", tx.ID, received.ID)
			}
		case <-time.After(1 * time.Second):
			t.Error("ch2: Timeout waiting for transaction")
		}
	}()

	wg.Wait()
}

func TestMonitorWatchAddress(t *testing.T) {
	t.Run("empty address", func(t *testing.T) {
		m := NewMonitor(nil)
		err := m.WatchAddress(&WatchedAddress{
			Address:   "",
			ChainType: iwallet.ChainBitcoin,
		})
		if err == nil {
			t.Error("Expected error for empty address")
		}
	})

	t.Run("empty chain type", func(t *testing.T) {
		m := NewMonitor(nil)
		err := m.WatchAddress(&WatchedAddress{
			Address:   "test_address",
			ChainType: "",
		})
		if err == nil {
			t.Error("Expected error for empty chain type")
		}
	})

	t.Run("success without sources", func(t *testing.T) {
		m := NewMonitor(nil)
		wa := &WatchedAddress{
			Address:   "test_address",
			ChainType: iwallet.ChainBitcoin,
			OrderID:   "order123",
		}
		err := m.WatchAddress(wa)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		m.watchMu.RLock()
		watched := m.watching["test_address"]
		m.watchMu.RUnlock()

		if watched == nil {
			t.Error("Address should be in watching map")
		}
	})

	t.Run("success with healthy source", func(t *testing.T) {
		m := NewMonitor(nil)
		mock := newMockPaymentSource(true)
		m.AddSource(iwallet.ChainBitcoin, mock)

		wa := &WatchedAddress{
			Address:      "test_address",
			ChainType:    iwallet.ChainBitcoin,
			ScriptPubKey: []byte{0x00, 0x14},
		}
		err := m.WatchAddress(wa)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if !wa.Subscribed {
			t.Error("Address should be marked as subscribed")
		}
	})

	t.Run("subscription failure falls back to polling", func(t *testing.T) {
		m := NewMonitor(nil)
		mock := newMockPaymentSource(true)
		mock.subscribeErr = errors.New("subscription failed")
		m.AddSource(iwallet.ChainBitcoin, mock)

		wa := &WatchedAddress{
			Address:   "test_address",
			ChainType: iwallet.ChainBitcoin,
		}
		err := m.WatchAddress(wa)
		if err != nil {
			t.Fatalf("Should succeed with polling fallback: %v", err)
		}

		if wa.Subscribed {
			t.Error("Address should not be marked as subscribed")
		}
	})

	t.Run("grace period set correctly", func(t *testing.T) {
		m := NewMonitor(&MonitorConfig{
			GracePeriod: 1 * time.Hour,
		})

		expiresAt := time.Now().Add(24 * time.Hour)
		wa := &WatchedAddress{
			Address:   "test_address",
			ChainType: iwallet.ChainBitcoin,
			ExpiresAt: expiresAt,
		}
		m.WatchAddress(wa)

		expectedGracePeriodEnd := expiresAt.Add(1 * time.Hour)
		if wa.GracePeriodEnd.Sub(expectedGracePeriodEnd) > time.Second {
			t.Errorf("GracePeriodEnd not set correctly")
		}
	})
}

func TestMonitorWatchAddress_SharedAddressNotifiesEveryRegisteredNode(t *testing.T) {
	m := NewMonitor(nil)
	source := newMockPaymentSource(true)
	m.AddSource(iwallet.ChainBitcoin, source)
	notified := map[string]int{}
	for _, nodeID := range []string{"buyer-node", "seller-node"} {
		nodeID := nodeID
		if err := m.RegisterNodeCallback(nodeID, func(_ iwallet.Transaction, wa *WatchedAddress) {
			notified[nodeID]++
			if wa.OrderID != "order-shared" {
				t.Errorf("%s order = %s, want order-shared", nodeID, wa.OrderID)
			}
		}); err != nil {
			t.Fatalf("RegisterNodeCallback(%s): %v", nodeID, err)
		}
	}

	first := &WatchedAddress{
		Address: "shared-address", ScriptPubKey: []byte{0x00, 0x20, 0x01},
		ChainType: iwallet.ChainBitcoin, OrderID: "order-shared", NodeID: "buyer-node",
	}
	second := &WatchedAddress{
		Address: "shared-address", ScriptPubKey: []byte{0x00, 0x20, 0x01},
		ChainType: iwallet.ChainBitcoin, OrderID: "order-shared", NodeID: "seller-node",
	}
	if err := m.WatchAddress(first); err != nil {
		t.Fatal(err)
	}
	if err := m.WatchAddress(second); err != nil {
		t.Fatal(err)
	}
	// The address is one chain subscription even though multiple hosted nodes
	// independently register their order-scoped callbacks for it.
	source.mu.RLock()
	subscribeCalls := append([]string(nil), source.subscribeCalls...)
	source.mu.RUnlock()
	if len(subscribeCalls) != 1 || subscribeCalls[0] != "shared-address" {
		t.Fatalf("subscribe calls = %v, want one shared subscription", subscribeCalls)
	}

	tx := &iwallet.Transaction{
		ID: "shared-tx", Value: iwallet.NewAmount(1000), Height: 1,
		To: []iwallet.SpendInfo{{
			Address: iwallet.NewAddress("shared-address", iwallet.CoinType("BTC")), Amount: iwallet.NewAmount(1000),
		}},
	}
	m.handleTransactionForAddress("shared-address", tx)

	if got := m.GetWatchedAddressCount(); got != 1 {
		t.Fatalf("watched addresses = %d, want 1", got)
	}
	if notified["buyer-node"] != 1 || notified["seller-node"] != 1 {
		t.Fatalf("notifications = %v, want one per node", notified)
	}

	// Removing the node that created the underlying watch must promote a
	// remaining registration instead of tearing down the shared subscription.
	m.UnregisterNode("buyer-node")
	if got := m.GetWatchedAddress("shared-address"); got == nil || got.NodeID != "seller-node" {
		t.Fatalf("primary watch after unregister = %#v, want seller-node", got)
	}
	tx.ID = "shared-tx-after-unregister"
	m.handleTransactionForAddress("shared-address", tx)
	if notified["buyer-node"] != 1 || notified["seller-node"] != 2 {
		t.Fatalf("notifications after unregister = %v, want only seller incremented", notified)
	}
}

func TestMonitorUnwatchAddress(t *testing.T) {
	t.Run("non-existent address", func(t *testing.T) {
		m := NewMonitor(nil)
		err := m.UnwatchAddress("non_existent")
		if err != nil {
			t.Errorf("Should not error for non-existent address: %v", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		m := NewMonitor(nil)
		mock := newMockPaymentSource(true)
		m.AddSource(iwallet.ChainBitcoin, mock)

		wa := &WatchedAddress{
			Address:   "test_address",
			ChainType: iwallet.ChainBitcoin,
		}
		m.WatchAddress(wa)

		err := m.UnwatchAddress("test_address")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		m.watchMu.RLock()
		watched := m.watching["test_address"]
		m.watchMu.RUnlock()

		if watched != nil {
			t.Error("Address should be removed from watching map")
		}
	})
}

func TestMonitorGetTransaction(t *testing.T) {
	t.Run("no sources", func(t *testing.T) {
		m := NewMonitor(nil)
		_, err := m.GetTransaction(iwallet.ChainBitcoin, "txid")
		if err == nil {
			t.Error("Expected error for no sources")
		}
	})

	t.Run("success", func(t *testing.T) {
		m := NewMonitor(nil)
		mock := newMockPaymentSource(true)
		mock.txByID["txid123"] = &iwallet.Transaction{
			ID:    "txid123",
			Value: iwallet.NewAmount(100000),
		}
		m.AddSource(iwallet.ChainBitcoin, mock)

		tx, err := m.GetTransaction(iwallet.ChainBitcoin, "txid123")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if string(tx.ID) != "txid123" {
			t.Errorf("Expected txid=txid123, got %s", tx.ID)
		}
	})

	t.Run("unhealthy source skipped", func(t *testing.T) {
		m := NewMonitor(nil)
		mockUnhealthy := newMockPaymentSource(false)
		mockUnhealthy.txByID["txid123"] = &iwallet.Transaction{ID: "txid123"}
		m.AddSource(iwallet.ChainBitcoin, mockUnhealthy)

		mockHealthy := newMockPaymentSource(true)
		mockHealthy.txByID["txid123"] = &iwallet.Transaction{ID: "txid123", Value: iwallet.NewAmount(200000)}
		m.AddSource(iwallet.ChainBitcoin, mockHealthy)

		tx, err := m.GetTransaction(iwallet.ChainBitcoin, "txid123")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		// Should get from healthy source
		if tx.Value.Int64() != 200000 {
			t.Error("Should have gotten transaction from healthy source")
		}
	})

	t.Run("all healthy sources agree transaction is missing", func(t *testing.T) {
		m := NewMonitor(nil)
		m.AddSource(iwallet.ChainBitcoin, newMockPaymentSource(true))
		m.AddSource(iwallet.ChainBitcoin, newMockPaymentSource(true))

		tx, err := m.GetTransaction(iwallet.ChainBitcoin, "missing")
		if tx != nil {
			t.Fatalf("expected no transaction, got %s", tx.ID)
		}
		if !errors.Is(err, ErrTransactionNotFound) {
			t.Fatalf("expected ErrTransactionNotFound, got %v", err)
		}
	})

	t.Run("unhealthy or failing source keeps absence inconclusive", func(t *testing.T) {
		m := NewMonitor(nil)
		m.AddSource(iwallet.ChainBitcoin, newMockPaymentSource(true))
		m.AddSource(iwallet.ChainBitcoin, newMockPaymentSource(false))

		_, err := m.GetTransaction(iwallet.ChainBitcoin, "missing")
		if err == nil || errors.Is(err, ErrTransactionNotFound) {
			t.Fatalf("expected inconclusive source error, got %v", err)
		}

		m = NewMonitor(nil)
		missing := newMockPaymentSource(true)
		failing := newMockPaymentSource(true)
		failing.transactionErr = errors.New("temporary rpc failure")
		m.AddSource(iwallet.ChainBitcoin, missing)
		m.AddSource(iwallet.ChainBitcoin, failing)
		_, err = m.GetTransaction(iwallet.ChainBitcoin, "missing")
		if err == nil || errors.Is(err, ErrTransactionNotFound) {
			t.Fatalf("expected transient source error, got %v", err)
		}
	})
}

func TestMonitorGetHealthySourceCount(t *testing.T) {
	m := NewMonitor(nil)

	// No sources
	if count := m.GetHealthySourceCount(iwallet.ChainBitcoin); count != 0 {
		t.Errorf("Expected 0, got %d", count)
	}

	// Add sources
	m.AddSource(iwallet.ChainBitcoin, newMockPaymentSource(true))
	m.AddSource(iwallet.ChainBitcoin, newMockPaymentSource(false))
	m.AddSource(iwallet.ChainBitcoin, newMockPaymentSource(true))

	if count := m.GetHealthySourceCount(iwallet.ChainBitcoin); count != 2 {
		t.Errorf("Expected 2, got %d", count)
	}
}

func TestMonitorGetWatchedAddressCount(t *testing.T) {
	m := NewMonitor(nil)

	if count := m.GetWatchedAddressCount(); count != 0 {
		t.Errorf("Expected 0, got %d", count)
	}

	m.WatchAddress(&WatchedAddress{Address: "addr1", ChainType: iwallet.ChainBitcoin})
	m.WatchAddress(&WatchedAddress{Address: "addr2", ChainType: iwallet.ChainBitcoin})

	if count := m.GetWatchedAddressCount(); count != 2 {
		t.Errorf("Expected 2, got %d", count)
	}
}

func TestMonitorGetSources(t *testing.T) {
	m := NewMonitor(nil)

	// No sources
	sources := m.GetSources(iwallet.ChainBitcoin)
	if len(sources) != 0 {
		t.Errorf("Expected 0 sources, got %d", len(sources))
	}

	// Add sources
	m.AddSource(iwallet.ChainBitcoin, newMockPaymentSource(true))
	m.AddSource(iwallet.ChainBitcoin, newMockPaymentSource(true))

	sources = m.GetSources(iwallet.ChainBitcoin)
	if len(sources) != 2 {
		t.Errorf("Expected 2 sources, got %d", len(sources))
	}
}

func TestMonitorGetFeeEstimate(t *testing.T) {
	t.Run("no sources returns default", func(t *testing.T) {
		m := NewMonitor(nil)
		fee := m.GetFeeEstimate(iwallet.ChainBitcoin, 6)
		if fee != 10 {
			t.Errorf("Expected default fee=10, got %d", fee)
		}
	})

	t.Run("success", func(t *testing.T) {
		m := NewMonitor(nil)
		mock := newMockPaymentSource(true)
		mock.feeRate = 25
		m.AddSource(iwallet.ChainBitcoin, mock)

		fee := m.GetFeeEstimate(iwallet.ChainBitcoin, 6)
		if fee != 25 {
			t.Errorf("Expected fee=25, got %d", fee)
		}
	})

	t.Run("fee rate capped at minimum", func(t *testing.T) {
		m := NewMonitor(nil)
		mock := newMockPaymentSource(true)
		mock.feeRate = 0       // Below minimum
		mock.feeRateSet = true // Force return 0
		m.AddSource(iwallet.ChainBitcoin, mock)

		fee := m.GetFeeEstimate(iwallet.ChainBitcoin, 6)
		if fee != 1 {
			t.Errorf("Expected min fee=1, got %d", fee)
		}
	})

	t.Run("fee rate capped at maximum", func(t *testing.T) {
		m := NewMonitor(nil)
		mock := newMockPaymentSource(true)
		mock.feeRate = 2000 // Above maximum
		m.AddSource(iwallet.ChainBitcoin, mock)

		fee := m.GetFeeEstimate(iwallet.ChainBitcoin, 6)
		if fee != 1000 {
			t.Errorf("Expected max fee=1000, got %d", fee)
		}
	})
}

func TestMonitorDeterminePaymentStatus(t *testing.T) {
	m := NewMonitor(nil)

	const watchAddr = "bc1qwatch"
	const changeAddr = "bc1qchange"
	const btcCoin = iwallet.CoinType("BTC")
	mkTx := func(toWatch, toChange uint64) *iwallet.Transaction {
		var outs []iwallet.SpendInfo
		if toWatch > 0 {
			outs = append(outs, iwallet.SpendInfo{
				Address: iwallet.NewAddress(watchAddr, btcCoin),
				Amount:  iwallet.NewAmount(toWatch),
			})
		}
		if toChange > 0 {
			outs = append(outs, iwallet.SpendInfo{
				Address: iwallet.NewAddress(changeAddr, btcCoin),
				Amount:  iwallet.NewAmount(toChange),
			})
		}
		// tx.Value is the *total* output value (incl. change). Status decisions
		// must rely on AmountPaidTo(tx, watchAddr) instead.
		return &iwallet.Transaction{Value: iwallet.NewAmount(toWatch + toChange), To: outs}
	}

	t.Run("normal payment with change still classifies as normal", func(t *testing.T) {
		wa := &WatchedAddress{
			Address:        watchAddr,
			ExpectedAmount: 100000,
			ExpiresAt:      time.Now().Add(1 * time.Hour),
		}
		tx := mkTx(100000, 900000)
		wa.TotalPaid.Store(AmountPaidTo(tx, wa.Address))
		status := m.determinePaymentStatus(wa, tx)
		if status != PaymentStatusNormal {
			t.Errorf("Expected normal, got %s", status)
		}
	})

	t.Run("expired payment", func(t *testing.T) {
		wa := &WatchedAddress{
			Address:        watchAddr,
			ExpectedAmount: 100000,
			ExpiresAt:      time.Now().Add(-1 * time.Hour),
		}
		tx := mkTx(100000, 0)
		wa.TotalPaid.Store(AmountPaidTo(tx, wa.Address))
		status := m.determinePaymentStatus(wa, tx)
		if status != PaymentStatusExpired {
			t.Errorf("Expected expired, got %s", status)
		}
	})

	t.Run("partial payment ignores change", func(t *testing.T) {
		wa := &WatchedAddress{
			Address:        watchAddr,
			ExpectedAmount: 100000,
			ExpiresAt:      time.Now().Add(1 * time.Hour),
		}
		tx := mkTx(50000, 1000000)
		wa.TotalPaid.Store(AmountPaidTo(tx, wa.Address))
		status := m.determinePaymentStatus(wa, tx)
		if status != PaymentStatusPartial {
			t.Errorf("Expected partial, got %s", status)
		}
	})

	t.Run("genuine overpay", func(t *testing.T) {
		wa := &WatchedAddress{
			Address:        watchAddr,
			ExpectedAmount: 100000,
			ExpiresAt:      time.Now().Add(1 * time.Hour),
		}
		tx := mkTx(150000, 0)
		wa.TotalPaid.Store(AmountPaidTo(tx, wa.Address))
		status := m.determinePaymentStatus(wa, tx)
		if status != PaymentStatusOverpay {
			t.Errorf("Expected overpay, got %s", status)
		}
	})

	t.Run("two outputs to same address aggregate", func(t *testing.T) {
		wa := &WatchedAddress{
			Address:        watchAddr,
			ExpectedAmount: 100000,
			ExpiresAt:      time.Now().Add(1 * time.Hour),
		}
		tx := &iwallet.Transaction{
			To: []iwallet.SpendInfo{
				{Address: iwallet.NewAddress(watchAddr, btcCoin), Amount: iwallet.NewAmount(60000)},
				{Address: iwallet.NewAddress(watchAddr, btcCoin), Amount: iwallet.NewAmount(40000)},
				{Address: iwallet.NewAddress(changeAddr, btcCoin), Amount: iwallet.NewAmount(900000)},
			},
		}
		if got := AmountPaidTo(tx, watchAddr); got != 100000 {
			t.Fatalf("AmountPaidTo expected 100000, got %d", got)
		}
		wa.TotalPaid.Store(AmountPaidTo(tx, wa.Address))
		status := m.determinePaymentStatus(wa, tx)
		if status != PaymentStatusNormal {
			t.Errorf("Expected normal, got %s", status)
		}
	})

	t.Run("no expected amount", func(t *testing.T) {
		wa := &WatchedAddress{
			Address:        watchAddr,
			ExpectedAmount: 0,
			ExpiresAt:      time.Now().Add(1 * time.Hour),
		}
		tx := mkTx(100000, 0)
		status := m.determinePaymentStatus(wa, tx)
		if status != PaymentStatusNormal {
			t.Errorf("Expected normal when no expected amount, got %s", status)
		}
	})
}

func TestMonitorCalculatePollInterval(t *testing.T) {
	m := NewMonitor(nil)
	now := time.Now()

	tests := []struct {
		name       string
		age        time.Duration
		subscribed bool
		minExpect  time.Duration
		maxExpect  time.Duration
	}{
		{"fresh address", 1 * time.Minute, false, 30 * time.Second, 30 * time.Second},
		{"5-30 min old", 10 * time.Minute, false, 1 * time.Minute, 1 * time.Minute},
		{"30min-2h old", 1 * time.Hour, false, 5 * time.Minute, 5 * time.Minute},
		{"2-12h old", 6 * time.Hour, false, 15 * time.Minute, 15 * time.Minute},
		{"12-24h old", 18 * time.Hour, false, 30 * time.Minute, 30 * time.Minute},
		{"over 24h old", 30 * time.Hour, false, 1 * time.Hour, 1 * time.Hour},
		{"subscribed - 5x slower", 1 * time.Minute, true, 150 * time.Second, 150 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wa := &WatchedAddress{
				CreatedAt:  now.Add(-tt.age),
				Subscribed: tt.subscribed,
			}
			interval := m.calculatePollInterval(wa, now)
			if interval < tt.minExpect || interval > tt.maxExpect {
				t.Errorf("Expected interval between %v and %v, got %v", tt.minExpect, tt.maxExpect, interval)
			}
		})
	}
}

func TestMonitorShouldPoll(t *testing.T) {
	m := NewMonitor(nil)
	now := time.Now()

	t.Run("never polled", func(t *testing.T) {
		wa := &WatchedAddress{
			CreatedAt:  now,
			LastPolled: time.Time{}, // Never polled
		}
		if !m.shouldPoll(wa, now) {
			t.Error("Should poll address that was never polled")
		}
	})

	t.Run("recently polled", func(t *testing.T) {
		wa := &WatchedAddress{
			CreatedAt:  now.Add(-1 * time.Minute),
			LastPolled: now.Add(-10 * time.Second), // Just polled
		}
		if m.shouldPoll(wa, now) {
			t.Error("Should not poll recently polled address")
		}
	})

	t.Run("poll interval elapsed", func(t *testing.T) {
		wa := &WatchedAddress{
			CreatedAt:  now.Add(-1 * time.Minute),
			LastPolled: now.Add(-1 * time.Minute), // Polled a minute ago
		}
		if !m.shouldPoll(wa, now) {
			t.Error("Should poll address when interval elapsed")
		}
	})
}

func TestMonitorHandleTransaction(t *testing.T) {
	m := NewMonitor(nil)

	// Subscribe to transactions
	ch := m.SubscribeTransactions()

	callbackCalled := false
	var callbackStatus PaymentStatus

	wa := &WatchedAddress{
		Address:        "test_addr",
		ChainType:      iwallet.ChainBitcoin,
		OrderID:        "order123",
		ExpectedAmount: 100000,
		ExpiresAt:      time.Now().Add(1 * time.Hour),
		OnPayment: func(tx *iwallet.Transaction, status PaymentStatus) {
			callbackCalled = true
			callbackStatus = status
		},
	}

	tx := &iwallet.Transaction{
		ID:    "txid123",
		Value: iwallet.NewAmount(100000),
		To: []iwallet.SpendInfo{
			{
				Address: iwallet.NewAddress("test_addr", iwallet.CoinType("BTC")),
				Amount:  iwallet.NewAmount(100000),
			},
		},
		Height: 3,
	}

	// Handle transaction
	m.handleTransaction(wa, tx)

	// Verify callback was called
	if !callbackCalled {
		t.Error("OnPayment callback should be called")
	}
	if callbackStatus != PaymentStatusNormal {
		t.Errorf("Expected normal status, got %s", callbackStatus)
	}

	// Verify broadcast
	select {
	case received := <-ch:
		if string(received.ID) != "txid123" {
			t.Errorf("Expected txid=txid123, got %s", received.ID)
		}
	case <-time.After(1 * time.Second):
		t.Error("Transaction should be broadcast to subscribers")
	}
}

func TestMonitorHandleTransaction_ReemitsConfirmedDuplicateWithoutDoubleCounting(t *testing.T) {
	m := NewMonitor(nil)

	callbacks := 0
	var statuses []PaymentStatus
	wa := &WatchedAddress{
		Address:        "test_addr",
		ChainType:      iwallet.ChainBitcoin,
		OrderID:        "order123",
		ExpectedAmount: 100000,
		ExpiresAt:      time.Now().Add(1 * time.Hour),
		OnPayment: func(tx *iwallet.Transaction, status PaymentStatus) {
			callbacks++
			statuses = append(statuses, status)
		},
	}
	baseTx := &iwallet.Transaction{
		ID:    "txid123",
		Value: iwallet.NewAmount(49_999_998_350),
		To: []iwallet.SpendInfo{
			{
				Address: iwallet.NewAddress("test_addr", iwallet.CoinType("BTC")),
				Amount:  iwallet.NewAmount(100000),
			},
			{
				Address: iwallet.NewAddress("change_addr", iwallet.CoinType("BTC")),
				Amount:  iwallet.NewAmount(49_999_898_350),
			},
		},
		Height: 0,
	}

	m.handleTransaction(wa, baseTx)

	confirmed := *baseTx
	confirmed.Height = 7
	m.handleTransaction(wa, &confirmed)
	m.handleTransaction(wa, &confirmed)

	// Confirmed facts are periodically redelivered so an idempotent consumer
	// can recover from a transient persistence failure without double-counting.
	dedupeKey := wa.Address + ":" + string(confirmed.ID)
	m.seenTxsMu.Lock()
	state := m.seenTxs[dedupeKey]
	state.lastDelivered = time.Now().Add(-m.confirmedRedeliveryInterval())
	m.seenTxs[dedupeKey] = state
	m.seenTxsMu.Unlock()
	m.handleTransaction(wa, &confirmed)

	confirmedAtLaterHeight := confirmed
	confirmedAtLaterHeight.Height = 8
	m.handleTransaction(wa, &confirmedAtLaterHeight)

	if callbacks != 3 {
		t.Fatalf("callbacks = %d, want 3 (mempool + first confirmation + periodic confirmed retry)", callbacks)
	}
	if wa.TotalPaid.Load() != 100000 {
		t.Fatalf("TotalPaid = %d, want 100000 without duplicate confirmation counting", wa.TotalPaid.Load())
	}
	for i, status := range statuses {
		if status != PaymentStatusNormal {
			t.Fatalf("status[%d] = %s, want normal", i, status)
		}
	}
}

func TestMonitorPollAddress(t *testing.T) {
	m := NewMonitor(nil)

	mock := newMockPaymentSource(true)
	mock.transactions = []*iwallet.Transaction{
		{ID: "tx1", Value: iwallet.NewAmount(100000), Height: 1},
	}
	m.AddSource(iwallet.ChainBitcoin, mock)

	callbackCalled := false
	wa := &WatchedAddress{
		Address:   "test_addr",
		ChainType: iwallet.ChainBitcoin,
		OnPayment: func(tx *iwallet.Transaction, status PaymentStatus) {
			callbackCalled = true
		},
	}

	m.watchMu.Lock()
	m.watching[wa.Address] = wa
	m.watchMu.Unlock()

	// Poll the address
	m.pollAddress(wa)

	// Verify callback was called
	if !callbackCalled {
		t.Error("OnPayment callback should be called during polling")
	}

	// Verify LastPolled was updated
	if wa.LastPolled.IsZero() {
		t.Error("LastPolled should be updated")
	}
}

func TestMonitorPollAllAddresses_RemovesExpired(t *testing.T) {
	m := NewMonitor(&MonitorConfig{
		GracePeriod: 1 * time.Hour,
	})

	// Add an expired address (past grace period)
	expiredWa := &WatchedAddress{
		Address:        "expired_addr",
		ChainType:      iwallet.ChainBitcoin,
		CreatedAt:      time.Now().Add(-48 * time.Hour),
		ExpiresAt:      time.Now().Add(-26 * time.Hour), // Expired 26h ago
		GracePeriodEnd: time.Now().Add(-25 * time.Hour), // Grace period ended 25h ago
	}

	// Add a valid address
	validWa := &WatchedAddress{
		Address:        "valid_addr",
		ChainType:      iwallet.ChainBitcoin,
		CreatedAt:      time.Now(),
		ExpiresAt:      time.Now().Add(24 * time.Hour),
		GracePeriodEnd: time.Now().Add(25 * time.Hour),
	}

	m.watchMu.Lock()
	m.watching[expiredWa.Address] = expiredWa
	m.watching[validWa.Address] = validWa
	m.watchMu.Unlock()

	// Poll all addresses
	m.pollAllAddresses()

	m.watchMu.RLock()
	_, expiredExists := m.watching["expired_addr"]
	_, validExists := m.watching["valid_addr"]
	m.watchMu.RUnlock()

	if expiredExists {
		t.Error("Expired address should be removed")
	}
	if !validExists {
		t.Error("Valid address should not be removed")
	}
}

func TestMonitor_SourceHealthRecovery_ResubscribesAddresses(t *testing.T) {
	source := newMockPaymentSource(true)
	config := DefaultMonitorConfig()
	config.PollInterval = 50 * time.Millisecond
	m := NewMonitor(config)
	m.AddSource(iwallet.ChainBitcoin, source)

	wa := &WatchedAddress{
		Address:   "recovery_addr_1",
		ChainType: iwallet.ChainBitcoin,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	if err := m.WatchAddress(wa); err != nil {
		t.Fatalf("WatchAddress failed: %v", err)
	}

	// Clear subscribe calls from initial WatchAddress
	source.mu.Lock()
	source.subscribeCalls = nil
	source.mu.Unlock()

	// Simulate source going unhealthy then recovering
	source.setHealthy(false)
	m.checkSourceHealth() // records unhealthy state

	// Mark address as un-subscribed (simulates subscription lost during disconnect)
	m.watchMu.Lock()
	wa.Subscribed = false
	m.watchMu.Unlock()

	source.setHealthy(true)
	m.checkSourceHealth() // detects recovery → re-subscribes

	// Verify re-subscription happened
	source.mu.RLock()
	calls := source.subscribeCalls
	source.mu.RUnlock()

	if len(calls) != 1 || calls[0] != "recovery_addr_1" {
		t.Errorf("Expected 1 re-subscribe call for recovery_addr_1, got %v", calls)
	}

	m.watchMu.RLock()
	subscribed := wa.Subscribed
	m.watchMu.RUnlock()

	if !subscribed {
		t.Error("WatchedAddress should be marked as Subscribed after recovery")
	}
}

func TestMonitor_SourceHealthRecovery_SkipsAlreadySubscribed(t *testing.T) {
	source := newMockPaymentSource(true)
	config := DefaultMonitorConfig()
	m := NewMonitor(config)
	m.AddSource(iwallet.ChainBitcoin, source)

	wa := &WatchedAddress{
		Address:    "already_sub_addr",
		ChainType:  iwallet.ChainBitcoin,
		Subscribed: true,
		CreatedAt:  time.Now(),
		ExpiresAt:  time.Now().Add(24 * time.Hour),
	}
	m.watchMu.Lock()
	m.watching[wa.Address] = wa
	m.watchMu.Unlock()

	source.mu.Lock()
	source.subscribeCalls = nil
	source.mu.Unlock()

	// Simulate unhealthy → healthy transition
	source.setHealthy(false)
	m.checkSourceHealth()
	source.setHealthy(true)
	m.checkSourceHealth()

	source.mu.RLock()
	calls := source.subscribeCalls
	source.mu.RUnlock()

	if len(calls) != 0 {
		t.Errorf("Should not re-subscribe already-subscribed addresses, got %d calls", len(calls))
	}
}

func TestMonitor_SourceHealthRecovery_NoTransitionNoResubscribe(t *testing.T) {
	source := newMockPaymentSource(true)
	config := DefaultMonitorConfig()
	m := NewMonitor(config)
	m.AddSource(iwallet.ChainBitcoin, source)

	wa := &WatchedAddress{
		Address:    "stable_addr",
		ChainType:  iwallet.ChainBitcoin,
		Subscribed: false,
		CreatedAt:  time.Now(),
		ExpiresAt:  time.Now().Add(24 * time.Hour),
	}
	m.watchMu.Lock()
	m.watching[wa.Address] = wa
	m.watchMu.Unlock()

	source.mu.Lock()
	source.subscribeCalls = nil
	source.mu.Unlock()

	// Source stays healthy — no transition
	m.checkSourceHealth() // first check: records healthy
	m.checkSourceHealth() // second check: healthy→healthy, no transition

	source.mu.RLock()
	calls := source.subscribeCalls
	source.mu.RUnlock()

	if len(calls) != 0 {
		t.Errorf("Should not re-subscribe when no unhealthy→healthy transition, got %d calls", len(calls))
	}
}
