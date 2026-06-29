//go:build !private_distribution

package core

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
)

const collectibleLifecycleDeliveryBatchSize = 50

func (n *MobazhaNode) runCollectibleLifecycleDeliveries(ctx context.Context) {
	if n == nil || n.db == nil {
		return
	}
	n.collectibleLifecycleDeliveryMu.Lock()
	defer n.collectibleLifecycleDeliveryMu.Unlock()

	now := time.Now().UTC()
	var pending []models.CollectibleLifecycleDelivery
	if err := n.db.View(func(tx database.Tx) error {
		return tx.Read().
			Where("delivered_at IS NULL AND (next_attempt_at IS NULL OR next_attempt_at <= ?)", now).
			Order("created_at ASC").
			Limit(collectibleLifecycleDeliveryBatchSize).
			Find(&pending).Error
	}); err != nil {
		logger.LogErrorWithIDf(log, n.nodeID, "Collectibles: query lifecycle deliveries: %v", err)
		return
	}

	for i := range pending {
		n.deliverCollectibleLifecycleJob(ctx, &pending[i])
	}
}

func (n *MobazhaNode) deliverCollectibleLifecycleJob(ctx context.Context, job *models.CollectibleLifecycleDelivery) {
	if job == nil {
		return
	}
	var err error
	switch job.Kind {
	case models.CollectibleLifecyclePaid:
		err = n.deliverCollectiblePrimarySalePaid(ctx, job.OrderID)
	case models.CollectibleLifecycleRelease:
		if n.collectibleFirstSaleReservationReleaseHook == nil {
			err = fmt.Errorf("hosting reservation release hook is unavailable")
		} else {
			err = n.collectibleFirstSaleReservationReleaseHook(ctx, CollectibleFirstSaleReservationReleaseSignal{
				OrderID: job.OrderID,
				Reason:  job.Reason,
			})
		}
	default:
		err = fmt.Errorf("unsupported collectible lifecycle delivery kind %q", job.Kind)
	}

	if err == nil {
		deliveredAt := time.Now().UTC()
		if updateErr := n.db.Update(func(tx database.Tx) error {
			_, updateErr := tx.UpdateColumns(map[string]interface{}{
				"attempts":        job.Attempts + 1,
				"last_error":      "",
				"next_attempt_at": nil,
				"delivered_at":    deliveredAt,
			}, map[string]interface{}{"job_id = ?": job.JobID}, &models.CollectibleLifecycleDelivery{})
			return updateErr
		}); updateErr != nil {
			logger.LogErrorWithIDf(log, n.nodeID, "Collectibles: mark lifecycle job %s delivered: %v", job.JobID, updateErr)
		}
		return
	}

	nextAttempt := time.Now().UTC().Add(collectibleLifecycleRetryBackoff(job.Attempts))
	if updateErr := n.db.Update(func(tx database.Tx) error {
		_, updateErr := tx.UpdateColumns(map[string]interface{}{
			"attempts":        job.Attempts + 1,
			"last_error":      strings.TrimSpace(err.Error()),
			"next_attempt_at": nextAttempt,
		}, map[string]interface{}{"job_id = ?": job.JobID}, &models.CollectibleLifecycleDelivery{})
		return updateErr
	}); updateErr != nil {
		logger.LogErrorWithIDf(log, n.nodeID, "Collectibles: record lifecycle job %s failure: %v", job.JobID, updateErr)
	}
	logger.LogWarningWithIDf(log, n.nodeID, "Collectibles: lifecycle job %s failed, retry at %s: %v", job.JobID, nextAttempt.Format(time.RFC3339), err)
}

func collectibleLifecycleRetryBackoff(attempts int) time.Duration {
	if attempts < 0 {
		attempts = 0
	}
	if attempts > 6 {
		attempts = 6
	}
	delay := 5 * time.Second * time.Duration(1<<attempts)
	if delay > 5*time.Minute {
		return 5 * time.Minute
	}
	return delay
}
