//go:build !private_distribution

package core

import (
	"context"
	"strconv"

	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

func (n *MobazhaNode) handleCryptoPaymentVerified(orderID string, paymentSent *pb.PaymentSent) {
	if n.orderService == nil || paymentSent == nil {
		return
	}
	amount, _ := strconv.ParseUint(paymentSent.Amount, 10, 64)
	pd := &models.PaymentData{
		OrderID:       orderID,
		TransactionID: paymentSent.TransactionID,
		Coin:          iwallet.CoinType(paymentSent.Coin),
		Amount:        amount,
		Method:        paymentSent.Method,
	}
	order, err := n.orderService.FetchOrder(orderID)
	if err != nil {
		logger.LogWarningWithIDf(log, n.nodeID, "payment verified: fetch order %s: %v", orderID, err)
		return
	}
	switch order.Role() {
	case models.RoleVendor:
		if err := n.orderService.EnsureRatingSignatures(context.Background(), models.OrderID(orderID)); err != nil {
			logger.LogWarningWithIDf(log, n.nodeID, "payment verified: ensure rating signatures for order %s: %v", orderID, err)
		}
		if n.hasLocalOrderRole(orderID, models.RoleBuyer) {
			return
		}
		n.orderService.RelayPaymentToBuyer(context.Background(), orderID, pd)
	case models.RoleBuyer:
		if n.hasLocalOrderRole(orderID, models.RoleVendor) {
			return
		}
		vendorPeerID, err := order.Vendor()
		if err != nil {
			logger.LogWarningWithIDf(log, n.nodeID, "payment verified: resolve vendor for order %s: %v", orderID, err)
			return
		}
		n.orderService.RelayPaymentToCounterparty(context.Background(), orderID, vendorPeerID, pd)
	default:
		logger.LogInfoWithIDf(log, n.nodeID, "payment verified: order %s role %q has no counterparty relay", orderID, order.Role())
	}
}

func (n *MobazhaNode) hasLocalOrderRole(orderID string, role models.OrderRole) bool {
	if n.db == nil {
		return false
	}
	var count int64
	err := n.db.View(func(tx database.Tx) error {
		return tx.Read().
			Model(&models.Order{}).
			Where("id = ? AND my_role = ?", orderID, string(role)).
			Count(&count).Error
	})
	if err != nil {
		logger.LogWarningWithIDf(log, n.nodeID, "payment verified: check local %s order mirror for %s: %v", role, orderID, err)
		return false
	}
	return count > 0
}
