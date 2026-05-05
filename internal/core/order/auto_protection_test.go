//go:build !private_distribution

package order

import (
	"context"
	"testing"
	"time"

	"github.com/mobazha/mobazha3.0/internal/orders/utils"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// --- isInReminderWindow tests ---

func TestIsInReminderWindow_InsideFirstTick(t *testing.T) {
	deadline := time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC)
	reminderDay := 3
	reminderAt := deadline.Add(-3 * 24 * time.Hour)

	now := reminderAt.Add(30 * time.Second)
	if !isInReminderWindow(now, deadline, reminderDay) {
		t.Error("should be in reminder window (first tick)")
	}
}

func TestIsInReminderWindow_AfterWindowClosed(t *testing.T) {
	deadline := time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC)
	reminderDay := 3
	reminderAt := deadline.Add(-3 * 24 * time.Hour)

	now := reminderAt.Add(3 * time.Minute) // window is 2*1min=2min
	if isInReminderWindow(now, deadline, reminderDay) {
		t.Error("should NOT be in reminder window after 2-minute window closes")
	}
}

func TestIsInReminderWindow_BeforeWindowOpens(t *testing.T) {
	deadline := time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC)
	reminderDay := 3
	reminderAt := deadline.Add(-3 * 24 * time.Hour)

	now := reminderAt.Add(-1 * time.Second)
	if isInReminderWindow(now, deadline, reminderDay) {
		t.Error("should NOT be in reminder window before it opens")
	}
}

func TestIsInReminderWindow_DeadlineAlreadyPassed(t *testing.T) {
	deadline := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	reminderDay := 3
	now := time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC)

	if isInReminderWindow(now, deadline, reminderDay) {
		t.Error("should NOT fire reminder when deadline has long passed")
	}
}

func TestIsInReminderWindow_ExactStart(t *testing.T) {
	deadline := time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC)
	reminderDay := 1
	reminderAt := deadline.Add(-1 * 24 * time.Hour)

	if !isInReminderWindow(reminderAt, deadline, reminderDay) {
		t.Error("should be in reminder window at exact start time")
	}
}

func TestIsInReminderWindow_ExactEnd(t *testing.T) {
	deadline := time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC)
	reminderDay := 1
	reminderAt := deadline.Add(-1 * 24 * time.Hour)
	windowEnd := reminderAt.Add(2 * orderTimeoutScheduleInterval)

	if isInReminderWindow(windowEnd, deadline, reminderDay) {
		t.Error("should NOT be in window at exact end (exclusive upper bound)")
	}
}

// --- isClientSignedModerated tests ---

type stubStrategy struct {
	model payment.PaymentModel
}

func (s *stubStrategy) Model() payment.PaymentModel        { return s.model }
func (s *stubStrategy) Capabilities() payment.ChainCapabilities { return payment.ChainCapabilities{} }
func (s *stubStrategy) AutoConfirm(_ context.Context, _ *events.CancelablePaymentReady) error {
	return nil
}
func (s *stubStrategy) SignEscrowRelease(_ context.Context, _ payment.SignEscrowParams) ([]iwallet.EscrowSignature, error) {
	return nil, nil
}
func (s *stubStrategy) EstimateEscrowFee(_ string, _, _ int, _ iwallet.FeeLevel) (iwallet.Amount, error) {
	return iwallet.NewAmount(0), nil
}
func (s *stubStrategy) GeneratePaymentInstructions(_ context.Context, _ payment.PaymentSetupParams) (*payment.PaymentSetupResult, error) {
	return &payment.PaymentSetupResult{PaymentModel: s.model}, nil
}
func (s *stubStrategy) GetConfirmInstructions(_ context.Context, _ payment.InstructionParams) (*payment.InstructionResult, error) {
	return &payment.InstructionResult{}, nil
}
func (s *stubStrategy) GetCancelInstructions(_ context.Context, _ payment.InstructionParams) (*payment.InstructionResult, error) {
	return &payment.InstructionResult{}, nil
}
func (s *stubStrategy) GetCompleteInstructions(_ context.Context, _ payment.InstructionParams) (*payment.InstructionResult, error) {
	return &payment.InstructionResult{}, nil
}
func (s *stubStrategy) GetDisputeReleaseInstructions(_ context.Context, _ payment.InstructionParams) (*payment.InstructionResult, error) {
	return &payment.InstructionResult{}, nil
}
func (s *stubStrategy) VerifyDeposit(_ context.Context, _ payment.DepositVerifyParams) error {
	return nil
}
func (s *stubStrategy) ValidatePaymentMessage(_ payment.PaymentMessageParams) error {
	return nil
}
func (s *stubStrategy) VerifyPreRelease(_ context.Context, _ payment.PreReleaseParams) error {
	return nil
}

func orderWithPaymentSent(t *testing.T, method pb.PaymentSent_Method, coin string) *models.Order {
	t.Helper()
	o := &models.Order{}
	ps := &pb.PaymentSent{Method: method, Coin: coin}
	if err := o.PutMessage(utils.MustWrapOrderMessage(ps)); err != nil {
		t.Fatalf("PutMessage: %v", err)
	}
	return o
}

func TestIsClientSignedModerated_CancelableReturnsFalse(t *testing.T) {
	reg := payment.NewRegistry()
	svc := &OrderAppService{paymentRegistry: reg}

	order := orderWithPaymentSent(t, pb.PaymentSent_CANCELABLE, "BTC")
	if svc.isClientSignedModerated(order) {
		t.Error("CANCELABLE order should not be identified as client-signed moderated")
	}
}

func TestIsClientSignedModerated_ModeratedMonitoredReturnsFalse(t *testing.T) {
	reg := payment.NewRegistry()
	reg.Register(iwallet.ChainBitcoin, &stubStrategy{model: payment.PaymentModelMonitored})
	svc := &OrderAppService{paymentRegistry: reg}

	order := orderWithPaymentSent(t, pb.PaymentSent_MODERATED, "BTC")
	if svc.isClientSignedModerated(order) {
		t.Error("MODERATED + Monitored (UTXO) should not be client-signed")
	}
}

func TestIsClientSignedModerated_ModeratedClientSignedReturnsTrue(t *testing.T) {
	reg := payment.NewRegistry()
	reg.Register(iwallet.ChainEthereum, &stubStrategy{model: payment.PaymentModelClientSigned})
	svc := &OrderAppService{paymentRegistry: reg}

	order := orderWithPaymentSent(t, pb.PaymentSent_MODERATED, "crypto:eip155:1:native")
	if !svc.isClientSignedModerated(order) {
		t.Error("MODERATED + ClientSigned (EVM) should be client-signed moderated")
	}
}
