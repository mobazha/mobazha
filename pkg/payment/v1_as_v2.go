package payment

import (
	"context"

	"github.com/mobazha/mobazha3.0/pkg/events"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// V1AsV2 adapts a V1 ChainEscrow implementation to the V2
// ChainEscrowV2 interface (D-Hybrid-17 A+ scheme).
//
// All shared methods (Auto-confirm, signing, fee estimation, deposit
// verification, payment-message validation, pre-release verification)
// forward verbatim. The action-centric methods (SetupPayment, Confirm,
// Cancel, Complete, DisputeRelease) translate the V1 instruction
// outputs into ActionResult{Mode: ActionModeInstructionsRequired},
// signaling to V2 callers that the frontend still drives the signing.
//
// GetActionStatus always returns ErrUnsupportedAction — V1
// implementations have no relay-task store. Callers handle this by
// falling back to existing per-chain status APIs (e.g., the
// transaction-monitor service for Solana / TRON, or the buyer's wallet
// receipt for EVM ClientSigned).
//
// V1AsV2 deliberately holds NO additional state beyond the wrapped V1
// implementation. This is what keeps Sprint 0 D8 + Sprint 2 migration
// safe: re-registering a V1 strategy automatically refreshes the
// wrapper without leaking caches.
type V1AsV2 struct {
	v1 ChainEscrow
}

// NewV1AsV2 wraps a V1 ChainEscrow as ChainEscrowV2. Returns nil when
// the input is nil so registry code can fold "no-op" cases.
func NewV1AsV2(v1 ChainEscrow) *V1AsV2 {
	if v1 == nil {
		return nil
	}
	return &V1AsV2{v1: v1}
}

// Underlying returns the wrapped V1 implementation. This is useful for tests
// that need to assert the exact adapter registered behind a V2 wrapper.
func (w *V1AsV2) Underlying() ChainEscrow { return w.v1 }

// chainEscrowV2Marker satisfies asV2Marker so Registry.ForCoinV2 can
// detect that this value already implements V2.
func (*V1AsV2) chainEscrowV2Marker() {}

// ── Meta ───────────────────────────────────────────────────────

func (w *V1AsV2) Model() PaymentModel             { return w.v1.Model() }
func (w *V1AsV2) Capabilities() ChainCapabilities { return w.v1.Capabilities() }

// ── Action-centric lifecycle ──────────────────────────────────

// SetupPayment forwards to V1.GeneratePaymentInstructions and folds
// the V1 result into an ActionResult. The Mode classification depends
// on whether the V1 adapter returned instructions:
//   - ClientSigned (EVM/Solana): instructions present →
//     ActionModeInstructionsRequired.
//   - Monitored (UTXO): no instructions →
//     ActionModeCompleted (the address is provisioned synchronously).
func (w *V1AsV2) SetupPayment(ctx context.Context, params PaymentSetupParams) (*ActionResult, error) {
	res, err := w.v1.GeneratePaymentInstructions(ctx, params)
	if err != nil {
		return nil, err
	}
	mode := ActionModeCompleted
	if res != nil && res.Instructions != nil {
		mode = ActionModeInstructionsRequired
	}
	out := &ActionResult{Mode: mode}
	if res != nil {
		out.EscrowAddr = res.EscrowAddr
		out.PaymentData = res.PaymentData
		out.Instructions = res.Instructions
	}
	return out, nil
}

func (w *V1AsV2) Confirm(ctx context.Context, params ActionParams) (*ActionResult, error) {
	return wrapInstructionResult(w.v1.GetConfirmInstructions(ctx, params.toInstructionParams()))
}

func (w *V1AsV2) Cancel(ctx context.Context, params ActionParams) (*ActionResult, error) {
	return wrapInstructionResult(w.v1.GetCancelInstructions(ctx, params.toInstructionParams()))
}

func (w *V1AsV2) Complete(ctx context.Context, params ActionParams) (*ActionResult, error) {
	return wrapInstructionResult(w.v1.GetCompleteInstructions(ctx, params.toInstructionParams()))
}

func (w *V1AsV2) DisputeRelease(ctx context.Context, params ActionParams) (*ActionResult, error) {
	return wrapInstructionResult(w.v1.GetDisputeReleaseInstructions(ctx, params.toInstructionParams()))
}

// GetActionStatus is unsupported on V1-backed adapters — V1 has no
// action ledger. Callers detect this and fall back to V1's
// chain-specific status surfaces.
func (w *V1AsV2) GetActionStatus(_ context.Context, _ string) (*ActionStatus, error) {
	return nil, ErrUnsupportedAction
}

// ── Shared methods (forward verbatim) ─────────────────────────

func (w *V1AsV2) AutoConfirm(ctx context.Context, event *events.CancelablePaymentReady) error {
	return w.v1.AutoConfirm(ctx, event)
}

func (w *V1AsV2) SignEscrowRelease(ctx context.Context, params SignEscrowParams) ([]iwallet.EscrowSignature, error) {
	return w.v1.SignEscrowRelease(ctx, params)
}

func (w *V1AsV2) EstimateEscrowFee(coinCode string, nIn, nOut int, feeLevel iwallet.FeeLevel) (iwallet.Amount, error) {
	return w.v1.EstimateEscrowFee(coinCode, nIn, nOut, feeLevel)
}

func (w *V1AsV2) VerifyDeposit(ctx context.Context, params DepositVerifyParams) error {
	return w.v1.VerifyDeposit(ctx, params)
}

func (w *V1AsV2) ValidatePaymentMessage(params PaymentMessageParams) error {
	return w.v1.ValidatePaymentMessage(params)
}

func (w *V1AsV2) VerifyPreRelease(ctx context.Context, params PreReleaseParams) error {
	return w.v1.VerifyPreRelease(ctx, params)
}

// ── helpers ────────────────────────────────────────────────────

// toInstructionParams projects ActionParams onto V1's InstructionParams.
// ReleaseInfo is type-asserted to *pb.EscrowRelease — the only concrete
// type V1 callers ever set; non-matching values are dropped (matches
// the V1 contract that ReleaseInfo is required only for Complete).
func (p ActionParams) toInstructionParams() InstructionParams {
	out := InstructionParams{
		OrderID:       p.OrderID,
		InitiatorAddr: p.InitiatorAddr,
		PayoutAddr:    p.PayoutAddr,
		PaymentCoin:   p.PaymentCoin,
		PaymentAmount: p.PaymentAmount,
		Chaincode:     p.Chaincode,
		Script:        p.Script,
		OrderData:     p.OrderData,
	}
	if rel, ok := p.ReleaseInfo.(*pb.EscrowRelease); ok {
		out.ReleaseInfo = rel
	}
	return out
}

// wrapInstructionResult converts a V1 (*InstructionResult, error)
// pair to a (*ActionResult, error) pair following the V1AsV2 mapping
// rule:
//   - non-nil Instructions → ActionModeInstructionsRequired.
//   - nil/empty Instructions → ActionModeCompleted (Monitored UTXO
//     where the backend signs inline).
func wrapInstructionResult(res *InstructionResult, err error) (*ActionResult, error) {
	if err != nil {
		return nil, err
	}
	out := &ActionResult{Mode: ActionModeCompleted}
	if res != nil && res.Instructions != nil {
		out.Mode = ActionModeInstructionsRequired
		out.Instructions = res.Instructions
	}
	return out, nil
}

// Compile-time assertion — V1AsV2 must satisfy ChainEscrowV2.
var _ ChainEscrowV2 = (*V1AsV2)(nil)
