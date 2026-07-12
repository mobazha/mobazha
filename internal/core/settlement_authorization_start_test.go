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
	keyRefs   []contracts.SettlementKeyRef
	publicKey []byte
}

func (s *buyerStartSettlementSigner) PublicKey(_ context.Context, keyRef contracts.SettlementKeyRef) ([]byte, error) {
	s.keyRefs = append(s.keyRefs, keyRef)
	if len(s.publicKey) > 0 {
		return append([]byte(nil), s.publicKey...), nil
	}
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
		BuyerRefundAddress: "0x70997970C51812dc3A010C7d01b50e0d17dc79C8",
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
	require.Empty(t, first.BuyerOffer.BuyerRefundAddress)
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

func TestBeginBuyerSettlementAuthorization_AcceptsCanonicalTokenRail(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:buyer-authorization-token-%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
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
	openBytes, err := protojson.Marshal(&pb.OrderOpen{
		BuyerID:  &pb.ID{PeerID: buyerPeerID.String()},
		Listings: []*pb.SignedListing{{Listing: &pb.Listing{VendorID: &pb.ID{PeerID: sellerPeerID.String()}}}},
	})
	require.NoError(t, err)
	order := &models.Order{
		TenantMixin: models.TenantMixin{TenantID: "tenant-a"}, ID: "order-token",
		MyRole: string(models.RoleBuyer), SerializedOrderOpen: openBytes, PaymentSelectionQuoteID: "quote-token",
	}
	tokenRail := "crypto:eip155:1:erc20:0x1111111111111111111111111111111111111111"
	require.NoError(t, db.Create(&models.PaymentSelectionQuote{
		TenantID: order.TenantID, QuoteID: order.PaymentSelectionQuoteID, OrderID: order.ID.String(),
		FeeQuoteID: "fee-token", DealLinkID: "deal-token", TermsHash: "terms-token",
		SchemaVersion: 1, PolicyVersion: models.PaymentSelectionQuotePilotZeroFeeV1,
		PricingCurrency: "USD", PricingAmount: "1000", PricingDivisibility: 2,
		PaymentCoin: tokenRail, PaymentCurrency: "USDT", PaymentDivisibility: 6,
		ExchangeRate: "1", ExchangeRateBase: "USD", ExchangeRateQuote: "USDT",
		ExchangeRateQuoteDivisibility: 6, PaymentSubtotal: "1000", ProviderOrNetworkCost: "0",
		PlatformPaymentCost: "0", BuyerPaymentTotal: "1000", ExpiresAt: time.Now().Add(time.Hour), CreatedAt: time.Now(),
	}).Error)
	publisher := &retainingSettlementOfferPublisher{db: db, tenantID: order.TenantID}
	started, err := beginBuyerSettlementAuthorization(
		t.Context(), db, order, contracts.NewKeyPairSigner(buyerKeys, buyerPeerID),
		new(buyerStartSettlementSigner), publisher,
		payment.RouteIdentity{
			ContributionID: "managed-evm.token", ModuleID: "managed-evm", ImplementationGeneration: "v1",
			RailKind: "escrow", NetworkID: "eip155:1", AssetID: tokenRail, ProtocolVersion: "1", StateSchemaVersion: "1",
		},
		StandardOrderSettlementAuthorizationRequest{
			OrderID: order.ID.String(), PaymentSelectionQuoteID: order.PaymentSelectionQuoteID,
			RailID: tokenRail, AmountAtomic: "1000",
		},
	)
	require.NoError(t, err)
	require.Equal(t, tokenRail, started.Attempt.Currency)
	require.Equal(t, tokenRail, started.BuyerOffer.RailID)
	require.Len(t, publisher.offers, 1)
	var attempts int64
	require.NoError(t, db.Model(&models.PaymentAttempt{}).Count(&attempts).Error)
	require.Equal(t, int64(1), attempts)
}

func TestRespondSellerSettlementAuthorization_AdoptsBuyerAttemptAndPublishesIdempotentOffer(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:seller-authorization-response-%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.PaymentAttempt{}, &models.PaymentRouteBinding{}, &models.PaymentAttemptSettlementOffer{},
	))

	buyerKeys, err := identity.GenerateKeyPair()
	require.NoError(t, err)
	buyerPeerID, err := identity.PeerIDFromPublicKey(buyerKeys.PubKey)
	require.NoError(t, err)
	buyerTarget, err := peer.Decode(buyerPeerID.String())
	require.NoError(t, err)
	sellerKeys, err := identity.GenerateKeyPair()
	require.NoError(t, err)
	sellerPeerID, err := identity.PeerIDFromPublicKey(sellerKeys.PubKey)
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
		TenantMixin: models.TenantMixin{TenantID: "tenant-seller"},
		ID:          "order-seller", MyRole: string(models.RoleVendor), SerializedOrderOpen: openBytes,
	}
	route := payment.RouteIdentity{
		ContributionID: "managed-evm.eip155-1", ModuleID: "managed-evm",
		ImplementationGeneration: "v1", RailKind: "escrow", NetworkID: "eip155:1",
		AssetID: "crypto:eip155:1:native", ProtocolVersion: "1", StateSchemaVersion: "1",
	}
	contextID, err := models.NewSettlementAuthorizationContextID()
	require.NoError(t, err)
	buyerOffer := models.SettlementKeyOffer{
		Version: models.SettlementAuthorizationVersion, AuthorizationContextID: contextID,
		OrderID: order.ID.String(), AttemptID: "attempt-seller-response",
		ParticipantPeerID: buyerPeerID.String(), ParticipantRole: models.SettlementParticipantBuyer,
		RailID: route.AssetID, Purpose: "standard-order-participant:buyer",
		PublicKey: []byte("buyer-response-settlement-key"),
	}
	payload, err := buyerOffer.SigningPayload()
	require.NoError(t, err)
	buyerOffer.Signature, err = contracts.NewKeyPairSigner(buyerKeys, buyerPeerID).Sign(payload)
	require.NoError(t, err)
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		return paymentintent.RetainReceivedSettlementKeyOfferInTransaction(tx, order.TenantID, buyerOffer)
	}))

	settlementSigner := &buyerStartSettlementSigner{publicKey: []byte("seller-response-settlement-key")}
	publisher := &retainingSettlementOfferPublisher{db: db, tenantID: order.TenantID}
	identitySigner := contracts.NewKeyPairSigner(sellerKeys, sellerPeerID)
	first, err := respondSellerSettlementAuthorization(
		t.Context(), db, order, buyerOffer, identitySigner, settlementSigner, publisher, route,
	)
	require.NoError(t, err)
	retry, err := respondSellerSettlementAuthorization(
		t.Context(), db, order, buyerOffer, identitySigner, settlementSigner, publisher, route,
	)
	require.NoError(t, err)

	require.Equal(t, models.PaymentAttemptAuthorizationDraft, first.Attempt.State)
	require.Equal(t, buyerOffer.AttemptID, first.Attempt.AttemptID)
	require.Equal(t, buyerOffer.AuthorizationContextID, first.Attempt.AuthorizationContextID)
	require.Empty(t, first.Attempt.FundingTarget)
	require.Equal(t, first.Attempt.AttemptID, retry.Attempt.AttemptID)
	require.Equal(t, first.Attempt.AuthorizationContextID, retry.Attempt.AuthorizationContextID)
	require.Equal(t, first.Attempt.State, retry.Attempt.State)
	require.Equal(t, first.Route.RouteBindingID, retry.Route.RouteBindingID)
	require.Equal(t, first.SellerOffer, retry.SellerOffer)
	require.Equal(t, buyerPeerID.String(), first.BuyerPeerID)
	require.Equal(t, models.SettlementParticipantSeller, first.SellerOffer.ParticipantRole)
	require.Equal(t, []peer.ID{buyerTarget, buyerTarget}, publisher.targets)
	require.Len(t, settlementSigner.keyRefs, 2)
	require.Equal(t, "standard-order-participant:seller", settlementSigner.keyRefs[0].Purpose)
	require.Equal(t, buyerOffer.AuthorizationContextID, settlementSigner.keyRefs[0].ReferenceID)

	offers, err := paymentintent.ListCryptoPaymentAttemptSettlementKeyOffers(
		db, order.TenantID, buyerOffer.AttemptID,
	)
	require.NoError(t, err)
	require.Len(t, offers, 2)
	require.Equal(t, models.SettlementParticipantBuyer, offers[0].ParticipantRole)
	require.Equal(t, models.SettlementParticipantSeller, offers[1].ParticipantRole)
}

