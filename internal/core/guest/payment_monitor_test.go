package guest

import (
	"context"
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/models"
)

// --- mock implementations ---

type mockBalanceChecker struct {
	mu       sync.Mutex
	balances map[string]*big.Int
}

func newMockBalanceChecker() *mockBalanceChecker {
	return &mockBalanceChecker{balances: make(map[string]*big.Int)}
}

func (m *mockBalanceChecker) setBalance(chainKey, addr string, bal *big.Int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.balances[chainKey+":"+addr] = bal
}

func (m *mockBalanceChecker) GetAddressBalance(_ context.Context, chainKey, address string) (*big.Int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if b, ok := m.balances[chainKey+":"+address]; ok {
		return new(big.Int).Set(b), nil
	}
	return big.NewInt(0), nil
}

type mockSolanaChecker struct {
	mu      sync.Mutex
	results map[string]string
}

func newMockSolanaChecker() *mockSolanaChecker {
	return &mockSolanaChecker{results: make(map[string]string)}
}

func (m *mockSolanaChecker) setResult(refKey, txHash string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.results[refKey] = txHash
}

func (m *mockSolanaChecker) FindTransferByReference(_ context.Context, referenceKey, _, _ string) (string, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if h, ok := m.results[referenceKey]; ok && h != "" {
		return h, true, nil
	}
	return "", false, nil
}

type recordingGuestService struct {
	mu       sync.Mutex
	detected []paymentDetection
}

type paymentDetection struct {
	orderToken string
	txHash     string
}

func (r *recordingGuestService) HandlePaymentDetected(orderToken, txHash string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.detected = append(r.detected, paymentDetection{orderToken, txHash})
	return nil
}

func (r *recordingGuestService) getDetections() []paymentDetection {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := make([]paymentDetection, len(r.detected))
	copy(cp, r.detected)
	return cp
}

func (r *recordingGuestService) CreateGuestOrder(context.Context, contracts.CreateGuestOrderRequest) (*contracts.GuestOrderResponse, error) {
	return nil, nil
}
func (r *recordingGuestService) GetGuestOrderStatus(context.Context, string) (*contracts.GuestOrderStatusResponse, error) {
	return nil, nil
}
func (r *recordingGuestService) ListGuestOrders(context.Context, contracts.GuestOrderFilter) ([]models.GuestOrder, int64, error) {
	return nil, 0, nil
}
func (r *recordingGuestService) ShipGuestOrder(context.Context, string, string, string) error {
	return nil
}
func (r *recordingGuestService) CompleteGuestOrder(context.Context, string) error { return nil }
func (r *recordingGuestService) HandleConfirmationUpdate(string, int) error       { return nil }
func (r *recordingGuestService) HandleLatePayment(string, string, string, uint64, uint64) error {
	return nil
}
func (r *recordingGuestService) CleanupExpiredOrders(context.Context)             {}
func (r *recordingGuestService) AutoCompleteOrders(context.Context)               {}
func (r *recordingGuestService) RunGuestCleanupOnce()                             {}
func (r *recordingGuestService) GetGuestCheckoutConfig(context.Context) (*models.GuestCheckoutConfig, error) {
	return nil, nil
}
func (r *recordingGuestService) IsEnabled(context.Context) bool { return true }
func (r *recordingGuestService) SaveGuestCheckoutConfig(context.Context, *models.GuestCheckoutConfig) error {
	return nil
}

// --- tests ---

func TestCheckEVMPayment_BalanceDetected(t *testing.T) {
	bc := newMockBalanceChecker()
	svc := &recordingGuestService{}
	monitor := NewGuestPaymentMonitor(nil, svc, bc, nil)

	order := &models.GuestOrder{
		OrderToken:     "gst_test1",
		PaymentCoin:    "crypto:eip155:1:native",
		PaymentAddress: "0xabc",
		PaymentAmount:  "1000000000000000000",
		ExpiresAt:      time.Now().Add(1 * time.Hour),
	}

	bc.setBalance("crypto:eip155:1:native", "0xabc", big.NewInt(0).SetUint64(1000000000000000000))

	found := monitor.checkBalancePayment(context.Background(), order)
	if !found {
		t.Fatal("expected payment to be detected")
	}

	detections := svc.getDetections()
	if len(detections) != 1 || detections[0].orderToken != "gst_test1" {
		t.Fatalf("expected 1 detection for gst_test1, got %v", detections)
	}
}

