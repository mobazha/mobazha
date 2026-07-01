package order

import (
	"context"

	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
)

// RepairMissingRatingSignatures backfills verified vendor orders that missed
// the normal RATING_SIGNATURES emission. This can happen when payment
// verification is recorded directly instead of through PAYMENT_SENT processing.
func (s *OrderAppService) RepairMissingRatingSignatures(ctx context.Context) {
	var orders []models.Order
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().WithContext(ctx).
			Where("my_role = ?", string(models.RoleVendor)).
			Where("serialized_order_open IS NOT NULL").
			Where("serialized_payment_sent IS NOT NULL").
			Where("payment_verification_status = ?", string(models.PaymentVerificationStatusVerified)).
			Where("(serialized_rating_signatures IS NULL OR length(serialized_rating_signatures) = 0)").
			Find(&orders).Error
	})
	if err != nil {
		logger.LogWarningWithIDf(log, s.nodeID, "repair rating signatures: query failed: %v", err)
		return
	}
	if len(orders) == 0 {
		return
	}

	repaired := 0
	for i := range orders {
		select {
		case <-ctx.Done():
			logger.LogInfoWithIDf(log, s.nodeID, "repair rating signatures: stopped after %d/%d orders", repaired, len(orders))
			return
		default:
		}
		if err := s.EnsureRatingSignatures(ctx, orders[i].ID); err != nil {
			logger.LogWarningWithIDf(log, s.nodeID, "repair rating signatures: order %s failed: %v", orders[i].ID, err)
			continue
		}
		repaired++
	}
	if repaired > 0 {
		logger.LogInfoWithIDf(log, s.nodeID, "repair rating signatures: repaired %d/%d orders", repaired, len(orders))
	}
}
