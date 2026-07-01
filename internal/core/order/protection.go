package order

import (
	"errors"
	"fmt"
	"time"

	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	npb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	"google.golang.org/protobuf/types/known/anypb"
)

var (
	ErrProtectionNotExtendable    = errors.New("protection period cannot be extended for this order")
	ErrProtectionAlreadyExtended  = errors.New("protection period has already been extended")
	ErrOrderNotInProtectionPeriod = errors.New("order is not in protection period")
	ErrNotBuyerOrder              = errors.New("only the buyer can extend protection")
)

// ExtendProtection allows the buyer to extend the auto-complete deadline by
// the policy's ExtendProtectionDays. Only allowed once, only for buyer-side
// SHIPPED orders whose policy permits extension (physical goods).
func (s *OrderAppService) ExtendProtection(orderID models.OrderID) (*models.OrderProtectionInfo, error) {
	var order models.Order
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID).First(&order).Error
	})
	if err != nil {
		return nil, err
	}

	if order.Role() != models.RoleBuyer {
		return nil, ErrNotBuyerOrder
	}

	if order.State != models.OrderState_SHIPPED {
		return nil, ErrOrderNotInProtectionPeriod
	}

	policy := models.ResolvePolicyForOrder(&order)
	if policy.ExtendProtectionDays <= 0 {
		return nil, ErrProtectionNotExtendable
	}

	if order.ProtectionExtendedAt != nil {
		return nil, ErrProtectionAlreadyExtended
	}

	now := time.Now()
	err = s.db.Update(func(tx database.Tx) error {
		return tx.Update("protection_extended_at", &now, map[string]interface{}{"id": orderID}, &models.Order{})
	})
	if err != nil {
		return nil, err
	}

	order.ProtectionExtendedAt = &now
	return order.ComputeProtection(now), nil
}

var (
	ErrNotInAfterSaleWindow        = errors.New("order is not in after-sale window")
	ErrAfterSaleReasonEmpty        = errors.New("after-sale dispute reason is required")
	ErrAfterSaleDisputeAlreadyOpen = errors.New("after-sale dispute already opened for this order")
)

