package orders

import (
	"encoding/hex"
	"math/big"

	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/internal/orders/utils"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	npb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"github.com/pkg/errors"
)

func (op *OrderProcessor) processRefundMessage(dbtx database.Tx, order *models.Order, peer peer.ID, message *npb.OrderMessage) (interface{}, error) {
	refund := new(pb.Refund)
	if err := message.Message.UnmarshalTo(refund); err != nil {
		return nil, err
	}

	if order.SerializedOrderCancel != nil {
		logger.LogInfoWithIDf(log, op.nodeID, "Received REFUND message for order %s after ORDER_CANCEL", order.ID)
		return nil, ErrUnexpectedMessage
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

	if err := order.PutMessage(message); err != nil {
		if models.IsDuplicateTransactionError(err) {
			return nil, nil
		}
		return nil, err
	}

	wallet, err := op.multiwallet.WalletForCurrencyCode(paymentSent.Coin)
	if err != nil {
		return nil, err
	}

	if refund.GetTransactionID() != "" && paymentSent.Method == pb.PaymentSent_DIRECT {
		// If this fails it's OK as the processor's unfunded order checking loop will
		// retry at it's next interval.
		tx, err := wallet.GetTransaction(iwallet.TransactionID(refund.GetTransactionID()), iwallet.CoinType(paymentSent.Coin))
		if err == nil && tx != nil {
			for _, from := range tx.From {
				if from.Address.String() == order.PaymentAddress {
					if err := op.processOutgoingPayment(dbtx, order, *tx); err != nil {
						return nil, err
					}
				}
			}
		}
	} else if order.Role() == models.RoleBuyer && refund.GetReleaseInfo() != nil && paymentSent.Method == pb.PaymentSent_MODERATED {
		if err := op.releaseRefundEscrowFunds(wallet, paymentSent, refund.GetReleaseInfo()); err != nil {
			logger.LogInfoWithIDf(log, op.nodeID, "Error releasing funds from escrow during refund processing: %s", err.Error())
			return nil, err
		}
	}

	if order.Role() == models.RoleBuyer {
		logger.LogInfoWithIDf(log, op.nodeID, "Received REFUND message for order %s", order.ID)
	} else if order.Role() == models.RoleVendor {
		logger.LogInfoWithIDf(log, op.nodeID, "Processed own REFUND for order %s", order.ID)
	}

	event := &events.Refund{
		OrderID: order.ID.String(),
		Thumbnail: events.Thumbnail{
			Tiny:  orderOpen.Listings[0].Listing.Item.Images[0].Tiny,
			Small: orderOpen.Listings[0].Listing.Item.Images[0].Small,
		},
		VendorHandle: orderOpen.Listings[0].Listing.VendorID.Handle,
		VendorID:     orderOpen.Listings[0].Listing.VendorID.PeerID,
	}
	return event, nil
}

func (op *OrderProcessor) releaseRefundEscrowFunds(wallet iwallet.Wallet, paymentSent *pb.PaymentSent, releaseInfo *pb.EscrowRelease) error {
	escrowWallet, ok := wallet.(iwallet.Escrow)
	if !ok {
		return errors.New("wallet for moderated order does not support escrow")
	}

	if releaseInfo.ToAddress != paymentSent.RefundAddress {
		return errors.New("refund does not pay out to our refund address")
	}
	_, ok = new(big.Int).SetString(releaseInfo.ToAmount, 10)
	if !ok {
		return errors.New("invalid payment amount")
	}
	txn := iwallet.Transaction{
		To: []iwallet.SpendInfo{
			{
				Address: iwallet.NewAddress(releaseInfo.ToAddress, iwallet.CoinType(paymentSent.Coin)),
				Amount:  iwallet.NewAmount(releaseInfo.ToAmount),
			},
		},
	}

	for _, outpoint := range releaseInfo.Outpoints {
		txn.From = append(txn.From, iwallet.SpendInfo{ID: outpoint.FromID, Amount: iwallet.NewAmount(outpoint.Value)})
	}

	var vendorSigs []iwallet.EscrowSignature
	for _, sig := range releaseInfo.EscrowSignatures {
		vendorSigs = append(vendorSigs, iwallet.EscrowSignature{
			Index:     int(sig.Index),
			Signature: sig.Signature,
		})
	}

	script, err := hex.DecodeString(paymentSent.Script)
	if err != nil {
		return err
	}

	chainCode, err := hex.DecodeString(paymentSent.Chaincode)
	if err != nil {
		return err
	}

	buyerKey, err := utils.GenerateEscrowPrivateKey(op.escrowPrivateKey, chainCode)
	if err != nil {
		return err
	}

	buyerSigs, err := escrowWallet.SignMultisigTransaction(txn, *buyerKey, script)
	if err != nil {
		return err
	}
	dbtx, err := wallet.Begin()
	if err != nil {
		return err
	}
	if _, err := escrowWallet.BuildAndSend(dbtx, txn, [][]iwallet.EscrowSignature{buyerSigs, vendorSigs}, script, iwallet.ORDER_FINISH_REFUND); err != nil {
		return err
	}

	return dbtx.Commit()
}
