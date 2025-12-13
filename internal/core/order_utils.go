package core

import (
	"fmt"

	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// saveTransactionToFreshOrder re-fetches order from DB and saves transaction.
// This is necessary because ProcessMessage loads its own order object and saves it,
// so any changes made to the original order object would be lost or would overwrite
// ProcessMessage's changes.
//
// Usage: Call this AFTER orderProcessor.ProcessMessage() to save a transaction.
func saveTransactionToFreshOrder(dbtx database.Tx, orderID models.OrderID, tx iwallet.Transaction) error {
	if tx.ID == "" {
		return nil
	}
	var freshOrder models.Order
	if err := dbtx.Read().Where("id = ?", orderID).First(&freshOrder).Error; err != nil {
		return fmt.Errorf("failed to re-fetch order: %w", err)
	}
	if err := freshOrder.PutTransaction(tx); err != nil && !models.IsDuplicateTransactionError(err) {
		return fmt.Errorf("save transaction: %w", err)
	}
	return dbtx.Save(&freshOrder)
}

// updateFreshOrder re-fetches order from DB and applies updateFn.
// This is necessary because ProcessMessage loads its own order object and saves it,
// so any changes made to the original order object would be lost or would overwrite
// ProcessMessage's changes.
//
// Usage: Call this AFTER orderProcessor.ProcessMessage() to update order fields.
func updateFreshOrder(dbtx database.Tx, orderID models.OrderID, updateFn func(*models.Order) error) error {
	var freshOrder models.Order
	if err := dbtx.Read().Where("id = ?", orderID).First(&freshOrder).Error; err != nil {
		return fmt.Errorf("failed to re-fetch order: %w", err)
	}
	if err := updateFn(&freshOrder); err != nil {
		return err
	}
	return dbtx.Save(&freshOrder)
}
