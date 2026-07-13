// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package order

import (
	"context"
	"testing"
	"time"

	testutil "github.com/mobazha/mobazha/internal/orders/testutil"
	orderutils "github.com/mobazha/mobazha/internal/orders/utils"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/identity"
	"github.com/mobazha/mobazha/pkg/models"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingSellerAffiliateService struct {
	attribution  *models.AffiliateAttribution
	payout       *models.AffiliateSettlementPayout
	termsPresent bool
	facts        []models.AffiliateOrderFacts
	recorded     bool
	status       models.AffiliateCommissionStatus
	reason       models.AffiliateCommissionReversalReason
}

func (*recordingSellerAffiliateService) PutProgram(context.Context, *models.AffiliateProgram) (*models.AffiliateProgram, error) {
	return nil, nil
}
func (*recordingSellerAffiliateService) GetProgram(context.Context) (*models.AffiliateProgram, error) {
	return nil, nil
}
func (*recordingSellerAffiliateService) CreateLink(context.Context, string, string, string, models.AffiliateUTXOPayoutAddresses) (*models.AffiliateLink, error) {
	return nil, nil
}
func (*recordingSellerAffiliateService) GetLinkByToken(context.Context, string) (*models.AffiliateLink, error) {
	return nil, nil
}
func (*recordingSellerAffiliateService) CreateReferralSession(context.Context, string, time.Time) (*models.AffiliateReferralSession, error) {
	return nil, nil
}
func (s *recordingSellerAffiliateService) AttributeOrder(_ context.Context, facts models.AffiliateOrderFacts) (*models.AffiliateOrderResult, error) {
	s.facts = append(s.facts, facts)
	s.attribution = &models.AffiliateAttribution{
		ID: "attribution-1", OrderID: facts.OrderID, ReferralSessionID: facts.ReferralSessionID,
		SellerPeerID: facts.SellerPeerID, BuyerPeerID: facts.BuyerPeerID,
	}
	return &models.AffiliateOrderResult{Attribution: *s.attribution}, nil
}
func (s *recordingSellerAffiliateService) PrepareOrderAttribution(_ context.Context, facts models.AffiliateOrderFacts) (*models.AffiliateOrderResult, error) {
	s.facts = append(s.facts, facts)
	return &models.AffiliateOrderResult{
		Attribution: models.AffiliateAttribution{
			ID: "attribution-prepared", OrderID: facts.OrderID, ReferralSessionID: facts.ReferralSessionID,
			SellerPeerID: facts.SellerPeerID, BuyerPeerID: facts.BuyerPeerID,
		},
		Lines: []models.AffiliateCommissionLine{{
			AttributionID: "attribution-prepared", OrderID: facts.OrderID, OrderLineID: facts.Lines[0].OrderLineID,
		}},
	}, nil
}
func (s *recordingSellerAffiliateService) RecordPreparedOrderTx(_ database.Tx, result *models.AffiliateOrderResult) (*models.AffiliateOrderResult, error) {
	s.recorded = true
	s.attribution = &result.Attribution
	return result, nil
}
func (*recordingSellerAffiliateService) TransitionCommissionTx(database.Tx, string, models.AffiliateCommissionStatus, models.AffiliateCommissionReversalReason, time.Time) ([]models.AffiliateCommissionLine, error) {
	return nil, nil
}
func (s *recordingSellerAffiliateService) TransitionCommission(_ context.Context, _ string, status models.AffiliateCommissionStatus, reason models.AffiliateCommissionReversalReason, _ time.Time) ([]models.AffiliateCommissionLine, error) {
	s.status = status
	s.reason = reason
	return nil, nil
}
func (s *recordingSellerAffiliateService) GetAttributionByOrder(context.Context, string) (*models.AffiliateAttribution, error) {
	if s.attribution == nil {
		return nil, models.ErrSellerAffiliateNotFound
	}
	return s.attribution, nil
}
func (s *recordingSellerAffiliateService) ListCommissionLinesByOrder(context.Context, string) ([]models.AffiliateCommissionLine, error) {
	return nil, nil
}

func TestSellerAffiliateSettlementPayout_UsesFrozenSettlementAssetAndAddress(t *testing.T) {
	affiliate := &recordingSellerAffiliateService{payout: &models.AffiliateSettlementPayout{
		Address: "0x1111111111111111111111111111111111111111", Amount: "132",
	}}
	service := newTestOrderAppService(t, OrderAppServiceConfig{SellerAffiliate: affiliate})
	payout, err := service.sellerAffiliateSettlementPayout(context.Background(), "affiliate-settlement-order", iwallet.CoinType("USDT"))
	require.NoError(t, err)
	require.NotNil(t, payout)
	assert.Equal(t, "0x1111111111111111111111111111111111111111", payout.Address)
	assert.Equal(t, "132", payout.Amount)
}

func (*recordingSellerAffiliateService) ListSellerStatement(context.Context) ([]models.AffiliateStatementLine, error) {
	return nil, nil
}
func (*recordingSellerAffiliateService) ListPromoterStatement(context.Context, string) ([]models.AffiliateStatementLine, error) {
	return nil, nil
}
func (*recordingSellerAffiliateService) ListPendingCommissionOrderIDs(context.Context) ([]string, error) {
	return nil, nil
}
func (s *recordingSellerAffiliateService) SettlementPayout(context.Context, string, string) (*models.AffiliateSettlementPayout, error) {
	return s.payout, nil
}
func (s *recordingSellerAffiliateService) HasSettlementTerms(context.Context, string) (bool, error) {
	return s.termsPresent, nil
}

func TestIncomingOrderOpen_RecordsAffiliateSnapshotWithSellerOrderTransaction(t *testing.T) {
	affiliate := new(recordingSellerAffiliateService)
	service := newTestOrderAppService(t, OrderAppServiceConfig{SellerAffiliate: affiliate})
	sellerPeerID := orderAffiliateTestPeerID(t)
	buyerPeerID := orderAffiliateTestPeerID(t)
	listing := &pb.SignedListing{Listing: &pb.Listing{
		VendorID: &pb.ID{PeerID: sellerPeerID},
		Metadata: &pb.Listing_Metadata{
			ContractType: pb.Listing_Metadata_DIGITAL_GOOD, Format: pb.Listing_Metadata_FIXED_PRICE,
			PricingCurrency: &pb.Currency{Code: "BTC", Divisibility: 8},
		},
		Item: &pb.Listing_Item{Price: "100000"},
	}}
	listingHash, err := orderutils.HashListing(listing)
	require.NoError(t, err)
	orderOpen := &pb.OrderOpen{
		BuyerID: &pb.ID{PeerID: buyerPeerID}, Listings: []*pb.SignedListing{listing},
		Items:       []*pb.OrderOpen_Item{{ListingHash: listingHash.B58String(), Quantity: "1"}},
		PricingCoin: "BTC", AffiliateReferralSessionID: "referral-prepared",
	}
	orderMsg := testutil.MustWrapOrderMessage(orderOpen)
	orderMsg.OrderID = "affiliate-order-prepared"

	ppCtx, err := service.preProcessOrderOpen(t.Context(), orderMsg)
	require.NoError(t, err)
	require.NotNil(t, ppCtx)
	require.True(t, ppCtx.AffiliateAttributionValidated)
	require.NotNil(t, ppCtx.AffiliateOrderResult)
	require.Len(t, affiliate.facts, 1)
	assert.Equal(t, buyerPeerID, affiliate.facts[0].BuyerPeerID)

	order := &models.Order{ID: models.OrderID(orderMsg.OrderID), MyRole: string(models.RoleVendor), Open: true}
	require.NoError(t, order.PutMessage(orderMsg))
	require.NoError(t, service.db.Update(func(tx database.Tx) error {
		if err := tx.Save(order); err != nil {
			return err
		}
		return service.postProcessOrderOpenInTx(tx, orderMsg, ppCtx)
	}))
	assert.True(t, affiliate.recorded)
	require.NotNil(t, affiliate.attribution)
	assert.Equal(t, orderMsg.OrderID, affiliate.attribution.OrderID)

	// Simulate an order accepted by an older binary: the invariant repair path
	// recreates the missing snapshot without putting attribution logic in a rail
	// finalizer.
	affiliate.attribution = nil
	affiliate.recorded = false
	require.NoError(t, service.EnsureSellerAffiliateOrderSnapshot(t.Context(), order.ID))
	assert.True(t, affiliate.recorded)
	require.NotNil(t, affiliate.attribution)

	order.SetFSMState(models.OrderState_CANCELED)
	require.NoError(t, service.db.Update(func(tx database.Tx) error { return tx.Save(order) }))
	require.NoError(t, service.ReconcileSellerAffiliateOrder(t.Context(), order.ID))
	assert.Equal(t, models.AffiliateCommissionStatusReversed, affiliate.status)
	assert.Equal(t, models.AffiliateReversalOrderInvalid, affiliate.reason)
}

func TestReconcileSellerAffiliateOrder_DerivesPendingCommissionFromSignedOrder(t *testing.T) {
	affiliate := new(recordingSellerAffiliateService)
	service := newTestOrderAppService(t, OrderAppServiceConfig{SellerAffiliate: affiliate})
	orderID := models.OrderID("affiliate-order-1")
	sellerPeerID := orderAffiliateTestPeerID(t)
	buyerPeerID := orderAffiliateTestPeerID(t)
	listing := &pb.SignedListing{Listing: &pb.Listing{
		VendorID: &pb.ID{PeerID: sellerPeerID},
		Metadata: &pb.Listing_Metadata{
			ContractType:    pb.Listing_Metadata_DIGITAL_GOOD,
			Format:          pb.Listing_Metadata_FIXED_PRICE,
			PricingCurrency: &pb.Currency{Code: "USD", Divisibility: 2},
		},
		Item: &pb.Listing_Item{Price: "1001"},
	}}
	listingHash, err := orderutils.HashListing(listing)
	require.NoError(t, err)
	orderOpen := &pb.OrderOpen{
		BuyerID:                    &pb.ID{PeerID: buyerPeerID},
		Listings:                   []*pb.SignedListing{listing},
		Items:                      []*pb.OrderOpen_Item{{ListingHash: listingHash.B58String(), Quantity: "1"}},
		PricingCoin:                "USD",
		AffiliateReferralSessionID: "referral-session-1",
		AppliedDiscounts: []*pb.OrderOpen_AppliedDiscount{{
			ValueType: "fixed", Amount: "-1",
		}},
	}
	order := &models.Order{ID: orderID, MyRole: string(models.RoleVendor), Open: true}
	order.SetFSMState(models.OrderState_AWAITING_SHIPMENT)
	require.NoError(t, order.PutMessage(testutil.MustWrapOrderMessage(orderOpen)))
	require.NoError(t, service.db.Update(func(tx database.Tx) error { return tx.Save(order) }))
	require.ErrorIs(t, service.PrepareSellerAffiliateSettlement(context.Background(), orderID), ErrSellerAffiliateSettlementNotReady)
	require.Empty(t, affiliate.facts)

	order.MarkPaymentVerified()
	require.NoError(t, service.db.Update(func(tx database.Tx) error { return tx.Save(order) }))
	require.NoError(t, service.PrepareSellerAffiliateSettlement(context.Background(), orderID))
	require.Len(t, affiliate.facts, 1)
	require.Len(t, affiliate.facts[0].Lines, 1)
	assert.Equal(t, "1000", affiliate.facts[0].Lines[0].NetMerchandiseAtomic)
	assert.Equal(t, sellerPeerID, affiliate.facts[0].SellerPeerID)
	assert.Equal(t, buyerPeerID, affiliate.facts[0].BuyerPeerID)
	assert.Empty(t, affiliate.status)

	order.SetFSMState(models.OrderState_REFUNDED)
	require.NoError(t, service.db.Update(func(tx database.Tx) error { return tx.Save(order) }))
	require.NoError(t, service.ReconcileSellerAffiliateOrder(context.Background(), orderID))
	assert.Equal(t, models.AffiliateCommissionStatusReversed, affiliate.status)
	assert.Equal(t, models.AffiliateReversalRefund, affiliate.reason)
}

func orderAffiliateTestPeerID(t *testing.T) string {
	t.Helper()
	peerID, _, err := identity.GeneratePeerID()
	require.NoError(t, err)
	return peerID.String()
}
