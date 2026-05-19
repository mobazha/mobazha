package payment

import (
	"context"
	"errors"

	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// ChainEscrowV2 is the action-centric counterpart to ChainEscrow (V1).
//
// Background — Phase EVM-ManagedEscrow v0.3.0 (D-Hybrid-17 A+ scheme):
//
// V1 ChainEscrow exposes per-action GetXxxInstructions() returning
// frontend-signable payloads. That works for ClientSigned chains
// (Solana / TRON) where the frontend wallet signs and submits the tx,
// but does not fit the ManagedEscrow v1.4.1 path: nodes do not hold private
// keys for the SaaS Relay's gas wallet; the backend submits the
// transaction itself after collecting ManagedEscrow owner signatures.
//
// V2 raises the abstraction one notch — callers express *intent*
// ("cancel this order") and the adapter decides the model:
//
//   - ManagedEscrow (Monitored): backend builds + relays the ManagedEscrowTx; the
//     ActionResult carries an ActionID so the frontend can poll status.
//   - ClientSigned (V1AsV2 wrap): the adapter returns its V1
//     instructions in ActionResult.Instructions and signals
//     ActionModeInstructionsRequired — the frontend keeps signing.
//
// The two interfaces coexist:
//   - V1 implementations stay registered as today;
//     Registry.ForCoinV2 wraps them via V1AsV2 transparently.
//   - V2-native implementations (ManagedEscrowAdapter) register via
//     Registry.RegisterV2.
//
// Migration path (no in-flight orders to preserve, per D-Hybrid-23):
//   - Sprint 0/1: introduce V2 + wrapper; existing V1 callers
//     unaffected.
//   - Sprint 2: migrate handlers to V2; Solana/TRON keep V1, are
//     wrapped on read.
//   - Sprint 3+: V1 may be removed when only V2 callers remain.
//
// V2 reuses every V1 shared method (signing, deposit verification,
// validation, etc.) verbatim — only the lifecycle methods change.
// This keeps V1AsV2 a single-file forwarder with no signature
// translation.
type ChainEscrowV2 interface {
	// Meta — same semantics as V1.
	Model() PaymentModel
	Capabilities() ChainCapabilities

	// ── Action-centric lifecycle ────────────────────────────

	// SetupPayment provisions the escrow account / address and returns
	// the data the frontend needs to begin payment.
	SetupPayment(ctx context.Context, params PaymentSetupParams) (*ActionResult, error)

	// Confirm releases funds for a CANCELABLE order (buyer auto-confirm
	// or seller-finalized release).
	Confirm(ctx context.Context, params ActionParams) (*ActionResult, error)

	// Cancel refunds the buyer for a CANCELABLE order.
	Cancel(ctx context.Context, params ActionParams) (*ActionResult, error)

	// Complete releases funds for a MODERATED order according to the
	// EscrowRelease shipment schedule.
	Complete(ctx context.Context, params ActionParams) (*ActionResult, error)

	// DisputeRelease executes a moderator-decided dispute payout.
	DisputeRelease(ctx context.Context, params ActionParams) (*ActionResult, error)

	// GetActionStatus reports the lifecycle of a previously submitted
	// action. Returns ErrActionNotFound if the ActionID is unknown.
	// V1AsV2 returns ErrUnsupportedAction (V1 has no action store).
	GetActionStatus(ctx context.Context, actionID string) (*ActionStatus, error)

	// ── Shared methods (identical signatures to V1) ────────

	// AutoConfirm handles auto-confirmation for a CANCELABLE payment.
	AutoConfirm(ctx context.Context, event *events.CancelablePaymentReady) error

	// SignEscrowRelease signs an escrow release transaction.
	SignEscrowRelease(ctx context.Context, params SignEscrowParams) ([]iwallet.EscrowSignature, error)

	// EstimateEscrowFee returns the estimated fee for an escrow release.
	EstimateEscrowFee(coinCode string, nIn, nOut int, feeLevel iwallet.FeeLevel) (iwallet.Amount, error)

	// VerifyDeposit checks that the buyer's deposit is valid on-chain.
	VerifyDeposit(ctx context.Context, params DepositVerifyParams) error

	// ValidatePaymentMessage validates the PaymentSent message structure.
	ValidatePaymentMessage(params PaymentMessageParams) error

	// VerifyPreRelease performs chain-specific safety checks before release.
	VerifyPreRelease(ctx context.Context, params PreReleaseParams) error
}

// ── Action types ───────────────────────────────────────────────

// ActionMode classifies how the chain adapter handled an action call.
type ActionMode string

const (
	// ActionModeInstructionsRequired — the adapter cannot finalize the
	// action without a frontend signature. Caller MUST forward
	// Instructions to the user and follow up with the existing
	// "submit signed tx" flow. Used by V1AsV2 wrapping ClientSigned
	// chains (Solana, TRON, and EVM until ManagedEscrowAdapter ships).
	ActionModeInstructionsRequired ActionMode = "instructions_required"

	// ActionModeSubmitted — the adapter has submitted (or queued) the
	// action on-chain on behalf of the caller. ActionID is populated
	// and can be polled via GetActionStatus.
	ActionModeSubmitted ActionMode = "submitted"

	// ActionModeCompleted — the adapter completed the action
	// synchronously (rare; used by no-op / monitored confirm paths
	// where the backend signs and broadcasts inline).
	ActionModeCompleted ActionMode = "completed"
)

// ActionResult is returned by every V2 action method.
type ActionResult struct {
	// Mode classifies whether the caller still needs to push
	// instructions to the frontend.
	Mode ActionMode

	// ActionID is the opaque lookup key for GetActionStatus.
	// Submitted actions always populate it. Instructions-required flows
	// may also pre-allocate one so the later relay submit can persist
	// projections under a stable client-visible ID.
	ActionID string

	// Instructions carries the chain-specific payload the frontend must
	// sign. Populated only in ActionModeInstructionsRequired.
	// Polymorphic by design (EVM map, Solana []Instruction, etc.) —
	// matches V1's InstructionResult semantics.
	Instructions any

	// EscrowAddr is the on-chain escrow address. Populated by
	// SetupPayment regardless of Mode (the address must be visible
	// before the buyer pays).
	EscrowAddr string

	// PaymentData is the chain-agnostic payment descriptor consumed by
	// the order state machine. Populated by SetupPayment.
	PaymentData *models.PaymentData

	// SubmittedTxHash is the relay/broadcast tx hash when Mode is
	// ActionModeSubmitted and the adapter already knows the hash
	// synchronously (e.g. ManagedEscrow relay). Callers should prefer this over
	// polling GetActionStatus immediately after submit.
	SubmittedTxHash string
}

// ActionOwnerSignature is a chain-agnostic carrier for a single owner
// authorization over a settlement action hash. It lets order-message
// producers embed backend-generated owner signatures without depending on
// chain-specific wire details.
type ActionOwnerSignature struct {
	// From is the 0x-prefixed owner address that produced Signature.
	From string

	// Signature is the raw owner signature payload.
	Signature []byte

	// Index is the owner ordinal when the chain has a stable owner list.
	// ManagedEscrow uses it as an audit hint; UTXO-style escrows may ignore it.
	Index uint32
}

// ActionSigner is an optional V2 capability implemented by backends that can
// produce a local owner signature for a specific settlement action without
// broadcasting it yet. ManagedEscrow uses this for threshold>1 flows:
//   - seller pre-signs "complete" into ORDER_SHIPMENT.ReleaseInfo
//   - moderator pre-signs "dispute_release" into DISPUTE_CLOSE.ReleaseInfo
//
// The counterparty later aggregates that remote signature with its own local
// signature and submits through Complete/DisputeRelease.
type ActionSigner interface {
	SignAction(ctx context.Context, action string, params ActionParams) ([]ActionOwnerSignature, error)
}

// ActionParams is the V2 input shape for confirm/cancel/complete/
// dispute-release. It is a strict superset of V1's InstructionParams,
// adding the policy and refund hints that ManagedEscrow-style adapters need
// without polluting the V1 interface.
type ActionParams struct {
	// OrderID identifies the order.
	OrderID string

	// InitiatorAddr is the legacy wallet address hint of the caller.
	// ClientSigned chains still use it as the frontend signer address.
	// ManagedEscrow-backed chains must not rely on it for authorization or owner
	// selection; the backend resolves the local node owner separately.
	InitiatorAddr string

	// PayoutAddr is the destination address for fund release.
	// For confirm: vendor payout address.
	// For cancel: buyer refund address (D-Hybrid-27 prefers
	// Order.RefundAddress when populated).
	PayoutAddr string

	// PaymentCoin is the payment coin code (e.g., "ETH", "BTC").
	PaymentCoin string

	// PaymentAmount is the payment amount in minimal units.
	PaymentAmount string

	// Chaincode is the hex-encoded chaincode from PaymentSent (V1 parity).
	Chaincode string

	// Script is the hex-encoded script from PaymentSent (V1 parity).
	Script string

	// OrderData carries the pre-fetched order object. ManagedEscrowAdapter
	// requires it to read RefundAddress / CancelFeeAmount / Tier
	// metadata locked at order creation.
	OrderData *models.Order

	// ReleaseInfo carries the moderator-signed shipment release schedule
	// for Complete. Ignored by other actions. Polymorphic to avoid
	// importing pb here — adapters cast to *pb.EscrowRelease.
	ReleaseInfo any
}

// ActionStatus describes the lifecycle of a submitted action. The
// shape mirrors PENDING_RELAY_DESIGN.md's relay_tasks state machine.
type ActionStatus struct {
	// State is one of "pending", "submitting", "submitted",
	// "confirmed", "failed", "abandoned".
	State string

	// TxHash is the on-chain transaction hash once the action lands.
	// May be updated across retries (relay re-broadcast); always the
	// latest broadcast.
	TxHash string

	// Confirmations is the on-chain confirmation count for the latest
	// TxHash. Zero when not yet confirmed.
	Confirmations int

	// LastError carries a human-readable description of the most
	// recent failure (if any). Empty on success.
	LastError string

	// OrderID is populated when available from relay projections。
	OrderID string `json:"orderId,omitempty"`

	// SettlementAction echoes the adapter settlement step (confirm, …)。
	SettlementAction string `json:"settlementAction,omitempty"`

	// RelayTaskID mirrors hosting RelayService task id when returned。
	RelayTaskID string `json:"relayTaskId,omitempty"`
}

// ── Errors ─────────────────────────────────────────────────────

// ErrActionNotFound signals that GetActionStatus could not locate the
// requested ActionID. Callers should treat this as a 404, not a 500.
var ErrActionNotFound = errors.New("payment: action not found")

// ErrUnsupportedAction signals that the underlying adapter cannot
// honor the requested action (e.g., V1AsV2 was asked for a status
// query but no action store exists). Callers SHOULD fall back to V1
// when they detect this error.
var ErrUnsupportedAction = errors.New("payment: action not supported by underlying adapter")

// asV2Marker is implemented by ChainEscrowV2 implementations that are
// already V2-native. Used by Registry to short-circuit wrapping when
// the same instance was registered via RegisterV2.
type asV2Marker interface {
	chainEscrowV2Marker()
}
