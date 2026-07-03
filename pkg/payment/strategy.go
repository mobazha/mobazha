// Package payment defines the chain escrow abstraction layer.
//
// This package provides the [ChainEscrow] interface and [Registry] that
// decouple chain-specific payment logic from the core order state machine.
// Each blockchain (UTXO, EVM, Solana, etc.) implements ChainEscrow to
// declare its payment paradigm and handle chain-specific operations.
//
// Architecture (ChainEscrow + Registry):
//
//	Order State Machine (chain-agnostic)
//	         ↓ queries
//	Payment Registry (maps ChainType → ChainEscrow)
//	         ↓ dispatches to
//	ChainEscrow implementations (chain-specific)
//
// The order state machine remains completely chain-agnostic. When a state
// transition requires a chain-specific operation (e.g., escrow release,
// fund refund), it queries the registry to get the appropriate ChainEscrow implementation.
//
// Capabilities:
//   - AutoConfirm dispatch: handles CANCELABLE payment auto-confirmation
//   - Instruction generation: full order lifecycle instruction generation
package payment

import (
	"context"

	"github.com/mobazha/mobazha/pkg/events"
	"github.com/mobazha/mobazha/pkg/models"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
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
)

// ── Chain Capabilities ──────────────────────────────────────────

// ChainCapabilities describes what a chain supports. Strategy consumers query
// capabilities instead of hardcoding chain type checks (IsEthTypeChain, etc.).
type ChainCapabilities struct {
	// HasReceiptVerification indicates whether the chain's transactions
	// produce verifiable receipts (EVM receipt.Status, Solana Meta.Err, TRON receipt.Result).
	// UTXO chains return false — deposits are detected by address monitoring.
	HasReceiptVerification bool

	// HasClientSignedEscrow indicates whether the escrow is controlled by
	// frontend-signed transactions (EVM/Solana/TRON smart contracts).
	// UTXO chains return false — escrow is backend-signed multisig.
	HasClientSignedEscrow bool

	// EscrowType classifies the adapter's broad escrow mechanism for
	// UI/logging/capability inspection only.
	//
	// IMPORTANT: this is not the same concept as ADR-010 SettlementSpec.EscrowType.
	// ChainCapabilities.EscrowType is a coarse adapter label
	// ("multisig", "smart-contract"), while SettlementSpec.EscrowType is the
	// per-order route triple used to recover payment semantics from persisted
	// order intent.
	//
	// Values: "multisig" (UTXO), "smart-contract" (EVM/Solana/TRON/managed escrow).
	EscrowType string
}

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
	OrderData *models.Order

	// ReleaseInfo carries shipment release data for complete operations.
	ReleaseInfo *pb.EscrowRelease
}

// InstructionResult contains chain-specific instructions for the frontend.
// A nil Instructions field means the backend handles the operation directly
// (e.g., UTXO chains where the backend signs and broadcasts).
type InstructionResult struct {
	// Instructions contains chain-specific data for the frontend.
	// nil = no frontend action needed (backend handles it).
	// EVM: map[string]interface{} with {to, data, value}.
	// Solana: []SolanaGoInstruction.
	// Retained as `any` — true polymorphism across chains; JSON-serialized to frontend.
	Instructions any
}

// ── Escrow Signing Params / Result ──────────────────────────────

// SignEscrowParams provides parameters for escrow signing operations.
// Used by SignEscrowRelease to generate chain-specific signatures.
type SignEscrowParams struct {
	// Transaction is the escrow transaction to sign.
	// UTXO: uses From (inputs) + To (outputs) for multisig signing.
	// EVM/Solana: only uses To (outputs) for signature generation.
	Transaction iwallet.Transaction

	// Script is the redeem script (hex-decoded).
	// UTXO: multisig redeem script. EVM: contract address bytes. Solana: unused.
	Script []byte

	// ChainCode is the HD chain code for key derivation (hex-decoded).
	ChainCode []byte

	// CoinCode is the currency code (e.g. "MCK", "BTC") for wallet resolution.
	// Needed because the UTXO adapter is shared across all UTXO chains.
	CoinCode string
}

// ── Payment Setup Params / Result ───────────────────────────────

// PaymentSetupParams provides parameters for generating initial payment
// instructions (the "order/payment" endpoint). Each strategy adapter uses
// these to build chain-specific escrow initialization or address generation.
type PaymentSetupParams struct {
	// OrderID is the order identifier.
	OrderID string

	// PayerAddress is the buyer's wallet address (format depends on chain).
	PayerAddress string

	// RefundAddress is the buyer-controlled address that receives refunds.
	RefundAddress string

	// Moderator is the moderator's peer ID (empty for no moderator).
	Moderator string

	// StorePolicyRevision snapshots the seller policy revision used to
	// validate Moderator during payment setup.
	StorePolicyRevision uint64

	// CoinType is the payment coin (e.g., "BTC", "ETH", "SOL").
	CoinType iwallet.CoinType

	// Amount is the payment amount in minimal units (satoshis, wei, lamports).
	Amount uint64

	// OrderData carries a caller-supplied order snapshot when the
	// setup/action flow already has one materialized. backend-managed
	// action re-resolution uses this to stay independent from whether
	// the current node persists a local orders row for the order.
	OrderData *models.Order
}

