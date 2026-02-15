// Package payment defines the payment strategy abstraction layer.
//
// This package provides the [PaymentStrategy] interface and [Registry] that
// decouple chain-specific payment logic from the core order state machine.
// Each blockchain (UTXO, EVM, Solana, etc.) implements PaymentStrategy to
// declare its payment paradigm and handle chain-specific operations.
//
// Architecture (Strategy + Registry):
//
//	Order State Machine (chain-agnostic)
//	         ↓ queries
//	Payment Registry (maps ChainType → Strategy)
//	         ↓ dispatches to
//	Chain Strategy Implementations (chain-specific)
//
// The order state machine remains completely chain-agnostic. When a state
// transition requires a chain-specific operation (e.g., escrow release,
// fund refund), it queries the registry to get the appropriate strategy.
//
// Phases:
//   - Phase 1-2: AutoConfirm dispatch (all chains registered)
//   - Phase 4:   Full order lifecycle instruction generation
package payment

import (
	"context"

	"github.com/mobazha/mobazha3.0/pkg/events"
)

// PaymentModel declares the payment paradigm used by a chain.
// This determines how the frontend and backend interact during payment.
type PaymentModel string

const (
	// PaymentModelMonitored is for chains where the backend monitors addresses
	// for incoming transactions (UTXO chains: BTC, BCH, LTC, ZEC).
	// Flow: buyer transfers to address → backend monitor detects tx → callback.
	PaymentModelMonitored PaymentModel = "monitored"

	// PaymentModelClientSigned is for chains where the frontend connects a wallet,
	// signs and sends the transaction, then submits txHash to the backend (EVM, Solana).
	// Flow: frontend gets instructions → wallet signs tx → submit txHash to backend.
	PaymentModelClientSigned PaymentModel = "client_signed"

	// PaymentModelThirdParty is for third-party payment providers (e.g., Stripe)
	// that handle payment outside the blockchain.
	// Flow: frontend uses provider SDK → webhook notifies backend.
	PaymentModelThirdParty PaymentModel = "third_party"
)

// ── Instruction Params / Result ─────────────────────────────────

// InstructionParams provides context for chain-specific instruction generation.
// Each lifecycle method uses the subset of fields it needs.
//
// The caller (in internal/core/) populates these from the order and request,
// then the strategy adapter generates chain-specific instructions.
type InstructionParams struct {
	// OrderID is the order identifier.
	OrderID string

	// InitiatorAddr is the wallet address of the caller (frontend user).
	// Used as the payer/signer address in generated instructions.
	InitiatorAddr string

	// PayoutAddr is the destination address for fund release.
	// For confirm: vendor payout address.
	// For cancel: buyer refund address.
	PayoutAddr string

	// PaymentCoin is the payment coin code (e.g., "ETH", "BTC").
	PaymentCoin string

	// PaymentAmount is the payment amount in minimal units (satoshis, wei, lamports).
	PaymentAmount string

	// Chaincode is the hex-encoded chaincode from PaymentSent message.
	Chaincode string

	// Script is the hex-encoded script from PaymentSent message (EVM only).
	Script string

	// OrderData carries the pre-fetched order object to avoid redundant DB fetches.
	// Type: *models.Order (passed as any to minimize pkg/payment dependencies).
	// Set by the calling code; adapters type-assert as needed.
	OrderData any

	// ReleaseInfo carries fulfillment release data for complete operations.
	// Type: *pb.EscrowRelease (passed as any to avoid pkg/payment importing pb).
	ReleaseInfo any
}

// InstructionResult contains chain-specific instructions for the frontend.
// A nil Instructions field means the backend handles the operation directly
// (e.g., UTXO chains where the backend signs and broadcasts).
type InstructionResult struct {
	// Instructions contains chain-specific data for the frontend.
	// nil = no frontend action needed (backend handles it).
	// EVM: contract call data ({to, data, value}).
	// Solana: program instructions ([]SolanaGoInstruction).
	Instructions any
}

// ── PaymentStrategy Interface ───────────────────────────────────

// PaymentStrategy defines chain-level payment operations covering the full
// order lifecycle.
//
// The order state machine itself is chain-agnostic, but each state transition
// involving an on-chain transaction needs chain-specific logic. PaymentStrategy
// provides a unified interface for these operations.
//
// # Instruction Generation Methods (Phase 4)
//
// Each GetXxxInstructions method returns:
//   - nil Instructions → backend handles the operation (UTXO monitored model)
//   - non-nil Instructions → frontend must sign and submit txHash
//
// The calling code (internal/core/ lifecycle files) handles order validation,
// DB operations, and message sending. Strategy methods only handle the
// chain-specific instruction generation or fund release.
type PaymentStrategy interface {
	// ── Meta ────────────────────────────────────────────

	// Model returns the payment paradigm for this chain.
	Model() PaymentModel

	// ── Auto-Confirm (Phase 1-2) ───────────────────────

	// AutoConfirm handles auto-confirmation for a CANCELABLE payment.
	// Called asynchronously by the cancelable payment dispatcher.
	AutoConfirm(ctx context.Context, event *events.CancelablePaymentReady) error

	// ── Instruction Generation (Phase 4) ───────────────

	// GetConfirmInstructions returns instructions for confirming a CANCELABLE order.
	//
	// Monitored (UTXO): returns nil — backend releases funds via ConfirmOrder.
	// ClientSigned (EVM/Solana): returns escrow release instructions for frontend.
	//
	// Params used: OrderID, InitiatorAddr, PayoutAddr, PaymentCoin, PaymentAmount,
	// Chaincode, Script.
	GetConfirmInstructions(ctx context.Context, params InstructionParams) (*InstructionResult, error)

	// GetCancelInstructions returns instructions for canceling a CANCELABLE order.
	//
	// Monitored (UTXO): returns nil — backend releases funds back to buyer.
	// ClientSigned (EVM/Solana): returns escrow release instructions for frontend.
	//
	// Params used: OrderID, InitiatorAddr, PayoutAddr (buyer refund address),
	// PaymentCoin, PaymentAmount, Chaincode, Script.
	GetCancelInstructions(ctx context.Context, params InstructionParams) (*InstructionResult, error)

	// GetCompleteInstructions returns instructions for completing a MODERATED order.
	//
	// Monitored (UTXO): returns nil — backend handles multisig signing.
	// ClientSigned (EVM/Solana): returns escrow release instructions for frontend.
	//
	// Params used: OrderID, InitiatorAddr, PaymentCoin, PaymentAmount,
	// Chaincode, Script, ReleaseInfo.
	GetCompleteInstructions(ctx context.Context, params InstructionParams) (*InstructionResult, error)

	// GetDisputeReleaseInstructions returns instructions for releasing dispute funds.
	//
	// Monitored (UTXO): returns nil — backend handles signing.
	// ClientSigned (EVM/Solana): returns release instructions for frontend.
	//
	// Params used: OrderID, InitiatorAddr, PaymentCoin, PaymentAmount,
	// Chaincode, Script.
	GetDisputeReleaseInstructions(ctx context.Context, params InstructionParams) (*InstructionResult, error)
}
