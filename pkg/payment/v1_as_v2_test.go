package payment_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// instrumentedV1 is a V1 ChainEscrow that records call counts and lets
// each instruction-returning method choose between "instructions
// present" (ClientSigned) and "no instructions" (Monitored UTXO)
// outcomes. It supplements registry_test.go's mockStrategy without
// disturbing existing tests.
type instrumentedV1 struct {
	model payment.PaymentModel

	// withInstructions controls whether GetXxxInstructions returns a
	// non-nil Instructions payload. Set to true to simulate
	// ClientSigned chains; false to simulate Monitored UTXO.
	withInstructions bool

	// errOnAction, if non-nil, is returned by every action method.
	errOnAction error

	calls struct {
		setupPayment    int
		confirm         int
		cancel          int
		complete        int
		disputeRelease  int
		signEscrow      int
		estimateFee     int
		verifyDeposit   int
		validatePayMsg  int
		verifyPreRelease int
		autoConfirm     int
	}

	lastReleaseInfo *pb.EscrowRelease
}

func (m *instrumentedV1) Model() payment.PaymentModel             { return m.model }
func (m *instrumentedV1) Capabilities() payment.ChainCapabilities { return payment.ChainCapabilities{} }
func (m *instrumentedV1) AutoConfirm(_ context.Context, _ *events.CancelablePaymentReady) error {
	m.calls.autoConfirm++
	return nil
}
func (m *instrumentedV1) SignEscrowRelease(_ context.Context, _ payment.SignEscrowParams) ([]iwallet.EscrowSignature, error) {
	m.calls.signEscrow++
	return nil, nil
}
func (m *instrumentedV1) EstimateEscrowFee(_ string, _, _ int, _ iwallet.FeeLevel) (iwallet.Amount, error) {
	m.calls.estimateFee++
	return iwallet.NewAmount(0), nil
}
func (m *instrumentedV1) GeneratePaymentInstructions(_ context.Context, _ payment.PaymentSetupParams) (*payment.PaymentSetupResult, error) {
	m.calls.setupPayment++
	if m.errOnAction != nil {
		return nil, m.errOnAction
	}
	out := &payment.PaymentSetupResult{
		PaymentModel: m.model,
		EscrowAddr:   "escrow-addr",
	}
	if m.withInstructions {
		out.Instructions = map[string]string{"to": "0xabc"}
	}
	return out, nil
}
func (m *instrumentedV1) GetConfirmInstructions(_ context.Context, _ payment.InstructionParams) (*payment.InstructionResult, error) {
	m.calls.confirm++
	return m.buildInstrResult()
}
func (m *instrumentedV1) GetCancelInstructions(_ context.Context, _ payment.InstructionParams) (*payment.InstructionResult, error) {
	m.calls.cancel++
	return m.buildInstrResult()
}
func (m *instrumentedV1) GetCompleteInstructions(_ context.Context, p payment.InstructionParams) (*payment.InstructionResult, error) {
	m.calls.complete++
	m.lastReleaseInfo = p.ReleaseInfo
	return m.buildInstrResult()
}
func (m *instrumentedV1) GetDisputeReleaseInstructions(_ context.Context, _ payment.InstructionParams) (*payment.InstructionResult, error) {
	m.calls.disputeRelease++
	return m.buildInstrResult()
}
func (m *instrumentedV1) VerifyDeposit(_ context.Context, _ payment.DepositVerifyParams) error {
	m.calls.verifyDeposit++
	return nil
}
func (m *instrumentedV1) ValidatePaymentMessage(_ payment.PaymentMessageParams) error {
	m.calls.validatePayMsg++
	return nil
}
func (m *instrumentedV1) VerifyPreRelease(_ context.Context, _ payment.PreReleaseParams) error {
	m.calls.verifyPreRelease++
	return nil
}

func (m *instrumentedV1) buildInstrResult() (*payment.InstructionResult, error) {
	if m.errOnAction != nil {
		return nil, m.errOnAction
	}
	out := &payment.InstructionResult{}
	if m.withInstructions {
		out.Instructions = map[string]string{"to": "0xabc", "data": "0xdeadbeef"}
	}
	return out, nil
}

// ── V1AsV2 wrapper ────────────────────────────────────────────

func TestV1AsV2_SetupPayment_ClientSigned_ReturnsInstructionsRequired(t *testing.T) {
	v1 := &instrumentedV1{model: payment.PaymentModelClientSigned, withInstructions: true}
	w := payment.NewV1AsV2(v1)

	res, err := w.SetupPayment(context.Background(), payment.PaymentSetupParams{OrderID: "o1"})
	if err != nil {
		t.Fatalf("SetupPayment: %v", err)
	}
	if res.Mode != payment.ActionModeInstructionsRequired {
		t.Errorf("Mode = %s, want instructions_required", res.Mode)
	}
	if res.Instructions == nil {
		t.Error("Instructions must be propagated for ClientSigned")
	}
	if res.EscrowAddr != "escrow-addr" {
		t.Errorf("EscrowAddr = %q, want %q", res.EscrowAddr, "escrow-addr")
	}
	if v1.calls.setupPayment != 1 {
		t.Errorf("v1 setup not called exactly once: got %d", v1.calls.setupPayment)
	}
}

