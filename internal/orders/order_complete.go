package orders

import (
	"errors"
	"os"

	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/events"
	"github.com/mobazha/mobazha3.0/internal/models"
	npb "github.com/mobazha/mobazha3.0/internal/net/mbzpb"
	pb "github.com/mobazha/mobazha3.0/internal/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/internal/orders/utils"
	"github.com/libp2p/go-libp2p/core/peer"
	"google.golang.org/protobuf/encoding/protojson"
)

func (op *OrderProcessor) processOrderCompleteMessage(dbtx database.Tx, order *models.Order, peer peer.ID, message *npb.OrderMessage) (interface{}, error) {
	complete := new(pb.OrderComplete)
	if err := message.Message.UnmarshalTo(complete); err != nil {
		return nil, err
	}

	dup, err := isDuplicate(complete, order.SerializedOrderComplete)
	if err != nil {
		return nil, err
	}
	if order.SerializedOrderComplete != nil && !dup {
		log.Errorf("Duplicate ORDER_COMPLETE message does not match original for order: %s", order.ID)
		return nil, ErrChangedMessage
	} else if dup {
		return nil, nil
	}

	if order.SerializedOrderCancel != nil {
		log.Errorf("Received ORDER_COMPLETE message for order %s after ORDER_CANCEL", order.ID)
		return nil, ErrUnexpectedMessage
	}

	orderOpen, err := order.OrderOpenMessage()
	if models.IsMessageNotExistError(err) {
		return nil, order.ParkMessage(message)
	}
	if err != nil {
		return nil, err
	}

	if len(complete.Ratings) != len(orderOpen.Items) {
		return nil, errors.New("number of ratings does not equal number of items in the order")
	}

	if len(complete.Ratings) > 0 {
		for _, rating := range complete.Ratings {
			if err := utils.ValidateRating(rating); err != nil {
				return nil, err
			}
		}
	}
	if order.Role() == models.RoleVendor && len(complete.Ratings) > 0 {
		index, err := dbtx.GetRatingIndex()
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		for _, rating := range complete.Ratings {
			m := protojson.MarshalOptions{Indent: "    "}
			out := m.Format(rating)

			id, err := op.calcCIDFunc([]byte(out))
			if err != nil {
				return nil, err
			}
			err = index.AddRating(rating, id)
			if err != nil {
				return nil, err
			}
			if err := dbtx.SetRatingIndex(index); err != nil {
				return nil, err
			}
			if err := dbtx.SetRating(rating); err != nil {
				return nil, err
			}
		}
	}

	if order.Role() == models.RoleVendor {
		log.Infof("Received ORDER_COMPLETE message for order %s", order.ID)
	} else if order.Role() == models.RoleBuyer {
		log.Infof("Processed own ORDER_COMPLETE for order %s", order.ID)
	}

	event := &events.OrderCompletion{
		OrderID: order.ID.String(),
		Thumbnail: events.Thumbnail{
			Tiny:  orderOpen.Listings[0].Listing.Item.Images[0].Tiny,
			Small: orderOpen.Listings[0].Listing.Item.Images[0].Small,
		},
		BuyerHandle: orderOpen.BuyerID.Handle,
		BuyerID:     orderOpen.BuyerID.PeerID,
	}
	return event, order.PutMessage(message)
}
