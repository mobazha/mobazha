package core

import (
	"context"
	"strings"

	"github.com/mobazha/mobazha/internal/core/guest"
	"github.com/mobazha/mobazha/internal/logger"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/events"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/mobazha/mobazha/pkg/payment"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

const managedSettlementReconciliationBatchSize = 100

var pendingManagedSettlementStates = []string{"submitting", "submitted", "failed"}

// runManagedSettlementReconciliationsOnce dispatches durable backend actions
// to the strategy that owns their chain semantics. Core never interprets an
// opaque transaction ID or private retry intent.
func (n *MobazhaNode) runManagedSettlementReconciliationsOnce(ctx context.Context) {
	if n == nil || n.db == nil || n.paymentRegistry == nil {
		return
	}
	rows, err := n.loadPendingManagedSettlementActions(ctx, managedSettlementReconciliationBatchSize)
	if err != nil {
		logger.LogWarningWithIDf(log, n.nodeID, "Managed settlement reconciliation: load pending actions: %v", err)
		return
	}
	for i := range rows {
		if err := ctx.Err(); err != nil {
			return
		}
		row := rows[i]
		coinType := iwallet.CoinType(strings.TrimSpace(row.SettlementCoin))
		if coinType == "" {
			continue
		}
		strategy, err := n.paymentRegistry.ForCoinV2(coinType)
		if err != nil {
			continue
		}
		reconciler, ok := strategy.(payment.ActionReconciler)
		if !ok {
			continue
		}
		status, err := reconciler.ReconcileAction(ctx, row.ActionID)
		if err != nil {
			logger.LogWarningWithIDf(log, n.nodeID, "Managed settlement reconciliation: action %s: %v", row.ActionID, err)
			continue
		}
		if status == nil || status.State != "confirmed" {
			continue
		}
		confirmed := row
		confirmed.TxHash = strings.TrimSpace(status.TxHash)
		n.emitOrderAutoConfirmAfterManagedSettlement(confirmed)
	}
}

func (n *MobazhaNode) loadPendingManagedSettlementActions(ctx context.Context, limit int) ([]models.SettlementAction, error) {
	var rows []models.SettlementAction
	err := n.db.View(func(tx database.Tx) error {
		query := tx.Read().WithContext(ctx).
			Where("state IN ? AND chain_id = ? AND settlement_coin <> ?", pendingManagedSettlementStates, 0, "").
			Order("updated_at ASC")
		if limit > 0 {
			query = query.Limit(limit)
		}
		return query.Find(&rows).Error
	})
	return rows, err
}

// emitOrderAutoConfirmAfterManagedSettlement advances a regular cancelable
// order after its provider-owned confirm action reaches chain finality.
func (n *MobazhaNode) emitOrderAutoConfirmAfterManagedSettlement(row models.SettlementAction) {
	if n == nil || n.eventBus == nil || n.db == nil || strings.TrimSpace(row.ActionKind) != "confirm" {
		return
	}
	if strings.HasPrefix(row.OrderID, guest.OrderTokenPrefix) {
		return
	}
	txID := strings.TrimSpace(row.TxHash)
	if txID == "" {
		return
	}
	var order models.Order
	if err := n.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", row.OrderID).First(&order).Error
	}); err != nil {
		logger.LogWarningWithIDf(log, n.nodeID, "Managed settlement confirm: load order %s: %v", row.OrderID, err)
		return
	}
	if order.SerializedOrderConfirmation != nil || !order.CanConfirm() || n.settlementService == nil {
		return
	}
	unlock := n.settlementService.TryLockAutoConfirm(row.OrderID)
	if unlock == nil {
		return
	}
	defer unlock()
	if err := n.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", row.OrderID).First(&order).Error
	}); err != nil || order.SerializedOrderConfirmation != nil || !order.CanConfirm() {
		return
	}
	payoutAddress := ""
	if paymentSent, err := order.PaymentSentMessage(); err == nil {
		if address, addressErr := n.settlementService.GetPayoutAddress(paymentSent.Coin); addressErr == nil {
			payoutAddress = address.String()
		}
	}
	authorization, err := n.confirmationAuthorizationForAction(row.OrderID, row.ActionID, txID)
	if err != nil {
		logger.LogWarningWithIDf(log, n.nodeID, "Managed settlement confirm: resolve extension authorization for order %s: %v", row.OrderID, err)
		return
	}
	attestationID := ""
	if authorization != nil {
		attestationID = authorization.AttestationID
		payoutAddress = authorization.PayoutAddress
	}
	n.eventBus.Emit(&events.OrderAutoConfirmRequest{
		OrderID: row.OrderID, TxID: txID, PayoutAddress: payoutAddress,
		SettlementAttestationID: attestationID,
	})
}
