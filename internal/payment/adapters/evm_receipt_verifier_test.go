package adapters

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	evm "github.com/mobazha/mobazha3.0/internal/chains/evm"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Mock receipt fetcher ────────────────────────────────────────────────

type mockReceiptFetcher struct {
	receipt    *types.Receipt
	receiptErr error
	callCount  int
}

func (m *mockReceiptFetcher) TransactionReceipt(_ context.Context, _ common.Hash) (*types.Receipt, error) {
	m.callCount++
	if m.receiptErr != nil {
		return nil, m.receiptErr
	}
	return m.receipt, nil
}

func (m *mockReceiptFetcher) TransactionByHash(_ context.Context, _ common.Hash) (*types.Transaction, bool, error) {
	return nil, false, nil
}

type delayedReceiptFetcher struct {
	receipt   *types.Receipt
	failCount int
	maxFails  int
}

func (d *delayedReceiptFetcher) TransactionReceipt(_ context.Context, _ common.Hash) (*types.Receipt, error) {
	d.failCount++
	if d.failCount <= d.maxFails {
		return nil, errors.New("not found")
	}
	return d.receipt, nil
}

func (d *delayedReceiptFetcher) TransactionByHash(_ context.Context, _ common.Hash) (*types.Transaction, bool, error) {
	return nil, false, nil
}

// ── Mock multiwallet ────────────────────────────────────────────────────

type mockMultiwalletForReceipt struct {
	wallet    iwallet.Wallet
	walletErr error
}

func (m *mockMultiwalletForReceipt) WalletForCurrencyCode(_ string) (iwallet.Wallet, error) {
	if m.walletErr != nil {
		return nil, m.walletErr
	}
	return m.wallet, nil
}

func (m *mockMultiwalletForReceipt) SupportedChains() []iwallet.ChainType { return nil }

func (m *mockMultiwalletForReceipt) WalletForChain(_ iwallet.ChainType) (iwallet.Wallet, bool) {
	if m.wallet == nil {
		return nil, false
	}
	return m.wallet, true
}

func (m *mockMultiwalletForReceipt) Start() error { return nil }
func (m *mockMultiwalletForReceipt) Close() error { return nil }

// ── Helpers ─────────────────────────────────────────────────────────────

func successReceipt() *types.Receipt {
	return &types.Receipt{Status: types.ReceiptStatusSuccessful}
}

func revertedReceipt() *types.Receipt {
	return &types.Receipt{Status: types.ReceiptStatusFailed}
}

func makeVerifier(fetcher evm.EVMReceiptFetcher) *EVMReceiptVerifier {
	ethWallet := &evm.ETHWallet{}
	ethWallet.ChainClient = fetcher
	mw := &mockMultiwalletForReceipt{wallet: ethWallet}
	return NewEVMReceiptVerifier(mw)
}

// ── Tests: VerifyTransactionReceipt ─────────────────────────────────────

func TestVerifyTransactionReceipt_Success(t *testing.T) {
	v := makeVerifier(&mockReceiptFetcher{receipt: successReceipt()})
	err := v.VerifyTransactionReceipt(context.Background(), "TETH", "0xabc123")
	require.NoError(t, err)
}

func TestVerifyTransactionReceipt_Reverted(t *testing.T) {
	v := makeVerifier(&mockReceiptFetcher{receipt: revertedReceipt()})
	err := v.VerifyTransactionReceipt(context.Background(), "TETH", "0xabc123")
	assert.ErrorIs(t, err, payment.ErrTransactionReverted)
}

func TestVerifyTransactionReceipt_NonEVM(t *testing.T) {
	v := &EVMReceiptVerifier{}
	err := v.VerifyTransactionReceipt(context.Background(), "BTC", "0xabc123")
	require.NoError(t, err, "non-EVM coin should return nil (noop)")
}

func TestVerifyTransactionReceipt_RPCError(t *testing.T) {
	v := makeVerifier(&mockReceiptFetcher{receiptErr: errors.New("RPC timeout")})
	err := v.VerifyTransactionReceipt(context.Background(), "TETH", "0xabc123")
	require.NoError(t, err, "RPC errors are best-effort — should return nil")
}

// ── Tests: WaitAndVerifyReceipt ─────────────────────────────────────────

func TestWaitAndVerifyReceipt_Success(t *testing.T) {
	v := makeVerifier(&delayedReceiptFetcher{receipt: successReceipt(), maxFails: 2})
	err := v.WaitAndVerifyReceipt(context.Background(), "TETH", "0xabc123")
	require.NoError(t, err)
}

func TestWaitAndVerifyReceipt_Timeout(t *testing.T) {
	v := makeVerifier(&mockReceiptFetcher{receiptErr: errors.New("not found")})
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	err := v.WaitAndVerifyReceipt(ctx, "TETH", "0xabc123")
	require.Error(t, err)
}

func TestWaitAndVerifyReceipt_Reverted(t *testing.T) {
	v := makeVerifier(&mockReceiptFetcher{receipt: revertedReceipt()})
	err := v.WaitAndVerifyReceipt(context.Background(), "TETH", "0xabc123")
	assert.ErrorIs(t, err, payment.ErrTransactionReverted)
}

// ── Tests: waitForReceipt (package-level function) ──────────────────────

func TestWaitForReceipt_DelayedSuccess(t *testing.T) {
	fetcher := &delayedReceiptFetcher{receipt: successReceipt(), maxFails: 2}
	receipt, err := waitForReceipt(context.Background(), fetcher, "0xabc123")
	require.NoError(t, err)
	require.NotNil(t, receipt)
	assert.Equal(t, types.ReceiptStatusSuccessful, receipt.Status)
	assert.Equal(t, 3, fetcher.failCount)
}

func TestWaitForReceipt_ContextCancelled(t *testing.T) {
	fetcher := &mockReceiptFetcher{receiptErr: errors.New("not found")}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	receipt, err := waitForReceipt(ctx, fetcher, "0xabc123")
	require.Error(t, err)
	assert.Nil(t, receipt)
}
