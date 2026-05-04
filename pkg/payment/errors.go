package payment

import "errors"

// Chain-agnostic deposit verification errors.
// Returned by ChainEscrow.VerifyDeposit implementations and checked
// by the OrderProcessor in payment_sent.go to distinguish permanent
// failures (reject order) from transient ones (retry later).
var (
	ErrDepositReverted      = errors.New("deposit transaction reverted on-chain")
	ErrDepositTargetInvalid = errors.New("deposit target does not match expected contract")
	ErrDepositEventNotFound = errors.New("deposit funding event not found in transaction logs")
)

// Chain-agnostic transaction receipt errors.
// Used by confirm receipt verification and relay receipt waiting.
var (
	ErrTransactionReverted = errors.New("transaction reverted on-chain (receipt status=0)")
	ErrReceiptTimeout      = errors.New("timed out waiting for transaction receipt")
)
