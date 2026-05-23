//go:build !private_distribution

package order

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

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

func (s *OrderAppService) submitSettlementCompleteAction(ctx context.Context, order *models.Order, coinType iwallet.CoinType, paymentSent *pb.PaymentSent, releaseInfo *pb.EscrowRelease) (*pb.EscrowRelease, *iwallet.Transaction, bool, error) {
	strategy, err := s.v2StrategyForCoin(coinType)
	if err != nil {
		return nil, nil, false, err
	}
	if strategy.Model() != payment.PaymentModelMonitored {
		return nil, nil, false, nil
	}
	spec, ok := payment.ResolveSettlementSpec(order, paymentSent)
	if !ok || (!spec.UsesManagedEscrow() && !spec.UsesSolanaEscrow()) {
		return nil, nil, false, nil
	}

	release := cloneEscrowRelease(releaseInfo)
	if release == nil {
		return nil, nil, true, fmt.Errorf("settlement complete release info is nil")
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
		return nil, nil, true, err
	}

	txHash := actionRelayTxHash(ctx, strategy, result)
	if txHash == "" {
		txHash = release.Txid
	}
	if txHash != "" {
		release.Txid = txHash
		return release, &iwallet.Transaction{ID: iwallet.TransactionID(txHash)}, true, nil
	}
	return release, nil, true, nil
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