// OpenAfterSaleDispute creates an application-level dispute for a COMPLETED
// order that is still within the after-sale window. Unlike on-chain disputes,
// after-sale disputes don't interact with escrow (funds are already released).
// Resolution depends on seller voluntary refund or platform mediation.
//
// The dispute is persisted on the buyer's order copy, then sent to the
// seller's node via P2P OrderMessage (AFTER_SALE_DISPUTE_OPEN).
func (s *OrderAppService) OpenAfterSaleDispute(orderID models.OrderID, reason string, description string) error {
	if reason == "" {
		return fmt.Errorf("%w", ErrAfterSaleReasonEmpty)
	}

	var order models.Order
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID).First(&order).Error
	})
	if err != nil {
		return fmt.Errorf("load order %s for after-sale dispute: %w", orderID, err)
	}

	now := time.Now()
	if !order.CanRequestAfterSale(now) {
		if order.AfterSaleDispute.OpenedAt != nil {
			return fmt.Errorf("%w", ErrAfterSaleDisputeAlreadyOpen)
		}
		return fmt.Errorf("%w: order %s state=%s", coreiface.ErrBadRequest, orderID, order.State)
	}

	afterSaleProto := &npb.AfterSaleDisputeOpen{
		OrderID:     orderID.String(),
		Reason:      reason,
		Description: description,
		Timestamp:   uint64(now.Unix()),
	}
	afterSaleAny := &anypb.Any{}
	if err := afterSaleAny.MarshalFrom(afterSaleProto); err != nil {
		return fmt.Errorf("failed to marshal AfterSaleDisputeOpen: %w", err)
	}

	orderMsg := &npb.OrderMessage{
		OrderID:      orderID.String(),
		MessageType:  npb.OrderMessage_AFTER_SALE_DISPUTE_OPEN,
		Message:      afterSaleAny,
		SenderPeerID: s.peerID().String(),
	}

	done := make(chan struct{})
	order.AfterSaleDispute.Reason = reason
	order.AfterSaleDispute.Description = description
	order.AfterSaleDispute.OpenedAt = &now

	err = s.db.Update(func(tx database.Tx) error {
		if err := tx.Save(&order); err != nil {
			return err
		}

		payload := &anypb.Any{}
		if err := payload.MarshalFrom(orderMsg); err != nil {
			return err
		}
		message := newMessageWithID()
		message.MessageType = npb.Message_ORDER
		message.Payload = payload

		vendorPeerID, err := order.Vendor()
		if err != nil {
			close(done)
			return fmt.Errorf("failed to resolve vendor peer ID: %w", err)
		}
		if err := s.messenger.ReliablySendMessage(tx, vendorPeerID, message, done); err != nil {
			close(done)
			return err
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to persist after-sale dispute: %w", err)
	}

	orderOpen, err := order.OrderOpenMessage()
	if err != nil {
		return fmt.Errorf("failed to get order open: %w", err)
	}

	var (
		buyerID    = orderOpen.BuyerID.PeerID
		buyerName  = orderOpen.BuyerID.DisplayName()
		vendorID   = orderOpen.Listings[0].Listing.VendorID.PeerID
		vendorName = orderOpen.Listings[0].Listing.VendorID.DisplayName()
		title      = orderOpen.Listings[0].Listing.Item.Title
		thumbnail  = events.Thumbnail{}
	)
	if len(orderOpen.Listings[0].Listing.Item.Images) > 0 {
		thumbnail.Tiny = orderOpen.Listings[0].Listing.Item.Images[0].Tiny
		thumbnail.Small = orderOpen.Listings[0].Listing.Item.Images[0].Small
	}

	s.eventBus.Emit(&events.AfterSaleDisputeOpened{
		OrderID:     orderID.String(),
		Reason:      reason,
		Description: description,
		BuyerName:   buyerName,
		BuyerID:     buyerID,
		VendorName:  vendorName,
		VendorID:    vendorID,
		Thumbnail:   thumbnail,
		Title:       title,
	})

	logger.LogInfoWithIDf(log, s.nodeID, "After-sale dispute opened for order %s, reason: %s", orderID, reason)
	return nil
}

// handleAfterSaleDisputeOpen processes an incoming AFTER_SALE_DISPUTE_OPEN
// P2P message on the seller's node. It validates the sender, checks order
// state, persists the dispute fields on the seller's order copy, and emits
// an AfterSaleDisputeReceived notification event.
func (s *OrderAppService) handleAfterSaleDisputeOpen(orderMsg *npb.OrderMessage) (interface{}, error) {
	disputeOpen := new(npb.AfterSaleDisputeOpen)
	if err := orderMsg.Message.UnmarshalTo(disputeOpen); err != nil {
		return nil, fmt.Errorf("failed to unmarshal AfterSaleDisputeOpen: %w", err)
	}

	orderID := models.OrderID(orderMsg.OrderID)
	var order models.Order
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID).First(&order).Error
	})
	if err != nil {
		return nil, fmt.Errorf("order %s not found: %w", orderID, err)
	}

	buyerPeerID, err := order.Buyer()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve buyer peer ID: %w", err)
	}
	if orderMsg.SenderPeerID != buyerPeerID.String() {
		return nil, fmt.Errorf("sender %s is not the buyer of order %s", orderMsg.SenderPeerID, orderID)
	}

	if order.State != models.OrderState_COMPLETED && order.State != models.OrderState_PAYMENT_FINALIZED {
		return nil, fmt.Errorf("order %s in invalid state %s for after-sale dispute", orderID, order.State)
	}

	if order.AfterSaleDispute.OpenedAt != nil {
		logger.LogInfoWithIDf(log, s.nodeID, "Ignoring duplicate after-sale dispute for order %s", orderID)
		return nil, nil
	}

	now := time.Now()
	order.AfterSaleDispute.Reason = disputeOpen.Reason
	order.AfterSaleDispute.Description = disputeOpen.Description
	order.AfterSaleDispute.OpenedAt = &now

	err = s.db.Update(func(tx database.Tx) error {
		return tx.Save(&order)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to persist received after-sale dispute: %w", err)
	}

	orderOpen, err := order.OrderOpenMessage()
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "Failed to get order open for notification: %v", err)
		return nil, nil
	}

	var (
		buyerName = orderOpen.BuyerID.DisplayName()
		buyerID   = orderOpen.BuyerID.PeerID
		title     = orderOpen.Listings[0].Listing.Item.Title
		thumbnail = events.Thumbnail{}
	)
	if len(orderOpen.Listings[0].Listing.Item.Images) > 0 {
		thumbnail.Tiny = orderOpen.Listings[0].Listing.Item.Images[0].Tiny
		thumbnail.Small = orderOpen.Listings[0].Listing.Item.Images[0].Small
	}

	logger.LogInfoWithIDf(log, s.nodeID, "After-sale dispute received for order %s from buyer %s", orderID, buyerID)

	return &events.AfterSaleDisputeReceived{
		OrderID:     orderID.String(),
		Reason:      disputeOpen.Reason,
		Description: disputeOpen.Description,
		BuyerName:   buyerName,
		BuyerID:     buyerID,
		Thumbnail:   thumbnail,
		Title:       title,
	}, nil
}