func TestCheckEVMPayment_InsufficientBalance(t *testing.T) {
	bc := newMockBalanceChecker()
	svc := &recordingGuestService{}
	monitor := NewGuestPaymentMonitor(nil, svc, bc, nil)

	order := &models.GuestOrder{
		OrderToken:     "gst_test2",
		PaymentCoin:    "crypto:eip155:1:native",
		PaymentAddress: "0xdef",
		PaymentAmount:  "2000000000000000000",
		ExpiresAt:      time.Now().Add(1 * time.Hour),
	}

	bc.setBalance("crypto:eip155:1:native", "0xdef", big.NewInt(1000000000000000000))

	found := monitor.checkBalancePayment(context.Background(), order)
	if found {
		t.Fatal("should not detect payment with insufficient balance")
	}

	if len(svc.getDetections()) != 0 {
		t.Fatal("no detection expected")
	}
}

func TestCheckEVMPayment_ZeroBalance(t *testing.T) {
	bc := newMockBalanceChecker()
	svc := &recordingGuestService{}
	monitor := NewGuestPaymentMonitor(nil, svc, bc, nil)

	order := &models.GuestOrder{
		OrderToken:     "gst_test3",
		PaymentCoin:    "crypto:eip155:1:native",
		PaymentAddress: "0x000",
		PaymentAmount:  "1000000000000000000",
		ExpiresAt:      time.Now().Add(1 * time.Hour),
	}

	found := monitor.checkBalancePayment(context.Background(), order)
	if found {
		t.Fatal("should not detect payment with zero balance")
	}
}

func TestCheckSolanaPayment_Found(t *testing.T) {
	sc := newMockSolanaChecker()
	svc := &recordingGuestService{}
	monitor := NewGuestPaymentMonitor(nil, svc, nil, sc)

	order := &models.GuestOrder{
		OrderToken:     "gst_sol1",
		PaymentCoin:    "SOL",
		PaymentAddress: "SomeSellerAddr",
		PaymentAmount:  "1000000000",
		ReferenceKey:   "refkey123",
		ExpiresAt:      time.Now().Add(1 * time.Hour),
	}

	sc.setResult("refkey123", "txhash_abc")

	found := monitor.checkSolanaPayment(context.Background(), order)
	if !found {
		t.Fatal("expected Solana payment to be detected")
	}

	detections := svc.getDetections()
	if len(detections) != 1 || detections[0].txHash != "txhash_abc" {
		t.Fatalf("expected detection with txhash_abc, got %v", detections)
	}
}

func TestCheckSolanaPayment_NotFound(t *testing.T) {
	sc := newMockSolanaChecker()
	svc := &recordingGuestService{}
	monitor := NewGuestPaymentMonitor(nil, svc, nil, sc)

	order := &models.GuestOrder{
		OrderToken:     "gst_sol2",
		PaymentCoin:    "SOL",
		PaymentAddress: "SomeAddr",
		PaymentAmount:  "1000000000",
		ReferenceKey:   "refkey_missing",
		ExpiresAt:      time.Now().Add(1 * time.Hour),
	}

	found := monitor.checkSolanaPayment(context.Background(), order)
	if found {
		t.Fatal("should not detect payment when reference not found")
	}
}

