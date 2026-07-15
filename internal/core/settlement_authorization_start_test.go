// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package core

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha/internal/core/paymentintent"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/identity"
	"github.com/mobazha/mobazha/pkg/models"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha/pkg/payment"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
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
	bases    []models.PaymentAttemptFundingBasis
}

type settlementAuthorizationExchangeRate struct {
	rate             iwallet.Amount
	updatedAt        time.Time
	base             models.CurrencyCode
	to               models.CurrencyCode
	breakCache       bool
	refreshOnGetRate bool
}

func (r *settlementAuthorizationExchangeRate) GetAllRates(models.CurrencyCode, bool) (map[models.CurrencyCode]iwallet.Amount, error) {
	return nil, nil
}

func (r *settlementAuthorizationExchangeRate) GetRate(base, to models.CurrencyCode, breakCache bool) (iwallet.Amount, error) {
	r.base, r.to, r.breakCache = base, to, breakCache
	if r.refreshOnGetRate {
		r.updatedAt = time.Now().UTC()
	}
	return r.rate, nil
}

func (r *settlementAuthorizationExchangeRate) LastUpdated(models.CurrencyCode) time.Time {
	return r.updatedAt
}

func (p *retainingSettlementOfferPublisher) PublishSettlementFundingBasisProposal(
	_ context.Context,
	_ peer.ID,
	basis models.PaymentAttemptFundingBasis,
) error {
	p.bases = append(p.bases, basis)
	return nil
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
	require.Equal(t, request.BuyerRefundAddress, first.BuyerOffer.BuyerRefundAddress)
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
		PricingCoin: "USD", Amount: "1000",
		BuyerID:  &pb.ID{PeerID: buyerPeerID.String()},
		Listings: []*pb.SignedListing{{Listing: &pb.Listing{VendorID: &pb.ID{PeerID: sellerPeerID.String()}}}},
	})
	require.NoError(t, err)
	order := &models.Order{
		TenantMixin: models.TenantMixin{TenantID: "tenant-a"}, ID: "order-token",
		MyRole: string(models.RoleBuyer), SerializedOrderOpen: openBytes, PaymentSelectionQuoteID: "quote-token",
	}
	tokenRail := "crypto:eip155:1:erc20:0xdAC17F958D2ee523a2206206994597C13D831ec7"
	now := time.Now().UTC()
	require.NoError(t, db.Create(&models.PaymentSelectionQuote{
		TenantID: order.TenantID, QuoteID: order.PaymentSelectionQuoteID, OrderID: order.ID.String(),
		FeeQuoteID: "fee-token", DealLinkID: "deal-token", TermsHash: "terms-token",
		SchemaVersion: 1, PolicyVersion: models.PaymentSelectionQuotePilotZeroFeeV1,
		PricingCurrency: "USD", PricingAmount: "1000", PricingDivisibility: 2,
		PaymentCoin: tokenRail, PaymentCurrency: "ETHUSDT", PaymentDivisibility: 6,
		ConversionRequired: true, ExchangeRate: "100", ExchangeRateBase: "ETHUSDT", ExchangeRateQuote: "USD",
		ExchangeRateQuoteDivisibility: 2, RateSourceUpdatedAt: now.Add(-time.Second),
		PaymentSubtotal: "10000000", ProviderOrNetworkCost: "0",
		PlatformPaymentCost: "0", BuyerPaymentTotal: "10000000", ExpiresAt: now.Add(time.Hour), CreatedAt: now,
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
			RailID: tokenRail, AmountAtomic: "10000000",
		},
	)
	require.NoError(t, err)
	require.Equal(t, tokenRail, started.Attempt.Currency)
	require.Equal(t, "10000000", started.Attempt.AmountValue)
	require.Equal(t, tokenRail, started.BuyerOffer.RailID)
	require.Len(t, publisher.offers, 1)
	require.Len(t, publisher.bases, 1)
	require.Equal(t, buyerPeerID.String(), publisher.bases[0].QuoteIssuer)
	require.Equal(t, started.Attempt.FundingBasisHash, mustFundingBasisHash(t, publisher.bases[0]))
	var attempts int64
	require.NoError(t, db.Model(&models.PaymentAttempt{}).Count(&attempts).Error)
	require.Equal(t, int64(1), attempts)
}

