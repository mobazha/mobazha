package core

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"testing"
	"time"

	corepayment "github.com/mobazha/mobazha/internal/core/payment"
	utils "github.com/mobazha/mobazha/internal/orders/testutil"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha/pkg/payment"
	pkgutxo "github.com/mobazha/mobazha/pkg/utxo"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========== Mock Payment Source for Testing ==========

func TestMobazhaNodeElectrumOverrides_MapsConfiguredUTXOEndpoints(t *testing.T) {
	node := &MobazhaNode{chainFields: chainFields{
		electrumEndpoints: map[string]string{
			"btc": "electrs:50001",
			"xmr": "ignored:50001",
		},
		electrumFingerprints: map[string]string{"btc": "abc123"},
	}}

	overrides := node.electrumOverrides()
	require.Len(t, overrides, 1)
	bitcoin := overrides[iwallet.ChainBitcoin]
	assert.Equal(t, []string{"electrs:50001"}, bitcoin.Servers)
	assert.True(t, bitcoin.UseTLS)
	assert.Equal(t, "abc123", bitcoin.TLSFingerprint)
}

// mockUTXOPaymentSource is a mock implementation of pkgutxo.PaymentSource for testing
type mockUTXOPaymentSource struct {
	mu            sync.RWMutex
	healthy       bool
	chain         iwallet.ChainType
	transactions  map[string][]*iwallet.Transaction // address -> transactions
	callbacks     map[string]func(tx *iwallet.Transaction)
	feeRate       uint64
	subscribeErr  error
	getTxErr      error
	confirmations int
}

func newMockUTXOPaymentSource(chain iwallet.ChainType) *mockUTXOPaymentSource {
	return &mockUTXOPaymentSource{
		healthy:      true,
		chain:        chain,
		transactions: make(map[string][]*iwallet.Transaction),
		callbacks:    make(map[string]func(tx *iwallet.Transaction)),
		feeRate:      10,
	}
}

func (m *mockUTXOPaymentSource) Subscribe(ctx context.Context, address string, scriptPubKey []byte, callback func(tx *iwallet.Transaction)) error {
	if m.subscribeErr != nil {
		return m.subscribeErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callbacks[address] = callback
	return nil
}

func (m *mockUTXOPaymentSource) Unsubscribe(ctx context.Context, address string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.callbacks, address)
	return nil
}

