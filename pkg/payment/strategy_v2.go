package payment

import (
	"context"
	"errors"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/mobazha/mobazha/pkg/events"
	"github.com/mobazha/mobazha/pkg/models"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

// ManagedEscrowFeePolicy is provider-owned pricing policy returned by a
// managed-escrow strategy. Core converts the USD-minor amount into the locked
// payment asset and persists the resulting fee with the payment intent.
type ManagedEscrowFeePolicy struct {
	ReleaseFeeUSDCents uint64
	ChargeCancellation bool
}

// ManagedEscrowFeePolicyProvider is implemented by managed-escrow strategies
// that require a service fee locked at payment-intent creation.
type ManagedEscrowFeePolicyProvider interface {
	ManagedEscrowFeePolicy() ManagedEscrowFeePolicy
}

// ManagedEscrowReceiptValidationRequest carries immutable relay projection
// context and its mined EVM receipt to the strategy that created the action.
type ManagedEscrowReceiptValidationRequest struct {
	ActionID      string
	OrderID       string
	ActionKind    string
	ChainID       uint64
	EscrowAddress string
	TxHash        string
	Receipt       *types.Receipt
}

// ManagedEscrowReceiptValidator lets a managed-escrow strategy validate
// provider-specific execution evidence before Core confirms an action.
type ManagedEscrowReceiptValidator interface {
	ValidateManagedEscrowReceipt(ctx context.Context, request ManagedEscrowReceiptValidationRequest) error
}

// AttemptSettlementFundingRequest contains the participant-authorized facts a
// monitored rail needs to deterministically project one immutable funding
// target. Implementations must not return provider secrets or private keys.
type AttemptSettlementFundingRequest struct {
	Attempt models.PaymentAttempt
	Route   models.PaymentRouteBinding
	Offers  []models.SettlementKeyOffer
}

// AttemptSettlementFundingProjector is an optional V2 capability for rails
// that can rebuild the same funding target on every participant node.
type AttemptSettlementFundingProjector interface {
	ProjectAttemptSettlementFundingTarget(
		context.Context,
		AttemptSettlementFundingRequest,
	) (models.PaymentAttemptFundingTarget, error)
}

// AttemptSettlementFundingActivator starts monitoring or materializes any
// chain state required after an authorization snapshot has been frozen. It
// must be idempotent because adoption and restart recovery can repeat it.
type AttemptSettlementFundingActivator interface {
	ActivateAttemptSettlementFunding(
		context.Context,
		AttemptSettlementFundingRequest,
		models.PaymentAttemptFundingTarget,
	) error
}

// AttemptSettlementActionSignRequest binds a chain action to the exact frozen
// attempt snapshot before a participant owner signature is produced.
type AttemptSettlementActionSignRequest struct {
	Action        string
	Sequence      uint64
	Authorization models.PaymentAttemptSettlementAuthorization
	Params        ActionParams
}

// AttemptSettlementActionAuthorizer is required before a monitored rail can
// opt into attempt-scoped funding. It prevents a projector-only integration
// from accepting funds that later fall back to order-global owner keys.
type AttemptSettlementActionAuthorizer interface {
	SignAttemptSettlementAction(
		context.Context,
		AttemptSettlementActionSignRequest,
	) ([]ActionOwnerSignature, error)
}

// ChainEscrowV2 is the action-centric counterpart to ChainEscrow (V1).
//
// Background — Phase managed EVM v0.3.0 (D-Hybrid-17 A+ scheme):
//
// V1 ChainEscrow exposes per-action GetXxxInstructions() returning
// frontend-signable payloads. That works for ClientSigned chains
// (Solana / TRON) where the frontend wallet signs and submits the tx,
// but does not fit the managed escrow v1.4.1 path: nodes do not hold private
// keys for the SaaS Relay's gas wallet; the backend submits the
// transaction itself after collecting escrow owner signatures. Solana
// Anchor follows the same V2 rule: the backend relays program
// instructions instead of returning settlement instructions to the
// frontend.
//
// V2 raises the abstraction one notch — callers express *intent*
// ("cancel this order") and the adapter decides the model:
//
//   - managed escrow (Monitored): backend builds + relays the EscrowTx; the
//     ActionResult carries an ActionID so the frontend can poll status.
//   - ClientSigned (V1AsV2 wrap): the adapter returns its V1
//     instructions in ActionResult.Instructions and signals
//     ActionModeInstructionsRequired — the frontend keeps signing.
//
// The two interfaces coexist:
//   - V1 implementations stay registered as today;
//     Registry.ForCoinV2 wraps them via V1AsV2 transparently.
//   - V2-native implementations (ManagedEscrowAdapter, SolanaAnchorAdapter) register via
//     Registry.RegisterV2.
//
// Migration path (no in-flight orders to preserve, per D-Hybrid-23):
//   - Sprint 0/1: introduce V2 + wrapper; existing V1 callers
//     unaffected.
//   - Sprint 2: migrate handlers to V2; TRON keeps V1 and is wrapped
//     on read, while Solana Anchor registers as V2-native.
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
	// nIn is the number of escrow UTXOs consumed by the release transaction.
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
	// "submit signed tx" flow. Used by V1AsV2 wrapping legacy
	// ClientSigned chains (TRON and non-managed EVM).
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
	// synchronously (e.g. managed relay). Callers should prefer this over
	// polling GetActionStatus immediately after submit.
	SubmittedTxHash string

	// SettlementCoin/GrossAmount/PlannedLines describe the chain-neutral
	// settlement outcome the submitted action is expected to produce.
	SettlementCoin string
	GrossAmount    string
	PlannedLines   []models.SettlementPayoutLine
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
	// managed escrow uses it as an audit hint; UTXO-style escrows may ignore it.
	Index uint32
}

