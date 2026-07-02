package order

import (
	"context"
	"fmt"

	ordersettlement "github.com/mobazha/mobazha3.0/internal/core/order/settlement"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// ExecuteSettlementDisputeReleaseAction submits backend escrow release for
// DECIDED moderated backend-managed contract (relay) or UTXO (sync) disputes via
// settlement-actions/dispute-release.
func (s *OrderAppService) ExecuteSettlementDisputeReleaseAction(
	ctx context.Context,
	orderID models.OrderID,
) (*payment.ActionResult, iwallet.CoinType, error) {
	if err := s.requireDisputeReleaseParticipant(orderID); err != nil {
		return nil, "", err
	}

	var order models.Order
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.String()).First(&order).Error
	})
	if err != nil {
		return nil, "", err
	}

	disputeClose, err := order.DisputeClosedMessage()
	if err != nil {
		return nil, "", fmt.Errorf("%w: dispute close message is missing", coreiface.ErrBadRequest)
	}
	if disputeClose.ReleaseInfo == nil {
		return nil, "", fmt.Errorf("%w: dispute release info is missing", coreiface.ErrBadRequest)
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
		return nil, coinType, errRetiredClientSignedModeratedSettlement("dispute_release")
	}
	if _, err := requireBackendSubmittedSettlementSpec(&order, paymentSent); err != nil {
		return nil, coinType, err
	}

	if err := s.attachSettlementActions(&order); err != nil {
		return nil, coinType, fmt.Errorf("load settlement actions for order %s: %w", orderID, err)
	}
	if existing, ok := ordersettlement.ExistingActionResult(&order, "dispute_release"); ok {
		return existing, coinType, nil
	}

	result, tx, handled, err := s.runMonitoredSettlementDisputeRelease(
		ctx,
		&order,
		coinType,
		paymentSent,
		disputeClose.ReleaseInfo,
	)
	if err != nil {
		return nil, coinType, err
	}
	if !handled {
		return nil, coinType, fmt.Errorf("%w: settlement dispute release is not supported for coin %s",
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
		}
	}
	return result, coinType, nil
}

func (s *OrderAppService) runMonitoredSettlementDisputeRelease(
	ctx context.Context,
	order *models.Order,
	coinType iwallet.CoinType,
	paymentSent *pb.PaymentSent,
	releaseInfo *pb.DisputeClose_ModeratedEscrowRelease,
) (*payment.ActionResult, *iwallet.Transaction, bool, error) {
	strategy, err := s.v2StrategyForCoin(coinType)
	if err != nil {
		return nil, nil, false, err
	}
	if strategy.Model() != payment.PaymentModelMonitored {
		return nil, nil, false, nil
	}
	spec, err := requireBackendSubmittedSettlementSpec(order, paymentSent)
	if err != nil {
		return nil, nil, true, err
	}
	if spec.UsesUTXOScript() {
		return s.runUTXOSyncSettlementDisputeRelease(order, coinType, paymentSent, releaseInfo)
	}
	if !ordersettlement.EscrowUsesRelayRelease(spec) {
		return nil, nil, false, nil
	}

	release := ordersettlement.CloneDisputeRelease(releaseInfo)
	if release == nil {
		return nil, nil, true, fmt.Errorf("settlement dispute release info is nil")
	}

	result, err := strategy.DisputeRelease(ctx, payment.ActionParams{
		OrderID:       order.ID.String(),
		PaymentCoin:   string(coinType),
		PaymentAmount: paymentSent.Amount,
		Chaincode:     paymentSent.Chaincode,
		Script:        paymentSent.Script,
		OrderData:     order,
		ReleaseInfo:   release,
	})
	if err != nil {
		return nil, nil, true, err
	}

	txHash := ordersettlement.ActionRelayTxHash(ctx, strategy, result)
	var tx *iwallet.Transaction
	if txHash != "" {
		tx = &iwallet.Transaction{ID: iwallet.TransactionID(txHash)}
	}
	return result, tx, true, nil
}

func (s *OrderAppService) runUTXOSyncSettlementDisputeRelease(
	order *models.Order,
	coinType iwallet.CoinType,
	paymentSent *pb.PaymentSent,
	releaseInfo *pb.DisputeClose_ModeratedEscrowRelease,
) (*payment.ActionResult, *iwallet.Transaction, bool, error) {
	orderID := order.ID.String()
	actionID, existingTxHash, err := s.beginSyncBackendSettlementAction(
		orderID, "dispute_release", string(coinType), paymentSent.Amount,
	)
	if err != nil {
		return nil, nil, true, err
	}
	if existingTxHash != "" {
		tx := &iwallet.Transaction{ID: iwallet.TransactionID(existingTxHash)}
		return &payment.ActionResult{
			Mode:            payment.ActionModeCompleted,
			ActionID:        actionID,
			SubmittedTxHash: existingTxHash,
		}, tx, true, nil
	}

	releaseTx, err := s.BuildDisputeReleaseTransaction(releaseInfo, paymentSent)
	if err != nil {
		s.failSyncBackendSettlementAction(actionID, err.Error())
		return nil, nil, true, err
	}
	disputeClose := &pb.DisputeClose{ReleaseInfo: releaseInfo}
	if err := s.signAndSendReleaseTransaction(&releaseTx, paymentSent, disputeClose); err != nil {
		s.failSyncBackendSettlementAction(actionID, err.Error())
		return nil, nil, true, err
	}
	if err := s.confirmSyncBackendSettlementAction(actionID, releaseTx.ID.String()); err != nil {
		s.failSyncBackendSettlementAction(actionID, err.Error())
		return nil, nil, true, err
	}
	return &payment.ActionResult{
		Mode:            payment.ActionModeCompleted,
		ActionID:        actionID,
		SubmittedTxHash: releaseTx.ID.String(),
	}, &releaseTx, true, nil
}
