package contracts

import "context"

// ReceiptVerifier abstracts on-chain transaction receipt verification.
// EVM implementation checks receipt.Status; non-EVM chains return nil (noop).
//
// Port interface — defined in pkg/contracts/ so that App Services in
// internal/core/ can depend on it without importing chain-specific packages.
type ReceiptVerifier interface {
	// VerifyTransactionReceipt checks the on-chain receipt status.
	//   - receipt.Status == 0 → payment.ErrTransactionReverted (fatal)
	//   - RPC / transient errors → nil (best-effort, logged)
	//   - non-EVM coin → nil (noop)
	VerifyTransactionReceipt(ctx context.Context, coinCode string, txHash string) error

	// WaitAndVerifyReceipt polls for a receipt and then verifies its status.
	//   - receipt.Status == 0 → payment.ErrTransactionReverted (fatal)
	//   - timeout → payment.ErrReceiptTimeout
	//   - non-EVM coin → nil (noop)
	WaitAndVerifyReceipt(ctx context.Context, coinCode string, txHash string) error
}