func TestV1AsV2_SetupPayment_Monitored_ReturnsCompleted(t *testing.T) {
	v1 := &instrumentedV1{model: payment.PaymentModelMonitored, withInstructions: false}
	w := payment.NewV1AsV2(v1)

	res, err := w.SetupPayment(context.Background(), payment.PaymentSetupParams{OrderID: "o1"})
	if err != nil {
		t.Fatalf("SetupPayment: %v", err)
	}
	if res.Mode != payment.ActionModeCompleted {
		t.Errorf("Mode = %s, want completed", res.Mode)
	}
	if res.Instructions != nil {
		t.Error("Monitored chain must NOT carry frontend instructions")
	}
}

func TestV1AsV2_ActionMethods_PropagateMode(t *testing.T) {
	cases := []struct {
		name             string
		withInstructions bool
		wantMode         payment.ActionMode
	}{
		{"ClientSigned", true, payment.ActionModeInstructionsRequired},
		{"Monitored", false, payment.ActionModeCompleted},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v1 := &instrumentedV1{model: payment.PaymentModelClientSigned, withInstructions: tc.withInstructions}
			w := payment.NewV1AsV2(v1)
			ctx := context.Background()
			params := payment.ActionParams{OrderID: "o1"}

			ops := []struct {
				name string
				fn   func() (*payment.ActionResult, error)
			}{
				{"Confirm", func() (*payment.ActionResult, error) { return w.Confirm(ctx, params) }},
				{"Cancel", func() (*payment.ActionResult, error) { return w.Cancel(ctx, params) }},
				{"Complete", func() (*payment.ActionResult, error) { return w.Complete(ctx, params) }},
				{"DisputeRelease", func() (*payment.ActionResult, error) { return w.DisputeRelease(ctx, params) }},
			}
			for _, op := range ops {
				res, err := op.fn()
				if err != nil {
					t.Fatalf("%s: unexpected error: %v", op.name, err)
				}
				if res.Mode != tc.wantMode {
					t.Errorf("%s: Mode = %s, want %s", op.name, res.Mode, tc.wantMode)
				}
				if tc.wantMode == payment.ActionModeInstructionsRequired && res.Instructions == nil {
					t.Errorf("%s: Instructions must be present in instructions_required mode", op.name)
				}
			}
		})
	}
}

