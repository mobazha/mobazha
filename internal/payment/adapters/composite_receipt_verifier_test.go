package adapters

import (
	"context"
	"testing"

	"github.com/ethereum/go-ethereum/core/types"
	evm "github.com/mobazha/mobazha3.0/internal/chains/evm"
	solchain "github.com/mobazha/mobazha3.0/internal/chains/solana"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// compositeMultiwallet returns different wallets depending on the chain type.
type compositeMultiwallet struct {
	evmWallet    iwallet.Wallet
	solanaWallet iwallet.Wallet
}

func (m *compositeMultiwallet) WalletForCurrencyCode(code string) (iwallet.Wallet, error) {
	coinInfo, err := iwallet.CoinInfoFromCoinType(iwallet.CoinType(code))
	if err != nil {
		return nil, err
	}
	switch {
	case coinInfo.IsEthTypeChain():
		return m.evmWallet, nil
	case coinInfo.Chain == iwallet.ChainSolana:
		return m.solanaWallet, nil
	}
	return nil, nil
}

func (m *compositeMultiwallet) SupportedChains() []iwallet.ChainType { return nil }

func (m *compositeMultiwallet) WalletForChain(chain iwallet.ChainType) (iwallet.Wallet, bool) {
	switch chain {
	case iwallet.ChainEthereum:
		if m.evmWallet == nil {
			return nil, false
		}
		return m.evmWallet, true
	case iwallet.ChainSolana:
		if m.solanaWallet == nil {
			return nil, false
		}
		return m.solanaWallet, true
	}
	return nil, false
}

func (m *compositeMultiwallet) Start() error { return nil }
func (m *compositeMultiwallet) Close() error { return nil }

func makeCompositeVerifier(evmFetcher iwallet.ChainClient, solanaChecker iwallet.ChainClient) *CompositeReceiptVerifier {
	ethWallet := &evm.ETHWallet{}
	ethWallet.ChainClient = evmFetcher

	solWallet := &solchain.SolanaWallet{}
	solWallet.ChainClient = solanaChecker

	mw := &compositeMultiwallet{evmWallet: ethWallet, solanaWallet: solWallet}
	return NewCompositeReceiptVerifier(mw)
}

func TestComposite_DispatchesToEVM(t *testing.T) {
	v := makeCompositeVerifier(
		&mockReceiptFetcher{receipt: revertedReceipt()},
		&mockSolanaChecker{status: solanaSuccessStatus()},
	)
	err := v.VerifyTransactionReceipt(context.Background(), testETH, "0xabc123")
	assert.ErrorIs(t, err, payment.ErrTransactionReverted)
}

func TestComposite_DispatchesToSolana(t *testing.T) {
	v := makeCompositeVerifier(
		&mockReceiptFetcher{receipt: successReceipt()},
		&mockSolanaChecker{status: solanaRevertedStatus()},
	)
	err := v.VerifyTransactionReceipt(context.Background(), testSOL, "5abc123")
	assert.ErrorIs(t, err, payment.ErrTransactionReverted)
}

func TestComposite_NonSupportedChainNoop(t *testing.T) {
	v := makeCompositeVerifier(
		&mockReceiptFetcher{receipt: revertedReceipt()},
		&mockSolanaChecker{status: solanaRevertedStatus()},
	)
	err := v.VerifyTransactionReceipt(context.Background(), "BTC", "txhash")
	require.NoError(t, err, "BTC should be noop")
}

func TestComposite_UnknownCoinNoop(t *testing.T) {
	v := makeCompositeVerifier(
		&mockReceiptFetcher{receipt: revertedReceipt()},
		&mockSolanaChecker{status: solanaRevertedStatus()},
	)
	err := v.VerifyTransactionReceipt(context.Background(), "UNKNOWN_COIN", "txhash")
	require.NoError(t, err, "unknown coin should be noop")
}

func TestComposite_WaitDispatchesToEVM(t *testing.T) {
	v := makeCompositeVerifier(
		&mockReceiptFetcher{receipt: &types.Receipt{Status: types.ReceiptStatusSuccessful}},
		&mockSolanaChecker{status: solanaSuccessStatus()},
	)
	err := v.WaitAndVerifyReceipt(context.Background(), testETH, "0xabc123")
	require.NoError(t, err)
}

func TestComposite_WaitDispatchesToSolana(t *testing.T) {
	v := makeCompositeVerifier(
		&mockReceiptFetcher{receipt: successReceipt()},
		&delayedSolanaChecker{status: solanaSuccessStatus(), maxFails: 1},
	)
	err := v.WaitAndVerifyReceipt(context.Background(), testSOL, "5abc123")
	require.NoError(t, err)
}
