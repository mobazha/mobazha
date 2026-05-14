package orders

import (
	"crypto/rand"
	"encoding/hex"
	"errors"

	crypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/internal/orders/utils"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	npb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (op *OrderProcessor) processRatingSignaturesMessage(dbtx database.Tx, order *models.Order, message *npb.OrderMessage) (interface{}, error) {
	rs := new(pb.RatingSignatures)
	if err := message.Message.UnmarshalTo(rs); err != nil {
		return nil, err
	}

	dup, err := isDuplicate(rs, order.SerializedRatingSignatures)
	if err != nil {
		return nil, err
	}
	if order.SerializedRatingSignatures != nil && !dup {
		logger.LogInfoWithIDf(log, op.nodeID, "Duplicate RATING_SIGNATURES message does not match original for order: %s", order.ID)
		return nil, ErrChangedMessage
	} else if dup {
		return nil, nil
	}

	orderOpen, err := order.OrderOpenMessage()
	if models.IsMessageNotExistError(err) {
		return nil, order.ParkMessage(message)
	}
	if err != nil {
		return nil, err
	}

	if len(rs.Sigs) != len(orderOpen.RatingKeys) {
		return nil, errors.New("vendor sent incorrect number of rating signatures")
	}

	pub, err := crypto.UnmarshalPublicKey(orderOpen.Listings[0].Listing.VendorID.Pubkeys.Identity)
	if err != nil {
		return nil, err
	}

	for i, sig := range rs.Sigs {
		listing, err := utils.ExtractListing(orderOpen.Items[i].ListingHash, orderOpen.Listings)
		if err != nil {
			return nil, err
		}

		if sig.Slug != listing.Slug {
			return nil, errors.New("rating signature contains incorrect slug")
		}

		cpy := proto.Clone(sig)
		cpy.(*pb.RatingSignature).VendorSignature = nil

		ser, err := proto.Marshal(cpy)
		if err != nil {
			return nil, err
		}

		valid, err := pub.Verify(ser, sig.VendorSignature)
		if err != nil {
			return nil, err
		}
		if !valid {
			return nil, errors.New("invalid vendor signature on rating key")
		}
	}

	logger.LogInfoWithIDf(log, op.nodeID, "Received RATING_SIGNATURES message for order %s", order.ID)

	event := &events.RatingSignaturesReceived{
		OrderID: order.ID.String(),
	}
	return event, order.PutMessage(message)
}

// sendRatingSignatures signs the buyer's rating keys and sends the signatures to the buyer. We want to do
// this right after the order is funded.
func (op *OrderProcessor) sendRatingSignatures(dbtx database.Tx, order *models.Order, orderOpen *pb.OrderOpen) error {
	rs := &pb.RatingSignatures{
		Timestamp: timestamppb.Now(),
	}
	for i, item := range orderOpen.Items {
		listing, err := utils.ExtractListing(item.ListingHash, orderOpen.Listings)
		if err != nil {
			return err
		}

		r := &pb.RatingSignature{
			Slug:      listing.Slug,
			RatingKey: orderOpen.RatingKeys[i],
		}

		ser, err := proto.Marshal(r)
		if err != nil {
			return err
		}

		sig, err := op.signer.Sign(ser)
		if err != nil {
			return err
		}
		r.VendorSignature = sig

		rs.Sigs = append(rs.Sigs, r)
	}

	rsAny := &anypb.Any{}
	if err := rsAny.MarshalFrom(rs); err != nil {
		return err
	}

	om := npb.OrderMessage{
		OrderID:     order.ID.String(),
		MessageType: npb.OrderMessage_RATING_SIGNATURES,
		Message:     rsAny,
	}

	if err := utils.SignOrderMessage(&om, op.signer); err != nil {
		return err
	}

	payload := &anypb.Any{}
	if err := payload.MarshalFrom(&om); err != nil {
		return err
	}

	messageID := make([]byte, 20)
	if _, err := rand.Read(messageID); err != nil {
		return err
	}

	message := npb.Message{
		MessageType: npb.Message_ORDER,
		MessageID:   hex.EncodeToString(messageID),
		Payload:     payload,
	}

	buyer, err := order.Buyer()
	if err != nil {
		return err
	}

	if err := op.messenger.ReliablySendMessage(dbtx, buyer, &message, nil); err != nil {
		return err
	}

	return order.PutMessage(&om)
}
