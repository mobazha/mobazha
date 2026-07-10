package core

import (
	"context"
	"testing"
	"time"

	coredatabase "github.com/mobazha/mobazha/internal/database"
	"github.com/mobazha/mobazha/internal/database/dbstore"
	"github.com/mobazha/mobazha/pkg/identity"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSellerAffiliateAppService_AutomatesMinimalCommissionLifecycle(t *testing.T) {
	base, err := dbstore.NewMemoryDB(t.TempDir())
	require.NoError(t, err)
	defer base.Close()
	require.NoError(t, coredatabase.MigrateSellerAffiliateModels(base))

	service := NewSellerAffiliateAppService(coredatabase.NewGormSellerAffiliateStore(base))
	ctx := context.Background()
	sellerPeerID := affiliateTestPeerID(t)
	promoterPeerID := affiliateTestPeerID(t)
	buyerPeerID := affiliateTestPeerID(t)
	program, err := service.PutProgram(ctx, &models.AffiliateProgram{
		SellerPeerID: sellerPeerID, Status: models.AffiliateProgramStatusActive,
		CommissionRateBPS: 1250, AttributionWindowSeconds: 3600,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, program.ID)

	payoutAddress := "0x1111111111111111111111111111111111111111"
	link, err := service.CreateLink(ctx, promoterPeerID, "affiliate-token-promoter-a", payoutAddress)
	require.NoError(t, err)
	assert.Equal(t, program.ID, link.ProgramID)
	replayedLink, err := service.CreateLink(ctx, promoterPeerID, "ignored-token-on-retry", payoutAddress)
	require.NoError(t, err)
	assert.Equal(t, link, replayedLink)

	issuedAt := time.Now().UTC().Add(-time.Minute)
	session, err := service.CreateReferralSession(ctx, link.PublicToken, issuedAt)
	require.NoError(t, err)
	assert.Equal(t, issuedAt.Add(time.Hour), session.ExpiresAt)

	facts := models.AffiliateOrderFacts{
		OrderID: "order-1", SellerPeerID: sellerPeerID, BuyerPeerID: buyerPeerID,
		ReferralSessionID: session.ID, AttributedAt: issuedAt.Add(time.Minute),
		Lines: []models.AffiliateOrderLineFact{
			{OrderLineID: "order-1:0", NetMerchandiseAtomic: "1001", Currency: "USD"},
			{OrderLineID: "order-1:1", NetMerchandiseAtomic: "7", Currency: "USD"},
		},
	}
	result, err := service.AttributeOrder(ctx, facts)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Lines, 2)
	assert.Equal(t, "125", result.Lines[0].CommissionAtomic)
	assert.Equal(t, "0", result.Lines[1].CommissionAtomic)
	assert.Equal(t, models.AffiliateCommissionStatusPending, result.Lines[0].Status)
	sellerStatement, err := service.ListSellerStatement(ctx)
	require.NoError(t, err)
	require.Len(t, sellerStatement, 2)
	assert.Equal(t, facts.OrderID, sellerStatement[0].Attribution.OrderID)
	promoterStatement, err := service.ListPromoterStatement(ctx, promoterPeerID)
	require.NoError(t, err)
	require.Len(t, promoterStatement, 2)
	otherStatement, err := service.ListPromoterStatement(ctx, affiliateTestPeerID(t))
	require.NoError(t, err)
	assert.Empty(t, otherStatement)
	pendingOrderIDs, err := service.ListPendingCommissionOrderIDs(ctx)
	require.NoError(t, err)
	assert.Equal(t, []string{facts.OrderID}, pendingOrderIDs)

	replayFacts := facts
	replayFacts.AttributedAt = facts.AttributedAt.Add(time.Minute)
	replay, err := service.AttributeOrder(ctx, replayFacts)
	require.NoError(t, err)
	assert.Equal(t, result.Attribution.ID, replay.Attribution.ID)

	earnedAt := facts.AttributedAt.Add(24 * time.Hour)
	earned, err := service.TransitionCommission(ctx, facts.OrderID, models.AffiliateCommissionStatusEarned, "", earnedAt)
	require.NoError(t, err)
	require.Len(t, earned, 2)
	assert.Equal(t, models.AffiliateCommissionStatusEarned, earned[0].Status)
	pendingOrderIDs, err = service.ListPendingCommissionOrderIDs(ctx)
	require.NoError(t, err)
	assert.Empty(t, pendingOrderIDs)

	earnedReplay, err := service.TransitionCommission(ctx, facts.OrderID, models.AffiliateCommissionStatusEarned, "", earnedAt.Add(time.Hour))
	require.NoError(t, err)
	assert.Equal(t, earned[0].UpdatedAt, earnedReplay[0].UpdatedAt)

	reversedAt := earnedAt.Add(time.Hour)
	reversed, err := service.TransitionCommission(ctx, facts.OrderID, models.AffiliateCommissionStatusReversed, models.AffiliateReversalRefund, reversedAt)
	require.NoError(t, err)
	assert.Equal(t, models.AffiliateCommissionStatusReversed, reversed[0].Status)
	assert.Equal(t, models.AffiliateReversalRefund, reversed[0].ReversalReason)

	_, err = service.TransitionCommission(ctx, facts.OrderID, models.AffiliateCommissionStatusEarned, "", reversedAt.Add(time.Hour))
	require.ErrorIs(t, err, coredatabase.ErrSellerAffiliateConflict)
}

func TestSellerAffiliateAppService_RejectsDeterministicSelfAttribution(t *testing.T) {
	base, err := dbstore.NewMemoryDB(t.TempDir())
	require.NoError(t, err)
	defer base.Close()
	require.NoError(t, coredatabase.MigrateSellerAffiliateModels(base))

	service := NewSellerAffiliateAppService(coredatabase.NewGormSellerAffiliateStore(base))
	ctx := context.Background()
	sellerPeerID := affiliateTestPeerID(t)
	promoterPeerID := affiliateTestPeerID(t)
	program, err := service.PutProgram(ctx, &models.AffiliateProgram{
		SellerPeerID: sellerPeerID, Status: models.AffiliateProgramStatusActive,
		CommissionRateBPS: 1000, AttributionWindowSeconds: 3600,
	})
	require.NoError(t, err)
	link, err := service.CreateLink(ctx, promoterPeerID, "affiliate-token-self-check", "0x1111111111111111111111111111111111111111")
	require.NoError(t, err)
	issuedAt := time.Now().UTC().Add(-time.Minute)
	session, err := service.CreateReferralSession(ctx, link.PublicToken, issuedAt)
	require.NoError(t, err)

	result, err := service.AttributeOrder(ctx, models.AffiliateOrderFacts{
		OrderID: "order-self", SellerPeerID: program.SellerPeerID, BuyerPeerID: " " + promoterPeerID + " ",
		ReferralSessionID: session.ID, AttributedAt: issuedAt.Add(time.Minute),
		Lines: []models.AffiliateOrderLineFact{{OrderLineID: "order-self:0", NetMerchandiseAtomic: "1000", Currency: "USD"}},
	})
	require.NoError(t, err)
	assert.Nil(t, result)
	_, err = service.GetAttributionByOrder(ctx, "order-self")
	require.ErrorIs(t, err, coredatabase.ErrSellerAffiliateNotFound)
}

func TestSellerAffiliateAppService_FreezesPayoutDestinationAndRateAtReferralIssue(t *testing.T) {
	base, err := dbstore.NewMemoryDB(t.TempDir())
	require.NoError(t, err)
	defer base.Close()
	require.NoError(t, coredatabase.MigrateSellerAffiliateModels(base))

	service := NewSellerAffiliateAppService(coredatabase.NewGormSellerAffiliateStore(base))
	ctx := context.Background()
	sellerPeerID := affiliateTestPeerID(t)
	promoterPeerID := affiliateTestPeerID(t)
	buyerPeerID := affiliateTestPeerID(t)
	program, err := service.PutProgram(ctx, &models.AffiliateProgram{
		SellerPeerID: sellerPeerID, Status: models.AffiliateProgramStatusActive,
		CommissionRateBPS: 1250, AttributionWindowSeconds: 3600,
	})
	require.NoError(t, err)
	payoutAddress := "0x1111111111111111111111111111111111111111"
	link, err := service.CreateLink(ctx, promoterPeerID, "affiliate-token-frozen-payout", payoutAddress)
	require.NoError(t, err)
	assert.Equal(t, payoutAddress, link.PromoterPayoutAddress)

	issuedAt := time.Now().UTC().Add(-time.Minute)
	session, err := service.CreateReferralSession(ctx, link.PublicToken, issuedAt)
	require.NoError(t, err)
	assert.Equal(t, uint32(1250), session.CommissionRateBPSSnapshot)
	assert.Equal(t, payoutAddress, session.PromoterPayoutAddress)

	program.CommissionRateBPS = 5000
	_, err = service.PutProgram(ctx, program)
	require.NoError(t, err)
	result, err := service.AttributeOrder(ctx, models.AffiliateOrderFacts{
		OrderID: "order-frozen-payout", SellerPeerID: sellerPeerID, BuyerPeerID: buyerPeerID,
		ReferralSessionID: session.ID, AttributedAt: issuedAt.Add(time.Minute),
		Lines: []models.AffiliateOrderLineFact{{OrderLineID: "order-frozen-payout:0", NetMerchandiseAtomic: "1000", Currency: "USDT"}},
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, uint32(1250), result.Attribution.CommissionRateBPSSnapshot)
	assert.Equal(t, payoutAddress, result.Attribution.PromoterPayoutAddress)
	assert.Equal(t, "125", result.Lines[0].CommissionAtomic)
	payout, err := service.SettlementPayout(ctx, "order-frozen-payout", "USDT")
	require.NoError(t, err)
	require.NotNil(t, payout)
	assert.Equal(t, payoutAddress, payout.Address)
	assert.Equal(t, "125", payout.Amount)
	_, err = service.SettlementPayout(ctx, "order-frozen-payout", "ETH")
	require.ErrorIs(t, err, models.ErrInvalidSellerAffiliate)

	_, err = service.CreateLink(ctx, promoterPeerID, "ignored-token", "0x2222222222222222222222222222222222222222")
	require.ErrorIs(t, err, models.ErrSellerAffiliateConflict)
}

func TestGormSellerAffiliateStore_IsTenantScoped(t *testing.T) {
	base, err := dbstore.NewMemoryDB(t.TempDir())
	require.NoError(t, err)
	defer base.Close()
	require.NoError(t, coredatabase.MigrateSellerAffiliateModels(base))

	tenantDB, ok := base.(*dbstore.TenantDB)
	require.True(t, ok)
	other, err := tenantDB.ForTenant("tenant-b")
	require.NoError(t, err)
	serviceA := NewSellerAffiliateAppService(coredatabase.NewGormSellerAffiliateStore(base))
	serviceB := NewSellerAffiliateAppService(coredatabase.NewGormSellerAffiliateStore(other))
	_, err = serviceA.PutProgram(context.Background(), &models.AffiliateProgram{
		SellerPeerID: affiliateTestPeerID(t), Status: models.AffiliateProgramStatusActive,
		CommissionRateBPS: 1000, AttributionWindowSeconds: 3600,
	})
	require.NoError(t, err)
	_, err = serviceB.GetProgram(context.Background())
	require.ErrorIs(t, err, coredatabase.ErrSellerAffiliateNotFound)
}

func affiliateTestPeerID(t *testing.T) string {
	t.Helper()
	peerID, _, err := identity.GeneratePeerID()
	require.NoError(t, err)
	return peerID.String()
}