func TestRespondSellerSettlementAuthorization_AdoptsBuyerAttemptAndPublishesIdempotentOffer(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:seller-authorization-response-%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.PaymentAttempt{}, &models.PaymentRouteBinding{}, &models.PaymentAttemptSettlementOffer{},
		&models.PaymentAttemptFundingBasisProposalRecord{},
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
		t.Context(), db, order, buyerOffer, identitySigner, settlementSigner, publisher, route, nil,
	)
	require.NoError(t, err)
	retry, err := respondSellerSettlementAuthorization(
		t.Context(), db, order, buyerOffer, identitySigner, settlementSigner, publisher, route, nil,
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

func TestRespondSellerSettlementAuthorization_RequiresAndBindsCrossCurrencyFundingBasis(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:seller-authorization-cross-currency-%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.PaymentAttempt{}, &models.PaymentRouteBinding{}, &models.PaymentAttemptSettlementOffer{},
		&models.PaymentAttemptFundingBasisProposalRecord{},
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

	route := payment.RouteIdentity{
		ContributionID: "managed-evm.eip155-1", ModuleID: "managed-evm", ImplementationGeneration: "v1",
		RailKind: "escrow", NetworkID: "eip155:1", AssetID: buyerOffer.RailID,
		ProtocolVersion: "1", StateSchemaVersion: "1",
	}
	_, err = respondSellerSettlementAuthorization(
		t.Context(), db, order, buyerOffer, contracts.NewKeyPairSigner(sellerKeys, sellerPeerID),
		new(buyerStartSettlementSigner), &retainingSettlementOfferPublisher{db: db, tenantID: order.TenantID},
		route,
		nil,
	)
	require.ErrorContains(t, err, "load retained settlement funding basis")
	var attempts int64
	require.NoError(t, db.Model(&models.PaymentAttempt{}).Count(&attempts).Error)
	require.Zero(t, attempts)

	now := time.Now().UTC()
	orderHash, err := order.OrderOpenCanonicalHash()
	require.NoError(t, err)
	basis := models.PaymentAttemptFundingBasis{
		Version: models.PaymentAttemptFundingBasisVersion, OrderID: order.ID.String(), AttemptID: buyerOffer.AttemptID,
		AuthorizationContextID: buyerOffer.AuthorizationContextID,
		OrderOpenHash:          orderHash, PricingCurrency: "BTC", PricingAmount: "1000", PricingDivisibility: 8,
		PaymentAssetID: buyerOffer.RailID, PaymentCurrency: "ETH", PaymentDivisibility: 18,
		ConversionRequired: true, ExchangeRate: "500000", ExchangeRateBase: "ETH", ExchangeRateQuote: "BTC",
		ExchangeRateQuoteDivisibility: 8, RateSourceUpdatedUnix: now.Add(-2 * time.Minute).Unix(),
		RoundingPolicy: models.PaymentAttemptFundingRoundingCeilV1, PaymentSubtotal: "2000000000000000",
		ProviderOrNetworkCost: "0", PlatformPaymentCost: "0", BuyerPaymentTotal: "2000000000000000",
		QuoteID: "quote-cross-currency", QuotePolicyVersion: models.PaymentSelectionQuotePilotZeroFeeV1,
		QuoteIssuer: buyerPeerID.String(), IssuedAtUnix: now.Add(-time.Minute).Unix(), ExpiresAtUnix: now.Add(10 * time.Minute).Unix(),
	}
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		return paymentintent.RetainReceivedFundingBasisProposalInTransaction(tx, order.TenantID, basis)
	}))
	rates := &settlementAuthorizationExchangeRate{rate: iwallet.NewAmount("500000"), updatedAt: now.Add(-time.Minute)}
	response, err := respondSellerSettlementAuthorization(
		t.Context(), db, order, buyerOffer, contracts.NewKeyPairSigner(sellerKeys, sellerPeerID),
		new(buyerStartSettlementSigner), &retainingSettlementOfferPublisher{db: db, tenantID: order.TenantID},
		route, rates,
	)
	require.NoError(t, err)
	require.Equal(t, basis.BuyerPaymentTotal, response.Attempt.AmountValue)
	require.Equal(t, mustFundingBasisHash(t, basis), response.Attempt.FundingBasisHash)
	require.Equal(t, basis, *mustAttemptFundingBasis(t, response.Attempt))
}

