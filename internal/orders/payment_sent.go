package orders

import (
	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/events"
	"github.com/mobazha/mobazha3.0/internal/models"
	iwallet "github.com/mobazha/mobazha3.0/internal/multiwallet/wallet-interface"
	npb "github.com/mobazha/mobazha3.0/internal/net/mbzpb"
	pb "github.com/mobazha/mobazha3.0/internal/orders/mbzpb"
	peer "github.com/libp2p/go-libp2p/core/peer"
)

func (op *OrderProcessor) processPaymentSentMessage(dbtx database.Tx, order *models.Order, peer peer.ID, message *npb.OrderMessage) (interface{}, error) {
	payment := new(pb.PaymentSent)
	if err := message.Message.UnmarshalTo(payment); err != nil {
		return nil, err
	}

	orderOpen, err := order.OrderOpenMessage()
	if models.IsMessageNotExistError(err) {
		return nil, order.ParkMessage(message)
	}
	if err != nil {
		return nil, err
	}

	err = order.PutMessage(message)
	if models.IsDuplicateTransactionError(err) {
		return nil, nil
	}

	wallet, err := op.multiwallet.WalletForCurrencyCode(orderOpen.Payment.Coin)
	if err != nil {
		return nil, err
	}

	txs, err := order.GetTransactions()
	if err != nil && !models.IsMessageNotExistError(err) {
		return nil, err
	}

	for _, tx := range txs {
		if tx.ID.String() == payment.TransactionID {
			log.Debugf("Received PAYMENT_SENT message for order %s but already know about transaction", order.ID)
			return nil, nil
		}
	}

	// If this fails it's OK as the processor's unfunded order checking loop will
	// retry at it's next interval.
	tx, err := wallet.GetTransaction(iwallet.TransactionID(payment.TransactionID))
	if err == nil && tx != nil {
		for _, to := range tx.To {
			if to.Address.String() == order.PaymentAddress {
				if err := op.processIncomingPayment(dbtx, order, *tx); err != nil {
					return nil, err
				}
			}
		}
	} else {
		log.Errorf("Failed to get transaction from id: %s", payment.TransactionID)
	}

	log.Infof("Received PAYMENT_SENT message for order %s", order.ID)

	event := &events.PaymentSentReceived{
		OrderID: order.ID.String(),
		Txid:    payment.TransactionID,
	}
	return event, nil
}
