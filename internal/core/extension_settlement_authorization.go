package core

import (
	"fmt"

	"github.com/mobazha/mobazha/internal/orderextensions"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
)

func (n *MobazhaNode) confirmationAuthorizationForAction(orderID, actionID, transactionID string) (*orderextensions.ConfirmationAuthorization, error) {
	if n == nil || n.db == nil {
		return nil, fmt.Errorf("order database is unavailable")
	}
	var authorization *orderextensions.ConfirmationAuthorization
	err := n.db.Update(func(tx database.Tx) error {
		var order models.Order
		if err := tx.Read().Where("id = ?", orderID).First(&order).Error; err != nil {
			return err
		}
		required, err := orderextensions.RequiresAttestedSettlementTx(tx, orderID)
		if err != nil || !required {
			return err
		}
		resolved, err := orderextensions.AuthorizationForSettlementActionTx(tx, &order, actionID, transactionID)
		if err != nil {
			return err
		}
		authorization = &resolved
		return nil
	})
	return authorization, err
}