func mustFundingBasisHash(t *testing.T, basis models.PaymentAttemptFundingBasis) string {
	t.Helper()
	_, hash, err := basis.CanonicalBytesAndHash()
	require.NoError(t, err)
	return hash
}

func mustAttemptFundingBasis(t *testing.T, attempt models.PaymentAttempt) *models.PaymentAttemptFundingBasis {
	t.Helper()
	basis, err := attempt.GetFundingBasis()
	require.NoError(t, err)
	require.NotNil(t, basis)
	return basis
}

func TestValidateSellerPaymentAttemptFundingBasis_UsesFreshSellerRateFloor(t *testing.T) {
	now := time.Now().UTC()
	open := &pb.OrderOpen{PricingCoin: "USD", Amount: "4900"}
	openBytes, err := protojson.Marshal(open)
	require.NoError(t, err)
	order := &models.Order{
		TenantMixin: models.TenantMixin{TenantID: "tenant-seller"}, ID: "order-rate-floor",
		MyRole: string(models.RoleVendor), SerializedOrderOpen: openBytes,
	}
	orderHash, err := order.OrderOpenCanonicalHash()
	require.NoError(t, err)
	basis := models.PaymentAttemptFundingBasis{
		Version: models.PaymentAttemptFundingBasisVersion, OrderID: order.ID.String(), AttemptID: "attempt-rate-floor",
		AuthorizationContextID: strings.Repeat("b", 64),
		OrderOpenHash:          orderHash, PricingCurrency: "USD", PricingAmount: "4900", PricingDivisibility: 2,
		PaymentAssetID: "crypto:eip155:1:native", PaymentCurrency: "ETH", PaymentDivisibility: 18,
		ConversionRequired: true, ExchangeRate: "250000", ExchangeRateBase: "ETH", ExchangeRateQuote: "USD",
		ExchangeRateQuoteDivisibility: 2, RateSourceUpdatedUnix: now.Add(-2 * time.Minute).Unix(),
		RoundingPolicy: models.PaymentAttemptFundingRoundingCeilV1, PaymentSubtotal: "19600000000000000",
		ProviderOrNetworkCost: "0", PlatformPaymentCost: "0", BuyerPaymentTotal: "19600000000000000",
		QuoteID: "quote-rate-floor", QuotePolicyVersion: models.PaymentSelectionQuotePilotZeroFeeV1,
		QuoteIssuer: "buyer-peer", IssuedAtUnix: now.Add(-time.Minute).Unix(), ExpiresAtUnix: now.Add(10 * time.Minute).Unix(),
	}

	// A seller rate of 2550 USD/ETH admits the buyer's 0.0196 ETH proposal.
	rates := &settlementAuthorizationExchangeRate{rate: iwallet.NewAmount("255000"), updatedAt: now.Add(-time.Minute)}
	require.NoError(t, validateSellerPaymentAttemptFundingBasis(basis, order, open, "buyer-peer", rates, now))
	require.Equal(t, models.CurrencyCode("ETH"), rates.base)
	require.Equal(t, models.CurrencyCode("USD"), rates.to)
	require.True(t, rates.breakCache, "seller acceptance must refresh its own provider snapshot")
	rates.refreshOnGetRate = true
	require.NoError(t, validateSellerPaymentAttemptFundingBasis(basis, order, open, "buyer-peer", rates, now),
		"a source timestamp refreshed during GetRate must not be compared with the pre-refresh clock sample")
	rates.refreshOnGetRate = false

	// A seller rate of 2450 USD/ETH requires 0.02 ETH, so this proposal underfunds it.
	rates.rate = iwallet.NewAmount("245000")
	require.ErrorContains(t,
		validateSellerPaymentAttemptFundingBasis(basis, order, open, "buyer-peer", rates, now),
		"below seller exchange-rate policy minimum",
	)

	rates.rate = iwallet.NewAmount("255000")
	rates.updatedAt = now.Add(-maxSellerSettlementRateAge)
	require.ErrorContains(t,
		validateSellerPaymentAttemptFundingBasis(basis, order, open, "buyer-peer", rates, now),
		"snapshot is stale",
	)
}