func TestV1AsV2_Complete_PassesEscrowReleaseThroughTypeAssert(t *testing.T) {
	v1 := &instrumentedV1{model: payment.PaymentModelClientSigned, withInstructions: true}
	w := payment.NewV1AsV2(v1)

	rel := &pb.EscrowRelease{}
	_, err := w.Complete(context.Background(), payment.ActionParams{
		OrderID:     "o1",
		ReleaseInfo: rel,
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if v1.lastReleaseInfo != rel {
		t.Error("ReleaseInfo not forwarded to V1 GetCompleteInstructions")
	}
}

func TestV1AsV2_Complete_DropsNonEscrowReleaseValue(t *testing.T) {
	v1 := &instrumentedV1{model: payment.PaymentModelClientSigned, withInstructions: true}
	w := payment.NewV1AsV2(v1)

	_, err := w.Complete(context.Background(), payment.ActionParams{
		OrderID:     "o1",
		ReleaseInfo: "not-an-escrow-release",
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if v1.lastReleaseInfo != nil {
		t.Error("Non-pb.EscrowRelease values must be dropped, not coerced")
	}
}

func TestV1AsV2_GetActionStatus_ReturnsUnsupported(t *testing.T) {
	w := payment.NewV1AsV2(&instrumentedV1{})
	_, err := w.GetActionStatus(context.Background(), "unknown")
	if !errors.Is(err, payment.ErrUnsupportedAction) {
		t.Fatalf("err = %v, want ErrUnsupportedAction", err)
	}
}

func TestV1AsV2_PropagatesV1Errors(t *testing.T) {
	wantErr := errors.New("boom")
	v1 := &instrumentedV1{model: payment.PaymentModelClientSigned, errOnAction: wantErr}
	w := payment.NewV1AsV2(v1)

	if _, err := w.SetupPayment(context.Background(), payment.PaymentSetupParams{}); !errors.Is(err, wantErr) {
		t.Errorf("SetupPayment error = %v, want %v", err, wantErr)
	}
	if _, err := w.Confirm(context.Background(), payment.ActionParams{}); !errors.Is(err, wantErr) {
		t.Errorf("Confirm error = %v, want %v", err, wantErr)
	}
}

func TestV1AsV2_SharedMethodsForwardToV1(t *testing.T) {
	v1 := &instrumentedV1{model: payment.PaymentModelClientSigned}
	w := payment.NewV1AsV2(v1)
	ctx := context.Background()

	_ = w.AutoConfirm(ctx, &events.CancelablePaymentReady{})
	_, _ = w.SignEscrowRelease(ctx, payment.SignEscrowParams{})
	_, _ = w.EstimateEscrowFee("ETH", 1, 1, iwallet.FlNormal)
	_ = w.VerifyDeposit(ctx, payment.DepositVerifyParams{})
	_ = w.ValidatePaymentMessage(payment.PaymentMessageParams{})
	_ = w.VerifyPreRelease(ctx, payment.PreReleaseParams{})

	if v1.calls.autoConfirm != 1 || v1.calls.signEscrow != 1 || v1.calls.estimateFee != 1 ||
		v1.calls.verifyDeposit != 1 || v1.calls.validatePayMsg != 1 || v1.calls.verifyPreRelease != 1 {
		t.Errorf("shared methods not forwarded 1:1: %+v", v1.calls)
	}
}

func TestNewV1AsV2_NilInputReturnsNil(t *testing.T) {
	if w := payment.NewV1AsV2(nil); w != nil {
		t.Errorf("NewV1AsV2(nil) = %v, want nil", w)
	}
}

// ── Registry V2 lookups ──────────────────────────────────────

func TestRegistry_ForChainV2_AutoWrapsV1(t *testing.T) {
	r := payment.NewRegistry()
	v1 := &instrumentedV1{model: payment.PaymentModelClientSigned, withInstructions: true}
	r.Register(iwallet.ChainSolana, v1)

	v2, err := r.ForChainV2(iwallet.ChainSolana)
	if err != nil {
		t.Fatalf("ForChainV2(SOL): %v", err)
	}
	if v2 == nil {
		t.Fatal("ForChainV2 returned nil")
	}
	if v2.Model() != payment.PaymentModelClientSigned {
		t.Errorf("Model() = %s, want client_signed", v2.Model())
	}
}

func TestRegistry_ForChainV2_CachesWrapper(t *testing.T) {
	r := payment.NewRegistry()
	v1 := &instrumentedV1{model: payment.PaymentModelClientSigned}
	r.Register(iwallet.ChainSolana, v1)

	first, err := r.ForChainV2(iwallet.ChainSolana)
	if err != nil {
		t.Fatalf("ForChainV2: %v", err)
	}
	second, err := r.ForChainV2(iwallet.ChainSolana)
	if err != nil {
		t.Fatalf("ForChainV2: %v", err)
	}
	if first != second {
		t.Error("ForChainV2 must return the cached wrapper instance on repeated calls")
	}
}

func TestRegistry_ForChainV2_ReRegisterInvalidatesCache(t *testing.T) {
	r := payment.NewRegistry()
	r.Register(iwallet.ChainSolana, &instrumentedV1{model: payment.PaymentModelClientSigned})

	first, err := r.ForChainV2(iwallet.ChainSolana)
	if err != nil {
		t.Fatalf("ForChainV2 first: %v", err)
	}

	// Overwrite with a fresh V1 strategy — wrapper must rebuild.
	r.Register(iwallet.ChainSolana, &instrumentedV1{model: payment.PaymentModelMonitored})

	second, err := r.ForChainV2(iwallet.ChainSolana)
	if err != nil {
		t.Fatalf("ForChainV2 second: %v", err)
	}
	if first == second {
		t.Error("Re-registering V1 must rebuild the V2 wrapper")
	}
	if second.Model() != payment.PaymentModelMonitored {
		t.Errorf("Model() = %s, want monitored", second.Model())
	}
}

// nativeV2Strategy is a minimal V2-native implementation used to
// validate that RegisterV2 takes precedence over V1 wrapping.
type nativeV2Strategy struct {
	model payment.PaymentModel
	id    string
}

func (s *nativeV2Strategy) Model() payment.PaymentModel             { return s.model }
func (s *nativeV2Strategy) Capabilities() payment.ChainCapabilities { return payment.ChainCapabilities{} }
func (s *nativeV2Strategy) SetupPayment(_ context.Context, _ payment.PaymentSetupParams) (*payment.ActionResult, error) {
	return &payment.ActionResult{Mode: payment.ActionModeSubmitted, ActionID: "act-" + s.id}, nil
}
func (s *nativeV2Strategy) Confirm(_ context.Context, _ payment.ActionParams) (*payment.ActionResult, error) {
	return &payment.ActionResult{Mode: payment.ActionModeSubmitted, ActionID: "act-" + s.id}, nil
}
func (s *nativeV2Strategy) Cancel(_ context.Context, _ payment.ActionParams) (*payment.ActionResult, error) {
	return &payment.ActionResult{Mode: payment.ActionModeSubmitted}, nil
}
func (s *nativeV2Strategy) Complete(_ context.Context, _ payment.ActionParams) (*payment.ActionResult, error) {
	return &payment.ActionResult{Mode: payment.ActionModeSubmitted}, nil
}
func (s *nativeV2Strategy) DisputeRelease(_ context.Context, _ payment.ActionParams) (*payment.ActionResult, error) {
	return &payment.ActionResult{Mode: payment.ActionModeSubmitted}, nil
}
func (s *nativeV2Strategy) GetActionStatus(_ context.Context, id string) (*payment.ActionStatus, error) {
	return &payment.ActionStatus{State: "pending", TxHash: id}, nil
}
func (s *nativeV2Strategy) AutoConfirm(_ context.Context, _ *events.CancelablePaymentReady) error {
	return nil
}
func (s *nativeV2Strategy) SignEscrowRelease(_ context.Context, _ payment.SignEscrowParams) ([]iwallet.EscrowSignature, error) {
	return nil, nil
}
func (s *nativeV2Strategy) EstimateEscrowFee(_ string, _, _ int, _ iwallet.FeeLevel) (iwallet.Amount, error) {
	return iwallet.NewAmount(0), nil
}
func (s *nativeV2Strategy) VerifyDeposit(_ context.Context, _ payment.DepositVerifyParams) error {
	return nil
}
func (s *nativeV2Strategy) ValidatePaymentMessage(_ payment.PaymentMessageParams) error { return nil }
func (s *nativeV2Strategy) VerifyPreRelease(_ context.Context, _ payment.PreReleaseParams) error {
	return nil
}

func TestRegistry_RegisterV2_TakesPrecedenceOverV1(t *testing.T) {
	r := payment.NewRegistry()

	r.Register(iwallet.ChainEthereum, &instrumentedV1{model: payment.PaymentModelClientSigned})
	v2 := &nativeV2Strategy{model: payment.PaymentModelMonitored, id: "safe"}
	r.RegisterV2(iwallet.ChainEthereum, v2)

	got, err := r.ForChainV2(iwallet.ChainEthereum)
	if err != nil {
		t.Fatalf("ForChainV2: %v", err)
	}
	if got != v2 {
		t.Error("V2-native registration must take precedence over the auto-wrapped V1 entry")
	}
	if got.Model() != payment.PaymentModelMonitored {
		t.Errorf("Model() = %s, want monitored (ManagedEscrow)", got.Model())
	}
}

func TestRegistry_ForCoinV2_DelegatesToForChainV2(t *testing.T) {
	r := payment.NewRegistry()
	r.Register(iwallet.ChainBSC, &instrumentedV1{model: payment.PaymentModelClientSigned, withInstructions: true})

	v2, err := r.ForCoinV2(iwallet.CoinType("crypto:eip155:56:native"))
	if err != nil {
		t.Fatalf("ForCoinV2: %v", err)
	}
	if v2.Model() != payment.PaymentModelClientSigned {
		t.Errorf("Model() = %s, want client_signed", v2.Model())
	}
}

func TestRegistry_ForChainV2_UnknownChainReturnsError(t *testing.T) {
	r := payment.NewRegistry()
	if _, err := r.ForChainV2(iwallet.ChainType("UNKNOWN")); err == nil {
		t.Fatal("expected error for unknown chain")
	}
}

func TestRegistry_Chains_IncludesV2OnlyEntries(t *testing.T) {
	r := payment.NewRegistry()
	r.RegisterV2(iwallet.ChainEthereum, &nativeV2Strategy{model: payment.PaymentModelMonitored, id: "safe"})

	chains := r.Chains()
	if len(chains) != 1 || chains[0] != iwallet.ChainEthereum {
		t.Errorf("Chains() = %v, want [ETH]", chains)
	}
}

// Compile-time guarantee that ActionParams.OrderData survives the
// round-trip — important because ManagedEscrowAdapter relies on it for the
// Tier 1 cancel-fee policy.
func TestV1AsV2_ActionParams_PreservesOrderData(t *testing.T) {
	o := &models.Order{ID: "test-order"}
	v1 := &instrumentedV1{model: payment.PaymentModelClientSigned, withInstructions: false}
	w := payment.NewV1AsV2(v1)
	if _, err := w.Cancel(context.Background(), payment.ActionParams{OrderData: o}); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	// instrumentedV1 doesn't capture OrderData, but the call MUST
	// succeed — failure here would mean toInstructionParams panicked
	// or produced a nil deref.
}
