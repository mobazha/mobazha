package order

import (
	"context"
	"fmt"

	ordersettlement "github.com/mobazha/mobazha/internal/core/order/settlement"
	"github.com/mobazha/mobazha/pkg/core/coreiface"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha/pkg/payment"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

// ExecuteSettlementCompleteAction submits backend escrow release for moderated
// backend-managed contract (relay) or UTXO (sync sign+broadcast) orders via
// settlement-actions/complete.
func (s *OrderAppService) ExecuteSettlementCompleteAction(
	ctx context.Context,
	orderID models.OrderID,
) (*payment.ActionResult, iwallet.CoinType, error) {
	var order models.Order
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.String()).First(&order).Error
	})
	if err != nil {
		return nil, "", err
	}

	if !order.CanComplete() {
		return nil, "", fmt.Errorf("%w: order cannot be completed in its current state",
			coreiface.ErrBadRequest)
	}

	paymentSent, err := order.PaymentSentMessage()
	if err != nil {
		return nil, "", fmt.Errorf("%w: payment not recorded for this order", coreiface.ErrBadRequest)
	}

	coinType, err := payment.SettlementCoinFromPaymentSent(paymentSent)
	if err != nil {
		return nil, "", err
	}

	if !orderRequiresMonitoredSettlementActions(&order, paymentSent, coinType, s.paymentRegistry) {
		method, ok := payment.ResolvedPaymentMethod(&order, paymentSent)
		if ok && payment.MethodIsModerated(method) {
			return nil, coinType, errRetiredClientSignedModeratedSettlement("complete")
		}
		return &payment.ActionResult{Mode: payment.ActionModeCompleted}, coinType, nil
	}
	if _, err := requireBackendSubmittedSettlementSpec(&order, paymentSent); err != nil {
		return nil, coinType, err
	}

	if err := s.attachSettlementActions(&order); err != nil {
		return nil, coinType, fmt.Errorf("load settlement actions for order %s: %w", orderID, err)
	}
	if existing, ok := ordersettlement.ExistingActionResult(&order, "complete"); ok {
		return existing, coinType, nil
	}

	shipments, err := order.OrderShipmentMessages()
	if err != nil {
		return nil, coinType, fmt.Errorf("order shipment messages: %w", err)
	}
	if len(shipments) == 0 || shipments[0].ReleaseInfo == nil {
		return nil, coinType, fmt.Errorf("%w: shipment release info is missing", coreiface.ErrBadRequest)
	}

	result, release, tx, handled, err := s.runMonitoredSettlementComplete(
		ctx,
		&order,
		coinType,
		paymentSent,
		shipments[0].ReleaseInfo,
	)
	if err != nil {
		return nil, coinType, err
	}
	if !handled {
		return nil, coinType, fmt.Errorf("%w: settlement complete is not supported for coin %s",
			coreiface.ErrBadRequest, coinType)
	}
	if result == nil {
		result = &payment.ActionResult{Mode: payment.ActionModeCompleted}
	}
	if result.Mode == payment.ActionModeInstructionsRequired || result.Instructions != nil {
		return nil, coinType, fmt.Errorf("%w: settlement-actions only support backend-submitted flows for coin %s",
			coreiface.ErrBadRequest, coinType)
	}
	if result.SubmittedTxHash == "" {
		if tx != nil && tx.ID != "" {
			result.SubmittedTxHash = tx.ID.String()
		} else if release != nil && release.Txid != "" {
			result.SubmittedTxHash = release.Txid
		}
	}
	return result, coinType, nil
}

