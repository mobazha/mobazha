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

// ErrSellerAffiliateSettlementNotReady means the signed Affiliate reference is
// present but the verified-payment transaction has not committed yet.
var ErrSellerAffiliateSettlementNotReady = errors.New("seller affiliate settlement facts are not ready")

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

// PrepareSellerAffiliateSettlement establishes the immutable attribution that
// a seller-funded auto-confirm must consume. Payment events can be delivered
// before the transaction that records verified payment has committed, so an
// affiliate order explicitly reports not-ready instead of being mistaken for
// an ordinary order with no payout.
func (s *OrderAppService) PrepareSellerAffiliateSettlement(ctx context.Context, orderID models.OrderID) error {
	if s == nil || s.sellerAffiliate == nil {
		return nil
	}
	var orderRecord models.Order
	if err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID).First(&orderRecord).Error
	}); err != nil {
		return fmt.Errorf("load seller affiliate settlement order: %w", err)
	}
	if orderRecord.Role() != models.RoleVendor {
		return nil
	}
	orderOpen, err := orderRecord.OrderOpenMessage()
	if err != nil {
		return fmt.Errorf("read seller affiliate settlement OrderOpen: %w", err)
	}
	referralSessionID := strings.TrimSpace(orderOpen.GetAffiliateReferralSessionID())
	if referralSessionID == "" {
		return nil
	}
	if !orderRecord.IsPaymentVerified() {
		return ErrSellerAffiliateSettlementNotReady
	}
	if err := s.ReconcileSellerAffiliateOrder(ctx, orderID); err != nil {
		return err
	}
	attribution, err := s.sellerAffiliate.GetAttributionByOrder(ctx, orderID.String())
	if err != nil {
		return err
	}
	if attribution == nil || attribution.ReferralSessionID != referralSessionID {
		return models.ErrSellerAffiliateConflict
	}
	return nil
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
	return orders.BuildAffiliateOrderFacts(orderID.String(), orderOpen, referralSessionID, time.Now().UTC(), s.exchangeRates)
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
