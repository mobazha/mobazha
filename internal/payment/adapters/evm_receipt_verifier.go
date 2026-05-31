package adapters

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	evm "github.com/mobazha/mobazha3.0/internal/chains/evm"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

const (
	receiptPollInterval = 3 * time.Second
	receiptPollTimeout  = 60 * time.Second
)

var errNotEVMChain = errors.New("not an EVM chain")

var _ contracts.ReceiptVerifier = (*EVMReceiptVerifier)(nil)

// EVMReceiptVerifier implements contracts.ReceiptVerifier for EVM chains.
// All EVM-specific type assertions (ETHWallet, ChainClient, EVMReceiptFetcher)
// are encapsulated here — App Services never touch go-ethereum types.
type EVMReceiptVerifier struct {
	multiwallet contracts.WalletOperator
}

func NewEVMReceiptVerifier(mw contracts.WalletOperator) *EVMReceiptVerifier {
	return &EVMReceiptVerifier{multiwallet: mw}
}

// getReceiptFetcher resolves the EVMReceiptFetcher for the given coin.
// Returns errNotEVMChain for non-EVM coins (callers treat as noop).
func (v *EVMReceiptVerifier) getReceiptFetcher(coinCode string) (evm.EVMReceiptFetcher, error) {
	coinInfo, err := payment.SettlementCoinInfoForCoin(iwallet.CoinType(coinCode))
	if err != nil || !coinInfo.IsEthTypeChain() {
		return nil, errNotEVMChain
	}

	wallet, ok := v.multiwallet.WalletForChain(coinInfo.Chain)
	if !ok {
		return nil, fmt.Errorf("no wallet for chain %s", coinInfo.Chain)
	}

	ethWallet, ok := wallet.(*evm.ETHWallet)
	if !ok || ethWallet.ChainClient == nil {
		return nil, fmt.Errorf("wallet for %s is not an ETHWallet or has no chain client", coinCode)
	}

	fetcher, ok := ethWallet.ChainClient.(evm.EVMReceiptFetcher)
	if !ok {
		return nil, fmt.Errorf("chain client for %s does not implement EVMReceiptFetcher", coinCode)
	}

	return fetcher, nil
}

func (v *EVMReceiptVerifier) VerifyTransactionReceipt(ctx context.Context, coinCode string, txHash string) error {
	fetcher, err := v.getReceiptFetcher(coinCode)
	if err != nil {
		if errors.Is(err, errNotEVMChain) {
			return nil
		}
		return nil
	}

	receipt, err := fetcher.TransactionReceipt(ctx, common.HexToHash(txHash))
	if err != nil {
		return nil
	}

	if receipt.Status != types.ReceiptStatusSuccessful {
		return payment.ErrTransactionReverted
	}

	return nil
}

func (v *EVMReceiptVerifier) WaitAndVerifyReceipt(ctx context.Context, coinCode string, txHash string) error {
	fetcher, err := v.getReceiptFetcher(coinCode)
	if err != nil {
		if errors.Is(err, errNotEVMChain) {
			return nil
		}
		return nil
	}

	receipt, err := waitForReceipt(ctx, fetcher, txHash)
	if err != nil {
		return err
	}

	if receipt.Status != types.ReceiptStatusSuccessful {
		return payment.ErrTransactionReverted
	}

	return nil
}

func waitForReceipt(ctx context.Context, fetcher evm.EVMReceiptFetcher, txHash string) (*types.Receipt, error) {
	deadline := time.After(receiptPollTimeout)
	hash := common.HexToHash(txHash)

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-deadline:
			return nil, payment.ErrReceiptTimeout
		default:
		}

		receipt, err := fetcher.TransactionReceipt(ctx, hash)
		if err == nil && receipt != nil {
			return receipt, nil
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-deadline:
			return nil, payment.ErrReceiptTimeout
		case <-time.After(receiptPollInterval):
		}
	}
}
