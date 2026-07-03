// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package order

import (
	"testing"

	"github.com/mobazha/mobazha/internal/orderextensions"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/extensions"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/stretchr/testify/require"
)

func TestConfirmOrder_ExtensionAttestedOrderRejectsPublicConfirmation(t *testing.T) {
	db := newTestDatabase(t)
	order := &models.Order{ID: "order-attested-confirm"}
	extension, err := extensions.NewOrderExtension(order.ID.String(), "provider", "test", "v1", "resource", map[string]string{"value": "test"})
	require.NoError(t, err)
	extension.SettlementPolicy = extensions.SettlementPolicyExtensionAttested
	extension.LifecycleEvents = []string{extensions.EventOrderPaymentVerified}
	require.NoError(t, db.Update(func(tx database.Tx) error {
		if err := tx.Save(order); err != nil {
			return err
		}
		return orderextensions.PersistTx(tx, order.ID.String(), extension)
	}))
	service := &OrderAppService{db: db, orderLockMgr: NewOrderLockManager()}
	err = service.ConfirmOrder(order.ID, "", "", nil)
	require.ErrorContains(t, err, "requires an executed conditional settlement")
}