func (m *mockUTXOPaymentSource) GetTransactions(ctx context.Context, address string, scriptPubKey []byte) ([]*iwallet.Transaction, error) {
	if m.getTxErr != nil {
		return nil, m.getTxErr
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.transactions[address], nil
}

func (m *mockUTXOPaymentSource) GetTransaction(ctx context.Context, txid string) (*iwallet.Transaction, error) {
	if m.getTxErr != nil {
		return nil, m.getTxErr
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, txs := range m.transactions {
		for _, tx := range txs {
			if string(tx.ID) == txid {
				return tx, nil
			}
		}
	}
	return nil, errors.New("transaction not found")
}

func (m *mockUTXOPaymentSource) IsHealthy() bool {
	return m.healthy
}

func (m *mockUTXOPaymentSource) Chain() iwallet.ChainType {
	return m.chain
}

func (m *mockUTXOPaymentSource) Close() error {
	return nil
}

func (m *mockUTXOPaymentSource) EstimateFee(ctx context.Context, targetBlocks int) (uint64, error) {
	return m.feeRate, nil
}

func (m *mockUTXOPaymentSource) BroadcastTransaction(ctx context.Context, txHex string) (string, error) {
	return "mock_broadcast_txid", nil
}

func (m *mockUTXOPaymentSource) ListUnspent(_ context.Context, _ []byte) ([]pkgutxo.UnspentOutput, error) {
	return nil, nil
}

func (m *mockUTXOPaymentSource) GetTxConfirmations(_ context.Context, _ string) (int, error) {
	return m.confirmations, nil
}

// SimulatePayment simulates a payment being detected
func (m *mockUTXOPaymentSource) SimulatePayment(address string, tx *iwallet.Transaction) {
	m.mu.Lock()
	m.transactions[address] = append(m.transactions[address], tx)
	callback := m.callbacks[address]
	m.mu.Unlock()

	if callback != nil {
		callback(tx)
	}
}

// AddTransaction adds a transaction for an address (without triggering callback)
func (m *mockUTXOPaymentSource) AddTransaction(address string, tx *iwallet.Transaction) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.transactions[address] = append(m.transactions[address], tx)
}

// ========== Helper Functions ==========

// generateMockTransaction creates a mock transaction for testing
func generateMockTransaction(toAddress string, amount uint64) *iwallet.Transaction {
	txidBytes := make([]byte, 32)
	rand.Read(txidBytes)

	fromIDBytes := make([]byte, 36)
	rand.Read(fromIDBytes)

	fromAddrBytes := make([]byte, 20)
	rand.Read(fromAddrBytes)

	addr := iwallet.NewAddress(toAddress, iwallet.CtMock)

	return &iwallet.Transaction{
		ID:    iwallet.TransactionID(hex.EncodeToString(txidBytes)),
		Value: iwallet.NewAmount(amount),
		From: []iwallet.SpendInfo{{
			ID:      fromIDBytes,
			Amount:  iwallet.NewAmount(amount + 1000),
			Address: iwallet.NewAddress(hex.EncodeToString(fromAddrBytes), iwallet.CtMock),
		}},
		To: []iwallet.SpendInfo{{
			Address: addr,
			Amount:  iwallet.NewAmount(amount),
			ID:      append(txidBytes, []byte{0x00, 0x00, 0x00, 0x00}...),
		}},
		Timestamp: time.Now(),
	}
}

// createTestOrder creates a test order with common fields set
func createTestOrder(id string, paymentAddr string, expectedAmount uint64) *models.Order {
	order := &models.Order{}
	order.ID = models.OrderID(id)
	order.PaymentAddress = paymentAddr
	_ = order.SetPendingPaymentInfo(&models.PendingUTXOPaymentInfo{
		Coin:   string(iwallet.CtMock),
		Amount: expectedAmount,
	})

	orderOpen := &pb.OrderOpen{}
	_ = order.PutMessage(utils.MustWrapOrderMessage(orderOpen))
	return order
}

// setupNodeWithMonitor creates a MockNode with UTXO monitor initialized
func setupNodeWithMonitor(t *testing.T) (*MobazhaNode, *mockUTXOPaymentSource) {
	node, err := MockNode()
	require.NoError(t, err)

	mockSource := newMockUTXOPaymentSource(iwallet.ChainMock)

	// Create monitor and add mock source
	monitor := pkgutxo.NewMonitor(pkgutxo.DefaultMonitorConfig())
	monitor.AddSource(iwallet.ChainMock, mockSource)
	node.SetUTXOMonitor(monitor)
	monitor.Start()

	time.Sleep(100 * time.Millisecond)
	return node, mockSource
}

// ========== Unit Tests ==========

// TestUTXOMonitor_StartStop tests monitor start and stop
func TestUTXOMonitor_StartStop(t *testing.T) {
	monitor := pkgutxo.NewMonitor(&pkgutxo.MonitorConfig{
		PollInterval: 100 * time.Millisecond,
	})

	mockSource := newMockUTXOPaymentSource(iwallet.ChainMock)
	monitor.AddSource(iwallet.ChainMock, mockSource)

	monitor.Start()
	time.Sleep(50 * time.Millisecond)

	// Multiple Stop calls should not panic (sync.Once protection)
	monitor.Stop()
	monitor.Stop() // Second call should be no-op
}

// TestUTXOMonitor_WatchAddress tests address watching
func TestUTXOMonitor_WatchAddress(t *testing.T) {
	monitor := pkgutxo.NewMonitor(&pkgutxo.MonitorConfig{
		PollInterval: 100 * time.Millisecond,
	})

	mockSource := newMockUTXOPaymentSource(iwallet.ChainMock)
	monitor.AddSource(iwallet.ChainMock, mockSource)

	wa := &pkgutxo.WatchedAddress{
		Address:        "test_address_123",
		ChainType:      iwallet.ChainMock,
		OrderID:        "order_123",
		ExpectedAmount: 100000,
		CreatedAt:      time.Now(),
		ExpiresAt:      time.Now().Add(corepayment.AddressMonitorDuration),
	}

	err := monitor.WatchAddress(wa)
	require.NoError(t, err)
	assert.True(t, wa.Subscribed, "Address should be marked as subscribed")
}

// TestUTXOMonitor_PaymentDetection tests payment detection via polling
func TestUTXOMonitor_PaymentDetection(t *testing.T) {
	monitor := pkgutxo.NewMonitor(&pkgutxo.MonitorConfig{
		PollInterval: 50 * time.Millisecond,
	})

	mockSource := newMockUTXOPaymentSource(iwallet.ChainMock)
	monitor.AddSource(iwallet.ChainMock, mockSource)

	txChan := monitor.SubscribeTransactions()

	address := "payment_address_456"
	wa := &pkgutxo.WatchedAddress{
		Address:        address,
		ChainType:      iwallet.ChainMock,
		OrderID:        "order_456",
		ExpectedAmount: 100000,
		CreatedAt:      time.Now(),
		ExpiresAt:      time.Now().Add(corepayment.AddressMonitorDuration),
	}

	err := monitor.WatchAddress(wa)
	require.NoError(t, err)

	monitor.Start()
	defer monitor.Stop()

	// Add transaction (will be detected via polling)
	mockTx := generateMockTransaction(address, 100000)
	mockSource.AddTransaction(address, mockTx)

	// Wait for transaction detection
	select {
	case tx := <-txChan:
		assert.Equal(t, string(mockTx.ID), string(tx.ID))
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for transaction detection")
	}
}

// TestChainMock_IsUTXOChain verifies ChainMock is now a UTXO chain for testing
func TestChainMock_IsUTXOChain(t *testing.T) {
	assert.True(t, iwallet.ChainMock.IsUTXOChain(), "ChainMock should be UTXO chain for testing")
	assert.True(t, iwallet.ChainBitcoin.IsUTXOChain(), "ChainBitcoin should be UTXO chain")
	assert.True(t, iwallet.ChainLitecoin.IsUTXOChain(), "ChainLitecoin should be UTXO chain")
	assert.False(t, iwallet.ChainEthereum.IsUTXOChain(), "ChainEthereum should NOT be UTXO chain")
}

// TestMobazhaNode_StartUTXOMonitorWithMockSources tests starting monitor with mock sources
func TestMobazhaNode_StartUTXOMonitorWithMockSources(t *testing.T) {
	node, err := MockNode()
	require.NoError(t, err)
	defer node.DestroyNode()

	mockSource := newMockUTXOPaymentSource(iwallet.ChainMock)

	// Create monitor and add mock source
	monitor := pkgutxo.NewMonitor(pkgutxo.DefaultMonitorConfig())
	monitor.AddSource(iwallet.ChainMock, mockSource)
	node.SetUTXOMonitor(monitor)
	monitor.Start()

	// Verify monitor is initialized
	assert.NotNil(t, node.GetMonitorService(), "UTXO monitor should be initialized")

	// Allow goroutines to start properly before stopping
	time.Sleep(200 * time.Millisecond)

	// Stop monitor - this triggers n.shutdown which will be handled by the goroutines
	node.StopUTXOPaymentMonitor()

	// Wait for graceful shutdown
	time.Sleep(200 * time.Millisecond)
}

// TestMobazhaNode_GetTotalPaidToAddress tests calculating total paid amount
func TestMobazhaNode_GetTotalPaidToAddress(t *testing.T) {
	node, err := MockNode()
	require.NoError(t, err)
	defer node.DestroyNode()

	// Create order with transactions
	order := &models.Order{}
	order.ID = models.OrderID("test_order_123")
	order.PaymentAddress = "test_payment_address"

	orderOpen := &pb.OrderOpen{}
	require.NoError(t, order.PutMessage(utils.MustWrapOrderMessage(orderOpen)))

	// Add some transactions
	tx1 := generateMockTransaction(order.PaymentAddress, 50000)
	tx1.To[0].Address = iwallet.NewAddress(order.PaymentAddress, iwallet.CtMock)
	require.NoError(t, order.PutTransaction(*tx1))

	tx2 := generateMockTransaction(order.PaymentAddress, 30000)
	tx2.To[0].Address = iwallet.NewAddress(order.PaymentAddress, iwallet.CtMock)
	require.NoError(t, order.PutTransaction(*tx2))

	// Save order
	err = node.repo.DB().Update(func(dbtx database.Tx) error {
		return dbtx.Save(order)
	})
	require.NoError(t, err)

	// Get total paid
	totalPaid, err := node.Wallet().GetTotalPaidToAddress(order)
	require.NoError(t, err)
	assert.Equal(t, uint64(80000), totalPaid, "Total should be 50000 + 30000 = 80000")
}

// ========== High Priority Tests ==========

// TestPaymentCalculation tests partial, full, and excess payment scenarios using table-driven tests
func TestPaymentCalculation(t *testing.T) {
	tests := []struct {
		name           string
		expectedAmount uint64
		payments       []uint64
		wantTotal      uint64
		wantPartial    bool // totalPaid < expected
		wantExcess     bool // totalPaid > expected
	}{
		{
			name:           "partial_single",
			expectedAmount: 100000,
			payments:       []uint64{50000},
			wantTotal:      50000,
			wantPartial:    true,
		},
		{
			name:           "partial_multiple",
			expectedAmount: 100000,
			payments:       []uint64{30000, 20000},
			wantTotal:      50000,
			wantPartial:    true,
		},
		{
			name:           "exact_single",
			expectedAmount: 100000,
			payments:       []uint64{100000},
			wantTotal:      100000,
		},
		{
			name:           "exact_multiple",
			expectedAmount: 100000,
			payments:       []uint64{30000, 40000, 30000},
			wantTotal:      100000,
		},
		{
			name:           "excess_single",
			expectedAmount: 50000,
			payments:       []uint64{75000},
			wantTotal:      75000,
			wantExcess:     true,
		},
		{
			name:           "excess_multiple",
			expectedAmount: 50000,
			payments:       []uint64{30000, 30000},
			wantTotal:      60000,
			wantExcess:     true,
		},
	}

	node, err := MockNode()
	require.NoError(t, err)
	defer node.DestroyNode()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			order := createTestOrder(tt.name+"_order", tt.name+"_addr", tt.expectedAmount)

			// Add payments
			for _, amount := range tt.payments {
				tx := generateMockTransaction(order.PaymentAddress, amount)
				require.NoError(t, order.PutTransaction(*tx))
			}

			err := node.repo.DB().Update(func(dbtx database.Tx) error {
				return dbtx.Save(order)
			})
			require.NoError(t, err)

			totalPaid, err := node.Wallet().GetTotalPaidToAddress(order)
			require.NoError(t, err)
			assert.Equal(t, tt.wantTotal, totalPaid)

			if tt.wantPartial {
				assert.Less(t, totalPaid, tt.expectedAmount, "Should be partial payment")
			}
			if tt.wantExcess {
				assert.Greater(t, totalPaid, tt.expectedAmount, "Should be excess payment")
			}
		})
	}
}