func TestRespondSellerSettlementAuthorization_RejectsCrossCurrencyBeforeDraft(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:seller-authorization-cross-currency-%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
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
		Amount: "1000", PricingCoin: "BTC", BuyerID: &pb.ID{PeerID: buyerPeerID.String()},
		Listings: []*pb.SignedListing{{Listing: &pb.Listing{VendorID: &pb.ID{PeerID: sellerPeerID.String()}}}},
	})
	require.NoError(t, err)
	order := &models.Order{
		TenantMixin: models.TenantMixin{TenantID: "tenant-seller"}, ID: "order-cross-currency",
		MyRole: string(models.RoleVendor), SerializedOrderOpen: openBytes,
	}
	contextID, err := models.NewSettlementAuthorizationContextID()
	require.NoError(t, err)
	buyerOffer := models.SettlementKeyOffer{
		Version: models.SettlementAuthorizationVersion, AuthorizationContextID: contextID,
		OrderID: order.ID.String(), AttemptID: "attempt-cross-currency",
		ParticipantPeerID: buyerPeerID.String(), ParticipantRole: models.SettlementParticipantBuyer,
		RailID: "crypto:eip155:1:native", Purpose: "standard-order-participant:buyer",
		PublicKey: []byte("buyer-cross-currency-settlement-key"),
	}
	payload, err := buyerOffer.SigningPayload()
	require.NoError(t, err)
	buyerOffer.Signature, err = contracts.NewKeyPairSigner(buyerKeys, buyerPeerID).Sign(payload)
	require.NoError(t, err)
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		return paymentintent.RetainReceivedSettlementKeyOfferInTransaction(tx, order.TenantID, buyerOffer)
	}))

	_, err = respondSellerSettlementAuthorization(
		t.Context(), db, order, buyerOffer, contracts.NewKeyPairSigner(sellerKeys, sellerPeerID),
		new(buyerStartSettlementSigner), &retainingSettlementOfferPublisher{db: db, tenantID: order.TenantID},
		payment.RouteIdentity{
			ContributionID: "managed-evm.eip155-1", ModuleID: "managed-evm", ImplementationGeneration: "v1",
			RailKind: "escrow", NetworkID: "eip155:1", AssetID: buyerOffer.RailID,
			ProtocolVersion: "1", StateSchemaVersion: "1",
		},
	)
	require.ErrorContains(t, err, "same-currency signed order amount")
	var attempts int64
	require.NoError(t, db.Model(&models.PaymentAttempt{}).Count(&attempts).Error)
	require.Zero(t, attempts)
}

