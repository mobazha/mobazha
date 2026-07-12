// SPDX-License-Identifier: MPL-2.0

package orders

import (
	"testing"

	"github.com/mobazha/mobazha/internal/repo"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"github.com/stretchr/testify/require"
)

func TestValidateFrozenStandardOrderUTXOPaymentSent_IsolatesTenantAttempts(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	tenantDB, ok := db.(interface {
		TenantID() string
		ForTenant(string) (database.Database, error)
	})
	require.True(t, ok)
	otherTenantDB, err := tenantDB.ForTenant("tenant-b")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, otherTenantDB.Close()) })

	const orderID = "order-shared-id"
	require.NoError(t, otherTenantDB.Update(func(tx database.Tx) error {
		if err := tx.Migrate(&models.PaymentAttempt{}); err != nil {
			return err
		}
		return tx.Save(&models.PaymentAttempt{
			AttemptID: "attempt-other-tenant",
			Kind:      models.PaymentAttemptKindCryptoFundingTarget, State: models.PaymentAttemptFundingTargetReady,
			PaymentSessionID: "ps_" + orderID, OrderID: orderID,
			RouteBindingID: "route-other-tenant", IdempotencyKey: "attempt-other-tenant",
		})
	}))

	order := &models.Order{
		TenantMixin: models.TenantMixin{TenantID: tenantDB.TenantID()},
		ID:          models.OrderID(orderID),
	}
	paymentSent := &pb.PaymentSent{Coin: "crypto:bip122:000000000019d6689c085ae165831e93:native"}

	var frozen bool
	require.NoError(t, db.View(func(tx database.Tx) error {
		var validateErr error
		frozen, validateErr = validateFrozenStandardOrderUTXOPaymentSent(tx, order, paymentSent)
		return validateErr
	}))
	require.False(t, frozen)

	require.NoError(t, db.Update(func(tx database.Tx) error {
		return tx.Save(&models.PaymentAttempt{
			AttemptID: "attempt-current-tenant",
			Kind:      models.PaymentAttemptKindCryptoFundingTarget, State: models.PaymentAttemptFundingTargetReady,
			PaymentSessionID: "ps_" + orderID, OrderID: orderID,
			RouteBindingID: "route-current-tenant", IdempotencyKey: "attempt-current-tenant",
		})
	}))
	require.ErrorIs(t, db.View(func(tx database.Tx) error {
		_, validateErr := validateFrozenStandardOrderUTXOPaymentSent(tx, order, paymentSent)
		return validateErr
	}), models.ErrPaymentAttemptSettlementTermsConflict)
}
