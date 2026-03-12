package adapters

import (
	"context"
	"fmt"

	"github.com/mobazha/mobazha3.0/internal/orders/utils"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// UTXOAutoConfirmAdapter wraps UTXO auto-confirm logic as a PaymentStrategy.
// This is a thin adapter — complex business logic (wallet, DB, messenger)
// is accessed via injected callbacks, not a direct MobazhaNode reference.
type UTXOAutoConfirmAdapter struct {
	Multiwallet    contracts.WalletOperator
	Keys           contracts.KeyProvider
	OnAutoConfirm  func(event *events.CancelablePaymentReady)
	GetPaymentInfo func(ctx context.Context, orderID, moderator string, coinType iwallet.CoinType) (*models.PaymentData, error)
}

// Model returns the payment model (Monitored for UTXO).
func (a *UTXOAutoConfirmAdapter) Model() payment.PaymentModel {
	return payment.PaymentModelMonitored
}

// AutoConfirm invokes the OnAutoConfirm callback.
func (a *UTXOAutoConfirmAdapter) AutoConfirm(_ context.Context, event *events.CancelablePaymentReady) error {
	a.OnAutoConfirm(event)
	return nil
}

// ── UTXO Escrow Operations ──────────────────────────────────────

// SignEscrowRelease signs the multisig escrow release transaction.
func (a *UTXOAutoConfirmAdapter) SignEscrowRelease(_ context.Context, params payment.SignEscrowParams) ([]iwallet.EscrowSignature, error) {
	wallet, err := a.Multiwallet.WalletForCurrencyCode(params.CoinCode)
	if err != nil {
		return nil, fmt.Errorf("failed to get wallet for %s: %w", params.CoinCode, err)
	}

	escrowWallet, ok := wallet.(iwallet.UTXOEscrow)
	if !ok {
		return nil, fmt.Errorf("wallet for %s does not support escrow", params.CoinCode)
	}

	escrowMasterKey, err := a.Keys.EscrowMasterKey()
	if err != nil {
		return nil, fmt.Errorf("failed to get escrow master key: %w", err)
	}

	key, err := utils.GenerateEscrowPrivateKey(escrowMasterKey, params.ChainCode)
	if err != nil {
		return nil, fmt.Errorf("failed to generate escrow private key: %w", err)
	}

	sigs, err := escrowWallet.SignMultisigTransaction(params.Transaction, *key, params.Script)
	if err != nil {
		return nil, fmt.Errorf("failed to sign multisig transaction: %w", err)
	}

	return sigs, nil
}

// EstimateEscrowFee estimates the escrow fee for the given parameters.
func (a *UTXOAutoConfirmAdapter) EstimateEscrowFee(coinCode string, nIn, nOut int, feeLevel iwallet.FeeLevel) (iwallet.Amount, error) {
	wallet, err := a.Multiwallet.WalletForCurrencyCode(coinCode)
	if err != nil {
		return iwallet.Amount{}, fmt.Errorf("failed to get wallet for %s: %w", coinCode, err)
	}

	escrowWallet, ok := wallet.(iwallet.UTXOEscrow)
	if !ok {
		return iwallet.Amount{}, fmt.Errorf("wallet for %s does not support escrow", coinCode)
	}

	return escrowWallet.EstimateEscrowFee(nIn, nOut, feeLevel)
}

// ── UTXO Payment Setup ──────────────────────────────────────────

// GeneratePaymentInstructions returns payment setup result via GetPaymentInfo.
func (a *UTXOAutoConfirmAdapter) GeneratePaymentInstructions(ctx context.Context, params payment.PaymentSetupParams) (*payment.PaymentSetupResult, error) {
	paymentData, err := a.GetPaymentInfo(ctx, params.OrderID, params.Moderator, params.CoinType)
	result := &payment.PaymentSetupResult{
		PaymentModel: payment.PaymentModelMonitored,
		PaymentData:  paymentData,
	}
	if err != nil {
		return result, err
	}
	return result, nil
}

// ── UTXO Instruction Methods ────────────────────────────────────

// UTXO chains are backend-monitored — the backend handles signing and
// broadcasting. All instruction methods return nil Instructions, meaning
// the frontend should call the action API directly without chain interaction.

// GetConfirmInstructions returns nil instructions (backend-monitored).
func (a *UTXOAutoConfirmAdapter) GetConfirmInstructions(_ context.Context, _ payment.InstructionParams) (*payment.InstructionResult, error) {
	return &payment.InstructionResult{Instructions: nil}, nil
}

// GetCancelInstructions returns nil instructions (backend-monitored).
func (a *UTXOAutoConfirmAdapter) GetCancelInstructions(_ context.Context, _ payment.InstructionParams) (*payment.InstructionResult, error) {
	return &payment.InstructionResult{Instructions: nil}, nil
}

// GetCompleteInstructions returns nil instructions (backend-monitored).
func (a *UTXOAutoConfirmAdapter) GetCompleteInstructions(_ context.Context, _ payment.InstructionParams) (*payment.InstructionResult, error) {
	return &payment.InstructionResult{Instructions: nil}, nil
}

// GetDisputeReleaseInstructions returns nil instructions (backend-monitored).
func (a *UTXOAutoConfirmAdapter) GetDisputeReleaseInstructions(_ context.Context, _ payment.InstructionParams) (*payment.InstructionResult, error) {
	return &payment.InstructionResult{Instructions: nil}, nil
}

// VerifyDeposit is a noop for UTXO — Electrum handles deposit detection.
func (a *UTXOAutoConfirmAdapter) VerifyDeposit(_ context.Context, _ payment.DepositVerifyParams) error {
	return nil
}
