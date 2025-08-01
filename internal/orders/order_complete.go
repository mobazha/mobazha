package orders

import (
	"errors"
	"os"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/internal/orders/utils"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	npb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
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
		logger.LogInfoWithIDf(log, op.nodeID, "Duplicate ORDER_COMPLETE message does not match original for order: %s", order.ID)
		return nil, ErrChangedMessage
	} else if dup {
		return nil, nil
	}

	if order.SerializedOrderCancel != nil {
		logger.LogInfoWithIDf(log, op.nodeID, "Received ORDER_COMPLETE message for order %s after ORDER_CANCEL", order.ID)
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

		// Process all ratings in a single batch to minimize database operations
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
		}

		// Save rating index once after processing all ratings
		if err := dbtx.SetRatingIndex(index); err != nil {
			logger.LogErrorWithIDf(log, op.nodeID, "Failed to save rating index for order %s: %v", order.ID, err)
			return nil, err
		}

		// Save individual ratings
		for _, rating := range complete.Ratings {
			if err := dbtx.SetRating(rating); err != nil {
				return nil, err
			}
		}
	}

	if order.Role() == models.RoleVendor {
		logger.LogInfoWithIDf(log, op.nodeID, "Received ORDER_COMPLETE message for order %s", order.ID)
	} else if order.Role() == models.RoleBuyer {
		logger.LogInfoWithIDf(log, op.nodeID, "Processed own ORDER_COMPLETE for order %s", order.ID)
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
