package orders

import (
	"errors"
	"fmt"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	npb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
)

func (op *OrderProcessor) processDisputeCloseMessage(dbtx database.Tx, order *models.Order, pid peer.ID, message *npb.OrderMessage) (interface{}, error) {
	disputeClose := new(pb.DisputeClose)
	if err := message.Message.UnmarshalTo(disputeClose); err != nil {
		return nil, err
	}
	dup, err := isDuplicate(disputeClose, order.SerializedDisputeClosed)
	if err != nil {
		return nil, err
	}
	if order.SerializedDisputeClosed != nil && !dup {
		log.Errorf("Duplicate DISPUTE_CLOSE message does not match original for order: %s", order.ID)
		return nil, ErrChangedMessage
	} else if dup {
		return nil, nil
	}

	if order.SerializedOrderComplete != nil {
		log.Errorf("Received DISPUTE_CLOSE message for order %s after ORDER_COMPLETION", order.ID)
		return nil, ErrUnexpectedMessage
	}

	if order.SerializedPaymentFinalized != nil {
		log.Errorf("Received DISPUTE_CLOSE message for order %s after PAYMENT_FINALIZED", order.ID)
		return nil, ErrUnexpectedMessage
	}

	if order.SerializedOrderReject != nil {
		log.Errorf("Received DISPUTE_CLOSE message for order %s after ORDER_REJECT", order.ID)
		return nil, ErrUnexpectedMessage
	}

	if order.SerializedOrderCancel != nil {
		log.Errorf("Received DISPUTE_CLOSE message for order %s after ORDER_CANCEL", order.ID)
		return nil, ErrUnexpectedMessage
	}

	orderOpen, err := order.OrderOpenMessage()
	if models.IsMessageNotExistError(err) {
		return nil, order.ParkMessage(message)
	}
	if err != nil {
		return nil, err
	}

	// Validate
	err = op.validateDisputeResolution(disputeClose, order)
	if err != nil {
		return nil, err
	}

	if op.identity == pid {
		log.Infof("Processed own DISPUTE_CLOSE for orderID: %s", order.ID)
	} else {
		log.Infof("Received DISPUTE_CLOSE message for order %s", order.ID)
	}

	var (
		otherPartyID     = orderOpen.Listings[0].Listing.VendorID.PeerID
		otherPartyHandle = orderOpen.Listings[0].Listing.VendorID.Handle
	)
	if order.Role() == models.RoleVendor {
		otherPartyID = orderOpen.BuyerID.PeerID
		otherPartyHandle = orderOpen.BuyerID.Handle
	}

	event := &events.DisputeClose{
		OrderID: order.ID.String(),
		Thumbnail: events.Thumbnail{
			Tiny:  orderOpen.Listings[0].Listing.Item.Images[0].Tiny,
			Small: orderOpen.Listings[0].Listing.Item.Images[0].Small,
		},
		OtherPartyID:     otherPartyID,
		OtherPartyHandle: otherPartyHandle,
		Buyer:            orderOpen.BuyerID.PeerID,
	}

	return event, order.PutMessage(message)
}

// validateDisputeResolution - validate dispute resolution
func (op *OrderProcessor) validateDisputeResolution(disputeClose *pb.DisputeClose, order *models.Order) error {
	releaseInfo := disputeClose.ReleaseInfo

	if len(releaseInfo.Outpoints) == 0 {
		return errors.New("no tx input in dispute resolution")
	}

	if len(releaseInfo.EscrowSignatures) == 0 {
		return errors.New("no moderator signature in dispute resolution")
	}

	orderOpen, err := order.OrderOpenMessage()
	if err != nil {
		errMsg := fmt.Sprintf("failed to get order open message, order id: %s", order.ID)
		log.Error(errMsg)
		return errors.New(errMsg)
	}

	_, err = op.multiwallet.WalletForCurrencyCode(orderOpen.Payment.Coin)
	if err != nil {
		return fmt.Errorf("cannot validate order. coin not supported. %w", err)
	}

	// TODO: HasKey() check is not passed for MATICUSDT, need check
	// var addr string
	// if order.Role() == models.RoleBuyer {
	// 	addr = releaseInfo.BuyerAddress
	// } else {
	// 	addr = releaseInfo.VendorAddress
	// }
	// if len(addr) > 0 {
	// 	pricingCurrency, err := models.CurrencyDefinitions.Lookup(orderOpen.Payment.Coin)
	// 	if err != nil {
	// 		return fmt.Errorf("unrecognized coin: %s, %w", orderOpen.Payment.Coin, err)
	// 	}
	// 	payAddr := iwallet.NewAddress(addr, iwallet.CoinType(pricingCurrency.String()))

	// 	if ok, err := wal.HasKey(payAddr); !ok {
	// 		return fmt.Errorf("dispute resolution payout address %s is not defined in your wallet to recieve funds. %w", addr, err)
	// 	}
	// }

	return nil
}
