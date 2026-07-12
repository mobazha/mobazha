// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package core

import (
	"context"
	"fmt"
	"testing"
	"time"

	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha/internal/core/paymentintent"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/identity"
	"github.com/mobazha/mobazha/pkg/models"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha/pkg/payment"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type buyerStartSettlementSigner struct {
	keyRefs []contracts.SettlementKeyRef
}

func (s *buyerStartSettlementSigner) PublicKey(_ context.Context, keyRef contracts.SettlementKeyRef) ([]byte, error) {
	s.keyRefs = append(s.keyRefs, keyRef)
	return []byte("buyer-attempt-settlement-key"), nil
}

func (*buyerStartSettlementSigner) Sign(context.Context, contracts.SettlementSignRequest) ([]byte, error) {
	return nil, fmt.Errorf("unexpected settlement action signature")
}

type retainingSettlementOfferPublisher struct {
	db       *gorm.DB
	tenantID string
	targets  []peer.ID
	offers   []models.SettlementKeyOffer
}

func (p *retainingSettlementOfferPublisher) PublishSettlementKeyOffer(
	_ context.Context,
	target peer.ID,
	offer models.SettlementKeyOffer,
) error {
	if err := paymentintent.StoreCryptoPaymentAttemptSettlementKeyOffer(
		p.db, p.tenantID, offer.AttemptID, offer,
	); err != nil {
		return err
	}
	p.targets = append(p.targets, target)
	p.offers = append(p.offers, offer)
	return nil
}

func TestBeginBuyerSettlementAuthorization_PersistsDraftAndPublishesIdempotentOffer(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:buyer-authorization-start-%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.PaymentAttempt{}, &models.PaymentRouteBinding{}, &models.PaymentAttemptSettlementOffer{}, &models.PaymentSelectionQuote{},
	))

	buyerKeys, err := identity.GenerateKeyPair()
	require.NoError(t, err)
	buyerPeerID, err := identity.PeerIDFromPublicKey(buyerKeys.PubKey)
	require.NoError(t, err)
	sellerKeys, err := identity.GenerateKeyPair()
	require.NoError(t, err)
	sellerPeerID, err := identity.PeerIDFromPublicKey(sellerKeys.PubKey)
	require.NoError(t, err)
	sellerTarget, err := peer.Decode(sellerPeerID.String())
	require.NoError(t, err)
	openBytes, err := protojson.Marshal(&pb.OrderOpen{
		Amount: "1000", PricingCoin: "ETH",
		BuyerID: &pb.ID{PeerID: buyerPeerID.String()},
		Listings: []*pb.SignedListing{{Listing: &pb.Listing{
			VendorID: &pb.ID{PeerID: sellerPeerID.String()},
		}}},
	})
	require.NoError(t, err)
	order := &models.Order{
		TenantMixin: models.TenantMixin{TenantID: "tenant-a"},
		ID:          "order-1", MyRole: string(models.RoleBuyer), SerializedOrderOpen: openBytes,
		PaymentSelectionQuoteID: "quote-1",
	}
	route := payment.RouteIdentity{
		ContributionID: "managed-evm.eip155-1", ModuleID: "managed-evm",
		ImplementationGeneration: "v1", RailKind: "escrow", NetworkID: "eip155:1",
		AssetID: "crypto:eip155:1:native", ProtocolVersion: "1", StateSchemaVersion: "1",
	}
	request := StandardOrderSettlementAuthorizationRequest{
		OrderID: order.ID.String(), PaymentSelectionQuoteID: "quote-1",
		RailID: route.AssetID, AmountAtomic: "1000",
	}
	require.NoError(t, db.Create(&models.PaymentSelectionQuote{
		TenantID: order.TenantID, QuoteID: order.PaymentSelectionQuoteID, OrderID: order.ID.String(),
		FeeQuoteID: "fee-1", DealLinkID: "deal-1", TermsHash: "terms-1",
		SchemaVersion: 1, PolicyVersion: models.PaymentSelectionQuotePilotZeroFeeV1,
		PricingCurrency: "ETH", PricingAmount: "1000", PricingDivisibility: 18,
		PaymentCoin: route.AssetID, PaymentCurrency: "ETH", PaymentDivisibility: 18,
		ExchangeRate: "1000000000000000000", ExchangeRateBase: "ETH", ExchangeRateQuote: "ETH",
		ExchangeRateQuoteDivisibility: 18, PaymentSubtotal: "1000", ProviderOrNetworkCost: "0",
		PlatformPaymentCost: "0", BuyerPaymentTotal: "1000", ExpiresAt: time.Now().Add(time.Hour), CreatedAt: time.Now(),
	}).Error)
	settlementSigner := new(buyerStartSettlementSigner)
	publisher := &retainingSettlementOfferPublisher{db: db, tenantID: order.TenantID}
	identitySigner := contracts.NewKeyPairSigner(buyerKeys, buyerPeerID)

	first, err := beginBuyerSettlementAuthorization(
		t.Context(), db, order, identitySigner, settlementSigner, publisher, route, request,
	)
	require.NoError(t, err)
	retry, err := beginBuyerSettlementAuthorization(
		t.Context(), db, order, identitySigner, settlementSigner, publisher, route, request,
	)
	require.NoError(t, err)
	require.Equal(t, models.PaymentAttemptAuthorizationDraft, first.Attempt.State)
	require.Equal(t, first.Attempt.AttemptID, retry.Attempt.AttemptID)
	require.Equal(t, first.Attempt.AuthorizationContextID, retry.Attempt.AuthorizationContextID)
	require.Equal(t, first.Route.RouteBindingID, retry.Route.RouteBindingID)
	require.Equal(t, first.BuyerOffer, retry.BuyerOffer)
	require.Equal(t, sellerPeerID.String(), first.SellerPeerID)
	require.Equal(t, models.SettlementParticipantBuyer, first.BuyerOffer.ParticipantRole)
	require.Equal(t, []peer.ID{sellerTarget, sellerTarget}, publisher.targets)
	require.Len(t, settlementSigner.keyRefs, 2)
	require.Equal(t, "standard-order-participant:buyer", settlementSigner.keyRefs[0].Purpose)
	require.Equal(t, first.Attempt.AuthorizationContextID, settlementSigner.keyRefs[0].ReferenceID)
	mutatedAmount := request
	mutatedAmount.AmountAtomic = "999"
	_, err = beginBuyerSettlementAuthorization(
		t.Context(), db, order, identitySigner, settlementSigner, publisher, route, mutatedAmount,
	)
	require.ErrorContains(t, err, "does not match active payment-selection quote")

	var retained int64
	require.NoError(t, db.Model(&models.PaymentAttemptSettlementOffer{}).
		Where("tenant_id = ? AND attempt_id = ?", order.TenantID, first.Attempt.AttemptID).
		Count(&retained).Error)
	require.Equal(t, int64(1), retained)
	var attempts int64
	require.NoError(t, db.Model(&models.PaymentAttempt{}).Count(&attempts).Error)
	require.Equal(t, int64(1), attempts)
}

