package orders

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/mobazha/mobazha/internal/logger"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/events"
	"github.com/mobazha/mobazha/pkg/models"
	npb "github.com/mobazha/mobazha/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha/pkg/payment"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
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
		if parkErr := order.ParkMessage(message); parkErr != nil {
			return nil, parkErr
		}
		return nil, ErrMessageParked
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
		otherPartyName   = orderOpen.Listings[0].Listing.VendorID.DisplayName()
		otherPartyAvatar = orderOpen.Listings[0].Listing.VendorID.DisplayAvatar()
	)
	if order.Role() == models.RoleVendor {
		otherPartyID = orderOpen.BuyerID.PeerID
		otherPartyName = orderOpen.BuyerID.DisplayName()
		otherPartyAvatar = orderOpen.BuyerID.DisplayAvatar()
	}

	// BuyerRefunded drives digital entitlement revoke on dispute close: only when
	// the buyer receives the full escrow share (vendor gets nothing). Split
	// payouts restore frozen grants; seller-only payouts keep BuyerRefunded false
	// (existing entitlement listener behavior).
	buyerRefunded := buyerReceivesFullDisputeRefund(disputeClose.ReleaseInfo)

	event := &events.DisputeClose{
		OrderID: order.ID.String(),
		Thumbnail: events.Thumbnail{
			Tiny:  orderOpen.Listings[0].Listing.Item.Images[0].Tiny,
			Small: orderOpen.Listings[0].Listing.Item.Images[0].Small,
		},
		OtherPartyID:     otherPartyID,
		OtherPartyName:   otherPartyName,
		OtherPartyAvatar: otherPartyAvatar,
		Buyer:            orderOpen.BuyerID.PeerID,
		BuyerRefunded:    buyerRefunded,
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

	paymentSent, err := order.PaymentSentMessage()
	if err != nil {
		errMsg := fmt.Sprintf("failed to get payment sent message, order id: %s", order.ID)
		logger.LogInfoWithID(log, op.nodeID, errMsg)
		return errors.New(errMsg)
	}
	spec, specOK := payment.ResolveSettlementSpec(order, paymentSent)

	if len(releaseInfo.Outpoints) == 0 && !(specOK && spec.UsesManagedEscrow()) {
		return errors.New("no tx input in dispute resolution")
	}

	if len(releaseInfo.EscrowSignatures) == 0 {
		return errors.New("no moderator signature in dispute resolution")
	}

	normalizedCoin := paymentSent.Coin
	if err := iwallet.CoinType(normalizedCoin).ValidateCanonicalPaymentCoin(); err != nil {
		return fmt.Errorf("cannot validate order. invalid payment coin: %w", err)
	}
	_, err = iwallet.CoinInfoFromCoinType(iwallet.CoinType(normalizedCoin))
	if err != nil {
		return fmt.Errorf("cannot validate order. coin not supported. %w", err)
	}

	buyerAddrs := newAddressSet(normalizedCoin)
	buyerAddrs.Add(order.RefundAddress)
	buyerAddrs.Add(paymentSent.PayerAddress)
	buyerAddrs.Add(paymentSent.RefundAddress)

	vendorAddrs := newAddressSet(normalizedCoin)
	if conf, err := order.OrderConfirmationMessage(); err == nil {
		vendorAddrs.Add(conf.PayoutAddress)
	}
	if shipMsgs, err := order.OrderShipmentMessages(); err == nil {
		for _, sm := range shipMsgs {
			if sm.ReleaseInfo != nil {
				vendorAddrs.Add(sm.ReleaseInfo.ToAddress)
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
	if err := payment.ValidateDisputeReleaseFunding(releaseInfo, paymentSent); err != nil {
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

// buyerReceivesFullDisputeRefund is true when the moderator awards the entire
// escrow payout to the buyer (vendor amount is zero). Used for entitlement
// revoke — split rulings must not set BuyerRefunded.
func buyerReceivesFullDisputeRefund(releaseInfo *pb.DisputeClose_ModeratedEscrowRelease) bool {
	if releaseInfo == nil {
		return false
	}
	return !isZeroAmount(releaseInfo.BuyerAmount) && isZeroAmount(releaseInfo.VendorAmount)
}
