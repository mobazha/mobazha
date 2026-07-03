package order

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"google.golang.org/protobuf/encoding/protojson"

	ordersettlement "github.com/mobazha/mobazha/internal/core/order/settlement"
	nodepayment "github.com/mobazha/mobazha/internal/core/payment"
	"github.com/mobazha/mobazha/pkg/core/coreiface"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha/pkg/payment"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"gorm.io/gorm"
)

func (s *OrderAppService) v2StrategyForCoin(coinType iwallet.CoinType) (payment.ChainEscrowV2, error) {
	if s.paymentRegistry == nil {
		return nil, fmt.Errorf("payment registry not initialized")
	}
	strategy, err := s.paymentRegistry.ForCoinV2(coinType)
	if err != nil {
		return nil, err
	}
	return strategy, nil
}

func (s *OrderAppService) signSettlementActionRelease(ctx context.Context, coinType iwallet.CoinType, action string, params payment.ActionParams) ([]*pb.Signature, bool, error) {
	strategy, err := s.v2StrategyForCoin(coinType)
	if err != nil {
		return nil, false, err
	}
	actionSigner, ok := strategy.(payment.ActionSigner)
	if !ok {
		return nil, false, nil
	}
	ownerSigs, err := actionSigner.SignAction(ctx, action, params)
	if err != nil {
		return nil, true, err
	}
	out := make([]*pb.Signature, 0, len(ownerSigs))
	for _, sig := range ownerSigs {
		out = append(out, &pb.Signature{
			From:      []byte(sig.From),
			Signature: append([]byte(nil), sig.Signature...),
			Index:     sig.Index,
		})
	}
	return out, true, nil
}

func orderDataWithPaymentSent(orderID models.OrderID, paymentSent *pb.PaymentSent) (*models.Order, error) {
	if paymentSent == nil {
		return nil, fmt.Errorf("payment sent message is nil")
	}
	order := &models.Order{ID: orderID}
	if err := order.SetPaymentSent(paymentSent); err != nil {
		return nil, err
	}
	return order, nil
}

func orderDataWithContract(orderID models.OrderID, orderOpen *pb.OrderOpen, paymentSent *pb.PaymentSent) (*models.Order, error) {
	if orderOpen == nil {
		return nil, fmt.Errorf("order open message is nil")
	}
	order, err := orderDataWithPaymentSent(orderID, paymentSent)
	if err != nil {
		return nil, err
	}
	raw, err := (protojson.MarshalOptions{}).Marshal(orderOpen)
	if err != nil {
		return nil, err
	}
	order.SerializedOrderOpen = raw
	return order, nil
}

// orderRequiresMonitoredSettlementActions reports moderated orders whose escrow
// release/complete must go through settlement-actions before domain handlers run.
func orderRequiresMonitoredSettlementActions(
	order *models.Order,
	paymentSent *pb.PaymentSent,
	coinType iwallet.CoinType,
	registry *payment.Registry,
) bool {
	if order == nil || paymentSent == nil || registry == nil {
		return false
	}
	method, ok := payment.ResolvedPaymentMethod(order, paymentSent)
	if !ok || !payment.MethodIsModerated(method) {
		return false
	}
	strategy, err := registry.ForCoinV2(coinType)
	if err != nil || strategy.Model() != payment.PaymentModelMonitored {
		return false
	}
	return true
}

func requireBackendSubmittedSettlementSpec(order *models.Order, paymentSent *pb.PaymentSent) (payment.SettlementSpec, error) {
	spec, ok := payment.ResolveSettlementSpec(order, paymentSent)
	if !ok {
		return payment.SettlementSpec{}, fmt.Errorf("%w: payment settlement spec is required", coreiface.ErrBadRequest)
	}
	if !ordersettlement.EscrowUsesBackendSubmittedRelease(spec) {
		return payment.SettlementSpec{}, fmt.Errorf("%w: escrow type %q must use settlement-actions; client-signed legacy routes are retired",
			coreiface.ErrBadRequest, spec.EscrowType)
	}
	return spec, nil
}

