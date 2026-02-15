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
// Phase 1 covers auto-confirmation (CANCELABLE → AWAITING_FULFILLMENT).
// Future phases will expand PaymentStrategy to cover the full order lifecycle:
// payment instructions, cancel, fulfill, complete, refund, and dispute.
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

// PaymentStrategy defines chain-level payment operations.
//
// The order state machine itself is chain-agnostic, but each state transition
// involving an on-chain transaction needs chain-specific logic. PaymentStrategy
// provides a unified interface for these operations.
//
// # Phase 1 (current)
//
// Only [PaymentStrategy.Model] and [PaymentStrategy.AutoConfirm] are active.
// These power the CANCELABLE payment auto-confirmation dispatch.
//
// # Future expansion (Phase 2-4)
//
// The interface will grow to cover the full order lifecycle:
//
//   - Payment:  GeneratePaymentInstructions, ValidatePaymentProof
//   - Confirm:  GetConfirmInstructions (manual MODERATED confirm)
//   - Cancel:   BuildCancelInstructions
//   - Fulfill:  BuildFulfillmentRelease
//   - Complete: BuildCompletionRelease
//   - Refund:   BuildRefundInstructions
//   - Dispute:  BuildDisputePayout
//
// Each new method will be added when its corresponding phase is implemented,
// ensuring all existing implementations compile without stub methods.
type PaymentStrategy interface {
	// Model returns the payment paradigm for this chain.
	// Used by the frontend to determine how to render the payment UI
	// (QR code for monitored, wallet connect for client_signed, etc.).
	Model() PaymentModel

	// AutoConfirm handles auto-confirmation for a CANCELABLE payment (vendor side).
	//
	// Called asynchronously (in a goroutine) by the cancelable payment dispatcher
	// when a [events.CancelablePaymentReady] event fires.
	//
	// Implementation responsibilities:
	//   - Fetch the order by event.OrderID
	//   - Acquire auto-confirm lock (prevent concurrent processing)
	//   - Release escrow funds to vendor (chain-specific)
	//   - Send ORDER_CONFIRMATION message to buyer
	//
	// Returns nil on success or if auto-confirm is not applicable.
	// Errors are logged by the dispatcher; the operation is fire-and-forget.
	AutoConfirm(ctx context.Context, event *events.CancelablePaymentReady) error
}
