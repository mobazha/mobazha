// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package core

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/mobazha/mobazha/internal/database"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// publishTestRequest puts one open request on a node's board.
func publishTestRequest(t *testing.T, node *MobazhaNode, title string, currencies ...string) *contracts.BuyRequestDTO {
	t.Helper()
	if len(currencies) == 0 {
		currencies = []string{"USDT"}
	}
	request, err := node.BuyRequests().Create(context.Background(), node.Identity().String(),
		contracts.CreateBuyRequestInput{
			Title:              title,
			Quantity:           1,
			BudgetAmount:       "100",
			BudgetCurrency:     currencies[0],
			AcceptedCurrencies: currencies,
			ProductType:        "stamps",
			ShipTo:             "US",
			ExpiresAt:          time.Now().Add(24 * time.Hour),
		})
	require.NoError(t, err)
	return contracts.NewBuyRequestDTO(request)
}

func decodeJSON(t *testing.T, payload []byte, into any) {
	t.Helper()
	require.NoError(t, json.Unmarshal(payload, into))
}

// TestBuyRequestRelay_OfferTravelsToTheOwningNode walks the cross-node flow a
// wanted board needs: a seller that does not own the request reads it, offers on
// it, and is told what the buyer decided. The request is only ever written on
// the node that owns it.
func TestBuyRequestRelay_OfferTravelsToTheOwningNode(t *testing.T) {
	mocknet, err := NewMocknet(2)
	require.NoError(t, err)
	defer mocknet.TearDown()

	buyer := mocknet.Nodes()[0]
	seller := mocknet.Nodes()[1]
	request := publishTestRequest(t, buyer, "A rare stamp")

	// The seller's node holds no copy of the request; it asks the owner.
	payload, err := seller.BuyRequestRelay().AskRequest(context.Background(), buyer.Identity().String(), request.Slug)
	require.NoError(t, err)
	var detail contracts.BuyRequestDetailDTO
	decodeJSON(t, payload, &detail)
	require.NotNil(t, detail.Request)
	assert.Equal(t, request.Slug, detail.Request.Slug)
	assert.Equal(t, request.Title, detail.Request.Title)
	assert.Empty(t, detail.Offers, "a fresh request has no offers yet")

	// Asking a node that does not own the slug is refused, and the refusal
	// keeps its meaning on this side.
	_, err = seller.BuyRequestRelay().AskRequest(context.Background(), mocknet.Nodes()[0].Identity().String(), "no-such-slug")
	require.Error(t, err)
	assert.True(t, errors.Is(err, database.ErrBuyRequestNotFound), "expected a not-found refusal, got %v", err)

	// The seller offers. Its identity on the owner's board is the peer it is
	// authenticated as, not anything the body claims.
	body, err := json.Marshal(contracts.CreateBuyRequestOfferRequest{
		UnitPrice:    &contracts.BuyRequestAmountDTO{Amount: "80", Currency: "USDT"},
		Quantity:     1,
		LeadTimeDays: 3,
		ShipsFrom:    "DE",
		Note:         "Sealed, from my own collection.",
	})
	require.NoError(t, err)
	offerPayload, err := seller.BuyRequestRelay().RelayOffer(context.Background(), buyer.Identity().String(), request.Slug, body)
	require.NoError(t, err)
	var offer contracts.BuyRequestOfferDTO
	decodeJSON(t, offerPayload, &offer)
	assert.Equal(t, "submitted", offer.Status)
	assert.Equal(t, seller.Identity().String(), offer.SellerID.PeerID)

	// The owner can read it back with its own service, which is the point of
	// relaying instead of mirroring: there is one copy of the offer.
	_, offers, err := buyer.BuyRequests().Get(context.Background(), request.Slug)
	require.NoError(t, err)
	require.Len(t, offers, 1)
	assert.Equal(t, offer.ID, offers[0].ID)
	assert.Equal(t, seller.Identity().String(), offers[0].SellerPeerID)

	// A seller that offers in a currency the buyer did not accept is refused
	// with the rule it broke, not with a transport error.
	rejected, err := json.Marshal(contracts.CreateBuyRequestOfferRequest{
		UnitPrice: &contracts.BuyRequestAmountDTO{Amount: "5", Currency: "USD"},
		Quantity:  1,
	})
	require.NoError(t, err)
	_, err = seller.BuyRequestRelay().RelayOffer(context.Background(), buyer.Identity().String(), request.Slug, rejected)
	require.Error(t, err)
	assert.True(t, errors.Is(err, database.ErrBuyRequestCurrencyRejected), "expected a currency refusal, got %v", err)

	// The seller's node is told the moment the buyer decides.
	decided, err := seller.SubscribeEvent(&events.BuyRequestOfferDecided{})
	require.NoError(t, err)
	_, accepted, err := buyer.BuyRequests().AcceptOffer(context.Background(), buyer.Identity().String(), request.Slug, offer.ID)
	require.NoError(t, err)
	require.Equal(t, offer.ID, accepted.ID)

	select {
	case raw := <-decided.Out():
		decision, ok := raw.(*events.BuyRequestOfferDecided)
		require.True(t, ok, "unexpected event %T", raw)
		assert.True(t, decision.Accepted)
		assert.Equal(t, offer.ID, decision.OfferID)
		assert.Equal(t, request.Slug, decision.Slug)
		assert.Equal(t, request.Title, decision.Title)
		assert.Equal(t, buyer.Identity().String(), decision.OwnerPeerID)
	case <-time.After(30 * time.Second):
		t.Fatal("the seller was never told that its offer won")
	}
}

