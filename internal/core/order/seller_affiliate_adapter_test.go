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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingSellerAffiliateService struct {
	attribution *models.AffiliateAttribution
	facts       []models.AffiliateOrderFacts
	status      models.AffiliateCommissionStatus
	reason      models.AffiliateCommissionReversalReason
}

func (*recordingSellerAffiliateService) PutProgram(context.Context, *models.AffiliateProgram) (*models.AffiliateProgram, error) {
	return nil, nil
}
func (*recordingSellerAffiliateService) GetProgram(context.Context) (*models.AffiliateProgram, error) {
	return nil, nil
}
func (*recordingSellerAffiliateService) CreateLink(context.Context, string, string) (*models.AffiliateLink, error) {
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
func (*recordingSellerAffiliateService) ListCommissionLinesByOrder(context.Context, string) ([]models.AffiliateCommissionLine, error) {
	return nil, nil
}
func (*recordingSellerAffiliateService) ListPendingCommissionOrderIDs(context.Context) ([]string, error) {
	return nil, nil
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
	order.MarkPaymentVerified()
	require.NoError(t, service.db.Update(func(tx database.Tx) error { return tx.Save(order) }))

	require.NoError(t, service.ReconcileSellerAffiliateOrder(context.Background(), orderID))
	require.Len(t, affiliate.facts, 1)
	require.Len(t, affiliate.facts[0].Lines, 1)
	assert.Equal(t, "1000", affiliate.facts[0].Lines[0].NetMerchandiseAtomic)
	assert.Equal(t, sellerPeerID, affiliate.facts[0].SellerPeerID)
	assert.Equal(t, buyerPeerID, affiliate.facts[0].BuyerPeerID)
	assert.Empty(t, affiliate.status)

	completedAt := time.Now().Add(-365 * 24 * time.Hour)
	order.SetFSMState(models.OrderState_COMPLETED)
	order.CompletedAt = &completedAt
	require.NoError(t, service.db.Update(func(tx database.Tx) error { return tx.Save(order) }))
	require.NoError(t, service.ReconcileSellerAffiliateOrder(context.Background(), orderID))
	assert.Equal(t, models.AffiliateCommissionStatusEarned, affiliate.status)

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