// ========== Medium Priority Tests ==========

// TestExchangeRateLocking tests that exchange rate is locked on first payment request
func TestExchangeRateLocking(t *testing.T) {
	node, err := MockNode()
	require.NoError(t, err)
	defer node.DestroyNode()

	// Create order without locked amount (initial state)
	order := &models.Order{}
	order.ID = models.OrderID("rate_lock_order")
	order.PaymentAddress = "rate_lock_address"
	_ = order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))

	// Initial state - no locked amount
	pendingInfo, _ := order.GetPendingPaymentInfo()
	assert.Nil(t, pendingInfo)

	// Simulate first payment info request locking the rate
	_ = order.SetPendingPaymentInfo(&models.PendingUTXOPaymentInfo{
		Coin:   string(iwallet.CtMock),
		Amount: 100000,
	})

	err = node.repo.DB().Update(func(dbtx database.Tx) error { return dbtx.Save(order) })
	require.NoError(t, err)

	// Verify rate persists after reload
	var savedOrder models.Order
	err = node.repo.DB().View(func(dbtx database.Tx) error {
		return dbtx.Read().Where("id = ?", order.ID).First(&savedOrder).Error
	})
	require.NoError(t, err)

	savedInfo, _ := savedOrder.GetPendingPaymentInfo()
	require.NotNil(t, savedInfo)
	assert.Equal(t, string(iwallet.CtMock), savedInfo.Coin)
	assert.Equal(t, uint64(100000), savedInfo.Amount)
}

