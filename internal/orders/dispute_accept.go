package orders

import (
	"errors"

	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	npb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
)

func (op *OrderProcessor) processDisputeAcceptMessage(dbtx database.Tx, order *models.Order, message *npb.OrderMessage) (interface{}, error) {
	disputeAccept := new(pb.DisputeAccept)
	if err := message.Message.UnmarshalTo(disputeAccept); err != nil {
		return nil, err
	}
	dup, err := isDuplicate(disputeAccept, order.SerializedDisputeAccepted)
	if err != nil {
		return nil, err
	}
	if order.SerializedDisputeAccepted != nil && !dup {
		log.Errorf("Duplicate DISPUTE_ACCEPT message does not match original for order: %s", order.ID)
		return nil, ErrChangedMessage
	} else if dup {
		return nil, nil
	}

	if order.SerializedPaymentFinalized != nil {
		log.Errorf("Received DISPUTE_ACCEPT message for order %s after PAYMENT_FINALIZED", order.ID)
		return nil, ErrUnexpectedMessage
	}

	orderOpen, err := order.OrderOpenMessage()
	if models.IsMessageNotExistError(err) {
		return nil, order.ParkMessage(message)
	}
	if err != nil {
		return nil, err
	}

	var (
		otherPartyID     = ""
		otherPartyName   = ""
		otherPartyAvatar = ""
	)

	// SenderPeerID is set by SignOrderMessage
	senderPeerID := message.SenderPeerID
	if orderOpen.Listings[0].Listing.VendorID.PeerID == senderPeerID {
		otherPartyID = orderOpen.Listings[0].Listing.VendorID.PeerID
		otherPartyName = orderOpen.Listings[0].Listing.VendorID.DisplayName()
		otherPartyAvatar = orderOpen.Listings[0].Listing.VendorID.DisplayAvatar()
	} else if orderOpen.BuyerID.PeerID == senderPeerID {
		otherPartyID = orderOpen.BuyerID.PeerID
		otherPartyName = orderOpen.BuyerID.DisplayName()
		otherPartyAvatar = orderOpen.BuyerID.DisplayAvatar()
	} else {
		return nil, errors.New("message from unexpected peer, not buyer and vendor")
	}

	event := &events.DisputeAccepted{
		OrderID: order.ID.String(),
		Thumbnail: events.Thumbnail{
			Tiny:  orderOpen.Listings[0].Listing.Item.Images[0].Tiny,
			Small: orderOpen.Listings[0].Listing.Item.Images[0].Small,
		},
		OtherPartyID:     otherPartyID,
		OtherPartyName:   otherPartyName,
		OtherPartyAvatar: otherPartyAvatar,
		Buyer:            orderOpen.BuyerID.PeerID,
	}

	if op.identity.String() == senderPeerID {
		logger.LogInfoWithIDf(log, op.nodeID, "Processed own DISPUTE_ACCEPT for orderID: %s", order.ID)
	} else {
		logger.LogInfoWithIDf(log, op.nodeID, "Received DISPUTE_ACCEPT message for order %s", order.ID)
	}

	order.Open = false

	return event, order.PutMessage(message)
}
