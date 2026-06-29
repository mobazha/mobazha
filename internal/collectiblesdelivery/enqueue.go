package collectiblesdelivery

import (
	"errors"
	"fmt"
	"strings"

	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func EnqueueTx(tx database.Tx, order *models.Order, kind, reason string) error {
	if tx == nil || order == nil || strings.TrimSpace(order.ID.String()) == "" {
		return nil
	}
	managed, err := isHubManagedCollectibleOrder(order)
	if err != nil || !managed {
		return err
	}
	jobID := deliveryJobID(kind, order.ID.String())
	var existing models.CollectibleLifecycleDelivery
	err = tx.Read().Where("job_id = ?", jobID).First(&existing).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return tx.Save(&models.CollectibleLifecycleDelivery{
		JobID:   jobID,
		OrderID: order.ID.String(),
		Kind:    strings.TrimSpace(kind),
		Reason:  strings.TrimSpace(reason),
	})
}

func EnqueueGorm(tx *gorm.DB, tenantID string, order *models.Order, kind, reason string) error {
	if tx == nil || order == nil || strings.TrimSpace(order.ID.String()) == "" {
		return nil
	}
	managed, err := isHubManagedCollectibleOrder(order)
	if err != nil || !managed {
		return err
	}
	record := &models.CollectibleLifecycleDelivery{
		TenantMixin: models.TenantMixin{TenantID: strings.TrimSpace(tenantID)},
		JobID:       deliveryJobID(kind, order.ID.String()),
		OrderID:     order.ID.String(),
		Kind:        strings.TrimSpace(kind),
		Reason:      strings.TrimSpace(reason),
	}
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(record).Error
}

func EnqueueTerminalEventTx(tx database.Tx, order *models.Order, event interface{}) error {
	_, reason, ok := TerminalEvent(event)
	if !ok {
		return nil
	}
	return EnqueueTx(tx, order, models.CollectibleLifecycleRelease, reason)
}

func EnqueueTerminalEventByLookupTx(tx database.Tx, event interface{}) error {
	orderID, reason, ok := TerminalEvent(event)
	if !ok || strings.TrimSpace(orderID) == "" {
		return nil
	}
	var order models.Order
	if err := tx.Read().Where("id = ?", orderID).First(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return fmt.Errorf("collectible lifecycle: load terminal order %s: %w", orderID, err)
	}
	return EnqueueTx(tx, &order, models.CollectibleLifecycleRelease, reason)
}

func TerminalEvent(event interface{}) (orderID, reason string, ok bool) {
	switch e := event.(type) {
	case *events.OrderCancel:
		return e.OrderID, "order cancelled", true
	case events.OrderCancel:
		return e.OrderID, "order cancelled", true
	case *events.OrderDeclined:
		return e.OrderID, "order declined", true
	case events.OrderDeclined:
		return e.OrderID, "order declined", true
	case *events.OrderExpired:
		return e.OrderID, "order expired: " + strings.TrimSpace(e.Reason), true
	case events.OrderExpired:
		return e.OrderID, "order expired: " + strings.TrimSpace(e.Reason), true
	case *events.OrderAutoCancelled:
		return e.OrderID, "order auto-cancelled: " + strings.TrimSpace(e.Reason), true
	case events.OrderAutoCancelled:
		return e.OrderID, "order auto-cancelled: " + strings.TrimSpace(e.Reason), true
	default:
		return "", "", false
	}
}

func deliveryJobID(kind, orderID string) string {
	return strings.TrimSpace(kind) + ":" + strings.TrimSpace(orderID)
}

func isHubManagedCollectibleOrder(order *models.Order) (bool, error) {
	open, err := order.OrderOpenMessage()
	if err != nil {
		if models.IsMessageNotExistError(err) {
			return false, nil
		}
		return false, fmt.Errorf("collectible lifecycle: decode order %s: %w", order.ID, err)
	}
	return models.IsHubManagedCollectiblePrimarySale(open), nil
}
