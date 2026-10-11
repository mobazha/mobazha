// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package core

import (
	"context"
	"testing"
	"time"

	"github.com/mobazha/mobazha/internal/database"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testBuyerPeer  = "12D3KooWBuyerTestPeerAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	testSellerPeer = "12D3KooWSellerTestPeerBBBBBBBBBBBBBBBBBBBBBBBBBB"
)

func newBuyRequestService(t *testing.T) *BuyRequestAppService {
	t.Helper()
	db := newTestDB(t)
	require.NoError(t, database.MigrateBuyRequestModels(db))
	return NewBuyRequestAppService(database.NewGormBuyRequestStore(db))
}

func defaultCreateInput() contracts.CreateBuyRequestInput {
	return contracts.CreateBuyRequestInput{
		Title:              "Vintage Technics turntable",
		ProductType:        "electronics",
		Quantity:           2,
		BudgetAmount:       "450",
		BudgetCurrency:     "USDT",
		AcceptedCurrencies: []string{"USDT", "USD"},
		ShipTo:             "de",
		ExpiresAt:          time.Now().UTC().Add(72 * time.Hour),
	}
}

func defaultOfferInput() contracts.CreateBuyRequestOfferInput {
	return contracts.CreateBuyRequestOfferInput{
		UnitPriceAmount:   "199.5",
		UnitPriceCurrency: "USD",
		Quantity:          2,
		LeadTimeDays:      3,
		ShipsFrom:         "us",
		Note:              "Serviced, new belt and stylus.",
	}
}

func TestBuyRequestCreateValidatesAndNormalizes(t *testing.T) {
	svc := newBuyRequestService(t)
	ctx := context.Background()

	created, err := svc.Create(ctx, testBuyerPeer, defaultCreateInput())
	require.NoError(t, err)
	assert.Equal(t, models.BuyRequestStatusOpen, created.Status)
	assert.NotEmpty(t, created.Slug)
	assert.Equal(t, "DE", created.DeliverTo, "country codes are normalized to upper case")
	assert.Equal(t, ",USDT,USD,", created.AcceptedCurrencies)

	missingTitle := defaultCreateInput()
	missingTitle.Title = "   "
	_, err = svc.Create(ctx, testBuyerPeer, missingTitle)
	require.ErrorIs(t, err, database.ErrBuyRequestInvalid)

	badQuantity := defaultCreateInput()
	badQuantity.Quantity = 0
	_, err = svc.Create(ctx, testBuyerPeer, badQuantity)
	require.ErrorIs(t, err, database.ErrBuyRequestInvalid)

	pastDeadline := defaultCreateInput()
	pastDeadline.ExpiresAt = time.Now().UTC().Add(-time.Hour)
	_, err = svc.Create(ctx, testBuyerPeer, pastDeadline)
	require.ErrorIs(t, err, database.ErrBuyRequestInvalid)

	noCurrency := defaultCreateInput()
	noCurrency.AcceptedCurrencies = nil
	fromBudget, err := svc.Create(ctx, testBuyerPeer, noCurrency)
	require.NoError(t, err, "a buyer who states only a budget implicitly accepts the budget currency")
	assert.Equal(t, ",USDT,", fromBudget.AcceptedCurrencies)
}

func TestBuyRequestBoardListingAndCurrencyFilter(t *testing.T) {
	svc := newBuyRequestService(t)
	ctx := context.Background()

	usdRequest, err := svc.Create(ctx, testBuyerPeer, defaultCreateInput())
	require.NoError(t, err)

	usdtOnly := defaultCreateInput()
	usdtOnly.Title = "Broken soldering station"
	usdtOnly.AcceptedCurrencies = []string{"USDT"}
	_, err = svc.Create(ctx, testBuyerPeer, usdtOnly)
	require.NoError(t, err)

	board, total, err := svc.List(ctx, contracts.BuyRequestFilter{})
	require.NoError(t, err)
	assert.EqualValues(t, 2, total)
	assert.Len(t, board, 2)
	assert.Equal(t, string(models.BuyRequestStatusOpen), string(board[0].Status))

	// "USD" must not match a request that only accepts "USDT".
	usdOnly, total, err := svc.List(ctx, contracts.BuyRequestFilter{Currency: "USD"})
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	require.Len(t, usdOnly, 1)
	assert.Equal(t, usdRequest.Slug, usdOnly[0].Slug)

	mine, mineTotal, err := svc.ListMine(ctx, testBuyerPeer, 1, 20)
	require.NoError(t, err)
	assert.EqualValues(t, 2, mineTotal)
	assert.Len(t, mine, 2)

	otherBuyer, otherTotal, err := svc.ListMine(ctx, testSellerPeer, 1, 20)
	require.NoError(t, err)
	assert.EqualValues(t, 0, otherTotal)
	assert.Empty(t, otherBuyer)
}

