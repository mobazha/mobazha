package settlement

import (
	"context"
	"errors"
	"testing"
	"time"

	ethWal "github.com/mobazha/mobazha/internal/chains/evm"
	"github.com/mobazha/mobazha/pkg/payment"
	"github.com/mobazha/mobazha/pkg/relay"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Mock ReceiptVerifier (port interface) ────────────────────────────────

type mockReceiptVerifier struct {
	verifyErr     error
	waitVerifyErr error
}

func (m *mockReceiptVerifier) VerifyTransactionReceipt(_ context.Context, _, _ string) error {
	return m.verifyErr
}

func (m *mockReceiptVerifier) WaitAndVerifyReceipt(_ context.Context, _, _ string) error {
	return m.waitVerifyErr
}

// ── Mock relay service ──────────────────────────────────────────────────

type mockRelayService struct {
	execFn func() (string, error)
}

func (m *mockRelayService) IsAvailable() bool { return true }

func (m *mockRelayService) Execute(_ context.Context, _ *relay.EVMRelayRequest) (*relay.EVMRelayResponse, error) {
	txHash, err := m.execFn()
	if err != nil {
		return nil, err
	}
	return &relay.EVMRelayResponse{TxHash: txHash}, nil
}

func (m *mockRelayService) GetSupportedChains() []string { return []string{"ethereum"} }
func (m *mockRelayService) GetGasWalletAddress(_ context.Context, _ uint64) (string, error) {
	return "0x0000000000000000000000000000000000000001", nil
}
func (m *mockRelayService) GetGasWalletStatus(_ context.Context, _ uint64) (*relay.EVMGasWalletStatus, error) {
	return &relay.EVMGasWalletStatus{Healthy: true}, nil
}
func (m *mockRelayService) ChainTypeForID(_ uint64) (string, error) {
	return "ethereum", nil
}

// ── Tests: VerifyEVMConfirmReceipt (SettlementService) ──────────────────

func TestVerifyEVMConfirmReceipt_Success(t *testing.T) {
	svc := &SettlementService{
		nodeID:          "test-node",
		receiptVerifier: &mockReceiptVerifier{verifyErr: nil},
	}
	err := svc.VerifyEVMConfirmReceipt("ETH", "0xabc123")
	require.NoError(t, err)
}

func TestVerifyEVMConfirmReceipt_Reverted(t *testing.T) {
	svc := &SettlementService{
		nodeID:          "test-node",
		receiptVerifier: &mockReceiptVerifier{verifyErr: payment.ErrTransactionReverted},
	}
	err := svc.VerifyEVMConfirmReceipt("ETH", "0xabc123")
	assert.ErrorIs(t, err, payment.ErrTransactionReverted)
}

func TestVerifyEVMConfirmReceipt_NoVerifier(t *testing.T) {
	svc := &SettlementService{nodeID: "test-node"}
	err := svc.VerifyEVMConfirmReceipt("ETH", "0xabc123")
	require.NoError(t, err, "nil receiptVerifier should noop")
}

// ── Tests: RelayEVMTransactionWithRetry (SettlementService) ─────────────

func TestRelayWithRetry_SecondAttemptSuccess(t *testing.T) {
	attempts := 0

	svc := &SettlementService{
		nodeID:          "test-node",
		receiptVerifier: &mockReceiptVerifier{waitVerifyErr: nil},
		evmRelayService: &mockRelayService{
			execFn: func() (string, error) {
				attempts++
				if attempts == 1 {
					return "", errors.New("temporary RPC error")
				}
				return "0xsuccess_hash", nil
			},
		},
	}

	txData := &ethWal.TransactionData{To: "0x1234", Data: "0xabcd"}
	txHash, err := svc.RelayEVMTransactionWithRetry(context.Background(), "order1", "ethereum", "ETH", txData)
	require.NoError(t, err)
	assert.Equal(t, "0xsuccess_hash", txHash)
	assert.Equal(t, 2, attempts)
}

func TestRelayWithRetry_AllFail(t *testing.T) {
	attempts := 0

	svc := &SettlementService{
		nodeID:          "test-node",
		receiptVerifier: &mockReceiptVerifier{waitVerifyErr: nil},
		evmRelayService: &mockRelayService{
			execFn: func() (string, error) {
				attempts++
				return "", errors.New("persistent RPC error")
			},
		},
	}

	txData := &ethWal.TransactionData{To: "0x1234", Data: "0xabcd"}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := svc.RelayEVMTransactionWithRetry(ctx, "order1", "ethereum", "ETH", txData)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "relay failed after")
	assert.Equal(t, 3, attempts)
}

func TestRelayWithRetry_ReceiptReverted(t *testing.T) {
	svc := &SettlementService{
		nodeID:          "test-node",
		receiptVerifier: &mockReceiptVerifier{waitVerifyErr: payment.ErrTransactionReverted},
		evmRelayService: &mockRelayService{
			execFn: func() (string, error) {
				return "0xreverted_hash", nil
			},
		},
	}

	txData := &ethWal.TransactionData{To: "0x1234", Data: "0xabcd"}
	txHash, err := svc.RelayEVMTransactionWithRetry(context.Background(), "order1", "ethereum", "ETH", txData)
	assert.ErrorIs(t, err, payment.ErrTransactionReverted)
	assert.Equal(t, "0xreverted_hash", txHash)
}