func errRetiredClientSignedModeratedSettlement(action string) error {
	return fmt.Errorf("%w: moderated client-signed %s is retired; use POST /v1/orders/{orderID}/settlement-actions/%s",
		coreiface.ErrBadRequest, action, payment.SettlementActionPathSegment(action))
}

func errBalanceMonitoredEscrowRequiresSettlementAction(order *models.Order, paymentSent *pb.PaymentSent, action string) error {
	if paymentSent == nil {
		return nil
	}
	spec, ok := payment.ResolveSettlementSpec(order, paymentSent)
	if !ok {
		return nil
	}
	switch {
	case spec.UsesManagedEscrow():
		return fmt.Errorf("%w: backend-managed orders must use POST /v1/orders/{orderID}/settlement-actions/%s",
			coreiface.ErrBadRequest, payment.SettlementActionPathSegment(action))
	case spec.UsesSolanaEscrow():
		return fmt.Errorf("%w: Solana escrow orders must use POST /v1/orders/{orderID}/settlement-actions/%s",
			coreiface.ErrBadRequest, payment.SettlementActionPathSegment(action))
	}
	return nil
}

func (s *OrderAppService) loadSyncBackendSettlementAction(orderID, action string) (*models.SettlementAction, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	actionID := ordersettlement.SyncActionID(orderID, action)
	var row models.SettlementAction
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("action_id = ?", actionID).First(&row).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// beginSyncBackendSettlementAction reserves a deterministic sync action row
// before UTXO sign+broadcast so retries do not double-spend on chain.
func (s *OrderAppService) beginSyncBackendSettlementAction(
	orderID, action, settlementCoin, grossAmount string,
) (actionID string, existingTxHash string, err error) {
	if s == nil || s.db == nil {
		return "", "", fmt.Errorf("database not initialized")
	}
	actionID = ordersettlement.SyncActionID(orderID, action)
	existing, err := s.loadSyncBackendSettlementAction(orderID, action)
	if err != nil {
		return "", "", err
	}
	if existing != nil {
		if existing.TxHash != "" {
			return actionID, existing.TxHash, nil
		}
		state := strings.ToLower(strings.TrimSpace(existing.State))
		if state == "submitting" || state == "submitted" || state == "confirmed" {
			if ordersettlement.StaleSyncAction(existing.ActionID, existing.State, existing.TxHash, existing.UpdatedAt, time.Now().UTC()) {
				goto reserve
			}
			return "", "", fmt.Errorf("%w: settlement %s release is still pending; retry after tx hash is available",
				coreiface.ErrBadRequest, action)
		}
	}

reserve:
	now := time.Now().UTC()
	row := &models.SettlementAction{
		ActionID:       actionID,
		OrderID:        orderID,
		ActionKind:     action,
		State:          "submitting",
		SettlementCoin: settlementCoin,
		GrossAmount:    grossAmount,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if existing != nil {
		row.CreatedAt = existing.CreatedAt
	}
	if err := s.db.Update(func(tx database.Tx) error {
		return tx.Save(row)
	}); err != nil {
		return "", "", err
	}
	return actionID, "", nil
}

func (s *OrderAppService) confirmSyncBackendSettlementAction(actionID, txHash string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("database not initialized")
	}
	if txHash == "" {
		return fmt.Errorf("settlement action %s confirmed without tx hash", actionID)
	}
	now := time.Now().UTC()
	return s.db.Update(func(tx database.Tx) error {
		_, err := tx.UpdateColumns(map[string]interface{}{
			"state":        "confirmed",
			"tx_hash":      txHash,
			"confirmed_at": now,
			"updated_at":   now,
		}, map[string]interface{}{
			"action_id = ?": actionID,
		}, &models.SettlementAction{})
		return err
	})
}

func (s *OrderAppService) failSyncBackendSettlementAction(actionID, reason string) {
	if s == nil || s.db == nil || strings.TrimSpace(actionID) == "" {
		return
	}
	reason = strings.TrimSpace(reason)
	if len(reason) > 2048 {
		reason = reason[:2048]
	}
	now := time.Now().UTC()
	_ = s.db.Update(func(tx database.Tx) error {
		_, err := tx.UpdateColumns(map[string]interface{}{
			"state":      "failed",
			"last_error": reason,
			"updated_at": now,
		}, map[string]interface{}{
			"action_id = ?": actionID,
		}, &models.SettlementAction{})
		return err
	})
}

func errSettlementReleaseActionRequired(orderID models.OrderID, action string) error {
	return fmt.Errorf("%w: submit POST /v1/orders/%s/settlement-actions/%s before continuing",
		coreiface.ErrBadRequest, orderID, payment.SettlementActionPathSegment(action))
}

type settlementActionIntent string

const (
	settlementIntentBuyerCancel               settlementActionIntent = "buyer_cancel"
	settlementIntentSellerDeclineFundedRefund settlementActionIntent = "seller_decline_funded_refund"
)

func (s *OrderAppService) settlementActionForIntent(
	order *models.Order,
	paymentSent *pb.PaymentSent,
	method pb.PaymentSent_Method,
	coinType iwallet.CoinType,
	intent settlementActionIntent,
) (string, bool) {
	if order == nil || paymentSent == nil {
		return "", false
	}
	if !payment.MethodIsCancelable(method) && !payment.MethodIsModerated(method) {
		return "", false
	}
	if payment.MethodIsModerated(method) && order.SerializedOrderConfirmation != nil {
		return "", false
	}
	strategy, err := s.v2StrategyForCoin(coinType)
	if err != nil || strategy.Model() != payment.PaymentModelMonitored {
		return "", false
	}
	spec, ok := payment.ResolveSettlementSpec(order, paymentSent)
	if !ok || !ordersettlement.EscrowUsesRelayRelease(spec) {
		return "", false
	}
	switch intent {
	case settlementIntentBuyerCancel:
		return payment.SettlementActionCancel, true
	case settlementIntentSellerDeclineFundedRefund:
		if _, ok := strategy.(payment.SellerDeclineRefunder); ok {
			return payment.SettlementActionSellerDeclineRefund, true
		}
		return payment.SettlementActionCancel, true
	default:
		return "", false
	}
}

func (s *OrderAppService) canSellerDeclineFundedRefund(order *models.Order) (bool, error) {
	if order == nil || order.Role() != models.RoleVendor {
		return false, nil
	}
	if order.SerializedOrderDecline != nil ||
		order.SerializedOrderCancel != nil ||
		order.SerializedOrderConfirmation != nil ||
		order.SerializedOrderShipments != nil ||
		order.SerializedOrderComplete != nil ||
		order.SerializedDisputeOpen != nil ||
		order.SerializedDisputeUpdate != nil ||
		order.SerializedDisputeClosed != nil ||
		order.SerializedRefunds != nil ||
		order.SerializedPaymentFinalized != nil {
		return false, nil
	}
	funded, err := order.IsFunded()
	if err != nil || !funded {
		return false, err
	}
	paymentSent, err := order.PaymentSentMessage()
	if err != nil {
		if models.IsMessageNotExistError(err) {
			return false, nil
		}
		return false, err
	}
	coinType, err := payment.SettlementCoinFromPaymentSent(paymentSent)
	if err != nil {
		return false, err
	}
	method, ok := payment.ResolvedPaymentMethod(order, paymentSent)
	if !ok || (!payment.MethodIsCancelable(method) && !payment.MethodIsModerated(method)) {
		return false, nil
	}
	_, ok = s.settlementActionForIntent(order, paymentSent, method, coinType, settlementIntentSellerDeclineFundedRefund)
	return ok, nil
}

// evaluateMonitoredSettlementRelease checks pending/ready state for a backend
// settlement release action (complete or dispute_release).
func evaluateMonitoredSettlementRelease(
	order *models.Order,
	txid iwallet.TransactionID,
	actionName string,
) (resolvedTxid iwallet.TransactionID, releaseAlreadySubmitted bool, err error) {
	resolved, submitted, err := ordersettlement.EvaluateRelease(order, txid, actionName)
	if err != nil {
		return "", false, fmt.Errorf("%w: %s", coreiface.ErrBadRequest, err)
	}
	return resolved, submitted, nil
}

func (s *OrderAppService) submitSettlementCancelAction(ctx context.Context, order *models.Order, coinType iwallet.CoinType, paymentSent *pb.PaymentSent, payoutAddr string, releaseInfo ...any) (iwallet.TransactionID, *iwallet.Transaction, bool, error) {
	return s.submitSettlementAction(ctx, payment.SettlementActionCancel, order, coinType, paymentSent, payoutAddr, releaseInfo...)
}

func (s *OrderAppService) submitSettlementSellerDeclineRefundAction(ctx context.Context, order *models.Order, coinType iwallet.CoinType, paymentSent *pb.PaymentSent, payoutAddr string, releaseInfo ...any) (iwallet.TransactionID, *iwallet.Transaction, bool, error) {
	return s.submitSettlementAction(ctx, payment.SettlementActionSellerDeclineRefund, order, coinType, paymentSent, payoutAddr, releaseInfo...)
}

func (s *OrderAppService) submitSettlementAction(ctx context.Context, action string, order *models.Order, coinType iwallet.CoinType, paymentSent *pb.PaymentSent, payoutAddr string, releaseInfo ...any) (iwallet.TransactionID, *iwallet.Transaction, bool, error) {
	strategy, err := s.v2StrategyForCoin(coinType)
	if err != nil {
		return "", nil, false, err
	}
	if strategy.Model() != payment.PaymentModelMonitored {
		return "", nil, false, nil
	}
	spec, ok := payment.ResolveSettlementSpec(order, paymentSent)
	if !ok || !ordersettlement.EscrowUsesRelayRelease(spec) {
		// UTXO cancelable confirm/cancel still uses ConfirmOrder / escrow inline release.
		return "", nil, false, nil
	}

	if payoutAddr == "" && (action == payment.SettlementActionCancel || action == payment.SettlementActionSellerDeclineRefund) {
		observations := payment.RefundResolutionObservations(s.db, order, paymentSent)
		refundResult := nodepayment.ResolveBuyerRefundForLocalNode(s.db, order, paymentSent, coinType, observations, false)
		if !refundResult.Found() {
			return "", nil, false, fmt.Errorf("%w: no buyer refund address available for settlement %s (%s)",
				models.ErrRefundAddressRequired, action, refundResult.Reason)
		}
		payoutAddr = refundResult.Address
	}

	params := payment.ActionParams{
		OrderID:       order.ID.String(),
		PaymentCoin:   string(coinType),
		PaymentAmount: paymentSent.Amount,
		Chaincode:     paymentSent.Chaincode,
		Script:        paymentSent.Script,
		OrderData:     order,
		PayoutAddr:    payoutAddr,
	}
	if len(releaseInfo) > 0 {
		params.ReleaseInfo = releaseInfo[0]
	}
	var result *payment.ActionResult
	switch action {
	case payment.SettlementActionCancel:
		result, err = strategy.Cancel(ctx, params)
	case payment.SettlementActionSellerDeclineRefund:
		refunder, ok := strategy.(payment.SellerDeclineRefunder)
		if !ok {
			return "", nil, true, fmt.Errorf("%w: settlement action %s is not supported for %s", payment.ErrUnsupportedAction, action, coinType)
		}
		result, err = refunder.SellerDeclineRefund(ctx, params)
	default:
		return "", nil, true, fmt.Errorf("%w: unsupported settlement action %s", payment.ErrUnsupportedAction, action)
	}
	if err != nil {
		return "", nil, true, err
	}

	txHash := ordersettlement.ActionRelayTxHash(ctx, strategy, result)
	if txHash == "" {
		return "", nil, true, fmt.Errorf("settlement %s action submitted without tx hash (order %s)", action, order.ID)
	}
	txid := iwallet.TransactionID(txHash)
	return txid, &iwallet.Transaction{ID: txid}, true, nil
}
