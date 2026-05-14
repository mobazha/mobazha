package orders

import (
	"errors"
	"os"
	"time"

	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/internal/orders/utils"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	npb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"google.golang.org/protobuf/encoding/protojson"
)

func (op *OrderProcessor) processOrderCompleteMessage(dbtx database.Tx, order *models.Order, message *npb.OrderMessage) (interface{}, error) {
	complete := new(pb.OrderComplete)
	if err := message.Message.UnmarshalTo(complete); err != nil {
		return nil, err
	}

	isRatingSupplement := false

	if order.SerializedOrderComplete != nil {
		dup, err := isDuplicate(complete, order.SerializedOrderComplete)
		if err != nil {
			return nil, err
		}
		if dup {
			return nil, nil
		}
		// Allow a "rating supplement": existing has no ratings, new has ratings.
		existing := new(pb.OrderComplete)
		if err := protojson.Unmarshal(order.SerializedOrderComplete, existing); err != nil {
			return nil, err
		}
		if len(existing.Ratings) == 0 && len(complete.Ratings) > 0 {
			isRatingSupplement = true
		} else {
			logger.LogInfoWithIDf(log, op.nodeID, "Duplicate ORDER_COMPLETE message does not match original for order: %s", order.ID)
			return nil, ErrChangedMessage
		}
	}

	if order.SerializedOrderCancel != nil {
		logger.LogInfoWithIDf(log, op.nodeID, "Received ORDER_COMPLETE message for order %s after ORDER_CANCEL", order.ID)
		return nil, ErrUnexpectedMessage
	}

	orderOpen, err := order.OrderOpenMessage()
	if models.IsMessageNotExistError(err) {
		if parkErr := order.ParkMessage(message); parkErr != nil {
			return nil, parkErr
		}
		return nil, ErrMessageParked
	}
	if err != nil {
		return nil, err
	}

	if !isRatingSupplement && len(order.SerializedOrderShipments) == 0 {
		logger.LogInfoWithIDf(log, op.nodeID, "Parking ORDER_COMPLETE for order %s: awaiting shipment", order.ID)
		if parkErr := order.ParkMessage(message); parkErr != nil {
			return nil, parkErr
		}
		return nil, ErrMessageParked
	}

	if len(complete.Ratings) > 0 && len(complete.Ratings) != len(orderOpen.Items) {
		return nil, errors.New("number of ratings does not equal number of items in the order")
	}

	for _, rating := range complete.Ratings {
		if err := utils.ValidateRating(rating); err != nil {
			return nil, err
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

	if isRatingSupplement {
		logger.LogInfoWithIDf(log, op.nodeID, "Processed rating supplement for completed order %s", order.ID)
		return nil, order.PutMessage(message)
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
		BuyerName:   orderOpen.BuyerID.DisplayName(),
		BuyerAvatar: orderOpen.BuyerID.DisplayAvatar(),
		BuyerID:     orderOpen.BuyerID.PeerID,
	}

	order.Open = false

	if err := order.PutMessage(message); err != nil {
		return nil, err
	}

	if order.CompletedAt == nil {
		now := time.Now()
		order.CompletedAt = &now
	}

	return event, nil
}
