//go:build !private_distribution

package order

import (
	"context"
	"fmt"
	"time"

	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// StartPaymentEventMonitor subscribes to internal domain events emitted by
// PaymentAppService. This replaces the direct cross-service method calls
// (ConfirmOrder, ProcessOrderPayment, ShipOrder) with event-driven
// decoupling, breaking the PaymentAppService → OrderAppService circular
// dependency for fire-and-forget operations.
func (s *OrderAppService) StartPaymentEventMonitor() {
	go s.RepairMissingRatingSignatures(context.Background())
	go s.subscribeAutoConfirmRequests()
	go s.subscribeUTXOPaymentDetected()
	go s.subscribeRwaInstantBuyCompleted()
}

func (s *OrderAppService) subscribeAutoConfirmRequests() {
	sub, err := s.eventBus.Subscribe(&events.OrderAutoConfirmRequest{})
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "Failed to subscribe to OrderAutoConfirmRequest: %v", err)
		return
	}

	logger.LogInfoWithIDf(log, s.nodeID, "Order auto-confirm event monitor started")

	for {
		select {
		case event := <-sub.Out():
			if e, ok := event.(*events.OrderAutoConfirmRequest); ok {
				go s.handleAutoConfirmRequest(e)
			}
		case <-s.shutdown:
			sub.Close()
			logger.LogInfoWithIDf(log, s.nodeID, "Order auto-confirm event monitor stopped")
			return
		}
	}
}

func (s *OrderAppService) handleAutoConfirmRequest(event *events.OrderAutoConfirmRequest) {
	if !autoConfirmRequestTargetsNode(event.TenantID, s.nodeID, s.dbTenantID()) {
		logger.LogDebugWithIDf(log, s.nodeID,
			"Skipping auto-confirm request for tenant %s order %s", event.TenantID, event.OrderID)
		return
	}

	err := s.ConfirmOrder(
		models.OrderID(event.OrderID),
		iwallet.TransactionID(event.TxID),
		event.PayoutAddress,
		nil,
	)
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "Failed to auto-confirm order %s via event: %v", event.OrderID, err)
		return
	}
	logger.LogInfoWithIDf(log, s.nodeID, "Successfully auto-confirmed order %s via event", event.OrderID)
}

func autoConfirmRequestTargetsNode(eventTenantID, nodeID, localTenantID string) bool {
	switch eventTenantID {
	case "":
		return true
	case nodeID, localTenantID:
		return true
	default:
		return false
	}
}

func (s *OrderAppService) dbTenantID() string {
	type tenantIDGetter interface {
		TenantID() string
	}
	if db, ok := s.db.(tenantIDGetter); ok {
		return db.TenantID()
	}
	return ""
}

func (s *OrderAppService) subscribeUTXOPaymentDetected() {
	sub, err := s.eventBus.Subscribe(&events.UTXOPaymentDetected{})
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "Failed to subscribe to UTXOPaymentDetected: %v", err)
		return
	}

	logger.LogInfoWithIDf(log, s.nodeID, "UTXO payment detected event monitor started")

	for {
		select {
		case event := <-sub.Out():
			if e, ok := event.(*events.UTXOPaymentDetected); ok {
				go s.handleUTXOPaymentDetected(e)
			}
		case <-s.shutdown:
			sub.Close()
			logger.LogInfoWithIDf(log, s.nodeID, "UTXO payment detected event monitor stopped")
			return
		}
	}
}

func (s *OrderAppService) handleUTXOPaymentDetected(event *events.UTXOPaymentDetected) {
	paymentData := &models.PaymentData{
		OrderID:          event.OrderID,
		TransactionID:    event.TransactionID,
		Coin:             iwallet.CoinType(event.Coin),
		Method:           pb.PaymentSent_Method(event.Method),
		Amount:           event.Amount,
		ToAddress:        event.ToAddress,
		Timestamp:        time.Unix(event.Timestamp, 0),
		Script:           event.Script,
		PayerAddress:     event.PayerAddress,
		RefundAddress:    event.RefundAddress,
		Moderator:        event.Moderator,
		ModeratorAddress: event.ModeratorAddress,
		UnlockHours:      event.UnlockHours,
	}
	if err := s.ProcessOrderPayment(context.Background(), paymentData); err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "Failed to process UTXO payment for order %s via event: %v", event.OrderID, err)
		return
	}
	logger.LogInfoWithIDf(log, s.nodeID, "Processed UTXO payment for order %s via event", event.OrderID)
}

func (s *OrderAppService) subscribeRwaInstantBuyCompleted() {
	sub, err := s.eventBus.Subscribe(&events.RwaInstantBuyCompleted{})
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "Failed to subscribe to RwaInstantBuyCompleted: %v", err)
		return
	}

	logger.LogInfoWithIDf(log, s.nodeID, "RWA instant buy event monitor started")

	for {
		select {
		case event := <-sub.Out():
			if e, ok := event.(*events.RwaInstantBuyCompleted); ok {
				go s.handleRwaAutoComplete(e)
			}
		case <-s.shutdown:
			sub.Close()
			logger.LogInfoWithIDf(log, s.nodeID, "RWA instant buy event monitor stopped")
			return
		}
	}
}

func (s *OrderAppService) handleRwaAutoComplete(event *events.RwaInstantBuyCompleted) {
	orderID := event.OrderID
	txID := event.TransactionID

	logger.LogInfoWithIDf(log, s.nodeID, "Processing RWA instant buy completion for order %s via event", orderID)

	confirmDone := make(chan struct{})
	err := s.ConfirmOrder(models.OrderID(orderID), iwallet.TransactionID(txID), "", confirmDone)
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "Failed to auto-confirm RWA order %s: %v", orderID, err)
		return
	}
	<-confirmDone
	logger.LogInfoWithIDf(log, s.nodeID, "RWA order %s auto-confirmed successfully", orderID)

	if err := s.autoShipRwaOrder(orderID, txID); err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "Failed to auto-ship RWA order %s: %v", orderID, err)
		return
	}

	logger.LogInfoWithIDf(log, s.nodeID, "RWA order %s auto-shipped successfully", orderID)
}

func (s *OrderAppService) autoShipRwaOrder(orderID string, txID string) error {
	var order models.Order
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID).Find(&order).Error
	})
	if err != nil {
		return fmt.Errorf("failed to get order: %w", err)
	}

	orderOpen, err := order.OrderOpenMessage()
	if err != nil {
		return fmt.Errorf("failed to get order open message: %w", err)
	}

	var rwaShipments []models.Shipment
	for i := range orderOpen.Items {
		rwaShipments = append(rwaShipments, models.Shipment{
			ItemIndex: i,
			Note:      "RWA Token delivered via atomic swap",
			CryptocurrencyDelivery: &models.CryptocurrencyDelivery{
				TransactionID: txID,
			},
		})
	}

	shipDone := make(chan struct{})
	err = s.ShipOrder(models.OrderID(orderID), rwaShipments, shipDone)
	if err != nil {
		return fmt.Errorf("failed to ship order: %w", err)
	}
	<-shipDone

	return nil
}