func TestRespondSellerSettlementAuthorization_RejectsOtherKeyPurpose(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:seller-authorization-purpose-%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	require.NoError(t, err)
	buyerKeys, err := identity.GenerateKeyPair()
	require.NoError(t, err)
	buyerPeerID, err := identity.PeerIDFromPublicKey(buyerKeys.PubKey)
	require.NoError(t, err)
	sellerKeys, err := identity.GenerateKeyPair()
	require.NoError(t, err)
	sellerPeerID, err := identity.PeerIDFromPublicKey(sellerKeys.PubKey)
	require.NoError(t, err)
	contextID, err := models.NewSettlementAuthorizationContextID()
	require.NoError(t, err)
	buyerOffer := models.SettlementKeyOffer{
		Version: models.SettlementAuthorizationVersion, AuthorizationContextID: contextID,
		OrderID: "order-other-purpose", AttemptID: "attempt-other-purpose",
		ParticipantPeerID: buyerPeerID.String(), ParticipantRole: models.SettlementParticipantBuyer,
		RailID: "crypto:eip155:1:native", Purpose: "other-settlement-protocol:buyer",
		PublicKey: []byte("other-purpose-settlement-key"),
	}
	payload, err := buyerOffer.SigningPayload()
	require.NoError(t, err)
	buyerOffer.Signature, err = contracts.NewKeyPairSigner(buyerKeys, buyerPeerID).Sign(payload)
	require.NoError(t, err)

	_, err = respondSellerSettlementAuthorization(
		t.Context(), db,
		&models.Order{ID: models.OrderID(buyerOffer.OrderID), MyRole: string(models.RoleVendor)},
		buyerOffer, contracts.NewKeyPairSigner(sellerKeys, sellerPeerID), new(buyerStartSettlementSigner),
		&retainingSettlementOfferPublisher{db: db}, payment.RouteIdentity{AssetID: buyerOffer.RailID},
	)
	require.ErrorContains(t, err, "purpose does not match standard order protocol")
}
