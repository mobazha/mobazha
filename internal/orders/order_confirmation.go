package orders

import (
	"fmt"

	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	npb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

func (op *OrderProcessor) processOrderConfirmationMessage(dbtx database.Tx, order *models.Order, message *npb.OrderMessage) (interface{}, error) {
	orderConfirmation := new(pb.OrderConfirmation)
	if err := message.Message.UnmarshalTo(orderConfirmation); err != nil {
		return nil, err
	}
	dup, err := isDuplicate(orderConfirmation, order.SerializedOrderConfirmation)
	if err != nil {
		return nil, err
	}
	if order.SerializedOrderConfirmation != nil && !dup {
		logger.LogInfoWithIDf(log, op.nodeID, "Duplicate ORDER_CONFIRMATION message does not match original for order: %s", order.ID)
		return nil, ErrChangedMessage
	} else if dup {
		return nil, nil
	}

	// FSM-covered: if the order is in DECLINED state, the FSM rejects EventVendorConfirm.
	if order.SerializedOrderDecline != nil {
		logger.LogInfoWithIDf(log, op.nodeID, "Received ORDER_CONFIRMATION message for order %s after ORDER_DECLINE", order.ID)
		return nil, ErrUnexpectedMessage
	}

	// FSM-covered: if the order is in CANCELED state, the FSM rejects EventVendorConfirm.
	if order.SerializedOrderCancel != nil {
		logger.LogInfoWithIDf(log, op.nodeID, "Possible race: Received ORDER_CONFIRMATION message for order %s after ORDER_CANCEL", order.ID)
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

	if orderConfirmation.TransactionID != "" && paymentSent.Method == pb.PaymentSent_CANCELABLE {
		// Verify on-chain receipt for EVM confirm transactions (H-ESC-4).
		// A reverted tx is FATAL — reject the confirmation message.
		// RPC errors are TRANSIENT and handled as best-effort inside the closure.
		coinInfo, coinErr := iwallet.CoinType(paymentSent.Coin).CoinInfo()
		if coinErr == nil && coinInfo.IsEthTypeChain() && op.verifyConfirmReceiptFunc != nil {
			if verifyErr := op.verifyConfirmReceiptFunc(paymentSent.Coin, orderConfirmation.TransactionID); verifyErr != nil {
				return nil, fmt.Errorf("confirm tx receipt verification failed: %w", verifyErr)
			}
		}

		// Best-effort: record the outgoing release transaction for bookkeeping.
		// Failure is acceptable — the funds have already moved on-chain; this
		// only affects the local transaction ledger display.
		tx, err := wallet.GetTransaction(iwallet.TransactionID(orderConfirmation.TransactionID), iwallet.CoinType(paymentSent.Coin))
		if err == nil && tx != nil {
			if coinErr == nil && coinInfo.IsEthTypeChain() {
				// For EVM chains: the escrow Executed event has different address structure:
				// - tx.From = script hash (not contract address)
				// - tx.To = destination addresses (seller, platform fee, etc.)
				// - Neither matches order.PaymentAddress (escrow contract)
				// Since GetTransaction succeeded, the transaction exists and is confirmed.
				// We can safely add it to the order without address verification.
				if err := op.processOutgoingPayment(dbtx, order, *tx); err != nil {
					return nil, err
				}
			} else {
				// For UTXO chains: the release transaction is sent from the multisig address
				// Check if 'from' address matches the payment address (multisig)
				for _, from := range tx.From {
					if from.Address.String() == order.PaymentAddress {
						if err := op.processOutgoingPayment(dbtx, order, *tx); err != nil {
							return nil, err
						}
						break
					}
				}
			}
		}
	}

	event := &events.OrderConfirmation{
		OrderID: order.ID.String(),
		Thumbnail: events.Thumbnail{
			Tiny:  orderOpen.Listings[0].Listing.Item.Images[0].Tiny,
			Small: orderOpen.Listings[0].Listing.Item.Images[0].Small,
		},
		VendorHandle: orderOpen.Listings[0].Listing.VendorID.Handle,
		VendorID:     orderOpen.Listings[0].Listing.VendorID.PeerID,
	}

	if order.Role() == models.RoleBuyer {
		logger.LogInfoWithIDf(log, op.nodeID, "Received ORDER_CONFIRMATION message for order %s", order.ID)
	} else if order.Role() == models.RoleVendor {
		logger.LogInfoWithIDf(log, op.nodeID, "Processed own ORDER_CONFIRMATION for order %s", order.ID)
	}

	return event, order.PutMessage(message)
}
