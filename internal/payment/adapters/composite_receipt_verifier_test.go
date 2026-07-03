package adapters

import (
	"context"
	"testing"

	"github.com/ethereum/go-ethereum/core/types"
	evm "github.com/mobazha/mobazha/internal/chains/evm"
	"github.com/mobazha/mobazha/pkg/payment"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type compositeMultiwallet struct {
	evmWallet iwallet.Wallet
}

func (m *compositeMultiwallet) WalletForCurrencyCode(code string) (iwallet.Wallet, error) {
	coinInfo, err := iwallet.CoinInfoFromCoinType(iwallet.CoinType(code))
	if err != nil {
		return nil, err
	}
	if coinInfo.IsEthTypeChain() {
		return m.evmWallet, nil
	}
	return nil, nil
}

func (m *compositeMultiwallet) SupportedChains() []iwallet.ChainType { return nil }

func (m *compositeMultiwallet) WalletForChain(chain iwallet.ChainType) (iwallet.Wallet, bool) {
	if chain != iwallet.ChainEthereum || m.evmWallet == nil {
		return nil, false
	}
	return m.evmWallet, true
}

func (m *compositeMultiwallet) Start() error { return nil }
func (m *compositeMultiwallet) Close() error { return nil }

func makeCompositeVerifier(evmFetcher iwallet.ChainClient) *CompositeReceiptVerifier {
	ethWallet := &evm.ETHWallet{}
	ethWallet.ChainClient = evmFetcher
	return NewCompositeReceiptVerifier(&compositeMultiwallet{evmWallet: ethWallet})
}

func TestComposite_DispatchesToEVM(t *testing.T) {
	v := makeCompositeVerifier(&mockReceiptFetcher{receipt: revertedReceipt()})
	err := v.VerifyTransactionReceipt(context.Background(), testETH, "0xabc123")
	assert.ErrorIs(t, err, payment.ErrTransactionReverted)
}

func TestComposite_NonSupportedChainNoop(t *testing.T) {
	v := makeCompositeVerifier(&mockReceiptFetcher{receipt: revertedReceipt()})
	err := v.VerifyTransactionReceipt(context.Background(), "BTC", "txhash")
	require.NoError(t, err, "BTC should be noop")
}

func TestComposite_UnknownCoinNoop(t *testing.T) {
	v := makeCompositeVerifier(&mockReceiptFetcher{receipt: revertedReceipt()})
	err := v.VerifyTransactionReceipt(context.Background(), "UNKNOWN_COIN", "txhash")
	require.NoError(t, err, "unknown coin should be noop")
}

func TestComposite_WaitDispatchesToEVM(t *testing.T) {
	v := makeCompositeVerifier(&mockReceiptFetcher{receipt: &types.Receipt{Status: types.ReceiptStatusSuccessful}})
	err := v.WaitAndVerifyReceipt(context.Background(), testETH, "0xabc123")
	require.NoError(t, err)
}
