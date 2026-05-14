//go:build !private_distribution

package payment

import (
	"fmt"

	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
)

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

func padOrTruncateBytes(b []byte, length int) []byte {
	if len(b) > length {
		return b[:length]
	}
	if len(b) < length {
		padded := make([]byte, length)
		copy(padded, b)
		return padded
	}
	return b
}