// TestCoinSwitchDetection tests detecting coin switch with partial payment
func TestCoinSwitchDetection(t *testing.T) {
	node, err := MockNode()
	require.NoError(t, err)
	defer node.DestroyNode()

	// Order with BTC payment
	order := createTestOrder("coin_switch", "btc_addr", 100000)
	_ = order.SetPendingPaymentInfo(&models.PendingUTXOPaymentInfo{
		Coin:   "BTC",
		Amount: 100000,
	})

	tx := generateMockTransaction(order.PaymentAddress, 50000)
	require.NoError(t, order.PutTransaction(*tx))

	err = node.repo.DB().Update(func(dbtx database.Tx) error { return dbtx.Save(order) })
	require.NoError(t, err)

	// Detect switch to LTC
	newCoin := iwallet.CoinType("crypto:bip122:12a765e31ffd4059bada1e25190f6e98:native")
	pendingInfo, _ := order.GetPendingPaymentInfo()
	hasCoinSwitch := pendingInfo != nil && pendingInfo.Coin != "" && pendingInfo.Coin != string(newCoin)
	assert.True(t, hasCoinSwitch, "Should detect coin switch from BTC to LTC")

	// Verify partial payment exists
	totalPaid, err := node.Wallet().GetTotalPaidToAddress(order)
	require.NoError(t, err)
	assert.Greater(t, totalPaid, uint64(0))
}