// PaymentSetupResult contains the result of payment setup generation.
type PaymentSetupResult struct {
	// PaymentModel indicates which payment paradigm this result follows.
	PaymentModel PaymentModel

	// IsManagedEscrowOrder is true when the strategy is a managed EVM adapter
	// (Model == PaymentModelMonitored but the chain is EVM, not UTXO).
	// Handlers use this to distinguish managed escrow monitored from UTXO monitored
	// response shapes — managed escrow has no Script / ScriptHash fields.
	IsManagedEscrowOrder bool

	// PaymentData carries chain-specific payment data.
	PaymentData *models.PaymentData

	// EscrowAddr is the escrow account address (ClientSigned only, empty for Monitored).
	EscrowAddr string

	// Instructions contains chain-specific instructions for the frontend.
	// nil for Monitored (backend handles it), non-nil for ClientSigned.
	// Retained as `any` — same polymorphism as InstructionResult.Instructions.
	Instructions any

	// ActionID is the opaque settlement/setup action key when setup was
	// submitted by a backend relayer.
	ActionID string

	// SubmittedTxHash is the relay/broadcast tx hash when setup submitted a
	// transaction synchronously.
	SubmittedTxHash string
}

// ── ChainEscrow Interface ───────────────────────────────────

// ChainEscrow defines chain-level payment operations covering the full
// order lifecycle.
//
// The order state machine itself is chain-agnostic, but each state transition
// involving an on-chain transaction needs chain-specific logic. ChainEscrow
// provides a unified interface for these operations.
//
// # Instruction Generation Methods
//
// Each GetXxxInstructions method returns:
//   - nil Instructions → backend handles the operation (UTXO monitored model)
//   - non-nil Instructions → frontend must sign and submit txHash
//
// The calling code (internal/core/ lifecycle files) handles order validation,
// DB operations, and message sending. Strategy methods only handle the
// chain-specific instruction generation or fund release.
//
// # Escrow Operations
//
// SignEscrowRelease and EstimateEscrowFee centralize chain-specific escrow
// operations that were previously scattered across order lifecycle files as
// 3-way if/else branches (UTXO vs EVM vs Solana).
type ChainEscrow interface {
	// ── Meta ────────────────────────────────────────────

	// Model returns the payment paradigm for this chain.
	Model() PaymentModel

	// Capabilities returns the chain's supported features. Consumers use this
	// instead of hardcoded chain type checks (IsEthTypeChain, ChainSolana, etc.).
	Capabilities() ChainCapabilities

	// ── Auto-Confirm ───────────────────────────────────

	// AutoConfirm handles auto-confirmation for a CANCELABLE payment.
	// Called asynchronously by the cancelable payment dispatcher.
	AutoConfirm(ctx context.Context, event *events.CancelablePaymentReady) error

	// ── Escrow Operations ──────────────────────────────

	// SignEscrowRelease signs an escrow release transaction using the
	// chain-specific key and signing method.
	//
	// UTXO: derives escrow private key from escrowMasterKey + chainCode,
	//       calls escrowWallet.SignMultisigTransaction.
	// EVM:  signs with ethMasterKey via evmpayment.SignEscrowRelease.
	// Solana: signs with solPrivKey via solanapayment.SignEscrowRelease.
	//
	// This method eliminates the 3-way signing branch that was previously
	// duplicated in buildEscrowRelease, releaseCompleteEscrowFunds, and
	// CloseDispute.
	SignEscrowRelease(ctx context.Context, params SignEscrowParams) ([]iwallet.EscrowSignature, error)

	// EstimateEscrowFee returns the estimated fee for an escrow release.
	//
	// UTXO: calculates based on nIn/nOut via escrowWallet.EstimateEscrowFee.
	// nIn is the number of escrow UTXOs consumed by the release transaction.
	// EVM/Solana: returns 0 (fees are paid separately on-chain by the signer).
	//
	// coinCode is needed to resolve the correct chain wallet (e.g. "BTC", "LTC").
	EstimateEscrowFee(coinCode string, nIn, nOut int, feeLevel iwallet.FeeLevel) (iwallet.Amount, error)

	// ── Payment Setup ────────────────────────────────────

	// GeneratePaymentInstructions provisions the initial chain-specific payment
	// setup used by payment-session. It creates the escrow or payment address
	// and returns data needed to project the funding target.
	//
	// Monitored (UTXO): returns payment address + script for monitoring.
	// ClientSigned (EVM/Solana): returns escrow init instructions for frontend.
	//
	// This method eliminates the 3-way chain type switch in the API handler.
	GeneratePaymentInstructions(ctx context.Context, params PaymentSetupParams) (*PaymentSetupResult, error)

	// ── Instruction Generation ─────────────────────────

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

	// ── Deposit Verification ──────────────────────────────
	//
	// VerifyDeposit checks that the buyer's deposit is valid on-chain.
	//
	// Monitored (UTXO): noop — Electrum handles deposit detection.
	// ClientSigned (EVM): checks receipt status + Funded event + escrow hash + amount.
	// ClientSigned (Solana): noop for now (Batch 2).
	//
	// Returns nil if verification succeeds or is not applicable.
	// Returns a sentinel error (ErrDepositReverted, etc.) for permanent failures.
	VerifyDeposit(ctx context.Context, params DepositVerifyParams) error

	// ── Payment Message Validation ───────────────────────
	//
	// ValidatePaymentMessage validates the PaymentSent message structure and
	// amounts against the OrderOpen. Purely computational — no network I/O.
	//
	// Each chain adapter knows its own validation rules:
	//   - UTXO: escrow pubkey derivation + multisig script verification + amount check
	//   - EVM/Solana: escrow address reconstruction + amount check
	//
	// Fiat payments are not routed through the Registry and are validated
	// separately by PaymentVerificationService.
	//
	// Returns nil if validation passes. Default noop for incremental adoption.
	ValidatePaymentMessage(params PaymentMessageParams) error

	// ── Pre-Release Verification ─────────────────────────
	//
	// VerifyPreRelease performs chain-specific safety checks before escrow
	// fund release. Called from PaymentAppService before signing/broadcasting.
	//
	// Monitored (UTXO): queries chain to verify UTXOs are still unspent.
	// ClientSigned (EVM): verifies receipt status + Funded event on-chain.
	// ClientSigned (Solana): noop (Batch 2).
	//
	// Best-effort: implementations should log warnings and return nil if the
	// underlying service is unavailable, to avoid blocking releases.
	VerifyPreRelease(ctx context.Context, params PreReleaseParams) error
}

