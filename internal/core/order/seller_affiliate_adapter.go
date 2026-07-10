package order

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mobazha/mobazha/internal/logger"
	"github.com/mobazha/mobazha/internal/orders"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/events"
	"github.com/mobazha/mobazha/pkg/models"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

// ReconcileSellerAffiliateOrder projects verified seller-order facts into the
// minimal affiliate ledger. It is safe to call after every order message.
func (s *OrderAppService) ReconcileSellerAffiliateOrder(ctx context.Context, orderID models.OrderID) error {
	if s == nil || s.sellerAffiliate == nil {
		return nil
	}
	var orderRecord models.Order
	if err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID).First(&orderRecord).Error
	}); err != nil {
		return fmt.Errorf("load seller affiliate order: %w", err)
	}
	if orderRecord.Role() != models.RoleVendor {
		return nil
	}
	orderOpen, err := orderRecord.OrderOpenMessage()
	if err != nil {
		return fmt.Errorf("read seller affiliate OrderOpen: %w", err)
	}
	referralSessionID := strings.TrimSpace(orderOpen.GetAffiliateReferralSessionID())
	if referralSessionID == "" || !orderRecord.IsPaymentVerified() {
		return nil
	}

	attribution, err := s.sellerAffiliate.GetAttributionByOrder(ctx, orderID.String())
	if errors.Is(err, models.ErrSellerAffiliateNotFound) {
		facts, factsErr := s.sellerAffiliateOrderFacts(orderID, orderOpen, referralSessionID)
		if factsErr != nil {
			return factsErr
		}
		result, attributeErr := s.sellerAffiliate.AttributeOrder(ctx, facts)
		if attributeErr != nil || result == nil {
			return attributeErr
		}
		attribution = &result.Attribution
	} else if err != nil {
		return err
	}
	if attribution.ReferralSessionID != referralSessionID {
		return models.ErrSellerAffiliateConflict
	}
	return s.reconcileSellerAffiliateCommissionStatus(ctx, &orderRecord)
}

// sellerAffiliateSettlementPayout returns the one payout line that Core has
// already attributed to an order.
func (s *OrderAppService) sellerAffiliateSettlementPayout(ctx context.Context, orderID models.OrderID, coinType iwallet.CoinType) (*models.AffiliateSettlementPayout, error) {
	if s == nil || s.sellerAffiliate == nil {
		return nil, nil
	}
	return s.sellerAffiliate.SettlementPayout(ctx, orderID.String(), string(coinType))
}

func (s *OrderAppService) sellerAffiliateOrderFacts(orderID models.OrderID, orderOpen *pb.OrderOpen, referralSessionID string) (models.AffiliateOrderFacts, error) {
	if orderOpen == nil || len(orderOpen.GetItems()) == 0 || orderOpen.GetBuyerID() == nil {
		return models.AffiliateOrderFacts{}, models.ErrInvalidSellerAffiliate
	}
	amounts, err := orders.CalculateOrderNetMerchandiseLines(orderOpen, s.exchangeRates)
	if err != nil {
		return models.AffiliateOrderFacts{}, fmt.Errorf("calculate affiliate merchandise lines: %w", err)
	}
	if len(amounts) != len(orderOpen.GetItems()) {
		return models.AffiliateOrderFacts{}, models.ErrInvalidSellerAffiliate
	}
	sellerPeerID := ""
	lines := make([]models.AffiliateOrderLineFact, 0, len(amounts))
	for index, item := range orderOpen.GetItems() {
		listing, err := extractOrderOpenListing(item.GetListingHash(), orderOpen.GetListings())
		if err != nil || listing.GetVendorID() == nil {
			return models.AffiliateOrderFacts{}, models.ErrInvalidSellerAffiliate
		}
		peerID := strings.TrimSpace(listing.GetVendorID().GetPeerID())
		if peerID == "" || (sellerPeerID != "" && sellerPeerID != peerID) {
			return models.AffiliateOrderFacts{}, models.ErrInvalidSellerAffiliate
		}
		sellerPeerID = peerID
		if amounts[index].Cmp(iwallet.NewAmount(0)) <= 0 {
			continue
		}
		lines = append(lines, models.AffiliateOrderLineFact{
			OrderLineID:          fmt.Sprintf("%s:%d", orderID, index),
			NetMerchandiseAtomic: amounts[index].String(),
			Currency:             orderOpen.GetPricingCoin(),
		})
	}
	if len(lines) == 0 {
		return models.AffiliateOrderFacts{}, models.ErrInvalidSellerAffiliate
	}
	return models.AffiliateOrderFacts{
		OrderID:           orderID.String(),
		SellerPeerID:      sellerPeerID,
		BuyerPeerID:       strings.TrimSpace(orderOpen.GetBuyerID().GetPeerID()),
		ReferralSessionID: referralSessionID,
		AttributedAt:      time.Now().UTC(),
		Lines:             lines,
	}, nil
}

