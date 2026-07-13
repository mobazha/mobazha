// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package core

import (
	"context"
	"testing"
	"time"

	coredatabase "github.com/mobazha/mobazha/internal/database"
	"github.com/mobazha/mobazha/internal/database/dbstore"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/identity"
	"github.com/mobazha/mobazha/pkg/models"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
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
	utxoPayoutAddresses := affiliateTestUTXOPayoutAddresses()
	payoutDestinations := affiliateTestPayoutDestinations(t, payoutAddress, utxoPayoutAddresses)
	link, err := service.CreateLink(ctx, promoterPeerID, "affiliate-token-promoter-a", payoutDestinations)
	require.NoError(t, err)
	assert.Equal(t, program.ID, link.ProgramID)
	replayedLink, err := service.CreateLink(ctx, promoterPeerID, "ignored-token-on-retry", payoutDestinations)
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

	reversedAt := facts.AttributedAt.Add(24 * time.Hour)
	reversed, err := service.TransitionCommission(ctx, facts.OrderID, models.AffiliateCommissionStatusReversed, models.AffiliateReversalRefund, reversedAt)
	require.NoError(t, err)
	assert.Equal(t, models.AffiliateCommissionStatusReversed, reversed[0].Status)
	assert.Equal(t, models.AffiliateReversalRefund, reversed[0].ReversalReason)

}