// ── Deposit Verification Params ─────────────────────────────────

// DepositVerifyParams provides chain-agnostic parameters for on-chain
// deposit verification. Each chain adapter extracts the subset it needs.
type DepositVerifyParams struct {
	// CoinType identifies the payment coin (for chain resolution).
	CoinType iwallet.CoinType

	// TxHash is the transaction hash reported by the buyer.
	TxHash string

	// Script is the hex-encoded escrow script (EVM: serialized EthRedeemScript).
	Script string

	// ContractAddr is the escrow contract address.
	ContractAddr string

	// PaymentAddress is the exact chain address that must receive the deposit.
	// Managed backends use it to verify transaction evidence without requiring
	// Core to construct a concrete chain wallet or RPC client.
	PaymentAddress string

	// PaymentAmount is the expected payment amount in the payment coin's
	// minimal units (wei for ETH, lamports for SOL). This must be in the
	// same currency as the on-chain deposit — NOT the pricing currency.
	PaymentAmount string
}

// ── Payment Message Validation Params ───────────────────────────

// PaymentMessageParams provides parameters for validating a PaymentSent
// message against the original OrderOpen.
type PaymentMessageParams struct {
	// OrderOpen is the original order opening message.
	OrderOpen *pb.OrderOpen

	// PaymentSent is the payment sent message to validate.
	PaymentSent *pb.PaymentSent

	// ExpectedPaymentAmount is the locked payment amount in the payment coin's
	// smallest unit. Address-monitored cross-currency routes must pass this
	// from the payment intent instead of comparing against OrderOpen.Amount.
	ExpectedPaymentAmount string

	// ExpectedPaymentCoin is the canonical coin locked with ExpectedPaymentAmount.
	ExpectedPaymentCoin string

	// EscrowTimeoutHours is the configured escrow timeout.
	EscrowTimeoutHours uint32
}

// ── Pre-Release Verification Params ─────────────────────────────

// PreReleaseParams provides parameters for chain-specific pre-release
// safety checks before escrow fund release.
type PreReleaseParams struct {
	// CoinType identifies the payment coin (for chain resolution).
	CoinType iwallet.CoinType

	// PaymentAddress is the escrow/payment address.
	PaymentAddress string

	// ExpectedUTXOs lists the UTXOs expected to be unspent (UTXO chains only).
	ExpectedUTXOs []iwallet.SpendInfo

	// TxHash is the deposit transaction hash (EVM only).
	TxHash string

	// Script is the hex-encoded escrow script (EVM only).
	Script string

	// ContractAddr is the escrow contract address (EVM only).
	ContractAddr string
}
