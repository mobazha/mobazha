package payment

import (
	"testing"

	"github.com/mobazha/mobazha3.0/internal/repo"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	paymentpkg "github.com/mobazha/mobazha3.0/pkg/payment"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"github.com/stretchr/testify/require"
)

func newRefundResolutionTestDB(t *testing.T) database.Database {
	t.Helper()
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestResolveBuyerRefundForLocalNode_BuyerUsesSavedPrefs(t *testing.T) {
	db := newRefundResolutionTestDB(t)
	require.NoError(t, db.Update(func(tx database.Tx) error {
		prefs := &models.UserPreferences{ID: 1}
		require.NoError(t, prefs.SetRefundReceivingAddresses(map[string]string{
			"crypto:eip155:1:native": "0x742d35Cc6634C0532925a3b844Bc454e4438f44e",
		}))
		return tx.Save(prefs)
	}))

	order := &models.Order{MyRole: string(models.RoleBuyer)}
	result := ResolveBuyerRefundForLocalNode(
		db,
		order,
		nil,
		iwallet.CoinType("crypto:eip155:1:native"),
		nil,
		false,
	)
	require.Equal(t, "0x742d35Cc6634C0532925a3b844Bc454e4438f44e", result.Address)
	require.Equal(t, paymentpkg.RefundAddressSourceAccountDefault, result.Source)
}

func TestResolveBuyerRefundForLocalNode_VendorIgnoresPrefs(t *testing.T) {
	db := newRefundResolutionTestDB(t)
	require.NoError(t, db.Update(func(tx database.Tx) error {
		prefs := &models.UserPreferences{ID: 1}
		require.NoError(t, prefs.SetRefundReceivingAddresses(map[string]string{
			"crypto:eip155:1:native": "0x742d35Cc6634C0532925a3b844Bc454e4438f44e",
		}))
		return tx.Save(prefs)
	}))

	order := &models.Order{MyRole: string(models.RoleVendor)}
	result := ResolveBuyerRefundForLocalNode(
		db,
		order,
		nil,
		iwallet.CoinType("crypto:eip155:1:native"),
		nil,
		false,
	)
	require.False(t, result.Found())
}
