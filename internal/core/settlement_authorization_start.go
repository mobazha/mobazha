// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package core

import (
	"context"
	"fmt"
	"strings"
	"time"

	peer "github.com/libp2p/go-libp2p/core/peer"
	corepayment "github.com/mobazha/mobazha/internal/core/payment"
	"github.com/mobazha/mobazha/internal/core/paymentintent"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/distribution"
	"github.com/mobazha/mobazha/pkg/models"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha/pkg/payment"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"gorm.io/gorm"
)

// StandardOrderSettlementAuthorizationRequest starts the non-actionable buyer
// half of one standard-order authorization ceremony. AmountAtomic must already
// come from the accepted order or immutable payment-selection quote.
type StandardOrderSettlementAuthorizationRequest struct {
	OrderID                 string
	PaymentSelectionQuoteID string
	RailID                  string
	AmountAtomic            string
	ModeratorPeerID         string
}

// StandardOrderSettlementAuthorizationStart is the durable result of starting
// a buyer ceremony. It contains public authorization material only.
type StandardOrderSettlementAuthorizationStart struct {
	Attempt      models.PaymentAttempt
	Route        models.PaymentRouteBinding
	BuyerOffer   models.SettlementKeyOffer
	SellerPeerID string
}

type settlementKeyOfferPublisher interface {
	PublishSettlementKeyOffer(context.Context, peer.ID, models.SettlementKeyOffer) error
}

type rawSettlementAuthorizationDB interface {
	RawDB() *gorm.DB
}

// BeginStandardOrderSettlementAuthorization persists the buyer's draft and
// reliably publishes its Identity-signed, attempt-scoped key offer to the
// seller. It never provisions or returns a funding target.
func (n *MobazhaNode) BeginStandardOrderSettlementAuthorization(
	ctx context.Context,
	request StandardOrderSettlementAuthorizationRequest,
) (StandardOrderSettlementAuthorizationStart, error) {
	if err := ctx.Err(); err != nil {
		return StandardOrderSettlementAuthorizationStart{}, err
	}
	if n == nil || n.db == nil || n.orderService == nil || n.signer == nil || n.settlementSigner == nil {
		return StandardOrderSettlementAuthorizationStart{}, fmt.Errorf("standard order settlement authorization is not configured")
	}
	request.OrderID = strings.TrimSpace(request.OrderID)
	request.RailID = strings.TrimSpace(request.RailID)
	request.AmountAtomic = strings.TrimSpace(request.AmountAtomic)
	request.ModeratorPeerID = strings.TrimSpace(request.ModeratorPeerID)
	request.PaymentSelectionQuoteID = strings.TrimSpace(request.PaymentSelectionQuoteID)
	if request.OrderID == "" || request.RailID == "" || request.AmountAtomic == "" {
		return StandardOrderSettlementAuthorizationStart{}, fmt.Errorf("standard order settlement authorization requires order, rail, and amount")
	}

	var order models.Order
	if err := n.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", request.OrderID).First(&order).Error
	}); err != nil {
		return StandardOrderSettlementAuthorizationStart{}, fmt.Errorf("load standard order for settlement authorization: %w", err)
	}
	rail, network, asset, _, err := provisioningCapabilityRoute(corepayment.SessionProvisioningPolicyInput{
		PaymentCoin: request.RailID,
	})
	if err != nil {
		return StandardOrderSettlementAuthorizationStart{}, err
	}
	route, err := n.ResolveNewPaymentRouteIdentity(ctx, distribution.PaymentCapabilityRequest{
		Rail: rail, Network: network, Asset: asset, Operation: distribution.PaymentOperationSetup,
	})
	if err != nil {
		return StandardOrderSettlementAuthorizationStart{}, err
	}
	rawProvider, ok := n.db.(rawSettlementAuthorizationDB)
	if !ok || rawProvider.RawDB() == nil {
		return StandardOrderSettlementAuthorizationStart{}, fmt.Errorf("standard order settlement authorization raw database is unavailable")
	}
	return beginBuyerSettlementAuthorization(
		ctx, rawProvider.RawDB(), &order, n.signer, n.settlementSigner, n.orderService, route, request,
	)
}

