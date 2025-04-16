package orders

import (
	"crypto/rand"
	"encoding/hex"
	"errors"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/orders/utils"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	npb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (op *OrderProcessor) processDisputeOpenMessage(dbtx database.Tx, order *models.Order, pid peer.ID, message *npb.OrderMessage) (interface{}, error) {
	disputeOpen := new(pb.DisputeOpen)
	if err := message.Message.UnmarshalTo(disputeOpen); err != nil {
		return nil, err
	}
	dup, err := isDuplicate(disputeOpen, order.SerializedDisputeOpen)
	if err != nil {
		return nil, err
	}
	if order.SerializedDisputeOpen != nil && !dup {
		log.Errorf("Duplicate DISPUTE_OPEN message does not match original for order: %s", order.ID)
		return nil, ErrChangedMessage
	} else if dup {
		return nil, nil
	}

	if order.SerializedOrderComplete != nil {
		log.Errorf("Received DISPUTE_OPEN message for order %s after ORDER_COMPLETION", order.ID)
		return nil, ErrUnexpectedMessage
	}

	if order.SerializedPaymentFinalized != nil {
		log.Errorf("Received DISPUTE_OPEN message for order %s after PAYMENT_FINALIZED", order.ID)
		return nil, ErrUnexpectedMessage
	}

	if order.SerializedOrderReject != nil {
		log.Errorf("Received DISPUTE_OPEN message for order %s after ORDER_REJECT", order.ID)
		return nil, ErrUnexpectedMessage
	}

	if order.SerializedOrderCancel != nil {
		log.Errorf("Received DISPUTE_OPEN message for order %s after ORDER_CANCEL", order.ID)
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
	if err != nil {
		return nil, err
	}

	if paymentSent.Moderator == "" || paymentSent.Method != pb.PaymentSent_MODERATED {
		return nil, errors.New("dispute opened processed for non-moderated order")
	}

	var (
		disputer       = orderOpen.BuyerID.PeerID
		disputerHandle = orderOpen.BuyerID.Handle
		disputee       = orderOpen.Listings[0].Listing.VendorID.PeerID
		disputeeHandle = orderOpen.Listings[0].Listing.VendorID.Handle
	)
	if disputeOpen.OpenedBy == pb.DisputeOpen_VENDOR {
		disputer = orderOpen.Listings[0].Listing.VendorID.PeerID
		disputerHandle = orderOpen.Listings[0].Listing.VendorID.Handle
		disputee = orderOpen.BuyerID.PeerID
		disputeeHandle = orderOpen.BuyerID.Handle
	}

	event := &events.DisputeOpen{
		OrderID: order.ID.String(),
		Thumbnail: events.Thumbnail{
			Tiny:  orderOpen.Listings[0].Listing.Item.Images[0].Tiny,
			Small: orderOpen.Listings[0].Listing.Item.Images[0].Small,
		},
		DisputerID:     disputer,
		DisputerHandle: disputerHandle,
		DisputeeID:     disputee,
		DisputeeHandle: disputeeHandle,
	}

	if (order.Role() == models.RoleBuyer && disputeOpen.OpenedBy == pb.DisputeOpen_BUYER) ||
		(order.Role() == models.RoleVendor && disputeOpen.OpenedBy == pb.DisputeOpen_VENDOR) {

		log.Infof("Processed own DISPUTE_OPEN for orderID: %s", order.ID)
	} else {
		serializedContract, err := order.MarshalBinary()
		if err != nil {
			return nil, err
		}

		payoutAddress, err := op.GetPayoutAddress(dbtx, paymentSent.Coin)
		if err != nil {
			return nil, err
		}

		update := pb.DisputeUpdate{
			Timestamp:     timestamppb.Now(),
			PayoutAddress: payoutAddress.String(),
			Contract:      serializedContract,
		}

		updateAny := &anypb.Any{}
		if err := updateAny.MarshalFrom(&update); err != nil {
			return nil, err
		}

		resp := npb.OrderMessage{
			OrderID:     order.ID.String(),
			MessageType: npb.OrderMessage_DISPUTE_UPDATE,
			Message:     updateAny,
		}

		if err := utils.SignOrderMessage(&resp, op.identityPrivateKey); err != nil {
			return nil, err
		}

		payload := &anypb.Any{}
		if err := payload.MarshalFrom(&resp); err != nil {
			return nil, err
		}

		messageID := make([]byte, 20)
		if _, err := rand.Read(messageID); err != nil {
			return nil, err
		}

		msg := npb.Message{
			MessageType: npb.Message_DISPUTE,
			MessageID:   hex.EncodeToString(messageID),
			Payload:     payload,
		}

		moderator, err := peer.Decode(paymentSent.Moderator)
		if err != nil {
			return nil, err
		}

		if err := order.PutMessage(&resp); err != nil {
			return nil, err
		}

		if err := op.messenger.ReliablySendMessage(dbtx, moderator, &msg, nil); err != nil {
			return nil, err
		}
		log.Infof("Received DISPUTE_OPEN message for order %s", order.ID)
	}

	return event, order.PutMessage(message)
}
