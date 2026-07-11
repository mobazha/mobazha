package settlement

import (
	"errors"
	"testing"

	"github.com/mobazha/mobazha/pkg/models"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type releaseCommitTx struct {
	err       error
	committed bool
}

func (t *releaseCommitTx) Commit() error {
	t.committed = true
	return t.err
}

func (t *releaseCommitTx) Rollback() error {
	return nil
}

func TestCommitCancelableReleaseWalletTxReturnsBroadcastError(t *testing.T) {
	tx := &releaseCommitTx{err: errors.New("broadcast rejected")}

	err := commitCancelableReleaseWalletTx(tx, models.OrderID("order-commit-fails"))

	require.Error(t, err)
	assert.True(t, tx.committed)
	assert.Contains(t, err.Error(), "failed to broadcast/commit CANCELABLE release transaction")
	assert.Contains(t, err.Error(), "order-commit-fails")
	assert.ErrorIs(t, err, tx.err)
}

func TestCommitCancelableReleaseWalletTxSuccess(t *testing.T) {
	tx := &releaseCommitTx{}

	err := commitCancelableReleaseWalletTx(tx, models.OrderID("order-ok"))

	require.NoError(t, err)
	assert.True(t, tx.committed)
}

func TestUTXOCancelableConfirmPayoutLinesIncludesFrozenAffiliateOutput(t *testing.T) {
	tx := iwallet.Transaction{To: []iwallet.SpendInfo{
		{Address: iwallet.NewAddress("seller-address", iwallet.CoinType("BTC")), Amount: iwallet.NewAmount("900")},
		{Address: iwallet.NewAddress("affiliate-address", iwallet.CoinType("BTC")), Amount: iwallet.NewAmount("100")},
	}}

	lines, err := utxoCancelableConfirmPayoutLines(tx, "BTC", &models.AffiliateSettlementPayout{
		Address: "affiliate-address", Amount: "100",
	})

	require.NoError(t, err)
	require.Len(t, lines, 2)
	assert.Equal(t, "seller", lines[0].Type)
	assert.Equal(t, "affiliate", lines[1].Type)
	assert.Equal(t, "100", lines[1].Amount)
	assert.Equal(t, "affiliate-address", lines[1].Address)
}

func TestUTXOCancelableConfirmPayoutLinesRejectsAffiliateMismatch(t *testing.T) {
	tx := iwallet.Transaction{To: []iwallet.SpendInfo{
		{Address: iwallet.NewAddress("seller-address", iwallet.CoinType("BTC")), Amount: iwallet.NewAmount("900")},
		{Address: iwallet.NewAddress("wrong-address", iwallet.CoinType("BTC")), Amount: iwallet.NewAmount("100")},
	}}

	_, err := utxoCancelableConfirmPayoutLines(tx, "BTC", &models.AffiliateSettlementPayout{
		Address: "affiliate-address", Amount: "100",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not match frozen settlement terms")
}
