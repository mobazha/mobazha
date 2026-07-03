package orders

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha/internal/logger"
	"github.com/mobazha/mobazha/internal/orders/utils"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/events"
	"github.com/mobazha/mobazha/pkg/models"
	npb "github.com/mobazha/mobazha/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha/pkg/payment"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (op *OrderProcessor) processDisputeOpenMessage(dbtx database.Tx, order *models.Order, message *npb.OrderMessage) (interface{}, error) {
	disputeOpen := new(pb.DisputeOpen)
	if err := message.Message.UnmarshalTo(disputeOpen); err != nil {
		return nil, err
	}
	dup, err := isDuplicate(disputeOpen, order.SerializedDisputeOpen)
	if err != nil {
		return nil, err
	}
	if order.SerializedDisputeOpen != nil && !dup {
		logger.LogInfoWithIDf(log, op.nodeID, "Duplicate DISPUTE_OPEN message does not match original for order: %s", order.ID)
		return nil, ErrChangedMessage
	} else if dup {
		return nil, nil
	}

	// FSM-covered: COMPLETED, PAYMENT_FINALIZED, DECLINED, CANCELED are all final states
	// with no outgoing transitions. The FSM rejects EventDisputeOpened from any of them.
	if order.SerializedOrderComplete != nil {
		logger.LogInfoWithIDf(log, op.nodeID, "Received DISPUTE_OPEN message for order %s after ORDER_COMPLETION", order.ID)
		return nil, ErrUnexpectedMessage
	}

	if order.SerializedPaymentFinalized != nil {
		logger.LogInfoWithIDf(log, op.nodeID, "Received DISPUTE_OPEN message for order %s after PAYMENT_FINALIZED", order.ID)
		return nil, ErrUnexpectedMessage
	}

	if order.SerializedOrderDecline != nil {
		logger.LogInfoWithIDf(log, op.nodeID, "Received DISPUTE_OPEN message for order %s after ORDER_DECLINE", order.ID)
		return nil, ErrUnexpectedMessage
	}

	if order.SerializedOrderCancel != nil {
		logger.LogInfoWithIDf(log, op.nodeID, "Received DISPUTE_OPEN message for order %s after ORDER_CANCEL", order.ID)
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

	paymentSent, err := order.PaymentSentMessage()
	if models.IsMessageNotExistError(err) {
		if parkErr := order.ParkMessage(message); parkErr != nil {
			return nil, parkErr
		}
		return nil, ErrMessageParked
	}
	if err != nil {
		return nil, err
	}

	method, ok := payment.ResolvedPaymentMethod(order, paymentSent)
	if paymentSent.Moderator == "" || !ok || !payment.MethodIsModerated(method) {
		return nil, errors.New("dispute opened processed for non-moderated order")
	}

	var (
		disputer       = orderOpen.BuyerID.PeerID
		disputerName   = orderOpen.BuyerID.DisplayName()
		disputerAvatar = orderOpen.BuyerID.DisplayAvatar()
		disputee       = orderOpen.Listings[0].Listing.VendorID.PeerID
		disputeeName   = orderOpen.Listings[0].Listing.VendorID.DisplayName()
		disputeeAvatar = orderOpen.Listings[0].Listing.VendorID.DisplayAvatar()
	)
	if disputeOpen.OpenedBy == pb.DisputeOpen_VENDOR {
		disputer = orderOpen.Listings[0].Listing.VendorID.PeerID
		disputerName = orderOpen.Listings[0].Listing.VendorID.DisplayName()
		disputerAvatar = orderOpen.Listings[0].Listing.VendorID.DisplayAvatar()
		disputee = orderOpen.BuyerID.PeerID
		disputeeName = orderOpen.BuyerID.DisplayName()
		disputeeAvatar = orderOpen.BuyerID.DisplayAvatar()
	}

	event := &events.DisputeOpen{
		OrderID: order.ID.String(),
		Thumbnail: events.Thumbnail{
			Tiny:  orderOpen.Listings[0].Listing.Item.Images[0].Tiny,
			Small: orderOpen.Listings[0].Listing.Item.Images[0].Small,
		},
		DisputerID:     disputer,
		DisputerName:   disputerName,
		DisputerAvatar: disputerAvatar,
		DisputeeID:     disputee,
		DisputeeName:   disputeeName,
		DisputeeAvatar: disputeeAvatar,
	}

	if (order.Role() == models.RoleBuyer && disputeOpen.OpenedBy == pb.DisputeOpen_BUYER) ||
		(order.Role() == models.RoleVendor && disputeOpen.OpenedBy == pb.DisputeOpen_VENDOR) {

		logger.LogInfoWithIDf(log, op.nodeID, "Processed own DISPUTE_OPEN for orderID: %s", order.ID)
	} else {
		serializedContract, err := order.MarshalBinary()
		if err != nil {
			return nil, err
		}

		var payoutAddress iwallet.Address
		if order.Role() == models.RoleBuyer {
			coin, coinErr := payment.SettlementCoinFromPaymentSent(paymentSent)
			if coinErr != nil {
				coin = iwallet.CoinType(paymentSent.Coin)
			}
			observations := payment.RefundResolutionObservationsTx(dbtx, order, paymentSent)
			var prefs map[string]string
			var userPrefs models.UserPreferences
			if err := dbtx.Read().First(&userPrefs).Error; err == nil {
				loaded, loadErr := userPrefs.RefundReceivingAddresses()
				if loadErr != nil {
					logger.LogWarningWithIDf(log, op.nodeID, "Failed to load refund receiving preferences for order %s: %v", order.ID, loadErr)
				} else {
					prefs = loaded
				}
			}
			refundResult := payment.RefundResolveRequest{
				Order:                  order,
				PaymentSent:            paymentSent,
				Coin:                   coin,
				Observations:           observations,
				LocalRefundPreferences: prefs,
			}.Resolve()
			if refundResult.Found() {
				payoutAddress = iwallet.NewAddress(refundResult.Address, coin)
			} else if !coin.IsFiatPayment() {
				if parkErr := order.ParkMessage(message); parkErr != nil {
					return nil, parkErr
				}
				return nil, ErrMessageParked
			}
		} else {
			orderConfirmation, err := order.OrderConfirmationMessage()
			if err != nil {
				logger.LogErrorWithIDf(log, op.nodeID, "Failed to get order confirmation message: %v", err)
			} else {
				payoutAddress = iwallet.NewAddress(orderConfirmation.PayoutAddress, iwallet.CoinType(paymentSent.Coin))
			}

			orderShipments, err := order.OrderShipmentMessages()
			if err == nil && len(orderShipments) > 0 && orderShipments[0].ReleaseInfo != nil {
				payoutAddress = iwallet.NewAddress(orderShipments[0].ReleaseInfo.ToAddress, iwallet.CoinType(paymentSent.Coin))
			}

			if payoutAddress.String() == "" {
				addr, err := op.GetPayoutAddress(dbtx, paymentSent.Coin)
				if err == nil {
					payoutAddress = addr
				} else {
					logger.LogErrorWithIDf(log, op.nodeID,
						"Vendor has no payout address for dispute update, order: %s", order.ID)
				}
			}
		}

		update := pb.DisputeUpdate{
			Timestamp:     timestamppb.Now(),
			PayoutAddress: payoutAddress.String(),
			Contract:      serializedContract,
		}

		updateAny := &anypb.Any{}
		if err := updateAny.MarshalFrom(&update); err != nil {
			return nil, fmt.Errorf("failed to marshal dispute update message: %w", err)
		}

		resp := npb.OrderMessage{
			OrderID:     order.ID.String(),
			MessageType: npb.OrderMessage_DISPUTE_UPDATE,
			Message:     updateAny,
		}

		if err := utils.SignOrderMessage(&resp, op.signer); err != nil {
			return nil, fmt.Errorf("failed to sign dispute update message: %w", err)
		}

		payload := &anypb.Any{}
		if err := payload.MarshalFrom(&resp); err != nil {
			return nil, fmt.Errorf("failed to marshal dispute update message: %w", err)
		}

		messageID := make([]byte, 20)
		if _, err := rand.Read(messageID); err != nil {
			return nil, fmt.Errorf("failed to generate message ID: %w", err)
		}

		msg := npb.Message{
			MessageType: npb.Message_DISPUTE,
			MessageID:   hex.EncodeToString(messageID),
			Payload:     payload,
		}

		moderator, err := peer.Decode(paymentSent.Moderator)
		if err != nil {
			return nil, fmt.Errorf("failed to get moderator: %w", err)
		}

		if err := order.PutMessage(&resp); err != nil {
			return nil, fmt.Errorf("failed to put dispute update message: %w", err)
		}

		if err := op.messenger.ReliablySendMessage(dbtx, moderator, &msg, nil); err != nil {
			return nil, fmt.Errorf("failed to send dispute update message: %w", err)
		}
		logger.LogInfoWithIDf(log, op.nodeID, "Received DISPUTE_OPEN message for order %s", order.ID)
	}

	if err := persistInboundDisputeEvidence(dbtx, order, disputeOpen.EvidenceHashes); err != nil {
		return nil, err
	}

	return event, order.PutMessage(message)
}

// persistInboundDisputeEvidence mirrors OpenDispute DB persistence for nodes that receive
// DISPUTE_OPEN over P2P (seller, moderator) so API merge and case views can read hashes.
func persistInboundDisputeEvidence(tx database.Tx, order *models.Order, hashes []string) error {
	if len(hashes) == 0 || len(order.DisputeEvidenceHashes) > 0 {
		return nil
	}
	order.DisputeEvidenceHashes = models.StringSlice(hashes)
	return tx.Update(
		"dispute_evidence_hashes",
		order.DisputeEvidenceHashes,
		map[string]interface{}{"id": order.ID.String()},
		&models.Order{},
	)
}