func TestBeginBuyerSettlementAuthorization_RejectsDifferentSignedSellers(t *testing.T) {
	firstKeys, err := identity.GenerateKeyPair()
	require.NoError(t, err)
	firstPeerID, err := identity.PeerIDFromPublicKey(firstKeys.PubKey)
	require.NoError(t, err)
	secondKeys, err := identity.GenerateKeyPair()
	require.NoError(t, err)
	secondPeerID, err := identity.PeerIDFromPublicKey(secondKeys.PubKey)
	require.NoError(t, err)
	_, _, err = standardOrderSettlementParticipants(&pb.OrderOpen{
		BuyerID: &pb.ID{PeerID: firstPeerID.String()},
		Listings: []*pb.SignedListing{
			{Listing: &pb.Listing{VendorID: &pb.ID{PeerID: secondPeerID.String()}}},
			{Listing: &pb.Listing{VendorID: &pb.ID{PeerID: firstPeerID.String()}}},
		},
	})
	require.ErrorContains(t, err, "requires one seller")
}

func TestBeginBuyerSettlementAuthorization_RejectsTokenRailBeforeDraft(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:buyer-authorization-token-%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.PaymentAttempt{}, &models.PaymentRouteBinding{}, &models.PaymentAttemptSettlementOffer{},
	))
	buyerKeys, err := identity.GenerateKeyPair()
	require.NoError(t, err)
	buyerPeerID, err := identity.PeerIDFromPublicKey(buyerKeys.PubKey)
	require.NoError(t, err)
	sellerKeys, err := identity.GenerateKeyPair()
	require.NoError(t, err)
	sellerPeerID, err := identity.PeerIDFromPublicKey(sellerKeys.PubKey)
	require.NoError(t, err)
	openBytes, err := protojson.Marshal(&pb.OrderOpen{
		BuyerID:  &pb.ID{PeerID: buyerPeerID.String()},
		Listings: []*pb.SignedListing{{Listing: &pb.Listing{VendorID: &pb.ID{PeerID: sellerPeerID.String()}}}},
	})
	require.NoError(t, err)
	order := &models.Order{TenantMixin: models.TenantMixin{TenantID: "tenant-a"}, ID: "order-token", MyRole: string(models.RoleBuyer), SerializedOrderOpen: openBytes}
	tokenRail := "crypto:eip155:1:erc20:0x1111111111111111111111111111111111111111"
	_, err = beginBuyerSettlementAuthorization(
		t.Context(), db, order, contracts.NewKeyPairSigner(buyerKeys, buyerPeerID),
		new(buyerStartSettlementSigner), &retainingSettlementOfferPublisher{db: db, tenantID: order.TenantID},
		payment.RouteIdentity{
			ContributionID: "managed-evm.token", ModuleID: "managed-evm", ImplementationGeneration: "v1",
			RailKind: "escrow", NetworkID: "eip155:1", AssetID: tokenRail, ProtocolVersion: "1", StateSchemaVersion: "1",
		},
		StandardOrderSettlementAuthorizationRequest{OrderID: order.ID.String(), RailID: tokenRail, AmountAtomic: "1000"},
	)
	require.ErrorContains(t, err, "canonical native rail")
	var attempts int64
	require.NoError(t, db.Model(&models.PaymentAttempt{}).Count(&attempts).Error)
	require.Zero(t, attempts)
}