// ActionSigner is an optional V2 capability implemented by backends that can
// produce a local owner signature for a specific settlement action without
// broadcasting it yet. managed escrow uses this for threshold>1 flows:
//   - seller pre-signs "complete" into ORDER_SHIPMENT.ReleaseInfo
//   - moderator pre-signs "dispute_release" into DISPUTE_CLOSE.ReleaseInfo
//
// The counterparty later aggregates that remote signature with its own local
// signature and submits through Complete/DisputeRelease.
type ActionSigner interface {
	SignAction(ctx context.Context, action string, params ActionParams) ([]ActionOwnerSignature, error)
}

// ActionReconciler is an optional private-module capability that confirms or
// retries one durable backend action. Core supplies only the opaque action ID;
// provider-specific intent decoding and chain status checks stay in the module.
type ActionReconciler interface {
	ReconcileAction(ctx context.Context, actionID string) (*ActionStatus, error)
}

// DepositTransactionVerifier is an optional strategy capability for managed
// chains whose transaction RPC and protocol interpretation live outside Open
// Core. Implementations return a generic transaction only after validating the
// reported transaction, recipient, asset, and minimum amount.
//
// PaymentVerificationService prefers this capability over the legacy
// multiwallet lookup. This keeps Core provider-neutral while preserving the
// same VerifiedPayment contract consumed by the order state machine.
type DepositTransactionVerifier interface {
	FetchAndVerifyDeposit(ctx context.Context, params DepositVerifyParams) (*iwallet.Transaction, error)
}

// SellerDeclineRefunder is an optional V2 capability for chains whose on-chain
// program distinguishes seller-initiated refunds from buyer cancel.
type SellerDeclineRefunder interface {
	SellerDeclineRefund(ctx context.Context, params ActionParams) (*ActionResult, error)
}

// ActionParams is the V2 input shape for confirm/cancel/complete/
// dispute-release. It is a strict superset of V1's InstructionParams,
// adding the policy and refund hints that managed escrow-style adapters need
// without polluting the V1 interface.
type ActionParams struct {
	// OrderID identifies the order.
	OrderID string

	// InitiatorAddr is the legacy wallet address hint of the caller.
	// ClientSigned chains still use it as the frontend signer address.
	// backend-managed chains must not rely on it for authorization or owner
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

	// AffiliatePayout is the Core-validated seller-funded commission split for
	// this settlement action. Adapters execute it as an additional output and
	// deduct the same amount from the seller output; they must not recalculate
	// affiliate terms from order or profile data.
	AffiliatePayout *models.AffiliateSettlementPayout
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

	SettlementCoin string                        `json:"settlementCoin,omitempty"`
	GrossAmount    string                        `json:"grossAmount,omitempty"`
	PlannedLines   []models.SettlementPayoutLine `json:"plannedLines,omitempty"`
	ObservedLines  []models.SettlementPayoutLine `json:"observedLines,omitempty"`
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