func TestValidateSellerPaymentAttemptFundingBasis_RejectsBindingAndRoundUpViolations(t *testing.T) {
	now := time.Date(2026, time.July, 15, 8, 0, 0, 0, time.UTC)
	open := &pb.OrderOpen{PricingCoin: "USD", Amount: "4"}
	openBytes, err := protojson.Marshal(open)
	require.NoError(t, err)
	order := &models.Order{
		TenantMixin: models.TenantMixin{TenantID: "tenant-seller"}, ID: "order-binding",
		MyRole: string(models.RoleVendor), SerializedOrderOpen: openBytes,
	}
	orderHash, err := order.OrderOpenCanonicalHash()
	require.NoError(t, err)
	basis := models.PaymentAttemptFundingBasis{
		Version: models.PaymentAttemptFundingBasisVersion, OrderID: order.ID.String(), AttemptID: "attempt-binding",
		AuthorizationContextID: strings.Repeat("c", 64),
		OrderOpenHash:          orderHash, PricingCurrency: "USD", PricingAmount: "4", PricingDivisibility: 2,
		PaymentAssetID: "crypto:eip155:1:native", PaymentCurrency: "ETH", PaymentDivisibility: 18,
		ConversionRequired: true, ExchangeRate: "4000000000000000000", ExchangeRateBase: "ETH", ExchangeRateQuote: "USD",
		ExchangeRateQuoteDivisibility: 2, RateSourceUpdatedUnix: now.Add(-2 * time.Minute).Unix(),
		RoundingPolicy: models.PaymentAttemptFundingRoundingCeilV1, PaymentSubtotal: "1",
		ProviderOrNetworkCost: "0", PlatformPaymentCost: "0", BuyerPaymentTotal: "1",
		QuoteID: "quote-binding", QuotePolicyVersion: models.PaymentSelectionQuotePilotZeroFeeV1,
		QuoteIssuer: "buyer-peer", IssuedAtUnix: now.Add(-time.Minute).Unix(), ExpiresAtUnix: now.Add(time.Minute).Unix(),
	}

	rates := &settlementAuthorizationExchangeRate{rate: iwallet.NewAmount("4000000000000000000"), updatedAt: now.Add(-time.Minute)}
	require.NoError(t, validateSellerPaymentAttemptFundingBasis(basis, order, open, "buyer-peer", rates, now))

	for _, test := range []struct {
		name    string
		mutate  func(*models.PaymentAttemptFundingBasis)
		buyerID string
		want    string
	}{
		{
			name:    "wrong quote issuer",
			mutate:  func(b *models.PaymentAttemptFundingBasis) {},
			buyerID: "another-buyer",
		},
		{
			name:    "wrong signed order hash",
			mutate:  func(b *models.PaymentAttemptFundingBasis) { b.OrderOpenHash = strings.Repeat("d", 64) },
			buyerID: "buyer-peer",
		},
		{
			name:    "expired before authorization",
			mutate:  func(b *models.PaymentAttemptFundingBasis) { b.ExpiresAtUnix = now.Unix() },
			buyerID: "buyer-peer",
		},
		{
			name: "nonzero unapproved cost",
			mutate: func(b *models.PaymentAttemptFundingBasis) {
				b.ProviderOrNetworkCost = "1"
				b.BuyerPaymentTotal = "2"
			},
			buyerID: "buyer-peer",
			want:    "does not admit proposed payment costs",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			candidate := basis
			test.mutate(&candidate)
			err := validateSellerPaymentAttemptFundingBasis(candidate, order, open, test.buyerID, rates, now)
			require.Error(t, err)
			if test.want != "" {
				require.ErrorContains(t, err, test.want)
			} else {
				require.ErrorIs(t, err, models.ErrPaymentAttemptSettlementTermsConflict)
			}
		})
	}

	// 4 pricing atomic units at a seller rate of 3 payment-scaled units require
	// ceil(4/3) = 2 payment atomic units. The buyer's quote-derived total of 1
	// must therefore be rejected instead of silently rounding down.
	rates.rate = iwallet.NewAmount("3000000000000000000")
	require.ErrorContains(t,
		validateSellerPaymentAttemptFundingBasis(basis, order, open, "buyer-peer", rates, now),
		"below seller exchange-rate policy minimum",
	)
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
		nil,
	)
	require.ErrorContains(t, err, "purpose does not match standard order protocol")
}