func (s *OrderAppService) runMonitoredSettlementComplete(
	ctx context.Context,
	order *models.Order,
	coinType iwallet.CoinType,
	paymentSent *pb.PaymentSent,
	releaseInfo *pb.EscrowRelease,
) (*payment.ActionResult, *pb.EscrowRelease, *iwallet.Transaction, bool, error) {
	strategy, err := s.v2StrategyForCoin(coinType)
	if err != nil {
		return nil, nil, nil, false, err
	}
	if strategy.Model() != payment.PaymentModelMonitored {
		return nil, nil, nil, false, nil
	}
	spec, err := requireBackendSubmittedSettlementSpec(order, paymentSent)
	if err != nil {
		return nil, nil, nil, true, err
	}
	if spec.UsesUTXOScript() {
		return s.runUTXOSyncSettlementComplete(ctx, order, coinType, paymentSent, releaseInfo)
	}
	if !ordersettlement.EscrowUsesRelayRelease(spec) {
		return nil, nil, nil, false, nil
	}

	release := ordersettlement.CloneEscrowRelease(releaseInfo)
	if release == nil {
		return nil, nil, nil, true, fmt.Errorf("settlement complete release info is nil")
	}
	affiliatePayout, err := affiliatePayoutFromEscrowRelease(release)
	if err != nil {
		return nil, nil, nil, true, fmt.Errorf("read seller-signed affiliate settlement payout: %w", err)
	}
	orderOpen, err := order.OrderOpenMessage()
	if err != nil {
		return nil, nil, nil, true, fmt.Errorf("read order affiliate terms: %w", err)
	}
	if err := requireInterimAffiliatePayout(orderOpen, affiliatePayout); err != nil {
		return nil, nil, nil, true, fmt.Errorf("seller-signed affiliate settlement payout is required: %w", err)
	}
	executionPayout, err := executableAffiliatePayout(affiliatePayout)
	if err != nil {
		return nil, nil, nil, true, fmt.Errorf("validate seller-signed affiliate settlement payout: %w", err)
	}

	result, err := strategy.Complete(ctx, payment.ActionParams{
		OrderID:         order.ID.String(),
		PaymentCoin:     string(coinType),
		PaymentAmount:   paymentSent.Amount,
		Chaincode:       paymentSent.Chaincode,
		Script:          paymentSent.Script,
		OrderData:       order,
		ReleaseInfo:     release,
		AffiliatePayout: executionPayout,
	})
	if err != nil {
		return nil, nil, nil, true, err
	}

	txHash := ordersettlement.ActionRelayTxHash(ctx, strategy, result)
	if txHash == "" {
		txHash = release.Txid
	}
	var tx *iwallet.Transaction
	if txHash != "" {
		release.Txid = txHash
		tx = &iwallet.Transaction{ID: iwallet.TransactionID(txHash)}
	}
	return result, release, tx, true, nil
}

func (s *OrderAppService) runUTXOSyncSettlementComplete(
	ctx context.Context,
	order *models.Order,
	coinType iwallet.CoinType,
	paymentSent *pb.PaymentSent,
	releaseInfo *pb.EscrowRelease,
) (*payment.ActionResult, *pb.EscrowRelease, *iwallet.Transaction, bool, error) {
	_ = ctx
	orderID := order.ID.String()
	actionID, existingTxHash, err := s.beginSyncBackendSettlementAction(
		orderID, "complete", string(coinType), paymentSent.Amount,
	)
	if err != nil {
		return nil, nil, nil, true, err
	}
	if existingTxHash != "" {
		release := ordersettlement.CloneEscrowRelease(releaseInfo)
		if release != nil {
			release.Txid = existingTxHash
		}
		tx := &iwallet.Transaction{ID: iwallet.TransactionID(existingTxHash)}
		return &payment.ActionResult{
			Mode:            payment.ActionModeCompleted,
			ActionID:        actionID,
			SubmittedTxHash: existingTxHash,
		}, release, tx, true, nil
	}

	wallet, err := s.multiwallet.WalletForCurrencyCode(string(coinType))
	if err != nil {
		s.failSyncBackendSettlementAction(actionID, err.Error())
		return nil, nil, nil, true, err
	}
	release, tx, err := s.executeUTXOSyncModeratedCompleteRelease(order, wallet, releaseInfo)
	if err != nil {
		s.failSyncBackendSettlementAction(actionID, err.Error())
		return nil, nil, nil, true, err
	}
	txHash := ""
	if tx != nil && tx.ID != "" {
		txHash = tx.ID.String()
	} else if release != nil {
		txHash = release.Txid
	}
	if err := s.confirmSyncBackendSettlementAction(actionID, txHash); err != nil {
		s.failSyncBackendSettlementAction(actionID, err.Error())
		return nil, nil, nil, true, err
	}
	return &payment.ActionResult{
		Mode:            payment.ActionModeCompleted,
		ActionID:        actionID,
		SubmittedTxHash: txHash,
	}, release, tx, true, nil
}
