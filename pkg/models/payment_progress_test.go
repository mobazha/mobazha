package models

import (
	"strings"
	"testing"

	utils "github.com/mobazha/mobazha/internal/orders/testutil"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
)

// putOrderOpenWithAmount stamps an OrderOpen on the order whose only relevant
// field for the progress calculation is Amount. Returns the populated order.
func putOrderOpenWithAmount(t *testing.T, amount string) *Order {
	t.Helper()
	order := &Order{}
	if err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{
		Amount: amount,
	})); err != nil {
		t.Fatalf("PutMessage(OrderOpen) failed: %v", err)
	}
	return order
}

func TestComputePaymentProgress_NoOrderOpen_ReturnsNil(t *testing.T) {
	order := &Order{}
	order.TotalReceived = "100"
	if got := order.ComputePaymentProgress(); got != nil {
		t.Fatalf("expected nil (no OrderOpen), got %+v", got)
	}
}

func TestComputePaymentProgress_EmptyExpectedAmount_ReturnsNil(t *testing.T) {
	order := putOrderOpenWithAmount(t, "")
	order.TotalReceived = "1"
	if got := order.ComputePaymentProgress(); got != nil {
		t.Fatalf("expected nil (empty expected), got %+v", got)
	}
}

func TestComputePaymentProgress_ZeroExpectedAmount_ReturnsNil(t *testing.T) {
	order := putOrderOpenWithAmount(t, "0")
	order.TotalReceived = "0"
	if got := order.ComputePaymentProgress(); got != nil {
		t.Fatalf("expected nil (zero expected), got %+v", got)
	}
}

func TestComputePaymentProgress_NegativeExpectedAmount_ReturnsNil(t *testing.T) {
	order := putOrderOpenWithAmount(t, "-100")
	order.TotalReceived = "50"
	if got := order.ComputePaymentProgress(); got != nil {
		t.Fatalf("expected nil (negative expected), got %+v", got)
	}
}

func TestComputePaymentProgress_NonNumericExpected_ReturnsNil(t *testing.T) {
	order := putOrderOpenWithAmount(t, "not-a-number")
	order.TotalReceived = "100"
	if got := order.ComputePaymentProgress(); got != nil {
		t.Fatalf("expected nil (non-numeric expected), got %+v", got)
	}
}

func TestComputePaymentProgress_EmptyTotalReceived_ReturnsZero(t *testing.T) {
	order := putOrderOpenWithAmount(t, "1000")
	order.TotalReceived = ""

	got := order.ComputePaymentProgress()
	if got == nil {
		t.Fatal("expected non-nil progress")
	}
	if got.TotalReceived != "0" {
		t.Errorf("expected totalReceived=0, got %q", got.TotalReceived)
	}
	if got.ExpectedAmount != "1000" {
		t.Errorf("expected expectedAmount=1000, got %q", got.ExpectedAmount)
	}
	if got.Percentage != 0 {
		t.Errorf("expected percentage=0, got %d", got.Percentage)
	}
	if got.OverpaidAmount != "" {
		t.Errorf("expected overpaid empty, got %q", got.OverpaidAmount)
	}
}

func TestComputePaymentProgress_UsesPaymentSentAmountWhenPendingInfoIsGone(t *testing.T) {
	order := putOrderOpenWithAmount(t, "11000000000000000")
	order.TotalReceived = ""
	if err := order.PutMessage(utils.MustWrapOrderMessage(&pb.PaymentSent{
		Amount: "29838",
		Coin:   "crypto:bip122:000000000019d6689c085ae165831e93:native",
	})); err != nil {
		t.Fatalf("PutMessage(PaymentSent) failed: %v", err)
	}

	got := order.ComputePaymentProgress()
	if got == nil {
		t.Fatal("expected non-nil progress")
	}
	if got.TotalReceived != "0" {
		t.Errorf("expected totalReceived=0, got %q", got.TotalReceived)
	}
	if got.ExpectedAmount != "29838" {
		t.Errorf("expected expectedAmount=29838, got %q", got.ExpectedAmount)
	}
}

