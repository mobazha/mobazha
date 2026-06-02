package database

import (
	"time"

	"github.com/mobazha/mobazha3.0/pkg/models"
)

// MarkOrderOpenPaymentReady records that the seller has processed ORDER_OPEN.
// Idempotent: safe to call from both processed-delivery and ACK paths.
// The bool return value is true only on the first transition to payment-ready.
func MarkOrderOpenPaymentReady(tx Tx, orderID string) (bool, error) {
	var order models.Order
	if err := tx.Read().Where("id = ?", orderID).First(&order).Error; err != nil {
		return false, err
	}
	if models.IsPaymentReady(&order) {
		return false, nil
	}
	now := time.Now()
	_, err := tx.UpdateColumns(map[string]interface{}{
		"payment_ready_at": now,
		"order_open_acked": true,
	}, map[string]interface{}{"id = ?": orderID}, &models.Order{})
	if err != nil {
		return false, err
	}
	return true, nil
}
