//go:build !private_distribution

package order

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
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

func cloneEscrowRelease(release *pb.EscrowRelease) *pb.EscrowRelease {
	if release == nil {
		return nil
	}
	cloned, ok := proto.Clone(release).(*pb.EscrowRelease)
	if !ok {
		return nil
	}
	return cloned
}

func cloneDisputeRelease(release *pb.DisputeClose_ModeratedEscrowRelease) *pb.DisputeClose_ModeratedEscrowRelease {
	if release == nil {
		return nil
	}
	cloned, ok := proto.Clone(release).(*pb.DisputeClose_ModeratedEscrowRelease)
	if !ok {
		return nil
	}
	return cloned
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

func actionStatusTxHash(ctx context.Context, strategy payment.ChainEscrowV2, actionID string) string {
	if strategy == nil || actionID == "" {
		return ""
	}
	status, err := strategy.GetActionStatus(ctx, actionID)
	if err != nil || status == nil {
		return ""
	}
	return status.TxHash
}

// actionRelayTxHash prefers the hash returned synchronously from relay
// submit, then falls back to GetActionStatus for recently recorded actions.
func actionRelayTxHash(ctx context.Context, strategy payment.ChainEscrowV2, result *payment.ActionResult) string {
	if result != nil && result.SubmittedTxHash != "" {
		return result.SubmittedTxHash
	}
	if result != nil && result.ActionID != "" {
		if h := actionStatusTxHash(ctx, strategy, result.ActionID); h != "" {
			return h
		}
	}
	return ""
}

func orderUsesMonitoredBackendSettlementComplete(
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
	spec, ok := payment.ResolveSettlementSpec(order, paymentSent)
	return ok && (spec.UsesManagedEscrow() || spec.UsesSolanaEscrow())
}

func settlementActionName(action models.SettlementActionSnapshot) string {
	name := strings.ToLower(strings.TrimSpace(action.SettlementAction))
	if name == "" {
		name = strings.ToLower(strings.TrimSpace(action.Action))
	}
	return name
}

// completeSettlementReleaseReady reports whether monitored complete release
// evidence exists. Completion needs a concrete release tx hash so the
// ORDER_COMPLETE message can carry auditable release info.
func completeSettlementReleaseReady(order *models.Order, txid iwallet.TransactionID) bool {
	if txid != "" {
		return true
	}
	if order == nil {
		return false
	}
	for _, action := range order.SettlementActions {
		if settlementActionName(action) != "complete" {
			continue
		}
		if action.TxHash != "" {
			return true
		}
	}
	return false
}

// completeSettlementReleasePending reports an in-flight settlement complete
// action that has not yet produced a tx hash.
func completeSettlementReleasePending(order *models.Order, txid iwallet.TransactionID) bool {
	if txid != "" {
		return false
	}
	if order == nil {
		return false
	}
	for _, action := range order.SettlementActions {
		if settlementActionName(action) != "complete" {
			continue
		}
		state := strings.ToLower(strings.TrimSpace(action.State))
		if action.TxHash != "" {
			return false
		}
		if state == "submitting" || state == "submitted" || state == "confirmed" {
			return true
		}
	}
	return false
}

// ExecuteSettlementCompleteAction submits the backend-monitored escrow release
// for a MODERATED order (ManagedEscrow / Solana Anchor). UTXO and client-signed routes
// return a completed no-op — CompleteOrder still handles release inline.
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

	if !orderUsesMonitoredBackendSettlementComplete(&order, paymentSent, coinType, s.paymentRegistry) {
		return &payment.ActionResult{Mode: payment.ActionModeCompleted}, coinType, nil
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
	_ = release
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
	spec, ok := payment.ResolveSettlementSpec(order, paymentSent)
	if !ok || (!spec.UsesManagedEscrow() && !spec.UsesSolanaEscrow()) {
		return nil, nil, nil, false, nil
	}

	release := cloneEscrowRelease(releaseInfo)
	if release == nil {
		return nil, nil, nil, true, fmt.Errorf("settlement complete release info is nil")
	}

	result, err := strategy.Complete(ctx, payment.ActionParams{
		OrderID:       order.ID.String(),
		PaymentCoin:   string(coinType),
		PaymentAmount: paymentSent.Amount,
		Chaincode:     paymentSent.Chaincode,
		Script:        paymentSent.Script,
		OrderData:     order,
		ReleaseInfo:   release,
	})
	if err != nil {
		return nil, nil, nil, true, err
	}

	txHash := actionRelayTxHash(ctx, strategy, result)
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

func (s *OrderAppService) submitSettlementCompleteAction(ctx context.Context, order *models.Order, coinType iwallet.CoinType, paymentSent *pb.PaymentSent, releaseInfo *pb.EscrowRelease) (*pb.EscrowRelease, *iwallet.Transaction, bool, error) {
	_, release, tx, handled, err := s.runMonitoredSettlementComplete(ctx, order, coinType, paymentSent, releaseInfo)
	return release, tx, handled, err
}

func (s *OrderAppService) submitSettlementCancelAction(ctx context.Context, order *models.Order, coinType iwallet.CoinType, paymentSent *pb.PaymentSent, payoutAddr string) (iwallet.TransactionID, *iwallet.Transaction, bool, error) {
	strategy, err := s.v2StrategyForCoin(coinType)
	if err != nil {
		return "", nil, false, err
	}
	if strategy.Model() != payment.PaymentModelMonitored {
		return "", nil, false, nil
	}
	spec, ok := payment.ResolveSettlementSpec(order, paymentSent)
	if !ok || (!spec.UsesManagedEscrow() && !spec.UsesSolanaEscrow()) {
		return "", nil, false, nil
	}

	if payoutAddr == "" {
		if paymentSent.RefundAddress != "" {
			payoutAddr = paymentSent.RefundAddress
		} else if paymentSent.PayerAddress != "" {
			payoutAddr = paymentSent.PayerAddress
		}
	}

	result, err := strategy.Cancel(ctx, payment.ActionParams{
		OrderID:       order.ID.String(),
		PaymentCoin:   string(coinType),
		PaymentAmount: paymentSent.Amount,
		Chaincode:     paymentSent.Chaincode,
		Script:        paymentSent.Script,
		OrderData:     order,
		PayoutAddr:    payoutAddr,
	})
	if err != nil {
		return "", nil, true, err
	}

	txHash := actionRelayTxHash(ctx, strategy, result)
	if txHash == "" {
		return "", nil, true, fmt.Errorf("settlement cancel action submitted without tx hash (order %s)", order.ID)
	}
	txid := iwallet.TransactionID(txHash)
	return txid, &iwallet.Transaction{ID: txid}, true, nil
}

func (s *OrderAppService) submitSettlementDisputeReleaseAction(ctx context.Context, order *models.Order, coinType iwallet.CoinType, paymentSent *pb.PaymentSent, releaseInfo *pb.DisputeClose_ModeratedEscrowRelease) (iwallet.TransactionID, *iwallet.Transaction, bool, error) {
	strategy, err := s.v2StrategyForCoin(coinType)
	if err != nil {
		return "", nil, false, err
	}
	if strategy.Model() != payment.PaymentModelMonitored {
		return "", nil, false, nil
	}
	spec, ok := payment.ResolveSettlementSpec(order, paymentSent)
	if !ok || (!spec.UsesManagedEscrow() && !spec.UsesSolanaEscrow()) {
		return "", nil, false, nil
	}

	release := cloneDisputeRelease(releaseInfo)
	if release == nil {
		return "", nil, true, fmt.Errorf("settlement dispute release info is nil")
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
		return "", nil, true, err
	}

	txHash := actionRelayTxHash(ctx, strategy, result)
	if txHash == "" {
		return "", nil, true, fmt.Errorf("settlement dispute release action submitted without tx hash (order %s)", order.ID)
	}
	tx := &iwallet.Transaction{ID: iwallet.TransactionID(txHash)}
	return tx.ID, tx, true, nil
}