func TestBuyRequestOfferRules(t *testing.T) {
	svc := newBuyRequestService(t)
	ctx := context.Background()

	request, err := svc.Create(ctx, testBuyerPeer, defaultCreateInput())
	require.NoError(t, err)

	_, err = svc.SubmitOffer(ctx, testBuyerPeer, request.Slug, defaultOfferInput())
	require.ErrorIs(t, err, database.ErrBuyRequestSelfOffer)

	rejectedCurrency := defaultOfferInput()
	rejectedCurrency.UnitPriceCurrency = "EUR"
	_, err = svc.SubmitOffer(ctx, testSellerPeer, request.Slug, rejectedCurrency)
	require.ErrorIs(t, err, database.ErrBuyRequestCurrencyRejected)

	zeroPrice := defaultOfferInput()
	zeroPrice.UnitPriceAmount = "0"
	_, err = svc.SubmitOffer(ctx, testSellerPeer, request.Slug, zeroPrice)
	require.ErrorIs(t, err, database.ErrBuyRequestInvalid)

	offer, err := svc.SubmitOffer(ctx, testSellerPeer, request.Slug, defaultOfferInput())
	require.NoError(t, err)
	assert.Equal(t, models.BuyRequestOfferSubmitted, offer.Status)
	assert.Equal(t, "US", offer.ShipsFrom)

	// Offers are binding: one shot per seller per request.
	_, err = svc.SubmitOffer(ctx, testSellerPeer, request.Slug, defaultOfferInput())
	require.ErrorIs(t, err, database.ErrBuyRequestDuplicateOffer)

	detail, offers, err := svc.Get(ctx, request.Slug)
	require.NoError(t, err)
	assert.Equal(t, 1, detail.OfferCount)
	require.Len(t, offers, 1)
	assert.Equal(t, offer.ID, offers[0].ID)
}

func TestBuyRequestAcceptLocksRequestAndRejectsOtherOffers(t *testing.T) {
	svc := newBuyRequestService(t)
	ctx := context.Background()

	request, err := svc.Create(ctx, testBuyerPeer, defaultCreateInput())
	require.NoError(t, err)

	winning, err := svc.SubmitOffer(ctx, testSellerPeer, request.Slug, defaultOfferInput())
	require.NoError(t, err)

	const secondSeller = "12D3KooWSecondSellerPeerCCCCCCCCCCCCCCCCCCCCCCCCC"
	losing, err := svc.SubmitOffer(ctx, secondSeller, request.Slug, defaultOfferInput())
	require.NoError(t, err)

	_, _, err = svc.AcceptOffer(ctx, testSellerPeer, request.Slug, winning.ID)
	require.ErrorIs(t, err, database.ErrBuyRequestNotOwner, "only the buyer may accept")

	locked, accepted, err := svc.AcceptOffer(ctx, testBuyerPeer, request.Slug, winning.ID)
	require.NoError(t, err)
	assert.Equal(t, models.BuyRequestStatusLocked, locked.Status)
	assert.Equal(t, winning.ID, locked.AcceptedOfferID)
	assert.Equal(t, models.BuyRequestOfferAccepted, accepted.Status)

	_, offers, err := svc.Get(ctx, request.Slug)
	require.NoError(t, err)
	statuses := map[string]models.BuyRequestOfferStatus{}
	for _, o := range offers {
		statuses[o.ID] = o.Status
	}
	assert.Equal(t, models.BuyRequestOfferAccepted, statuses[winning.ID])
	assert.Equal(t, models.BuyRequestOfferRejected, statuses[losing.ID])

	// A locked request stops taking offers.
	_, err = svc.SubmitOffer(ctx, "12D3KooWThirdSellerPeerDDDDDDDDDDDDDDDDDDDDDDDDDDDD", request.Slug, defaultOfferInput())
	require.ErrorIs(t, err, database.ErrBuyRequestClosed)

	// ...and cannot be accepted twice.
	_, _, err = svc.AcceptOffer(ctx, testBuyerPeer, request.Slug, winning.ID)
	require.ErrorIs(t, err, database.ErrBuyRequestClosed)
}

func TestBuyRequestExpiredRequestsLeaveTheBoard(t *testing.T) {
	svc := newBuyRequestService(t)
	ctx := context.Background()

	request, err := svc.Create(ctx, testBuyerPeer, defaultCreateInput())
	require.NoError(t, err)

	// Move the clock past the deadline: the lazy sweep closes it on read.
	svc.now = func() time.Time { return time.Now().UTC().Add(96 * time.Hour) }

	board, total, err := svc.List(ctx, contracts.BuyRequestFilter{})
	require.NoError(t, err)
	assert.EqualValues(t, 0, total, "an expired request leaves the board")
	assert.Empty(t, board, "the page and the total must agree after the sweep")

	// The buyer still sees it, closed rather than gone.
	detail, _, err := svc.Get(ctx, request.Slug)
	require.NoError(t, err)
	assert.Equal(t, models.BuyRequestStatusCancelled, detail.Status)

	_, err = svc.SubmitOffer(ctx, testSellerPeer, request.Slug, defaultOfferInput())
	require.ErrorIs(t, err, database.ErrBuyRequestExpired)

	mine, mineTotal, err := svc.ListMine(ctx, testBuyerPeer, 1, 20)
	require.NoError(t, err)
	assert.EqualValues(t, 1, mineTotal)
	require.Len(t, mine, 1)
	assert.Equal(t, models.BuyRequestStatusCancelled, mine[0].Status)
}

func TestBuyRequestCancelRequiresOwner(t *testing.T) {
	svc := newBuyRequestService(t)
	ctx := context.Background()

	request, err := svc.Create(ctx, testBuyerPeer, defaultCreateInput())
	require.NoError(t, err)

	_, err = svc.Cancel(ctx, testSellerPeer, request.Slug)
	require.ErrorIs(t, err, database.ErrBuyRequestNotOwner)

	cancelled, err := svc.Cancel(ctx, testBuyerPeer, request.Slug)
	require.NoError(t, err)
	assert.Equal(t, models.BuyRequestStatusCancelled, cancelled.Status)

	_, err = svc.Cancel(ctx, testBuyerPeer, request.Slug)
	require.ErrorIs(t, err, database.ErrBuyRequestClosed)
}