// TestBuyRequestRelay_CommentTravelsToTheOwningNode covers the second thing a
// seller does on somebody else's request: ask a question in its thread.
func TestBuyRequestRelay_CommentTravelsToTheOwningNode(t *testing.T) {
	mocknet, err := NewMocknet(2)
	require.NoError(t, err)
	defer mocknet.TearDown()

	buyer := mocknet.Nodes()[0]
	seller := mocknet.Nodes()[1]
	request := publishTestRequest(t, buyer, "A question about a watch")

	threadPayload, err := seller.BuyRequestRelay().AskComments(context.Background(), buyer.Identity().String(), request.Slug)
	require.NoError(t, err)
	var thread contracts.BuyRequestCommentThreadDTO
	decodeJSON(t, threadPayload, &thread)
	assert.Empty(t, thread.Comments)

	body, err := json.Marshal(contracts.CreateBuyRequestCommentRequest{Body: "Is the box included?"})
	require.NoError(t, err)
	commentPayload, err := seller.BuyRequestRelay().RelayComment(context.Background(), buyer.Identity().String(), request.Slug, body)
	require.NoError(t, err)
	var comment contracts.BuyRequestCommentDTO
	decodeJSON(t, commentPayload, &comment)
	assert.Equal(t, "Is the box included?", comment.Body)
	assert.Equal(t, seller.Identity().String(), comment.AuthorID.PeerID)

	threadPayload, err = seller.BuyRequestRelay().AskComments(context.Background(), buyer.Identity().String(), request.Slug)
	require.NoError(t, err)
	decodeJSON(t, threadPayload, &thread)
	require.Len(t, thread.Comments, 1)
	assert.Equal(t, comment.ID, thread.Comments[0].ID)
}

// TestBuyRequestRelay_RefusalCodesRoundTrip keeps the two nodes agreeing on
// every reason an operation can be refused, in both directions.
func TestBuyRequestRelay_RefusalCodesRoundTrip(t *testing.T) {
	sentinels := map[string]error{
		contracts.BuyRequestCodeNotFound:         database.ErrBuyRequestNotFound,
		contracts.BuyRequestCodeInvalid:          database.ErrBuyRequestInvalid,
		contracts.BuyRequestCodeClosed:           database.ErrBuyRequestClosed,
		contracts.BuyRequestCodeExpired:          database.ErrBuyRequestExpired,
		contracts.BuyRequestCodeSelfOffer:        database.ErrBuyRequestSelfOffer,
		contracts.BuyRequestCodeDuplicateOffer:   database.ErrBuyRequestDuplicateOffer,
		contracts.BuyRequestCodeCurrencyRejected: database.ErrBuyRequestCurrencyRejected,
		contracts.BuyRequestCodeNotOwner:         database.ErrBuyRequestNotOwner,
		contracts.BuyRequestCodeLimitReached:     database.ErrBuyRequestLimitReached,
		contracts.BuyRequestCodeSlugTaken:        database.ErrBuyRequestSlugTaken,
		contracts.BuyRequestCodeReplyToReply:     database.ErrBuyRequestReplyToReply,
		contracts.BuyRequestCodeUnreachable:      database.ErrBuyRequestOwnerUnreachable,
		contracts.BuyRequestCodeInternal:         database.ErrBuyRequestInternal,
	}
	for code, sentinel := range sentinels {
		assert.Equal(t, code, buyRequestErrorCode(sentinel), "code for %v", sentinel)
		assert.True(t, errors.Is(buyRequestErrorFromCode(code, "because"), sentinel),
			"round trip of %s", code)
	}
	// An unknown code still fails loudly instead of looking like a refusal.
	err := buyRequestErrorFromCode("something-new", "")
	assert.True(t, errors.Is(err, database.ErrBuyRequestInternal), "unknown code: %v", err)
}