func TestSellerAffiliateAppService_GuestIdentityPersistsInCallerTransaction(t *testing.T) {
	base, err := dbstore.NewMemoryDB(t.TempDir())
	require.NoError(t, err)
	defer base.Close()
	require.NoError(t, coredatabase.MigrateSellerAffiliateModels(base))
	service := NewSellerAffiliateAppService(coredatabase.NewGormSellerAffiliateStore(base))
	ctx := context.Background()
	sellerPeerID, promoterPeerID := affiliateTestPeerID(t), affiliateTestPeerID(t)
	_, err = service.PutProgram(ctx, &models.AffiliateProgram{
		SellerPeerID: sellerPeerID, Status: models.AffiliateProgramStatusActive,
		CommissionRateBPS: 500, AttributionWindowSeconds: 3600,
	})
	require.NoError(t, err)
	link, err := service.CreateLink(ctx, promoterPeerID, "guest-link", affiliateTestPayoutDestinations(t, "0x1111111111111111111111111111111111111111", affiliateTestUTXOPayoutAddresses()))
	require.NoError(t, err)
	session, err := service.CreateReferralSession(ctx, link.PublicToken, time.Now().UTC())
	require.NoError(t, err)
	prepared, err := service.PrepareOrderAttribution(ctx, models.AffiliateOrderFacts{
		OrderID: "gst_order", SellerPeerID: sellerPeerID, BuyerKind: models.AffiliateBuyerKindGuest,
		GuestBuyerID: "anonymous-order-buyer", ReferralSessionID: session.ID, AttributedAt: time.Now().UTC(),
		Lines: []models.AffiliateOrderLineFact{{OrderLineID: "gst_order:0", NetMerchandiseAtomic: "10000", Currency: "BTC"}},
	})
	require.NoError(t, err)
	require.NoError(t, base.Update(func(tx database.Tx) error {
		_, err := service.RecordPreparedOrderTx(tx, prepared)
		return err
	}))
	attribution, err := service.GetAttributionByOrder(ctx, "gst_order")
	require.NoError(t, err)
	assert.Equal(t, models.AffiliateBuyerKindGuest, attribution.BuyerKind)
	assert.Empty(t, attribution.BuyerPeerID)
	assert.Equal(t, "anonymous-order-buyer", attribution.GuestBuyerID)
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
	link, err := service.CreateLink(ctx, promoterPeerID, "affiliate-token-self-check", affiliateTestPayoutDestinations(t, "0x1111111111111111111111111111111111111111", affiliateTestUTXOPayoutAddresses()))
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
	settlementCoin := "crypto:eip155:1:erc20:0xdAC17F958D2ee523a2206206994597C13D831ec7"
	utxoPayoutAddresses := affiliateTestUTXOPayoutAddresses()
	payoutDestinations := affiliateTestPayoutDestinations(t, payoutAddress, utxoPayoutAddresses)
	link, err := service.CreateLink(ctx, promoterPeerID, "affiliate-token-frozen-payout", payoutDestinations)
	require.NoError(t, err)
	assert.Equal(t, payoutAddress, link.PromoterPayoutAddress)
	assert.Equal(t, utxoPayoutAddresses, link.PromoterUTXOPayoutAddresses)
	assert.True(t, payoutDestinations.Equal(link.PromoterPayoutDestinations))

	issuedAt := time.Now().UTC().Add(-time.Minute)
	session, err := service.CreateReferralSession(ctx, link.PublicToken, issuedAt)
	require.NoError(t, err)
	assert.Equal(t, uint32(1250), session.CommissionRateBPSSnapshot)
	assert.Equal(t, payoutAddress, session.PromoterPayoutAddress)
	assert.Equal(t, utxoPayoutAddresses, session.PromoterUTXOPayoutAddresses)
	assert.True(t, payoutDestinations.Equal(session.PromoterPayoutDestinations))

	program.CommissionRateBPS = 5000
	_, err = service.PutProgram(ctx, program)
	require.NoError(t, err)
	result, err := service.AttributeOrder(ctx, models.AffiliateOrderFacts{
		OrderID: "order-frozen-payout", SellerPeerID: sellerPeerID, BuyerPeerID: buyerPeerID,
		ReferralSessionID: session.ID, AttributedAt: issuedAt.Add(time.Minute),
		Lines: []models.AffiliateOrderLineFact{{OrderLineID: "order-frozen-payout:0", NetMerchandiseAtomic: "1000", Currency: settlementCoin}},
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, uint32(1250), result.Attribution.CommissionRateBPSSnapshot)
	assert.Equal(t, payoutAddress, result.Attribution.PromoterPayoutAddress)
	assert.Equal(t, utxoPayoutAddresses, result.Attribution.PromoterUTXOPayoutAddresses)
	assert.True(t, payoutDestinations.Equal(result.Attribution.PromoterPayoutDestinations))
	assert.Equal(t, "125", result.Lines[0].CommissionAtomic)
	payout, err := service.SettlementPayout(ctx, "order-frozen-payout", settlementCoin)
	require.NoError(t, err)
	require.NotNil(t, payout)
	assert.Equal(t, payoutAddress, payout.Address)
	assert.Equal(t, "125", payout.Amount)
	attemptTerm, err := service.SettlementAttemptTerm(ctx, "order-frozen-payout", settlementCoin)
	require.NoError(t, err)
	require.NotNil(t, attemptTerm)
	assert.Equal(t, session.ID, attemptTerm.ReferralSessionID)
	assert.Equal(t, buyerPeerID, attemptTerm.BuyerPeerID)
	assert.Equal(t, payoutAddress, attemptTerm.Address)
	assert.Equal(t, "125", attemptTerm.Amount)
	assert.Equal(t, "1000", attemptTerm.SellerGrossBasis)
	require.Len(t, attemptTerm.Lines, 1)
	hasTerms, err := service.HasSettlementTerms(ctx, "order-frozen-payout")
	require.NoError(t, err)
	assert.True(t, hasTerms)
	hasTerms, err = service.HasSettlementTerms(ctx, "order-without-affiliate")
	require.NoError(t, err)
	assert.False(t, hasTerms)
	zeroResult, err := service.AttributeOrder(ctx, models.AffiliateOrderFacts{
		OrderID: "order-zero-commission", SellerPeerID: sellerPeerID, BuyerPeerID: buyerPeerID,
		ReferralSessionID: session.ID, AttributedAt: issuedAt.Add(2 * time.Minute),
		Lines: []models.AffiliateOrderLineFact{{OrderLineID: "order-zero-commission:0", NetMerchandiseAtomic: "1", Currency: settlementCoin}},
	})
	require.NoError(t, err)
	require.NotNil(t, zeroResult)
	assert.Equal(t, "0", zeroResult.Lines[0].CommissionAtomic)
	zeroPayout, err := service.SettlementPayout(ctx, "order-zero-commission", settlementCoin)
	require.NoError(t, err)
	assert.Nil(t, zeroPayout)
	zeroAttemptTerm, err := service.SettlementAttemptTerm(ctx, "order-zero-commission", settlementCoin)
	require.NoError(t, err)
	require.NotNil(t, zeroAttemptTerm)
	assert.Equal(t, "0", zeroAttemptTerm.Amount)
	assert.Equal(t, payoutAddress, zeroAttemptTerm.Address)
	hasTerms, err = service.HasSettlementTerms(ctx, "order-zero-commission")
	require.NoError(t, err)
	assert.True(t, hasTerms)
	_, err = service.SettlementPayout(ctx, "order-frozen-payout", "ETH")
	require.ErrorIs(t, err, models.ErrInvalidSellerAffiliate)

	_, err = service.CreateLink(ctx, promoterPeerID, "ignored-token", affiliateTestPayoutDestinations(t, "0x2222222222222222222222222222222222222222", utxoPayoutAddresses))
	require.ErrorIs(t, err, models.ErrSellerAffiliateConflict)
}

func TestSellerAffiliateAppService_RejectsCryptoOrderOutsideFrozenPayoutRails(t *testing.T) {
	base, err := dbstore.NewMemoryDB(t.TempDir())
	require.NoError(t, err)
	defer base.Close()
	require.NoError(t, coredatabase.MigrateSellerAffiliateModels(base))

	service := NewSellerAffiliateAppService(coredatabase.NewGormSellerAffiliateStore(base))
	ctx := context.Background()
	sellerPeerID, promoterPeerID, buyerPeerID := affiliateTestPeerID(t), affiliateTestPeerID(t), affiliateTestPeerID(t)
	_, err = service.PutProgram(ctx, &models.AffiliateProgram{
		SellerPeerID: sellerPeerID, Status: models.AffiliateProgramStatusActive,
		CommissionRateBPS: 1000, AttributionWindowSeconds: 3600,
	})
	require.NoError(t, err)
	ethereumRail, ok := iwallet.CanonicalNativeCoinType(iwallet.ChainEthereum)
	require.True(t, ok)
	link, err := service.CreateLink(ctx, promoterPeerID, "affiliate-token-eth-only", models.PayoutDestinationSet{
		Destinations: []models.PayoutDestination{{
			RailID: ethereumRail.String(), Address: "0x1111111111111111111111111111111111111111", Version: 1,
		}},
	})
	require.NoError(t, err)
	issuedAt := time.Now().UTC().Add(-time.Minute)
	session, err := service.CreateReferralSession(ctx, link.PublicToken, issuedAt)
	require.NoError(t, err)

	_, err = service.PrepareOrderAttribution(ctx, models.AffiliateOrderFacts{
		OrderID: "order-sol-with-eth-referral", SellerPeerID: sellerPeerID, BuyerPeerID: buyerPeerID,
		ReferralSessionID: session.ID, AttributedAt: issuedAt.Add(time.Second),
		Lines: []models.AffiliateOrderLineFact{{
			OrderLineID: "order-sol-with-eth-referral:0", NetMerchandiseAtomic: "1000", Currency: "SOL",
		}},
	})
	require.ErrorIs(t, err, models.ErrInvalidSellerAffiliate)

	result, err := service.PrepareOrderAttribution(ctx, models.AffiliateOrderFacts{
		OrderID: "order-eth-with-eth-referral", SellerPeerID: sellerPeerID, BuyerPeerID: buyerPeerID,
		ReferralSessionID: session.ID, AttributedAt: issuedAt.Add(2 * time.Second),
		Lines: []models.AffiliateOrderLineFact{{
			OrderLineID: "order-eth-with-eth-referral:0", NetMerchandiseAtomic: "1000", Currency: "ETH",
		}},
	})
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestSellerAffiliateAppService_ReissueLinkRotatesFutureSessionsOnly(t *testing.T) {
	base, err := dbstore.NewMemoryDB(t.TempDir())
	require.NoError(t, err)
	defer base.Close()
	require.NoError(t, coredatabase.MigrateSellerAffiliateModels(base))

	service := NewSellerAffiliateAppService(coredatabase.NewGormSellerAffiliateStore(base))
	ctx := context.Background()
	_, err = service.PutProgram(ctx, &models.AffiliateProgram{
		SellerPeerID: affiliateTestPeerID(t), Status: models.AffiliateProgramStatusActive,
		CommissionRateBPS: 1250, AttributionWindowSeconds: 3600,
	})
	require.NoError(t, err)
	oldEVM := "0x1111111111111111111111111111111111111111"
	oldUTXO := affiliateTestUTXOPayoutAddresses()
	oldDestinations := affiliateTestPayoutDestinations(t, oldEVM, oldUTXO)
	link, err := service.CreateLink(ctx, affiliateTestPeerID(t), "affiliate-token-before-rotation", oldDestinations)
	require.NoError(t, err)
	oldSession, err := service.CreateReferralSession(ctx, link.PublicToken, time.Now().UTC())
	require.NoError(t, err)

	newEVM := "0x2222222222222222222222222222222222222222"
	newUTXO := oldUTXO.Clone()
	newUTXO[models.AffiliatePayoutRailBitcoin] = "bc1qrotatedaffiliatepayoutdestination000000000"
	newDestinations := affiliateTestPayoutDestinations(t, newEVM, newUTXO)
	reissued, err := service.ReissueLink(ctx, link.ID, "affiliate-token-after-rotation", newDestinations)
	require.NoError(t, err)
	assert.Equal(t, link.ID, reissued.ID)
	assert.Equal(t, newEVM, reissued.PromoterPayoutAddress)
	assert.Equal(t, newUTXO, reissued.PromoterUTXOPayoutAddresses)
	assert.True(t, newDestinations.Equal(reissued.PromoterPayoutDestinations))

	_, err = service.GetLinkByToken(ctx, "affiliate-token-before-rotation")
	require.Error(t, err)
	newSession, err := service.CreateReferralSession(ctx, reissued.PublicToken, time.Now().UTC())
	require.NoError(t, err)
	assert.Equal(t, oldEVM, oldSession.PromoterPayoutAddress)
	assert.Equal(t, oldUTXO, oldSession.PromoterUTXOPayoutAddresses)
	assert.True(t, oldDestinations.Equal(oldSession.PromoterPayoutDestinations))
	assert.Equal(t, newEVM, newSession.PromoterPayoutAddress)
	assert.Equal(t, newUTXO, newSession.PromoterUTXOPayoutAddresses)
	assert.True(t, newDestinations.Equal(newSession.PromoterPayoutDestinations))

	links, err := service.ListLinks(ctx)
	require.NoError(t, err)
	require.Len(t, links, 1)
	assert.Equal(t, link.ID, links[0].ID)
	revoked, err := service.RevokeLink(ctx, link.ID)
	require.NoError(t, err)
	assert.Equal(t, models.AffiliateLinkStatusRevoked, revoked.Status)
	_, err = service.CreateReferralSession(ctx, revoked.PublicToken, time.Now().UTC())
	require.ErrorIs(t, err, models.ErrInvalidSellerAffiliate)
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

func TestAffiliatePayoutAddressForSettlementCoin_UsesFrozenNativeRail(t *testing.T) {
	destinations := affiliateTestPayoutDestinations(t, "0x1111111111111111111111111111111111111111", affiliateTestUTXOPayoutAddresses())
	attribution := &models.AffiliateAttribution{
		PromoterPayoutAddress:       "0x1111111111111111111111111111111111111111",
		PromoterUTXOPayoutAddresses: affiliateTestUTXOPayoutAddresses(),
		PromoterPayoutDestinations:  destinations,
	}
	tests := []struct {
		name  string
		chain iwallet.ChainType
		rail  string
	}{
		{name: "bitcoin", chain: iwallet.ChainBitcoin, rail: models.AffiliatePayoutRailBitcoin},
		{name: "bitcoin cash", chain: iwallet.ChainBitcoinCash, rail: models.AffiliatePayoutRailBitcoinCash},
		{name: "litecoin", chain: iwallet.ChainLitecoin, rail: models.AffiliatePayoutRailLitecoin},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			coin, err := iwallet.RequireCanonicalNativeCoinType(tt.chain)
			require.NoError(t, err)
			address, err := affiliatePayoutAddressForSettlementCoin(attribution, coin.String())
			require.NoError(t, err)
			expected, ok := attribution.PromoterUTXOPayoutAddresses.AddressForRail(tt.rail)
			require.True(t, ok)
			assert.Equal(t, expected, address)
		})
	}
}

func TestAffiliatePayoutAddressForSettlementCoin_UsesFrozenSolanaRail(t *testing.T) {
	destinations := affiliateTestPayoutDestinations(t, "0x1111111111111111111111111111111111111111", affiliateTestUTXOPayoutAddresses())
	solanaRail, err := iwallet.RequireCanonicalNativeCoinType(iwallet.ChainSolana)
	require.NoError(t, err)
	expected, ok := destinations.DestinationForRail(solanaRail.String())
	require.True(t, ok)
	address, err := affiliatePayoutAddressForSettlementCoin(&models.AffiliateAttribution{PromoterPayoutDestinations: destinations}, solanaRail.String())
	require.NoError(t, err)
	assert.Equal(t, expected.Address, address)
}

func TestAffiliatePayoutAddressForSettlementCoin_GenericSnapshotMissingRailFailsClosed(t *testing.T) {
	destinations := affiliateTestPayoutDestinations(t, "0x1111111111111111111111111111111111111111", affiliateTestUTXOPayoutAddresses())
	bscRail, err := iwallet.RequireCanonicalNativeCoinType(iwallet.ChainBSC)
	require.NoError(t, err)
	filtered := destinations.Destinations[:0]
	for _, destination := range destinations.Destinations {
		if destination.RailID != bscRail.String() {
			filtered = append(filtered, destination)
		}
	}
	destinations.Destinations = filtered

	_, err = affiliatePayoutAddressForSettlementCoin(&models.AffiliateAttribution{
		PromoterPayoutAddress:      "0x1111111111111111111111111111111111111111",
		PromoterPayoutDestinations: destinations,
	}, bscRail.String())
	require.ErrorIs(t, err, models.ErrInvalidSellerAffiliate)
}

func TestAffiliatePayoutAddressForSettlementCoin_LegacySnapshotStillFallsBack(t *testing.T) {
	bscRail, err := iwallet.RequireCanonicalNativeCoinType(iwallet.ChainBSC)
	require.NoError(t, err)
	const legacyAddress = "0x1111111111111111111111111111111111111111"

	address, err := affiliatePayoutAddressForSettlementCoin(&models.AffiliateAttribution{
		PromoterPayoutAddress: legacyAddress,
	}, bscRail.String())
	require.NoError(t, err)
	assert.Equal(t, legacyAddress, address)
}

func TestSameAffiliateSettlementCoin_NormalizesLegacyNativeTicker(t *testing.T) {
	btc, err := iwallet.RequireCanonicalNativeCoinType(iwallet.ChainBitcoin)
	require.NoError(t, err)
	assert.True(t, sameAffiliateSettlementCoin("ETH", "crypto:eip155:11155111:native"))
	assert.True(t, sameAffiliateSettlementCoin("BTC", btc.String()))
	assert.False(t, sameAffiliateSettlementCoin("ETH", "BTC"))
}

type staticAffiliateSettlementActionReader struct {
	actions []models.SettlementActionSnapshot
}

func (r staticAffiliateSettlementActionReader) ListSettlementActions(_ context.Context, _ []string) ([]models.SettlementActionSnapshot, error) {
	return r.actions, nil
}

func TestSellerAffiliateStatement_ProjectsVerifiedSettlementOutputs(t *testing.T) {
	baseLine := models.AffiliateStatementLine{
		Attribution: models.AffiliateAttribution{
			OrderID:               "affiliate-order",
			PromoterPayoutAddress: "0x1111111111111111111111111111111111111111",
		},
		CommissionLine: models.AffiliateCommissionLine{CommissionAtomic: "125", Status: models.AffiliateCommissionStatusPending},
	}
	planned := models.SettlementPayoutLine{
		Type: "affiliate", Amount: "125", Address: "0x1111111111111111111111111111111111111111", Coin: "ETH",
	}
	now := time.Now().UTC()
	tests := []struct {
		name   string
		action models.SettlementActionSnapshot
		want   string
	}{
		{
			name: "planned", action: models.SettlementActionSnapshot{
				OrderID: "affiliate-order", ActionID: "act-planned", Action: "confirm", State: "submitting", SettlementCoin: "ETH", PlannedLines: []models.SettlementPayoutLine{planned}, UpdatedAt: now,
			}, want: "planned",
		},
		{
			name: "submitted", action: models.SettlementActionSnapshot{
				OrderID: "affiliate-order", ActionID: "act-submitted", Action: "confirm", State: "submitted", TxHash: "0xsubmitted", SettlementCoin: "ETH", PlannedLines: []models.SettlementPayoutLine{planned}, UpdatedAt: now,
			}, want: "submitted",
		},
		{
			name: "reorged returns to submitted", action: models.SettlementActionSnapshot{
				OrderID: "affiliate-order", ActionID: "act-reorged", Action: "guest_affiliate_transfer", State: "reorged", TxHash: "0xreorged", SettlementCoin: "ETH", PlannedLines: []models.SettlementPayoutLine{planned}, UpdatedAt: now,
			}, want: "submitted",
		},
		{
			name: "confirmed only after matching observed output", action: models.SettlementActionSnapshot{
				OrderID: "affiliate-order", ActionID: "act-confirmed", Action: "complete", State: "confirmed", TxHash: "0xconfirmed", SettlementCoin: "ETH", PlannedLines: []models.SettlementPayoutLine{planned},
				ObservedLines: []models.SettlementPayoutLine{{Type: "affiliate", Amount: "125", Address: "0x1111111111111111111111111111111111111111", TxHash: "0xconfirmed"}}, UpdatedAt: now,
			}, want: "confirmed",
		},
		{
			name: "confirmed without chain output is not paid", action: models.SettlementActionSnapshot{
				OrderID: "affiliate-order", ActionID: "act-unverified", Action: "complete", State: "confirmed", SettlementCoin: "ETH", PlannedLines: []models.SettlementPayoutLine{planned}, UpdatedAt: now,
			}, want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewSellerAffiliateAppService(nil, staticAffiliateSettlementActionReader{actions: []models.SettlementActionSnapshot{tt.action}})
			statement, err := service.projectStatementSettlement(context.Background(), []models.AffiliateStatementLine{baseLine})
			require.NoError(t, err)
			require.Len(t, statement, 1)
			if tt.want == "" {
				assert.Nil(t, statement[0].Settlement)
				return
			}
			require.NotNil(t, statement[0].Settlement)
			assert.Equal(t, tt.want, statement[0].Settlement.State)
			assert.Equal(t, "125", statement[0].Settlement.Amount)
		})
	}
}