func beginBuyerSettlementAuthorization(
	ctx context.Context,
	db *gorm.DB,
	order *models.Order,
	identitySigner contracts.Signer,
	settlementSigner contracts.SettlementSigner,
	publisher settlementKeyOfferPublisher,
	route payment.RouteIdentity,
	request StandardOrderSettlementAuthorizationRequest,
) (StandardOrderSettlementAuthorizationStart, error) {
	if db == nil || order == nil || identitySigner == nil || settlementSigner == nil || publisher == nil {
		return StandardOrderSettlementAuthorizationStart{}, fmt.Errorf("buyer settlement authorization dependencies are required")
	}
	if order.ID.String() != request.OrderID || order.Role() != models.RoleBuyer {
		return StandardOrderSettlementAuthorizationStart{}, fmt.Errorf("settlement authorization must start from the local buyer order")
	}
	coinInfo, err := iwallet.CoinInfoFromCoinType(iwallet.CoinType(request.RailID))
	if err != nil || !coinInfo.IsNative {
		return StandardOrderSettlementAuthorizationStart{}, fmt.Errorf("standard order settlement authorization requires a canonical native rail")
	}
	orderOpen, err := order.OrderOpenMessage()
	if err != nil {
		return StandardOrderSettlementAuthorizationStart{}, fmt.Errorf("load signed order participants: %w", err)
	}
	buyerPeerID, sellerPeerID, err := standardOrderSettlementParticipants(orderOpen)
	if err != nil {
		return StandardOrderSettlementAuthorizationStart{}, err
	}
	if buyerPeerID != identitySigner.PeerID().String() {
		return StandardOrderSettlementAuthorizationStart{}, fmt.Errorf("local identity does not match signed order buyer")
	}
	if err := validateStandardOrderAuthorizationAmount(db, order, orderOpen, request, time.Now().UTC()); err != nil {
		return StandardOrderSettlementAuthorizationStart{}, err
	}
	seller, err := peer.Decode(sellerPeerID)
	if err != nil {
		return StandardOrderSettlementAuthorizationStart{}, fmt.Errorf("decode signed order seller: %w", err)
	}
	tenantID := strings.TrimSpace(order.TenantID)
	attemptSeed := strings.Join([]string{
		request.OrderID, request.PaymentSelectionQuoteID, request.RailID, request.AmountAtomic,
		request.ModeratorPeerID, route.ContributionID, route.ImplementationGeneration, route.ProtocolVersion,
	}, "\x00")
	attemptID := stablePaymentIdentity("pa_", attemptSeed)
	attempt, binding, err := paymentintent.PrepareCryptoPaymentAttemptDraft(db, paymentintent.CryptoPaymentAttemptDraftRequest{
		TenantID: tenantID, AttemptID: attemptID, OrderID: request.OrderID,
		AmountAtomic: request.AmountAtomic, RailID: request.RailID,
		ExpectedModeratorPeerID: request.ModeratorPeerID,
	}, route)
	if err != nil {
		return StandardOrderSettlementAuthorizationStart{}, err
	}
	offer, err := paymentintent.IssueSettlementKeyOffer(
		ctx, identitySigner, settlementSigner,
		contracts.SettlementKeyRef{
			TenantID: tenantID, RailID: request.RailID,
			Purpose: "standard-order-participant", ReferenceID: attempt.AuthorizationContextID,
		},
		request.OrderID, attempt.AttemptID, models.SettlementParticipantBuyer,
	)
	if err != nil {
		return StandardOrderSettlementAuthorizationStart{}, err
	}
	if err := publisher.PublishSettlementKeyOffer(ctx, seller, offer); err != nil {
		return StandardOrderSettlementAuthorizationStart{}, fmt.Errorf("publish buyer settlement key offer: %w", err)
	}
	return StandardOrderSettlementAuthorizationStart{
		Attempt: attempt, Route: binding, BuyerOffer: offer, SellerPeerID: sellerPeerID,
	}, nil
}

func validateStandardOrderAuthorizationAmount(
	db *gorm.DB,
	order *models.Order,
	orderOpen *pb.OrderOpen,
	request StandardOrderSettlementAuthorizationRequest,
	now time.Time,
) error {
	boundQuoteID := strings.TrimSpace(order.PaymentSelectionQuoteID)
	if boundQuoteID != strings.TrimSpace(request.PaymentSelectionQuoteID) {
		return fmt.Errorf("settlement authorization quote does not match order binding")
	}
	if boundQuoteID != "" {
		var quote models.PaymentSelectionQuote
		if err := db.Where(
			"tenant_id = ? AND quote_id = ? AND order_id = ?",
			strings.TrimSpace(order.TenantID), boundQuoteID, order.ID.String(),
		).First(&quote).Error; err != nil {
			return fmt.Errorf("load settlement authorization quote: %w", err)
		}
		if quote.PaymentCoin != request.RailID || quote.BuyerPaymentTotal != request.AmountAtomic ||
			!quote.ExpiresAt.After(now) {
			return fmt.Errorf("settlement authorization does not match active payment-selection quote")
		}
		return nil
	}
	paymentCurrency, err := iwallet.CoinType(request.RailID).PricingCurrencyCode()
	if err != nil || !strings.EqualFold(strings.TrimSpace(paymentCurrency), strings.TrimSpace(orderOpen.PricingCoin)) ||
		strings.TrimSpace(orderOpen.Amount) != request.AmountAtomic {
		return fmt.Errorf("settlement authorization requires a matching payment-selection quote")
	}
	return nil
}

func standardOrderSettlementParticipants(orderOpen *pb.OrderOpen) (string, string, error) {
	if orderOpen == nil || orderOpen.BuyerID == nil || strings.TrimSpace(orderOpen.BuyerID.PeerID) == "" ||
		len(orderOpen.Listings) == 0 {
		return "", "", fmt.Errorf("signed order participants are incomplete")
	}
	buyerPeerID := strings.TrimSpace(orderOpen.BuyerID.PeerID)
	if _, err := peer.Decode(buyerPeerID); err != nil {
		return "", "", fmt.Errorf("signed order buyer is invalid")
	}
	sellerPeerID := ""
	for _, signedListing := range orderOpen.Listings {
		if signedListing == nil || signedListing.Listing == nil || signedListing.Listing.VendorID == nil {
			return "", "", fmt.Errorf("signed order seller is incomplete")
		}
		candidate := strings.TrimSpace(signedListing.Listing.VendorID.PeerID)
		if candidate == "" || (sellerPeerID != "" && candidate != sellerPeerID) {
			return "", "", fmt.Errorf("standard order settlement authorization requires one seller")
		}
		sellerPeerID = candidate
	}
	if sellerPeerID == buyerPeerID {
		return "", "", fmt.Errorf("standard order buyer and seller must differ")
	}
	if _, err := peer.Decode(sellerPeerID); err != nil {
		return "", "", fmt.Errorf("signed order seller is invalid")
	}
	return buyerPeerID, sellerPeerID, nil
}
