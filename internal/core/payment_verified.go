//go:build !private_distribution

package core

import (
	"context"
	"strconv"
	"time"

	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	paymentpkg "github.com/mobazha/mobazha3.0/pkg/payment"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

func (n *MobazhaNode) handleCryptoPaymentVerified(orderID string, paymentSent *pb.PaymentSent) {
	if n.orderService == nil || paymentSent == nil {
		return
	}
	pd := paymentDataFromVerifiedPaymentSent(orderID, paymentSent)
	if pd == nil {
		logger.LogWarningWithIDf(log, n.nodeID, "payment verified: order %s missing settlement spec in PaymentSent", orderID)
		return
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

func paymentDataFromVerifiedPaymentSent(orderID string, paymentSent *pb.PaymentSent) *models.PaymentData {
	if paymentSent == nil {
		return nil
	}
	spec := paymentSent.GetSettlementSpec()
	if spec == nil {
		return nil
	}

	amount, _ := strconv.ParseUint(paymentSent.Amount, 10, 64)
	pd := &models.PaymentData{
		OrderID:             orderID,
		TransactionID:       paymentSent.TransactionID,
		Coin:                iwallet.CoinType(paymentSent.Coin),
		Method:              spec.GetMethod(),
		ContractAddress:     paymentSent.ContractAddress,
		PayerAddress:        paymentSent.PayerAddress,
		Moderator:           paymentSent.Moderator,
		ModeratorAddress:    paymentSent.ModeratorAddress,
		Amount:              amount,
		ToAddress:           paymentSent.ToAddress,
		Script:              paymentSent.Script,
		UnlockHours:         paymentSent.EscrowTimeoutHours,
		EscrowReleaseFee:    paymentSent.EscrowReleaseFee,
		PlatformAmount:      paymentSent.PlatformAmount,
		PlatformAddr:        paymentSent.PlatformAddr,
		RefundAddress:       paymentSent.RefundAddress,
		PaymentTokenAddress: paymentSent.PaymentTokenAddress,
		BuyerReceiveAddress: paymentSent.BuyerReceiveAddress,
	}
	pd.SettlementSpec = (&paymentpkg.SettlementSpec{
		Method:     spec.GetMethod(),
		PayMode:    paymentpkg.PayMode(spec.GetPayMode()),
		EscrowType: paymentpkg.EscrowType(spec.GetEscrowType()),
	}).ToPending()
	if ts := paymentSent.Timestamp; ts != nil {
		pd.Timestamp = ts.AsTime()
	} else {
		pd.Timestamp = time.Now().UTC()
	}
	if pm := paymentSent.GetPaymentMethod(); pm != nil {
		pd.PaymentMethod.Type = pm.Type
		pd.PaymentMethod.Brand = pm.Brand
		pd.PaymentMethod.Last4 = pm.Last4
	}
	return pd
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
