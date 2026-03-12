package evm

import (
	"context"
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockReceiptFetcher struct {
	receipt *types.Receipt
	tx      *types.Transaction
	pending bool
	err     error
	txErr   error
}

func (m *mockReceiptFetcher) TransactionReceipt(_ context.Context, _ common.Hash) (*types.Receipt, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.receipt, nil
}

func (m *mockReceiptFetcher) TransactionByHash(_ context.Context, _ common.Hash) (*types.Transaction, bool, error) {
	if m.txErr != nil {
		return nil, false, m.txErr
	}
	return m.tx, m.pending, nil
}

var (
	testTxHash     = "0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	testEscrowAddr = common.HexToAddress("0x1111111111111111111111111111111111111111")
	testTokenAddr  = common.HexToAddress("0x2222222222222222222222222222222222222222")
	testEscrowHash = common.HexToHash("0x3333333333333333333333333333333333333333333333333333333333333333")
	testFromHash   = common.HexToHash("0x0000000000000000000000004444444444444444444444444444444444444444")
	testMinAmount  = big.NewInt(1000000)
)

func makeSuccessReceipt(logs []*types.Log) *types.Receipt {
	return &types.Receipt{
		Status: types.ReceiptStatusSuccessful,
		Logs:   logs,
	}
}

func makeRevertedReceipt() *types.Receipt {
	return &types.Receipt{
		Status: types.ReceiptStatusFailed,
	}
}

func makeFundedLog(addr common.Address, escrowHash common.Hash, amount *big.Int) *types.Log {
	return &types.Log{
		Address: addr,
		Topics:  []common.Hash{fundedEventTopic, escrowHash, testFromHash},
		Data:    common.LeftPadBytes(amount.Bytes(), 32),
	}
}

func makeNativeTx(to common.Address) *types.Transaction {
	return types.NewTx(&types.LegacyTx{
		To:    &to,
		Value: big.NewInt(1000000),
	})
}

func makeERC20Tx(tokenAddr common.Address) *types.Transaction {
	return types.NewTx(&types.LegacyTx{
		To:    &tokenAddr,
		Value: big.NewInt(0),
	})
}

func TestVerifyDeposit_HappyPath_NativeETH(t *testing.T) {
	fundedLog := makeFundedLog(testEscrowAddr, testEscrowHash, testMinAmount)
	fetcher := &mockReceiptFetcher{
		receipt: makeSuccessReceipt([]*types.Log{fundedLog}),
		tx:      makeNativeTx(testEscrowAddr),
	}

	err := VerifyDeposit(context.Background(), fetcher, DepositVerification{
		TxHash:       testTxHash,
		EscrowHash:   testEscrowHash,
		ExpectedAddr: testEscrowAddr,
		MinAmount:    testMinAmount,
	})

	require.NoError(t, err)
}

func TestVerifyDeposit_HappyPath_ERC20(t *testing.T) {
	fundedLog := makeFundedLog(testEscrowAddr, testEscrowHash, testMinAmount)
	fetcher := &mockReceiptFetcher{
		receipt: makeSuccessReceipt([]*types.Log{fundedLog}),
		tx:      makeERC20Tx(testTokenAddr),
	}

	err := VerifyDeposit(context.Background(), fetcher, DepositVerification{
		TxHash:       testTxHash,
		EscrowHash:   testEscrowHash,
		ExpectedAddr: testEscrowAddr,
		MinAmount:    testMinAmount,
	})

	require.NoError(t, err)
}

func TestVerifyDeposit_RevertedTx(t *testing.T) {
	fetcher := &mockReceiptFetcher{
		receipt: makeRevertedReceipt(),
		tx:      makeNativeTx(testEscrowAddr),
	}

	err := VerifyDeposit(context.Background(), fetcher, DepositVerification{
		TxHash:       testTxHash,
		EscrowHash:   testEscrowHash,
		ExpectedAddr: testEscrowAddr,
		MinAmount:    testMinAmount,
	})

	assert.ErrorIs(t, err, payment.ErrDepositReverted)
}

func TestVerifyDeposit_WrongEscrowTarget(t *testing.T) {
	otherAddr := common.HexToAddress("0x9999999999999999999999999999999999999999")
	fundedLog := makeFundedLog(otherAddr, testEscrowHash, testMinAmount)
	fetcher := &mockReceiptFetcher{
		receipt: makeSuccessReceipt([]*types.Log{fundedLog}),
		tx:      makeNativeTx(otherAddr),
	}

	err := VerifyDeposit(context.Background(), fetcher, DepositVerification{
		TxHash:       testTxHash,
		EscrowHash:   testEscrowHash,
		ExpectedAddr: testEscrowAddr,
		MinAmount:    testMinAmount,
	})

	assert.ErrorIs(t, err, payment.ErrDepositTargetInvalid)
}

func TestVerifyDeposit_MismatchedEscrowHash(t *testing.T) {
	wrongHash := common.HexToHash("0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	fundedLog := makeFundedLog(testEscrowAddr, wrongHash, testMinAmount)
	fetcher := &mockReceiptFetcher{
		receipt: makeSuccessReceipt([]*types.Log{fundedLog}),
		tx:      makeNativeTx(testEscrowAddr),
	}

	err := VerifyDeposit(context.Background(), fetcher, DepositVerification{
		TxHash:       testTxHash,
		EscrowHash:   testEscrowHash,
		ExpectedAddr: testEscrowAddr,
		MinAmount:    testMinAmount,
	})

	assert.ErrorIs(t, err, payment.ErrDepositEventNotFound)
}

func TestVerifyDeposit_InsufficientAmount(t *testing.T) {
	tooLow := big.NewInt(500000)
	fundedLog := makeFundedLog(testEscrowAddr, testEscrowHash, tooLow)
	fetcher := &mockReceiptFetcher{
		receipt: makeSuccessReceipt([]*types.Log{fundedLog}),
		tx:      makeNativeTx(testEscrowAddr),
	}

	err := VerifyDeposit(context.Background(), fetcher, DepositVerification{
		TxHash:       testTxHash,
		EscrowHash:   testEscrowHash,
		ExpectedAddr: testEscrowAddr,
		MinAmount:    testMinAmount,
	})

	assert.ErrorIs(t, err, payment.ErrDepositEventNotFound)
}

func TestVerifyDeposit_ReceiptNotAvailable(t *testing.T) {
	fetcher := &mockReceiptFetcher{
		err: fmt.Errorf("not found"),
	}

	err := VerifyDeposit(context.Background(), fetcher, DepositVerification{
		TxHash:       testTxHash,
		EscrowHash:   testEscrowHash,
		ExpectedAddr: testEscrowAddr,
		MinAmount:    testMinAmount,
	})

	require.Error(t, err)
	assert.NotErrorIs(t, err, payment.ErrDepositReverted)
	assert.Contains(t, err.Error(), "get receipt")
}

func TestVerifyDeposit_NoLogs(t *testing.T) {
	fetcher := &mockReceiptFetcher{
		receipt: makeSuccessReceipt(nil),
		tx:      makeNativeTx(testEscrowAddr),
	}

	err := VerifyDeposit(context.Background(), fetcher, DepositVerification{
		TxHash:       testTxHash,
		EscrowHash:   testEscrowHash,
		ExpectedAddr: testEscrowAddr,
		MinAmount:    testMinAmount,
	})

	assert.ErrorIs(t, err, payment.ErrDepositEventNotFound)
}

func TestVerifyDeposit_ExactAmountPasses(t *testing.T) {
	exact := big.NewInt(1000000)
	fundedLog := makeFundedLog(testEscrowAddr, testEscrowHash, exact)
	fetcher := &mockReceiptFetcher{
		receipt: makeSuccessReceipt([]*types.Log{fundedLog}),
		tx:      makeNativeTx(testEscrowAddr),
	}

	err := VerifyDeposit(context.Background(), fetcher, DepositVerification{
		TxHash:       testTxHash,
		EscrowHash:   testEscrowHash,
		ExpectedAddr: testEscrowAddr,
		MinAmount:    exact,
	})

	require.NoError(t, err)
}

func TestVerifyDeposit_MultipleLogs_CorrectOneMatches(t *testing.T) {
	wrongHash := common.HexToHash("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	wrongLog := makeFundedLog(testEscrowAddr, wrongHash, testMinAmount)
	correctLog := makeFundedLog(testEscrowAddr, testEscrowHash, testMinAmount)
	unrelatedLog := &types.Log{Address: testTokenAddr, Topics: []common.Hash{common.HexToHash("0x00")}}

	fetcher := &mockReceiptFetcher{
		receipt: makeSuccessReceipt([]*types.Log{unrelatedLog, wrongLog, correctLog}),
		tx:      makeNativeTx(testEscrowAddr),
	}

	err := VerifyDeposit(context.Background(), fetcher, DepositVerification{
		TxHash:       testTxHash,
		EscrowHash:   testEscrowHash,
		ExpectedAddr: testEscrowAddr,
		MinAmount:    testMinAmount,
	})

	require.NoError(t, err)
}