func TestWatchOrder_EVM_DetectsPayment(t *testing.T) {
	bc := newMockBalanceChecker()
	svc := &recordingGuestService{}
	monitor := NewGuestPaymentMonitor(nil, svc, bc, nil)

	order := &models.GuestOrder{
		OrderToken:     "gst_watch1",
		PaymentCoin:    "crypto:eip155:1:native",
		PaymentAddress: "0xwatch",
		PaymentAmount:  "500000000000000000",
		ExpiresAt:      time.Now().Add(1 * time.Hour),
	}

	monitor.WatchOrder(order)

	if monitor.ActiveWatchCount() != 1 {
		t.Fatalf("expected 1 active watch, got %d", monitor.ActiveWatchCount())
	}

	bc.setBalance("crypto:eip155:1:native", "0xwatch", big.NewInt(500000000000000000))

	time.Sleep(evmPollInterval + 2*time.Second)

	detections := svc.getDetections()
	if len(detections) == 0 {
		t.Fatal("expected payment detection after balance appeared")
	}

	time.Sleep(1 * time.Second)
	if monitor.ActiveWatchCount() != 0 {
		t.Fatalf("expected 0 watches after detection, got %d", monitor.ActiveWatchCount())
	}
}

func TestWatchOrder_DuplicateIgnored(t *testing.T) {
	bc := newMockBalanceChecker()
	svc := &recordingGuestService{}
	monitor := NewGuestPaymentMonitor(nil, svc, bc, nil)
	defer monitor.StopAll()

	order := &models.GuestOrder{
		OrderToken:     "gst_dup",
		PaymentCoin:    "crypto:eip155:1:native",
		PaymentAddress: "0xdup",
		PaymentAmount:  "100",
		ExpiresAt:      time.Now().Add(1 * time.Hour),
	}

	monitor.WatchOrder(order)
	monitor.WatchOrder(order)

	if monitor.ActiveWatchCount() != 1 {
		t.Fatalf("expected 1 watch (no duplicates), got %d", monitor.ActiveWatchCount())
	}
}

func TestStopAll_CancelsWatches(t *testing.T) {
	bc := newMockBalanceChecker()
	svc := &recordingGuestService{}
	monitor := NewGuestPaymentMonitor(nil, svc, bc, nil)

	for i := 0; i < 3; i++ {
		order := &models.GuestOrder{
			OrderToken:     "gst_stop" + string(rune('0'+i)),
			PaymentCoin:    "crypto:eip155:1:native",
			PaymentAddress: "0xstop" + string(rune('0'+i)),
			PaymentAmount:  "100",
			ExpiresAt:      time.Now().Add(1 * time.Hour),
		}
		monitor.WatchOrder(order)
	}

	if monitor.ActiveWatchCount() != 3 {
		t.Fatalf("expected 3 watches, got %d", monitor.ActiveWatchCount())
	}

	monitor.StopAll()
	time.Sleep(500 * time.Millisecond)

	if monitor.ActiveWatchCount() != 0 {
		t.Fatalf("expected 0 watches after StopAll, got %d", monitor.ActiveWatchCount())
	}
}

func TestStopAll_DoubleCallNoPanic(t *testing.T) {
	bc := newMockBalanceChecker()
	svc := &recordingGuestService{}
	monitor := NewGuestPaymentMonitor(nil, svc, bc, nil)

	order := &models.GuestOrder{
		OrderToken:     "gst_double",
		PaymentCoin:    "crypto:eip155:1:native",
		PaymentAddress: "0xdouble",
		PaymentAmount:  "100",
		ExpiresAt:      time.Now().Add(1 * time.Hour),
	}
	monitor.WatchOrder(order)
	monitor.StopAll()
	monitor.StopAll()
}

func TestParseGuestOrderState_CaseInsensitive(t *testing.T) {
	tests := []struct {
		input    string
		expected models.GuestOrderState
		ok       bool
	}{
		{"FUNDED", models.GuestOrderFunded, true},
		{"funded", models.GuestOrderFunded, true},
		{"Funded", models.GuestOrderFunded, true},
		{"awaiting_payment", models.GuestOrderAwaitingPayment, true},
		{"EXPIRED", models.GuestOrderExpired, true},
		{"invalid", -1, false},
	}
	for _, tc := range tests {
		got, ok := models.ParseGuestOrderState(tc.input)
		if ok != tc.ok || got != tc.expected {
			t.Errorf("ParseGuestOrderState(%q) = (%v, %v), want (%v, %v)",
				tc.input, got, ok, tc.expected, tc.ok)
		}
	}
}
