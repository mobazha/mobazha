package core

import (
	"context"
	"strconv"
	"time"

	"github.com/mobazha/mobazha/internal/logger"
	"github.com/mobazha/mobazha/pkg/models"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	paymentpkg "github.com/mobazha/mobazha/pkg/payment"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

func (n *MobazhaNode) handleCryptoPaymentVerified(orderID string, paymentSent *pb.PaymentSent) {
	if n.orderService == nil || paymentSent == nil {
		return
	}
	ctx := context.Background()
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
	var verifiedTx *iwallet.Transaction
	if tx, ok := n.verifiedTransactionFromPaymentSent(ctx, paymentSent); ok {
		verifiedTx = tx
		hydratePaymentDataFromTransaction(pd, *tx)
	} else {
		hydratePaymentDataFromObservedTransaction(pd, order)
	}
	if verifiedTx == nil && paymentDataRequiresUTXOOutpoint(pd) && len(pd.ToID) == 0 {
		if n.paymentVerificationService == nil {
			logger.LogWarningWithIDf(log, n.nodeID, "payment verified: UTXO order %s has no verifier available to resolve outpoint for tx %s", orderID, pd.TransactionID)
			return
		}
		tx, err := n.paymentVerificationService.FetchTransaction(ctx, pd.Coin, pd.TransactionID, pd.Coin.FiatProviderID())
		if err != nil {
			logger.LogWarningWithIDf(log, n.nodeID, "payment verified: resolve UTXO outpoint for order %s tx %s: %v", orderID, pd.TransactionID, err)
			return
		}
		if tx == nil {
			logger.LogWarningWithIDf(log, n.nodeID, "payment verified: resolve UTXO outpoint for order %s tx %s returned no transaction", orderID, pd.TransactionID)
			return
		}
		verifiedTx = tx
		hydratePaymentDataFromTransaction(pd, *tx)
		if len(pd.ToID) == 0 {
			logger.LogInfoWithIDf(log, n.nodeID, "payment verified: UTXO order %s tx %s has multiple or no unique outputs for address %s; relaying verified transaction without synthetic outpoint", orderID, pd.TransactionID, pd.ToAddress)
		}
	}
	n.runExtensionDeliveries(ctx)
	switch order.Role() {
	case models.RoleVendor:
		if err := n.orderService.EnsureRatingSignatures(ctx, models.OrderID(orderID)); err != nil {
			logger.LogWarningWithIDf(log, n.nodeID, "payment verified: ensure rating signatures for order %s: %v", orderID, err)
		}
		if verifiedTx != nil {
			buyerPeerID, err := order.Buyer()
			if err != nil {
				logger.LogWarningWithIDf(log, n.nodeID, "payment verified: resolve buyer for order %s: %v", orderID, err)
				return
			}
			n.orderService.RelayPaymentToCounterpartyWithTransaction(ctx, orderID, buyerPeerID, pd, verifiedTx)
		} else {
			n.orderService.RelayPaymentToBuyer(ctx, orderID, pd)
		}
	case models.RoleBuyer:
		vendorPeerID, err := order.Vendor()
		if err != nil {
			logger.LogWarningWithIDf(log, n.nodeID, "payment verified: resolve vendor for order %s: %v", orderID, err)
			return
		}
		n.orderService.RelayPaymentToCounterpartyWithTransaction(ctx, orderID, vendorPeerID, pd, verifiedTx)
	default:
		logger.LogInfoWithIDf(log, n.nodeID, "payment verified: order %s role %q has no counterparty relay", orderID, order.Role())
	}
}

func (n *MobazhaNode) verifiedTransactionFromPaymentSent(ctx context.Context, paymentSent *pb.PaymentSent) (*iwallet.Transaction, bool) {
	if n == nil || n.paymentVerificationService == nil || paymentSent == nil || len(paymentSent.GetFundingFacts()) == 0 {
		return nil, false
	}
	vp, err := n.paymentVerificationService.FetchAndVerify(ctx, &pb.OrderOpen{}, paymentSent, paymentSent.GetToAddress())
	if err != nil || vp == nil {
		if err != nil {
			logger.LogInfoWithIDf(log, n.nodeID, "payment verified: funding facts verification fallback for tx %s: %v", paymentSent.GetTransactionID(), err)
		}
		return nil, false
	}
	return &vp.Transaction, true
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
		CancelFeeAmount:     paymentSent.CancelFeeAmount,
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

func hydratePaymentDataFromObservedTransaction(pd *models.PaymentData, order *models.Order) {
	if pd == nil || order == nil || pd.TransactionID == "" || pd.ToAddress == "" {
		return
	}
	txs, err := order.GetTransactions()
	if err != nil {
		return
	}
	for i := range txs {
		tx := txs[i]
		if tx.ID.String() != pd.TransactionID {
			continue
		}
		hydratePaymentDataFromTransaction(pd, tx)
		return
	}
}

func hydratePaymentDataFromTransaction(pd *models.PaymentData, tx iwallet.Transaction) {
	if pd == nil || pd.ToAddress == "" {
		return
	}
	if tx.ID.String() != "" {
		pd.TransactionID = tx.ID.String()
	}
	if tx.Height > 0 {
		pd.BlockHeight = tx.Height
	}
	var match iwallet.SpendInfo
	matches := 0
	for _, out := range tx.To {
		if !paymentpkg.SameUTXOAddress(out.Address.String(), pd.ToAddress) || len(out.ID) == 0 {
			continue
		}
		match = out
		matches++
	}
	if matches == 1 {
		pd.ToID = append([]byte(nil), match.ID...)
	}
}

func paymentDataRequiresUTXOOutpoint(pd *models.PaymentData) bool {
	if pd == nil || pd.TransactionID == "" || pd.ToAddress == "" {
		return false
	}
	coinInfo, err := iwallet.CoinInfoFromCoinType(pd.Coin)
	if err != nil {
		return false
	}
	return coinInfo.Chain.IsUTXOChain()
}
