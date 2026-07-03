package core

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mobazha/mobazha/internal/logger"
	"github.com/mobazha/mobazha/internal/orderextensions"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/extensions"
	"github.com/mobazha/mobazha/pkg/models"
)

const (
	extensionDeliveryBatchSize   = 50
	extensionDeliveryMaxAttempts = 10
	extensionDeliveryLease       = 5 * time.Minute
)

func (n *MobazhaNode) runExtensionDeliveries(ctx context.Context) {
	if n == nil || n.db == nil {
		return
	}
	now := time.Now().UTC()
	pending, err := n.claimExtensionDeliveries(now)
	if err != nil {
		logger.LogErrorWithIDf(log, n.nodeID, "Extensions: claim deliveries: %v", err)
		return
	}
	for i := range pending {
		n.deliverExtensionJob(ctx, &pending[i])
	}
}

func (n *MobazhaNode) claimExtensionDeliveries(now time.Time) ([]models.ExtensionDelivery, error) {
	leaseOwner := strings.TrimSpace(n.nodeID) + ":" + uuid.NewString()
	leaseExpiresAt := now.Add(extensionDeliveryLease)
	var claimed []models.ExtensionDelivery
	err := n.db.Update(func(tx database.Tx) error {
		var candidates []models.ExtensionDelivery
		if err := tx.Read().
			Where("delivered_at IS NULL AND dead_lettered_at IS NULL AND (next_attempt_at IS NULL OR next_attempt_at <= ?) AND (lease_expires_at IS NULL OR lease_expires_at <= ?)", now, now).
			Order("created_at ASC").
			Limit(extensionDeliveryBatchSize).
			Find(&candidates).Error; err != nil {
			return err
		}
		for i := range candidates {
			updated, err := tx.UpdateColumns(
				map[string]interface{}{
					"lease_owner":      leaseOwner,
					"lease_expires_at": leaseExpiresAt,
				},
				map[string]interface{}{
					"event_id = ?":          candidates[i].EventID,
					"delivered_at IS ?":     nil,
					"dead_lettered_at IS ?": nil,
					"(next_attempt_at IS NULL OR next_attempt_at <= ?)":   now,
					"(lease_expires_at IS NULL OR lease_expires_at <= ?)": now,
				},
				&models.ExtensionDelivery{},
			)
			if err != nil {
				return err
			}
			if updated != 1 {
				continue
			}
			candidates[i].LeaseOwner = leaseOwner
			candidates[i].LeaseExpiresAt = &leaseExpiresAt
			claimed = append(claimed, candidates[i])
		}
		return nil
	})
	return claimed, err
}

func (n *MobazhaNode) deliverExtensionJob(ctx context.Context, job *models.ExtensionDelivery) {
	if job == nil {
		return
	}
	event := orderextensions.EventFromDelivery(*job)
	if err := event.Validate(); err != nil {
		n.recordExtensionDeliveryResult(job, fmt.Errorf("invalid extension event: %w", err))
		return
	}
	controller := n.extensionController(event.ProviderID)
	if controller == nil {
		n.recordExtensionDeliveryResult(job, fmt.Errorf("extension controller %q is unavailable", event.ProviderID))
		return
	}
	n.recordExtensionDeliveryResult(job, controller.HandleExtensionEvent(ctx, event))
}

func (n *MobazhaNode) extensionController(providerID string) extensions.Controller {
	registered := n.extensionModule(providerID)
	if registered == nil || !registered.hasContract(extensions.ContractOrderExtensionDeliveryV1) {
		return nil
	}
	return registered.controller
}

func (n *MobazhaNode) recordExtensionDeliveryResult(job *models.ExtensionDelivery, deliveryErr error) {
	now := time.Now().UTC()
	columns := map[string]interface{}{"attempts": job.Attempts + 1}
	columns["lease_owner"] = ""
	columns["lease_expires_at"] = nil
	if deliveryErr == nil {
		columns["last_error"] = ""
		columns["next_attempt_at"] = nil
		columns["delivered_at"] = now
	} else {
		columns["last_error"] = strings.TrimSpace(deliveryErr.Error())
		if job.Attempts+1 >= extensionDeliveryMaxAttempts {
			columns["next_attempt_at"] = nil
			columns["dead_lettered_at"] = now
		} else {
			columns["next_attempt_at"] = now.Add(extensionDeliveryRetryBackoff(job.Attempts))
		}
	}
	if err := n.db.Update(func(tx database.Tx) error {
		updated, updateErr := tx.UpdateColumns(columns, map[string]interface{}{
			"event_id = ?":    job.EventID,
			"lease_owner = ?": job.LeaseOwner,
		}, &models.ExtensionDelivery{})
		if updateErr != nil {
			return updateErr
		}
		if updated != 1 {
			return fmt.Errorf("delivery lease is no longer owned by this worker")
		}
		return nil
	}); err != nil {
		logger.LogErrorWithIDf(log, n.nodeID, "Extensions: record delivery %s result: %v", job.EventID, err)
	} else if deliveryErr != nil {
		logger.LogWarningWithIDf(log, n.nodeID, "Extensions: delivery %s failed: %v", job.EventID, deliveryErr)
	}
}

func extensionDeliveryRetryBackoff(attempts int) time.Duration {
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
