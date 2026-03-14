package adapters

import (
	"context"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

var _ contracts.ReceiptVerifier = (*CompositeReceiptVerifier)(nil)

// CompositeReceiptVerifier dispatches receipt verification to the appropriate
// chain-specific verifier based on the coin code. Non-supported chains are
// treated as noop (return nil).
type CompositeReceiptVerifier struct {
	evm    *EVMReceiptVerifier
	solana *SolanaReceiptVerifier
}

func NewCompositeReceiptVerifier(mw contracts.WalletOperator) *CompositeReceiptVerifier {
	return &CompositeReceiptVerifier{
		evm:    NewEVMReceiptVerifier(mw),
		solana: NewSolanaReceiptVerifier(mw),
	}
}

func (c *CompositeReceiptVerifier) VerifyTransactionReceipt(ctx context.Context, coinCode string, txHash string) error {
	coinInfo, err := iwallet.CoinInfoFromCoinType(iwallet.CoinType(coinCode))
	if err != nil {
		return nil
	}

	switch {
	case coinInfo.IsEthTypeChain():
		return c.evm.VerifyTransactionReceipt(ctx, coinCode, txHash)
	case coinInfo.Chain == iwallet.ChainSolana:
		return c.solana.VerifyTransactionReceipt(ctx, coinCode, txHash)
	default:
		return nil
	}
}

func (c *CompositeReceiptVerifier) WaitAndVerifyReceipt(ctx context.Context, coinCode string, txHash string) error {
	coinInfo, err := iwallet.CoinInfoFromCoinType(iwallet.CoinType(coinCode))
	if err != nil {
		return nil
	}

	switch {
	case coinInfo.IsEthTypeChain():
		return c.evm.WaitAndVerifyReceipt(ctx, coinCode, txHash)
	case coinInfo.Chain == iwallet.ChainSolana:
		return c.solana.WaitAndVerifyReceipt(ctx, coinCode, txHash)
	default:
		return nil
	}
}
