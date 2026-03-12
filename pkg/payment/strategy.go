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
// Capabilities:
//   - AutoConfirm dispatch: handles CANCELABLE payment auto-confirmation
//   - Instruction generation: full order lifecycle instruction generation
package payment

import (
	"context"

	"github.com/mobazha/mobazha3.0/pkg/events"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
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
	//
	// Concrete type: *models.Order (pkg/models.Order).
	// Passed as any to keep pkg/payment free of model imports — adapters in
	// internal/core/ type-assert to *models.Order.
	OrderData any

	// ReleaseInfo carries fulfillment release data for complete operations.
	//
	// Concrete type: *pb.EscrowRelease (pkg/orders/mbzpb.EscrowRelease).
	// Passed as any to keep pkg/payment free of protobuf imports — adapters
	// in internal/core/ type-assert to *pb.EscrowRelease.
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

	// Moderator is the moderator's peer ID (empty for no moderator).
	Moderator string

	// CoinType is the payment coin (e.g., "BTC", "ETH", "SOL").
	CoinType iwallet.CoinType

	// Amount is the payment amount in minimal units (satoshis, wei, lamports).
	Amount uint64
}

// PaymentSetupResult contains the result of payment instruction generation.
// The handler formats the JSON response differently based on PaymentModel.
type PaymentSetupResult struct {
	// PaymentModel indicates which payment paradigm this result follows.
	PaymentModel PaymentModel

	// PaymentData carries chain-specific payment data.
	// Concrete type: *models.PaymentData — adapters populate this.
	PaymentData any

	// EscrowAddr is the escrow account address (ClientSigned only, empty for Monitored).
	EscrowAddr string

	// Instructions contains chain-specific instructions for the frontend.
	// nil for Monitored (backend handles it), non-nil for ClientSigned.
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
type PaymentStrategy interface {
	// ── Meta ────────────────────────────────────────────

	// Model returns the payment paradigm for this chain.
	Model() PaymentModel

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
	// EVM/Solana: returns 0 (fees are paid separately on-chain by the signer).
	//
	// coinCode is needed to resolve the correct chain wallet (e.g. "BTC", "LTC").
	EstimateEscrowFee(coinCode string, nIn, nOut int, feeLevel iwallet.FeeLevel) (iwallet.Amount, error)

	// ── Payment Setup ────────────────────────────────────

	// GeneratePaymentInstructions generates initial payment instructions
	// for the "order/payment" endpoint. This is the entry point for setting
	// up a payment — it creates the escrow or payment address and returns
	// data the frontend needs to execute the payment.
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

	// OrderAmount is the expected payment amount in minimal units (wei, lamports).
	OrderAmount string
}

// ── Payment Message Validation Params ───────────────────────────

// PaymentMessageParams provides parameters for validating a PaymentSent
// message against the original OrderOpen. Each chain adapter type-asserts
// the any fields to their concrete protobuf types.
type PaymentMessageParams struct {
	// OrderOpen is the original order opening message.
	// Concrete type: *pb.OrderOpen (pkg/orders/mbzpb.OrderOpen).
	OrderOpen any

	// PaymentSent is the payment sent message to validate.
	// Concrete type: *pb.PaymentSent (pkg/orders/mbzpb.PaymentSent).
	PaymentSent any

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
