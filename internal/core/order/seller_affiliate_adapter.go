// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package order

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mobazha/mobazha/internal/logger"
	"github.com/mobazha/mobazha/internal/orders"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/events"
	"github.com/mobazha/mobazha/pkg/models"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

// ErrSellerAffiliateSettlementNotReady means the signed Affiliate reference is
// present but the verified-payment transaction has not committed yet.
var ErrSellerAffiliateSettlementNotReady = errors.New("seller affiliate settlement facts are not ready")

func (s *OrderAppService) prepareSellerAffiliateOrderSnapshot(
	ctx context.Context,
	orderID string,
	orderOpen *pb.OrderOpen,
) (*models.AffiliateOrderResult, error) {
	if s == nil || s.sellerAffiliate == nil {
		return nil, fmt.Errorf("seller affiliate attribution is not configured")
	}
	referralSessionID := strings.TrimSpace(orderOpen.GetAffiliateReferralSessionID())
	if referralSessionID == "" {
		return nil, nil
	}
	attributedAt := time.Now().UTC()
	if timestamp := orderOpen.GetTimestamp(); timestamp != nil && !timestamp.AsTime().IsZero() {
		attributedAt = timestamp.AsTime().UTC()
	}
	facts, err := orders.BuildAffiliateOrderFacts(orderID, orderOpen, referralSessionID, attributedAt, s.exchangeRates)
	if err != nil {
		return nil, err
	}
	existing, err := s.sellerAffiliate.GetAttributionByOrder(ctx, orderID)
	if err == nil {
		if existing == nil || existing.ReferralSessionID != referralSessionID ||
			existing.SellerPeerID != facts.SellerPeerID || existing.BuyerPeerID != facts.BuyerPeerID {
			return nil, models.ErrSellerAffiliateConflict
		}
		return nil, nil
	}
	if !errors.Is(err, models.ErrSellerAffiliateNotFound) {
		return nil, err
	}
	attributionService, ok := s.sellerAffiliate.(contracts.SellerAffiliateAttributionService)
	if !ok {
		return nil, fmt.Errorf("seller affiliate attribution does not support transactional order snapshots")
	}
	result, err := attributionService.PrepareOrderAttribution(ctx, facts)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, models.ErrInvalidSellerAffiliate
	}
	return result, nil
}

// EnsureSellerAffiliateOrderSnapshot repairs the invariant for referred
// seller orders accepted by an older binary before OrderOpen attribution was
// made transactional. Normal orders already have the snapshot and take the
// read-only idempotent path.
func (s *OrderAppService) EnsureSellerAffiliateOrderSnapshot(ctx context.Context, orderID models.OrderID) error {
	if s == nil || s.sellerAffiliate == nil {
		return nil
	}
	if err := s.acquireOrderLock(orderID); err != nil {
		return fmt.Errorf("acquire seller affiliate order lock: %w", err)
	}
	defer s.releaseOrderLock(orderID)

	var order models.Order
	if err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.String()).First(&order).Error
	}); err != nil {
		return fmt.Errorf("load seller affiliate order snapshot: %w", err)
	}
	if order.Role() != models.RoleVendor {
		return nil
	}
	orderOpen, err := order.OrderOpenMessage()
	if err != nil {
		return fmt.Errorf("read seller affiliate OrderOpen snapshot: %w", err)
	}
	if strings.TrimSpace(orderOpen.GetAffiliateReferralSessionID()) == "" {
		return nil
	}
	result, err := s.prepareSellerAffiliateOrderSnapshot(ctx, orderID.String(), orderOpen)
	if err != nil || result == nil {
		return err
	}
	attributionService, ok := s.sellerAffiliate.(contracts.SellerAffiliateAttributionService)
	if !ok {
		return fmt.Errorf("seller affiliate attribution does not support transactional order snapshots")
	}
	return s.db.Update(func(tx database.Tx) error {
		_, err := attributionService.RecordPreparedOrderTx(tx, result)
		return err
	})
}

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
	if referralSessionID == "" {
		return nil
	}

	attribution, err := s.sellerAffiliate.GetAttributionByOrder(ctx, orderID.String())
	if errors.Is(err, models.ErrSellerAffiliateNotFound) {
		if !orderRecord.IsPaymentVerified() {
			return nil
		}
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

func (s *OrderAppService) sellerAffiliateSettlementTermsPresent(ctx context.Context, orderID models.OrderID) (bool, error) {
	if s == nil || s.sellerAffiliate == nil {
		return false, nil
	}
	provider, ok := s.sellerAffiliate.(contracts.SellerAffiliateSettlementTermsProvider)
	if !ok {
		return false, nil
	}
	return provider.HasSettlementTerms(ctx, orderID.String())
}

func (s *OrderAppService) sellerAffiliateOrderFacts(orderID models.OrderID, orderOpen *pb.OrderOpen, referralSessionID string) (models.AffiliateOrderFacts, error) {
	return orders.BuildAffiliateOrderFacts(orderID.String(), orderOpen, referralSessionID, time.Now().UTC(), s.exchangeRates)
}

func (s *OrderAppService) reconcileSellerAffiliateCommissionStatus(ctx context.Context, order *models.Order) error {
	if len(order.SerializedRefunds) > 0 || order.State == models.OrderState_REFUNDED {
		var refundedLineIDs []string
		allLines := order.State == models.OrderState_REFUNDED
		if len(order.SerializedRefunds) > 0 {
			refunds, err := order.Refunds()
			if err != nil {
				return fmt.Errorf("read seller affiliate refund facts: %w", err)
			}
			seen := make(map[uint32]struct{})
			for _, refund := range refunds {
				if refund == nil || len(refund.GetRefundedItemIndexes()) == 0 {
					allLines = true
					break
				}
				for _, itemIndex := range refund.GetRefundedItemIndexes() {
					seen[itemIndex] = struct{}{}
				}
			}
			if !allLines {
				commissionLines, err := s.sellerAffiliate.ListCommissionLinesByOrder(ctx, order.ID.String())
				if err != nil {
					return fmt.Errorf("list seller affiliate commission lines: %w", err)
				}
				commissionLineIDs := make(map[string]struct{}, len(commissionLines))
				for _, line := range commissionLines {
					commissionLineIDs[line.OrderLineID] = struct{}{}
				}
				indexes := make([]int, 0, len(seen))
				for itemIndex := range seen {
					indexes = append(indexes, int(itemIndex))
				}
				sort.Ints(indexes)
				for _, itemIndex := range indexes {
					lineID := fmt.Sprintf("%s:%d", order.ID, itemIndex)
					if _, ok := commissionLineIDs[lineID]; ok {
						refundedLineIDs = append(refundedLineIDs, lineID)
					}
				}
			}
		}
		if !allLines && len(refundedLineIDs) > 0 {
			_, err := s.sellerAffiliate.TransitionCommissionLines(ctx, order.ID.String(), refundedLineIDs, models.AffiliateCommissionStatusReversed, models.AffiliateReversalRefund, time.Now().UTC())
			return err
		}
		if !allLines {
			return nil
		}
		_, err := s.sellerAffiliate.TransitionCommission(ctx, order.ID.String(), models.AffiliateCommissionStatusReversed, models.AffiliateReversalRefund, time.Now().UTC())
		return err
	}
	if order.State == models.OrderState_CANCELED || order.State == models.OrderState_DECLINED {
		_, err := s.sellerAffiliate.TransitionCommission(ctx, order.ID.String(), models.AffiliateCommissionStatusReversed, models.AffiliateReversalOrderInvalid, time.Now().UTC())
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