// TestStopWatchingPayment tests stopping watch behavior with and without payment
func TestStopWatchingPayment(t *testing.T) {
	tests := []struct {
		name              string
		hasPayment        bool
		expectCoinCleared bool
		expectAddrCleared bool
	}{
		{
			name:              "no_payment_clears_info",
			hasPayment:        false,
			expectCoinCleared: true,
			expectAddrCleared: true,
		},
		{
			name:              "with_payment_preserves_info",
			hasPayment:        true,
			expectCoinCleared: false,
			expectAddrCleared: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node, _ := setupNodeWithMonitor(t)
			defer node.StopUTXOPaymentMonitor()
			defer node.DestroyNode()

			order := createTestOrder(tt.name+"_order", tt.name+"_addr", 100000)
			// Update pending info with ScriptPubKey
			_ = order.SetPendingPaymentInfo(&models.PendingUTXOPaymentInfo{
				Coin:         string(iwallet.CtMock),
				Amount:       100000,
				ScriptPubKey: []byte{0x00, 0x14},
			})

			if tt.hasPayment {
				tx := generateMockTransaction(order.PaymentAddress, 50000)
				require.NoError(t, order.PutTransaction(*tx))
			}

			err := node.repo.DB().Update(func(dbtx database.Tx) error {
				return dbtx.Save(order)
			})
			require.NoError(t, err)

			err = node.StopWatchingPayment(order.ID.String())
			require.NoError(t, err)

			var savedOrder models.Order
			err = node.repo.DB().View(func(dbtx database.Tx) error {
				return dbtx.Read().Where("id = ?", order.ID).First(&savedOrder).Error
			})
			require.NoError(t, err)

			savedInfo, _ := savedOrder.GetPendingPaymentInfo()
			if tt.expectCoinCleared {
				assert.True(t, savedInfo == nil || savedInfo.Coin == "")
			} else {
				require.NotNil(t, savedInfo)
				assert.Equal(t, string(iwallet.CtMock), savedInfo.Coin)
				assert.Equal(t, uint64(100000), savedInfo.Amount)
			}

			if tt.expectAddrCleared {
				assert.Empty(t, savedOrder.PaymentAddress)
			}
		})
	}
}