func TestComputePaymentProgress_PartialPayment_Computes50Percent(t *testing.T) {
	order := putOrderOpenWithAmount(t, "1000")
	order.TotalReceived = "500"

	got := order.ComputePaymentProgress()
	if got == nil {
		t.Fatal("expected non-nil progress")
	}
	if got.Percentage != 50 {
		t.Errorf("expected percentage=50, got %d", got.Percentage)
	}
	if got.TotalReceived != "500" || got.ExpectedAmount != "1000" {
		t.Errorf("unexpected totals: total=%q expected=%q", got.TotalReceived, got.ExpectedAmount)
	}
	if got.OverpaidAmount != "" {
		t.Errorf("partial path must not surface overpaid, got %q", got.OverpaidAmount)
	}
}

func TestComputePaymentProgress_PartialPayment_60PercentTruncates(t *testing.T) {
	// 6 / 10 = 60% (clean)
	order := putOrderOpenWithAmount(t, "10")
	order.TotalReceived = "6"

	got := order.ComputePaymentProgress()
	if got == nil {
		t.Fatal("expected non-nil progress")
	}
	if got.Percentage != 60 {
		t.Errorf("expected percentage=60, got %d", got.Percentage)
	}
}

func TestComputePaymentProgress_PartialPayment_FloorRounding(t *testing.T) {
	// 1 / 3 = 33.33% -> floor to 33
	order := putOrderOpenWithAmount(t, "3")
	order.TotalReceived = "1"

	got := order.ComputePaymentProgress()
	if got == nil {
		t.Fatal("expected non-nil progress")
	}
	if got.Percentage != 33 {
		t.Errorf("expected percentage=33 (floor), got %d", got.Percentage)
	}
}

func TestComputePaymentProgress_ExactMatch_Clamps100(t *testing.T) {
	order := putOrderOpenWithAmount(t, "1000")
	order.TotalReceived = "1000"

	got := order.ComputePaymentProgress()
	if got == nil {
		t.Fatal("expected non-nil progress")
	}
	if got.Percentage != 100 {
		t.Errorf("expected percentage=100, got %d", got.Percentage)
	}
	if got.OverpaidAmount != "" {
		t.Errorf("exact match must not surface overpaid, got %q", got.OverpaidAmount)
	}
}

func TestComputePaymentProgress_PendingManagedAmountOverridesOrderOpenAmount(t *testing.T) {
	order := putOrderOpenWithAmount(t, "1500")
	if err := order.SetPendingManagedEscrowInfo(&PendingManagedEscrowInfo{
		Coin:    "crypto:eip155:1:native",
		Amount:  1000,
		Address: "0xmanagedescrow",
	}); err != nil {
		t.Fatalf("SetPendingManagedEscrowInfo failed: %v", err)
	}
	order.TotalReceived = "1000"

	got := order.ComputePaymentProgress()
	if got == nil {
		t.Fatal("expected non-nil progress")
	}
	if got.ExpectedAmount != "1000" {
		t.Errorf("expected pending managed escrow amount to win, got %q", got.ExpectedAmount)
	}
	if got.Percentage != 100 {
		t.Errorf("expected percentage=100, got %d", got.Percentage)
	}
	if got.OverpaidAmount != "" {
		t.Errorf("exact pending managed escrow match must not surface overpaid, got %q", got.OverpaidAmount)
	}
}

func TestComputePaymentProgress_PendingManagedRequiresLockedAmount(t *testing.T) {
	order := putOrderOpenWithAmount(t, "1500")
	if err := order.SetPendingManagedEscrowInfo(&PendingManagedEscrowInfo{
		Coin:    "crypto:eip155:1:native",
		Address: "0xmanagedescrow",
	}); err != nil {
		t.Fatalf("SetPendingManagedEscrowInfo failed: %v", err)
	}
	order.TotalReceived = "1500"

	if got := order.ComputePaymentProgress(); got != nil {
		t.Fatalf("expected nil progress when managed escrow pending amount is missing, got %+v", got)
	}
}

