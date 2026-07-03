package adapters

import (
	"context"
	"fmt"

	"github.com/mobazha/mobazha/pkg/events"
	"github.com/mobazha/mobazha/pkg/models"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha/pkg/payment"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

// ChainOps defines the chain-specific operations that differ between
// ClientSigned chains (EVM, Solana, future SUI, etc.).
// All orchestration logic lives in ClientSignedAdapter; only the
// truly chain-specific bits are injected via this interface.
type ChainOps interface {
	AutoConfirm(event *events.CancelablePaymentReady) error
	SignEscrowRelease(params payment.SignEscrowParams) ([]iwallet.EscrowSignature, error)
	BuildCancelableRelease(order *models.Order, initiator, receiver string) (any, error)
	BuildCompleteEscrow(order *models.Order, initiator string, release *pb.EscrowRelease) (any, error)
	BuildDisputeRelease(order *models.Order, initiator string) (any, error)

	// VerifyDeposit verifies that the buyer's deposit transaction is valid on-chain.
	// For EVM: checks receipt status, Funded event, escrow hash, and minimum amount.
	// For Solana: noop (Batch 2).
	VerifyDeposit(ctx context.Context, params payment.DepositVerifyParams) error

	// ValidatePaymentMessage validates PaymentSent message structure per chain.
	// For EVM: escrow address reconstruction + amount verification.
	// For Solana: noop (Batch 2).
	ValidatePaymentMessage(params payment.PaymentMessageParams) error

	// VerifyPreRelease performs chain-specific safety checks before escrow release.
	// For EVM: receipt status + Funded event on-chain verification.
	// For Solana: noop (Batch 2).
	VerifyPreRelease(ctx context.Context, params payment.PreReleaseParams) error
}

// BuildInitEscrowFn builds escrow initialization instructions.
// Signature matches MobazhaNode.BuildInitEscrowInstructions.
type BuildInitEscrowFn func(ctx context.Context, params models.InitializeEscrowData) (*models.PaymentData, iwallet.Address, any, error)

// GetEscrowReleaseFn returns escrow release instructions for confirm flow.
// Signature matches MobazhaNode.GetEscrowReleaseInstructions.
type GetEscrowReleaseFn func(orderID models.OrderID, initiator, payout string) (iwallet.CoinType, any, error)

// ClientSignedAdapter implements ChainEscrow for all ClientSigned chains
// (EVM, Solana, future SUI). Shared logic lives here; chain-specific operations
// are delegated to the injected ChainOps implementation.
//
// Adding a new ClientSigned chain only requires implementing ChainOps (~5 methods)
// and registering via NewClientSignedAdapter — no boilerplate duplication.
//
// Dependencies are injected at construction time — no direct reference to MobazhaNode.
type ClientSignedAdapter struct {
	ops             ChainOps
	buildInitEscrow BuildInitEscrowFn
	getReleaseInstr GetEscrowReleaseFn
}

// NewClientSignedAdapter creates a ClientSignedAdapter with the given chain ops and callbacks.
func NewClientSignedAdapter(ops ChainOps, buildInitEscrow BuildInitEscrowFn, getReleaseInstr GetEscrowReleaseFn) *ClientSignedAdapter {
	return &ClientSignedAdapter{
		ops:             ops,
		buildInitEscrow: buildInitEscrow,
		getReleaseInstr: getReleaseInstr,
	}
}

// ── Meta ────────────────────────────────────────────────────────

func (a *ClientSignedAdapter) Model() payment.PaymentModel {
	return payment.PaymentModelClientSigned
}

func (a *ClientSignedAdapter) Capabilities() payment.ChainCapabilities {
	return payment.ChainCapabilities{
		HasReceiptVerification: true,
		HasClientSignedEscrow:  true,
		EscrowType:             "smart-contract",
	}
}

// ── Delegated to ChainOps ───────────────────────────────────────

func (a *ClientSignedAdapter) AutoConfirm(_ context.Context, event *events.CancelablePaymentReady) error {
	return a.ops.AutoConfirm(event)
}

func (a *ClientSignedAdapter) SignEscrowRelease(_ context.Context, params payment.SignEscrowParams) ([]iwallet.EscrowSignature, error) {
	return a.ops.SignEscrowRelease(params)
}

// ── Shared: fee is always 0 for ClientSigned chains ─────────────

func (a *ClientSignedAdapter) EstimateEscrowFee(_ string, _, _ int, _ iwallet.FeeLevel) (iwallet.Amount, error) {
	return iwallet.NewAmount(0), nil
}

// ── Shared: payment setup via BuildInitEscrowInstructions ───────

func (a *ClientSignedAdapter) GeneratePaymentInstructions(ctx context.Context, params payment.PaymentSetupParams) (*payment.PaymentSetupResult, error) {
	initParams := models.InitializeEscrowData{
		OrderID:       params.OrderID,
		PayerAddress:  params.PayerAddress,
		RefundAddress: params.RefundAddress,
		Moderator:     params.Moderator,
		CoinType:      params.CoinType,
		Amount:        params.Amount,
	}
	paymentData, escrowAccount, instructions, err := a.buildInitEscrow(ctx, initParams)
	if err != nil {
		return nil, err
	}
	return &payment.PaymentSetupResult{
		PaymentModel: payment.PaymentModelClientSigned,
		PaymentData:  paymentData,
		EscrowAddr:   escrowAccount.String(),
		Instructions: instructions,
	}, nil
}

// ── Shared: confirm uses GetEscrowReleaseInstructions ───────────

func (a *ClientSignedAdapter) GetConfirmInstructions(_ context.Context, params payment.InstructionParams) (*payment.InstructionResult, error) {
	_, instructions, err := a.getReleaseInstr(
		models.OrderID(params.OrderID),
		params.InitiatorAddr,
		params.PayoutAddr,
	)
	if err != nil {
		return nil, err
	}
	return &payment.InstructionResult{Instructions: instructions}, nil
}

// ── Delegated to ChainOps (with shared type-assert boilerplate) ─

func (a *ClientSignedAdapter) GetCancelInstructions(_ context.Context, params payment.InstructionParams) (*payment.InstructionResult, error) {
	order := params.OrderData
	if order == nil {
		return nil, fmt.Errorf("GetCancelInstructions: OrderData is required")
	}
	instructions, err := a.ops.BuildCancelableRelease(order, params.InitiatorAddr, params.PayoutAddr)
	if err != nil {
		return nil, err
	}
	return &payment.InstructionResult{Instructions: instructions}, nil
}

func (a *ClientSignedAdapter) GetCompleteInstructions(_ context.Context, params payment.InstructionParams) (*payment.InstructionResult, error) {
	order := params.OrderData
	if order == nil {
		return nil, fmt.Errorf("GetCompleteInstructions: OrderData is required")
	}
	releaseInfo := params.ReleaseInfo
	if releaseInfo == nil {
		return nil, fmt.Errorf("GetCompleteInstructions: ReleaseInfo is required")
	}
	instructions, err := a.ops.BuildCompleteEscrow(order, params.InitiatorAddr, releaseInfo)
	if err != nil {
		return nil, err
	}
	return &payment.InstructionResult{Instructions: instructions}, nil
}

func (a *ClientSignedAdapter) GetDisputeReleaseInstructions(_ context.Context, params payment.InstructionParams) (*payment.InstructionResult, error) {
	order := params.OrderData
	if order == nil {
		return nil, fmt.Errorf("GetDisputeReleaseInstructions: OrderData is required")
	}
	instructions, err := a.ops.BuildDisputeRelease(order, params.InitiatorAddr)
	if err != nil {
		return nil, err
	}
	return &payment.InstructionResult{Instructions: instructions}, nil
}

// ── Deposit Verification ────────────────────────────────────────

func (a *ClientSignedAdapter) VerifyDeposit(ctx context.Context, params payment.DepositVerifyParams) error {
	return a.ops.VerifyDeposit(ctx, params)
}

// ── Payment Message Validation ──────────────────────────────────

func (a *ClientSignedAdapter) ValidatePaymentMessage(params payment.PaymentMessageParams) error {
	return a.ops.ValidatePaymentMessage(params)
}

// ── Pre-Release Verification ────────────────────────────────────

func (a *ClientSignedAdapter) VerifyPreRelease(ctx context.Context, params payment.PreReleaseParams) error {
	return a.ops.VerifyPreRelease(ctx, params)
}
