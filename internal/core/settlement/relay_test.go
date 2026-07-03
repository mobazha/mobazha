package settlement

import (
	"errors"
	"testing"

	"github.com/mobazha/mobazha/pkg/models"
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