func TestComputePaymentProgress_Overpaid_ExposesDelta(t *testing.T) {
	order := putOrderOpenWithAmount(t, "1000")
	order.TotalReceived = "1500"
	order.OverpaidAmount = "500"

	got := order.ComputePaymentProgress()
	if got == nil {
		t.Fatal("expected non-nil progress")
	}
	if got.Percentage != 100 {
		t.Errorf("expected percentage=100 (clamped), got %d", got.Percentage)
	}
	if got.OverpaidAmount != "500" {
		t.Errorf("expected overpaidAmount=500, got %q", got.OverpaidAmount)
	}
}

func TestComputePaymentProgress_OverpaidZeroSentinel_Hidden(t *testing.T) {
	// Earlier aggregator versions may have left "0" lying around. The
	// helper must hide that so clients don't render "you overpaid 0".
	order := putOrderOpenWithAmount(t, "1000")
	order.TotalReceived = "1000"
	order.OverpaidAmount = "0"

	got := order.ComputePaymentProgress()
	if got == nil {
		t.Fatal("expected non-nil progress")
	}
	if got.OverpaidAmount != "" {
		t.Errorf("stale '0' overpaid must be hidden, got %q", got.OverpaidAmount)
	}
}

func TestComputePaymentProgress_OverpaidNonNumeric_Hidden(t *testing.T) {
	order := putOrderOpenWithAmount(t, "1000")
	order.TotalReceived = "1500"
	order.OverpaidAmount = "garbage"

	got := order.ComputePaymentProgress()
	if got == nil {
		t.Fatal("expected non-nil progress")
	}
	if got.OverpaidAmount != "" {
		t.Errorf("non-numeric overpaid must be hidden, got %q", got.OverpaidAmount)
	}
}

func TestComputePaymentProgress_NegativeTotalReceived_TreatedAsZero(t *testing.T) {
	// Defense in depth: a corrupt row should not break the dashboard
	// or surface negative percentages. The helper falls back to zero.
	order := putOrderOpenWithAmount(t, "1000")
	order.TotalReceived = "-100"

	got := order.ComputePaymentProgress()
	if got == nil {
		t.Fatal("expected non-nil progress")
	}
	if got.TotalReceived != "0" {
		t.Errorf("expected totalReceived=0 (corrupt input), got %q", got.TotalReceived)
	}
	if got.Percentage != 0 {
		t.Errorf("expected percentage=0, got %d", got.Percentage)
	}
}

func TestComputePaymentProgress_WeiScale_BigIntMathCorrect(t *testing.T) {
	// 0.3 ETH out of 0.5 ETH (60%) in wei. Validates that the 18-digit
	// smallest-unit math doesn't overflow int64.
	expected := "500000000000000000" // 0.5 ETH
	received := "300000000000000000" // 0.3 ETH

	order := putOrderOpenWithAmount(t, expected)
	order.TotalReceived = received

	got := order.ComputePaymentProgress()
	if got == nil {
		t.Fatal("expected non-nil progress")
	}
	if got.Percentage != 60 {
		t.Errorf("expected percentage=60, got %d", got.Percentage)
	}
	if got.TotalReceived != received {
		t.Errorf("totalReceived mangled: got %q", got.TotalReceived)
	}
	if got.ExpectedAmount != expected {
		t.Errorf("expectedAmount mangled: got %q", got.ExpectedAmount)
	}
}

func TestComputePaymentProgress_HugeWeiOverpayment_ClampsAt100(t *testing.T) {
	// 78-digit smallest-unit range — int64 would overflow at scaled*100.
	// Ensures big.Int comparison clamps cleanly at 100 without truncation.
	expected := "1"
	received := strings.Repeat("9", 78) // 78-digit "9...9"

	order := putOrderOpenWithAmount(t, expected)
	order.TotalReceived = received
	order.OverpaidAmount = strings.Repeat("9", 77) + "8" // expected - received

	got := order.ComputePaymentProgress()
	if got == nil {
		t.Fatal("expected non-nil progress")
	}
	if got.Percentage != 100 {
		t.Errorf("expected percentage=100 (clamped), got %d", got.Percentage)
	}
}