func (s *OrderAppService) reconcileSellerAffiliateCommissionStatus(ctx context.Context, order *models.Order) error {
	if len(order.SerializedRefunds) > 0 || order.State == models.OrderState_REFUNDED {
		if len(order.SerializedRefunds) > 0 {
			if _, err := order.Refunds(); err != nil {
				return fmt.Errorf("read seller affiliate refund facts: %w", err)
			}
		}
		_, err := s.sellerAffiliate.TransitionCommission(ctx, order.ID.String(), models.AffiliateCommissionStatusReversed, models.AffiliateReversalRefund, time.Now().UTC())
		return err
	}
	fiatEvidence, err := order.FiatDisputeEvidence()
	if err != nil {
		return fmt.Errorf("read seller affiliate fiat dispute facts: %w", err)
	}
	if fiatEvidence.ChargebackObserved {
		_, err := s.sellerAffiliate.TransitionCommission(ctx, order.ID.String(), models.AffiliateCommissionStatusReversed, models.AffiliateReversalChargeback, time.Now().UTC())
		return err
	}
	protection := order.ComputeProtection(time.Now())
	if protection != nil && protection.Stage == models.ProtectionStageCompleted {
		_, err := s.sellerAffiliate.TransitionCommission(ctx, order.ID.String(), models.AffiliateCommissionStatusEarned, "", time.Now().UTC())
		return err
	}
	return nil
}

// StartSellerAffiliatePaymentFactListener reconciles provider facts that may
// arrive without another P2P order message. The listener exits with the Node.
func (s *OrderAppService) StartSellerAffiliatePaymentFactListener() {
	if s == nil || s.sellerAffiliate == nil || s.eventBus == nil {
		return
	}
	subscription, err := s.eventBus.Subscribe(&events.ProviderPaymentRiskObserved{})
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "SellerAffiliate: subscribe payment facts: %v", err)
		return
	}
	go func() {
		defer subscription.Close()
		for {
			select {
			case raw, ok := <-subscription.Out():
				if !ok {
					return
				}
				event, ok := raw.(*events.ProviderPaymentRiskObserved)
				if !ok {
					continue
				}
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				err := s.ReconcileSellerAffiliateOrder(ctx, models.OrderID(event.OrderID))
				cancel()
				if err != nil {
					logger.LogInfoWithIDf(log, s.nodeID, "SellerAffiliate: payment fact reconciliation deferred for order %s: %v", event.OrderID, err)
				}
			case <-s.shutdown:
				return
			}
		}
	}()
}

func (s *OrderAppService) reconcilePendingSellerAffiliateOrders() {
	if s == nil || s.sellerAffiliate == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	orderIDs, err := s.sellerAffiliate.ListPendingCommissionOrderIDs(ctx)
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "SellerAffiliate: list pending commissions: %v", err)
		return
	}
	for _, orderID := range orderIDs {
		if ctx.Err() != nil {
			return
		}
		if err := s.ReconcileSellerAffiliateOrder(ctx, models.OrderID(orderID)); err != nil {
			logger.LogInfoWithIDf(log, s.nodeID, "SellerAffiliate: scheduled reconciliation deferred for order %s: %v", orderID, err)
		}
	}
}
