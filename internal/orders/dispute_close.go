package orders

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	npb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
)

func (op *OrderProcessor) processDisputeCloseMessage(dbtx database.Tx, order *models.Order, message *npb.OrderMessage) (interface{}, error) {
	disputeClose := new(pb.DisputeClose)
	if err := message.Message.UnmarshalTo(disputeClose); err != nil {
		return nil, err
	}
	dup, err := isDuplicate(disputeClose, order.SerializedDisputeClosed)
	if err != nil {
		return nil, err
	}
	if order.SerializedDisputeClosed != nil && !dup {
		logger.LogInfoWithIDf(log, op.nodeID, "Duplicate DISPUTE_CLOSE message does not match original for order: %s", order.ID)
		return nil, ErrChangedMessage
	} else if dup {
		return nil, nil
	}

	if order.SerializedOrderComplete != nil {
		logger.LogInfoWithIDf(log, op.nodeID, "Received DISPUTE_CLOSE message for order %s after ORDER_COMPLETION", order.ID)
		return nil, ErrUnexpectedMessage
	}

	if order.SerializedPaymentFinalized != nil {
		logger.LogInfoWithIDf(log, op.nodeID, "Received DISPUTE_CLOSE message for order %s after PAYMENT_FINALIZED", order.ID)
		return nil, ErrUnexpectedMessage
	}

	if order.SerializedOrderDecline != nil {
		logger.LogInfoWithIDf(log, op.nodeID, "Received DISPUTE_CLOSE message for order %s after ORDER_DECLINE", order.ID)
		return nil, ErrUnexpectedMessage
	}

	if order.SerializedOrderCancel != nil {
		logger.LogInfoWithIDf(log, op.nodeID, "Received DISPUTE_CLOSE message for order %s after ORDER_CANCEL", order.ID)
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

	if op.identity.String() == message.SenderPeerID {
		logger.LogInfoWithIDf(log, op.nodeID, "Processed own DISPUTE_CLOSE for orderID: %s", order.ID)
	} else {
		logger.LogInfoWithIDf(log, op.nodeID, "Received DISPUTE_CLOSE message for order %s", order.ID)
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

	order.Open = false

	return event, order.PutMessage(message)
}

// validateDisputeResolution validates the dispute resolution including payout address integrity.
func (op *OrderProcessor) validateDisputeResolution(disputeClose *pb.DisputeClose, order *models.Order) error {
	releaseInfo := disputeClose.ReleaseInfo
	if releaseInfo == nil {
		return errors.New("dispute resolution missing release info")
	}

	if len(releaseInfo.Outpoints) == 0 {
		return errors.New("no tx input in dispute resolution")
	}

	if len(releaseInfo.EscrowSignatures) == 0 {
		return errors.New("no moderator signature in dispute resolution")
	}

	paymentSent, err := order.PaymentSentMessage()
	if err != nil {
		errMsg := fmt.Sprintf("failed to get payment sent message, order id: %s", order.ID)
		logger.LogInfoWithID(log, op.nodeID, errMsg)
		return errors.New(errMsg)
	}

	_, err = op.multiwallet.WalletForCurrencyCode(paymentSent.Coin)
	if err != nil {
		return fmt.Errorf("cannot validate order. coin not supported. %w", err)
	}

	buyerAddrs := newAddressSet(paymentSent.Coin)
	buyerAddrs.Add(paymentSent.PayerAddress)
	buyerAddrs.Add(paymentSent.RefundAddress)

	vendorAddrs := newAddressSet(paymentSent.Coin)
	if conf, err := order.OrderConfirmationMessage(); err == nil {
		vendorAddrs.Add(conf.PayoutAddress)
	}
	if ffs, err := order.OrderFulfillmentMessages(); err == nil {
		for _, ff := range ffs {
			if ff.ReleaseInfo != nil {
				vendorAddrs.Add(ff.ReleaseInfo.ToAddress)
			}
		}
	}

	if disputeOpen, err := order.DisputeOpenMessage(); err == nil {
		switch disputeOpen.OpenedBy {
		case pb.DisputeOpen_BUYER:
			buyerAddrs.Add(disputeOpen.PayoutAddress)
		case pb.DisputeOpen_VENDOR:
			vendorAddrs.Add(disputeOpen.PayoutAddress)
		default:
			return fmt.Errorf("unknown dispute opener: %v", disputeOpen.OpenedBy)
		}

		if disputeUpdate, err := order.DisputeUpdateMessage(); err == nil {
			switch disputeOpen.OpenedBy {
			case pb.DisputeOpen_BUYER:
				vendorAddrs.Add(disputeUpdate.PayoutAddress)
			case pb.DisputeOpen_VENDOR:
				buyerAddrs.Add(disputeUpdate.PayoutAddress)
			}
		}
	}

	if err := validatePayoutAmountsNonNegative(releaseInfo); err != nil {
		return err
	}

	if !isZeroAmount(releaseInfo.BuyerAmount) {
		if !buyerAddrs.Contains(releaseInfo.BuyerAddress) {
			return fmt.Errorf("buyer payout address %s not in allowed set for order %s",
				releaseInfo.BuyerAddress, order.ID)
		}
	}

	if !isZeroAmount(releaseInfo.VendorAmount) {
		if !vendorAddrs.Contains(releaseInfo.VendorAddress) {
			return fmt.Errorf("vendor payout address %s not in allowed set for order %s",
				releaseInfo.VendorAddress, order.ID)
		}
	}

	return nil
}

func validatePayoutAmountsNonNegative(r *pb.DisputeClose_ModeratedEscrowRelease) error {
	for _, pair := range []struct {
		label  string
		amount string
	}{
		{"buyer", r.BuyerAmount},
		{"vendor", r.VendorAmount},
		{"moderator", r.ModeratorAmount},
	} {
		if isZeroAmount(pair.amount) {
			continue
		}
		val, ok := new(big.Int).SetString(pair.amount, 10)
		if !ok {
			return fmt.Errorf("invalid %s payout amount: %q", pair.label, pair.amount)
		}
		if val.Sign() < 0 {
			return fmt.Errorf("%s payout amount is negative: %s", pair.label, pair.amount)
		}
	}
	return nil
}
