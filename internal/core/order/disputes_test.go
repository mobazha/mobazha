//go:build !private_distribution

package order

import (
	"errors"
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"github.com/stretchr/testify/require"
)

func TestReleaseFundsRejectsModeratorRole(t *testing.T) {
	svc := newTestOrderAppService(t, OrderAppServiceConfig{})
	orderID := models.OrderID("moderator-release-order")

	require.NoError(t, svc.db.Update(func(tx database.Tx) error {
		return tx.Save(&models.Order{
			ID:     orderID,
			MyRole: string(models.RoleModerator),
		})
	}))

	err := svc.ReleaseFunds(orderID, iwallet.TransactionID(""), nil)
	require.Error(t, err)
	require.True(t, errors.Is(err, coreiface.ErrBadRequest))
	require.Contains(t, err.Error(), "moderator must resolve disputes via close dispute")
}
