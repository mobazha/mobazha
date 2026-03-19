package adapters_test

import (
	"context"
	"testing"

	"github.com/mobazha/mobazha3.0/internal/payment/adapters"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTRONChainOps_AutoConfirm_WithCallback(t *testing.T) {
	var called bool
	ops := &adapters.TRONChainOps{
		OnAutoConfirm: func(event *events.CancelablePaymentReady) {
			called = true
		},
		NodeID: "test-node",
	}

	err := ops.AutoConfirm(&events.CancelablePaymentReady{OrderID: "tron-order-1"})
	require.NoError(t, err)
	assert.True(t, called)
}

func TestTRONChainOps_AutoConfirm_NoCallback(t *testing.T) {
	ops := &adapters.TRONChainOps{NodeID: "test-node"}
	err := ops.AutoConfirm(&events.CancelablePaymentReady{OrderID: "tron-order-1"})
	require.NoError(t, err)
}

func TestTRONChainOps_SignEscrowRelease_KeyProviderError(t *testing.T) {
	kp := newTestKeyProvider()
	kp.err = assert.AnError

	ops := &adapters.TRONChainOps{Keys: kp}
	_, err := ops.SignEscrowRelease(payment.SignEscrowParams{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get TRON master key")
}

func TestTRONChainOps_BuildCancelableRelease_KeyProviderError(t *testing.T) {
	kp := newTestKeyProvider()
	kp.err = assert.AnError

	ops := &adapters.TRONChainOps{Keys: kp}
	_, err := ops.BuildCancelableRelease(nil, "", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get TRON master key")
}

func TestTRONChainOps_BuildCompleteEscrow_KeyProviderError(t *testing.T) {
	kp := newTestKeyProvider()
	kp.err = assert.AnError

	ops := &adapters.TRONChainOps{Keys: kp}
	_, err := ops.BuildCompleteEscrow(nil, "", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get TRON master key")
}

func TestTRONChainOps_BuildDisputeRelease_KeyProviderError(t *testing.T) {
	kp := newTestKeyProvider()
	kp.err = assert.AnError

	ops := &adapters.TRONChainOps{Keys: kp}
	_, err := ops.BuildDisputeRelease(nil, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get TRON master key")
}

func TestTRONChainOps_VerifyDeposit_NilClient(t *testing.T) {
	ops := &adapters.TRONChainOps{TronClient: nil}
	err := ops.VerifyDeposit(context.Background(), payment.DepositVerifyParams{
		CoinType: iwallet.CtTRC20USDT,
		Script:   "aabb",
	})
	assert.NoError(t, err)
}

func TestTRONChainOps_VerifyDeposit_NonTRONCoin(t *testing.T) {
	ops := &adapters.TRONChainOps{}
	err := ops.VerifyDeposit(context.Background(), payment.DepositVerifyParams{
		CoinType: iwallet.CtBNB,
	})
	assert.NoError(t, err)
}

func TestTRONChainOps_VerifyPreRelease_EmptyTxHash(t *testing.T) {
	ops := &adapters.TRONChainOps{}
	err := ops.VerifyPreRelease(context.Background(), payment.PreReleaseParams{
		TxHash: "",
	})
	assert.NoError(t, err)
}
