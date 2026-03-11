package orders

import (
	"encoding/hex"
	"errors"

	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/internal/orders/utils"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	npb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

func (op *OrderProcessor) processOrderCancelMessage(dbtx database.Tx, order *models.Order, message *npb.OrderMessage) (interface{}, error) {
	orderCancel := new(pb.OrderCancel)
	if err := message.Message.UnmarshalTo(orderCancel); err != nil {
		return nil, err
	}
	dup, err := isDuplicate(orderCancel, order.SerializedOrderCancel)
	if err != nil {
		return nil, err
	}
	if order.SerializedOrderCancel != nil && !dup {
		logger.LogInfoWithIDf(log, op.nodeID, "Duplicate ORDER_CANCEL message does not match original for order: %s", order.ID)
		return nil, ErrChangedMessage
	} else if dup {
		return nil, nil
	}

	if order.SerializedOrderDecline != nil {
		logger.LogInfoWithIDf(log, op.nodeID, "Possible race: Received ORDER_CANCEL message for order %s after ORDER_DECLINE", order.ID)
	}

	if order.SerializedOrderConfirmation != nil {
		logger.LogInfoWithIDf(log, op.nodeID, "Possible race: Received ORDER_CANCEL message for order %s after ORDER_CONFIRMATION", order.ID)
	}

	orderOpen, err := order.OrderOpenMessage()
	if models.IsMessageNotExistError(err) {
		return nil, order.ParkMessage(message)
	}
	if err != nil {
		return nil, err
	}

	paymentSent, err := order.PaymentSentMessage()
	if models.IsMessageNotExistError(err) {
		return nil, order.ParkMessage(message)
	}
	if err != nil {
		return nil, err
	}

	wallet, err := op.multiwallet.WalletForCurrencyCode(paymentSent.Coin)
	if err != nil {
		return nil, err
	}

	if orderCancel.TransactionID != "" && paymentSent.Method == pb.PaymentSent_CANCELABLE {
		// Best-effort: record the outgoing refund transaction for bookkeeping.
		// Failure is acceptable — the funds have already moved on-chain; this
		// only affects the local transaction ledger display.
		tx, err := wallet.GetTransaction(iwallet.TransactionID(orderCancel.TransactionID), iwallet.CoinType(paymentSent.Coin))
		if err == nil && tx != nil {
			for _, from := range tx.From {
				if from.Address.String() == order.PaymentAddress {
					if err := op.processOutgoingPayment(dbtx, order, *tx); err != nil {
						return nil, err
					}
				}
			}
		}
	}

	event := &events.OrderCancel{
		OrderID: order.ID.String(),
		Thumbnail: events.Thumbnail{
			Tiny:  orderOpen.Listings[0].Listing.Item.Images[0].Tiny,
			Small: orderOpen.Listings[0].Listing.Item.Images[0].Small,
		},
		BuyerHandle: orderOpen.BuyerID.Handle,
		BuyerID:     orderOpen.BuyerID.PeerID,
	}

	if order.Role() == models.RoleBuyer {
		logger.LogInfoWithIDf(log, op.nodeID, "Processed own ORDER_CANCEL for orderID: %s", order.ID)
	} else if order.Role() == models.RoleVendor {
		logger.LogInfoWithIDf(log, op.nodeID, "Received ORDER_CANCEL message for order %s", order.ID)
	}

	order.Open = false

	return event, order.PutMessage(message)
}

func (op *OrderProcessor) releaseFromCancelableAddress(tx database.Tx, order *models.Order) (iwallet.Tx, iwallet.TransactionID, error) {
	paymentSent, err := order.PaymentSentMessage()
	if err != nil {
		return nil, "", err
	}

	if paymentSent.Method != pb.PaymentSent_CANCELABLE {
		return nil, "", errors.New("order payment method is not CANCELABLE")
	}

	wallet, err := op.multiwallet.WalletForCurrencyCode(paymentSent.Coin)
	if err != nil {
		return nil, "", err
	}

	coinType := iwallet.CoinType(paymentSent.Coin)
	var toAddress iwallet.Address

	if order.Role() == models.RoleBuyer {
		// Buyer receiving ORDER_DECLINE or ORDER_CANCEL: refund to refund address
		if paymentSent.PayerAddress == "" {
			return nil, "", errors.New("refund address is empty")
		}
		toAddress = iwallet.NewAddress(paymentSent.PayerAddress, coinType)
		logger.LogInfoWithIDf(log, op.nodeID, "Releasing CANCELABLE funds to payer address: %s", paymentSent.PayerAddress)
	} else {
		// Vendor confirming order: use payout address (receiving account)
		toAddress, err = op.GetPayoutAddress(tx, paymentSent.Coin)
		if err != nil {
			return nil, "", err
		}
		logger.LogInfoWithIDf(log, op.nodeID, "Releasing CANCELABLE funds to payout address: %s", toAddress.String())
	}

	escrowWallet, ok := wallet.(iwallet.UTXOEscrow)
	if !ok {
		return nil, "", errors.New("wallet does not support escrow")
	}

	txs, err := order.GetTransactions()
	if err != nil {
		return nil, "", err
	}

	var (
		txn      iwallet.Transaction
		totalOut = iwallet.NewAmount(0)
	)
	spent := make(map[string]bool)
	for _, tx := range txs {
		for _, from := range tx.From {
			spent[hex.EncodeToString(from.ID)] = true
		}
	}
	isETHLikeCoin := wallet.CoinCategory() == iwallet.CoinCategoryEthereum
	for _, tx := range txs {
		for _, to := range tx.To {
			if ((!isETHLikeCoin && !spent[hex.EncodeToString(to.ID)]) || isETHLikeCoin) && to.Address.String() == paymentSent.ToAddress {
				txn.From = append(txn.From, to)
				totalOut = totalOut.Add(to.Amount)
			}
		}
	}

	if len(txn.From) == 0 {
		return nil, "", errors.New("payment address is empty")
	}

	escrowFee, err := escrowWallet.EstimateEscrowFee(1, 1, iwallet.FlNormal)
	if err != nil {
		return nil, "", err
	}
	// The escrow fee is calculated as 100% of EstimateEscrowFee for the first input.
	// Plus 50% of EstimateEscrowFee for each additional input.
	escrowFee = escrowFee.Add(escrowFee.Div(iwallet.NewAmount(2)).Mul(iwallet.NewAmount(len(txn.From) - 1)))

	txn.To = append(txn.To, iwallet.SpendInfo{
		Address: toAddress,
		Amount:  totalOut.Sub(escrowFee),
	})

	script, err := hex.DecodeString(paymentSent.Script)
	if err != nil {
		return nil, "", err
	}

	chainCode, err := hex.DecodeString(paymentSent.Chaincode)
	if err != nil {
		return nil, "", err
	}

	key, err := utils.GenerateEscrowPrivateKey(op.escrowPrivateKey, chainCode)
	if err != nil {
		return nil, "", err
	}

	sigs, err := escrowWallet.SignMultisigTransaction(txn, *key, script)
	if err != nil {
		return nil, "", err
	}

	dbTx, err := wallet.Begin()
	if err != nil {
		return nil, "", err
	}

	finishType := iwallet.ORDER_FINISH_CANCEL
	if order.Role() == models.RoleVendor {
		finishType = iwallet.ORDER_FINISH_COMPLETE
	}
	txid, err := escrowWallet.BuildAndSend(dbTx, txn, [][]iwallet.EscrowSignature{sigs}, script, finishType)
	if err != nil {
		return nil, "", err
	}
	return dbTx, txid, nil
}