// TestWatchPaymentAddress tests watching a payment address
func TestWatchPaymentAddress(t *testing.T) {
	node, _ := setupNodeWithMonitor(t)
	defer node.StopUTXOPaymentMonitor()
	defer node.DestroyNode()

	orderID := "watch_test_order"
	address := "watch_test_address"

	err := node.WatchPaymentAddress(orderID, address, iwallet.ChainMock, []byte{0x00, 0x14})
	require.NoError(t, err)

	wa := node.GetMonitorService().GetWatchedAddress(address)
	require.NotNil(t, wa)
	assert.Equal(t, orderID, wa.OrderID)
	assert.Equal(t, iwallet.ChainMock, wa.ChainType)
}

// TestCheckOrderPendingPayment tests pending payment check conditions for buyer and vendor
func TestCheckOrderPendingPayment(t *testing.T) {
	t.Run("buyer_recovery", func(t *testing.T) {
		node, err := MockNode()
		require.NoError(t, err)
		defer node.DestroyNode()

		order := createTestOrder("buyer_recovery", "recovery_addr", 100000)
		order.State = models.OrderState_AWAITING_PAYMENT
		// Update pending info with ScriptPubKey
		_ = order.SetPendingPaymentInfo(&models.PendingUTXOPaymentInfo{
			Coin:         string(iwallet.CtMock),
			Amount:       100000,
			ScriptPubKey: []byte{0x00, 0x14},
		})
		order.SetRole(models.RoleBuyer)

		// Buyer should check when: AWAITING_PAYMENT + has pending payment info
		pendingInfo, _ := order.GetPendingPaymentInfo()
		shouldCheck := order.State == models.OrderState_AWAITING_PAYMENT &&
			order.Role() == models.RoleBuyer &&
			order.PaymentAddress != "" &&
			pendingInfo != nil && pendingInfo.Coin != ""
		assert.True(t, shouldCheck)
	})

	t.Run("vendor_confirm", func(t *testing.T) {
		node, err := MockNode()
		require.NoError(t, err)
		defer node.DestroyNode()

		order := &models.Order{}
		order.ID = models.OrderID("vendor_confirm")
		order.State = models.OrderState_PENDING
		order.SetRole(models.RoleVendor)
		_ = order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))
		_ = order.PutMessage(utils.MustWrapOrderMessage(&pb.PaymentSent{
			SettlementSpec: payment.NewUTXOSpec(false).ToPaymentSent(),
			Coin:           string(iwallet.CtMock),
		}))

		ps, err := order.PaymentSentMessage()
		require.NoError(t, err)

		// Vendor should check when: CANCELABLE + is vendor
		shouldCheck := ps.GetSettlementSpec() != nil && ps.GetSettlementSpec().GetMethod() == pb.PaymentSent_CANCELABLE && order.Role() == models.RoleVendor
		assert.True(t, shouldCheck)
	})
}

// Note: TestMobazhaNode_releaseFromCancelableAddress is defined in cancel_test.go
//
// TestExtractBuyerAddressFromRealNetwork lives in
// payment_monitor_utxo_integration_test.go behind the `integration` build
// tag because it requires outbound TLS to a public LTC testnet Electrum
// server. Run it with `go test -tags integration ./internal/core/...`.