func affiliateTestPeerID(t *testing.T) string {
	t.Helper()
	peerID, _, err := identity.GeneratePeerID()
	require.NoError(t, err)
	return peerID.String()
}

func affiliateTestUTXOPayoutAddresses() models.AffiliateUTXOPayoutAddresses {
	return models.AffiliateUTXOPayoutAddresses{
		models.AffiliatePayoutRailBitcoin:     "bc1qpromoter",
		models.AffiliatePayoutRailBitcoinCash: "bitcoincash:qpromoter",
		models.AffiliatePayoutRailLitecoin:    "ltc1qpromoter",
	}
}

func affiliateTestPayoutDestinations(t *testing.T, evm string, utxo models.AffiliateUTXOPayoutAddresses) models.PayoutDestinationSet {
	t.Helper()
	entries := []struct {
		chain   iwallet.ChainType
		address string
	}{
		{iwallet.ChainBitcoin, utxo[models.AffiliatePayoutRailBitcoin]},
		{iwallet.ChainBitcoinCash, utxo[models.AffiliatePayoutRailBitcoinCash]},
		{iwallet.ChainLitecoin, utxo[models.AffiliatePayoutRailLitecoin]},
		{iwallet.ChainEthereum, evm},
		{iwallet.ChainBSC, evm},
		{iwallet.ChainPolygon, evm},
		{iwallet.ChainBase, evm},
		{iwallet.ChainSolana, "4Nd1mYjLrS1gN33Zf3Yc6wNwVozN5K6oNUwG3XhYhXyd"},
	}
	set := models.PayoutDestinationSet{Destinations: make([]models.PayoutDestination, 0, len(entries))}
	for _, entry := range entries {
		railID, err := iwallet.RequireCanonicalNativeCoinType(entry.chain)
		require.NoError(t, err)
		set.Destinations = append(set.Destinations, models.PayoutDestination{RailID: railID.String(), Address: entry.address, Version: 1})
	}
	return set
}
