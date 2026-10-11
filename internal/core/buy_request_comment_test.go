// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package core

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mobazha/mobazha/internal/database"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func commentInput(body string) contracts.CreateBuyRequestCommentInput {
	return contracts.CreateBuyRequestCommentInput{Body: body}
}

func TestBuyRequestCommentThreading(t *testing.T) {
	svc := newBuyRequestService(t)
	ctx := context.Background()

	request, err := svc.Create(ctx, testBuyerPeer, defaultCreateInput())
	require.NoError(t, err)

	top, err := svc.CreateComment(ctx, testSellerPeer, request.Slug, commentInput("Is the platter original?"))
	require.NoError(t, err)
	assert.Equal(t, "", top.ParentCommentID, "a comment without a parent starts a thread")
	assert.Equal(t, testSellerPeer, top.AuthorPeerID)
	assert.Equal(t, request.Slug, top.RequestSlug)

	reply, err := svc.CreateComment(ctx, testBuyerPeer, request.Slug, contracts.CreateBuyRequestCommentInput{
		Body:            "Yes, original platter and dust cover.",
		ParentCommentID: top.ID,
	})
	require.NoError(t, err)
	assert.Equal(t, top.ID, reply.ParentCommentID)

	// Threads are one level deep on purpose.
	_, err = svc.CreateComment(ctx, testSellerPeer, request.Slug, contracts.CreateBuyRequestCommentInput{
		Body:            "And the tonearm?",
		ParentCommentID: reply.ID,
	})
	require.ErrorIs(t, err, database.ErrBuyRequestReplyToReply)

	comments, err := svc.ListComments(ctx, request.Slug)
	require.NoError(t, err)
	require.Len(t, comments, 2)
	assert.Equal(t, top.ID, comments[0].ID, "threads read oldest first")
	assert.Equal(t, reply.ID, comments[1].ID)
}

func TestBuyRequestCommentValidation(t *testing.T) {
	svc := newBuyRequestService(t)
	ctx := context.Background()

	request, err := svc.Create(ctx, testBuyerPeer, defaultCreateInput())
	require.NoError(t, err)

	_, err = svc.CreateComment(ctx, testSellerPeer, request.Slug, commentInput("   "))
	require.ErrorIs(t, err, database.ErrBuyRequestInvalid)

	tooLong := strings.Repeat("x", models.MaxBuyRequestCommentLength+1)
	_, err = svc.CreateComment(ctx, testSellerPeer, request.Slug, commentInput(tooLong))
	require.ErrorIs(t, err, database.ErrBuyRequestInvalid)

	// Exactly at the limit is fine — the bound is inclusive.
	atLimit := strings.Repeat("x", models.MaxBuyRequestCommentLength)
	_, err = svc.CreateComment(ctx, testSellerPeer, request.Slug, commentInput(atLimit))
	require.NoError(t, err)

	_, err = svc.CreateComment(ctx, "  ", request.Slug, commentInput("hello"))
	require.ErrorIs(t, err, database.ErrBuyRequestInvalid)

	_, err = svc.CreateComment(ctx, testSellerPeer, "no-such-request", commentInput("hello"))
	require.ErrorIs(t, err, database.ErrBuyRequestNotFound)

	_, err = svc.ListComments(ctx, "no-such-request")
	require.ErrorIs(t, err, database.ErrBuyRequestNotFound)
}

func TestBuyRequestCommentCannotCrossRequests(t *testing.T) {
	svc := newBuyRequestService(t)
	ctx := context.Background()

	first, err := svc.Create(ctx, testBuyerPeer, defaultCreateInput())
	require.NoError(t, err)

	secondInput := defaultCreateInput()
	secondInput.Title = "Another request"
	second, err := svc.Create(ctx, testBuyerPeer, secondInput)
	require.NoError(t, err)

	otherThread, err := svc.CreateComment(ctx, testSellerPeer, second.Slug, commentInput("on the other request"))
	require.NoError(t, err)

	_, err = svc.CreateComment(ctx, testBuyerPeer, first.Slug, contracts.CreateBuyRequestCommentInput{
		Body:            "wrong thread",
		ParentCommentID: otherThread.ID,
	})
	require.ErrorIs(t, err, database.ErrBuyRequestCommentNotFound)
}

func TestBuyRequestCommentClosedRequestIsReadOnly(t *testing.T) {
	svc := newBuyRequestService(t)
	ctx := context.Background()

	request, err := svc.Create(ctx, testBuyerPeer, defaultCreateInput())
	require.NoError(t, err)

	early, err := svc.CreateComment(ctx, testSellerPeer, request.Slug, commentInput("asking early"))
	require.NoError(t, err)

	_, err = svc.Cancel(ctx, testBuyerPeer, request.Slug)
	require.NoError(t, err)

	_, err = svc.CreateComment(ctx, testSellerPeer, request.Slug, commentInput("too late"))
	require.ErrorIs(t, err, database.ErrBuyRequestClosed)

	// The existing thread must stay readable after the request closes.
	comments, err := svc.ListComments(ctx, request.Slug)
	require.NoError(t, err)
	require.Len(t, comments, 1)
	assert.Equal(t, early.ID, comments[0].ID)
}

func TestBuyRequestCommentAllowedWhileLocked(t *testing.T) {
	svc := newBuyRequestService(t)
	ctx := context.Background()

	request, err := svc.Create(ctx, testBuyerPeer, defaultCreateInput())
	require.NoError(t, err)

	offer, err := svc.SubmitOffer(ctx, testSellerPeer, request.Slug, defaultOfferInput())
	require.NoError(t, err)

	_, _, err = svc.AcceptOffer(ctx, testBuyerPeer, request.Slug, offer.ID)
	require.NoError(t, err)

	// A locked request is mid-deal, so the two parties can still talk.
	_, err = svc.CreateComment(ctx, testSellerPeer, request.Slug, commentInput("shipping tomorrow"))
	require.NoError(t, err)

	// Even past the original deadline: closing the thread on a deal that is
	// already in flight would strand the two parties without a channel.
	svc.now = func() time.Time { return time.Now().UTC().Add(96 * time.Hour) }
	_, err = svc.CreateComment(ctx, testBuyerPeer, request.Slug, commentInput("still coordinating"))
	require.NoError(t, err)
}

func TestBuyRequestCommentExpiryClosesTheThread(t *testing.T) {
	svc := newBuyRequestService(t)
	ctx := context.Background()

	request, err := svc.Create(ctx, testBuyerPeer, defaultCreateInput())
	require.NoError(t, err)

	svc.now = func() time.Time { return time.Now().UTC().Add(96 * time.Hour) }

	// An expired request reports expiry, not the generic "closed" state, even
	// before the lazy sweep has flipped its status.
	_, err = svc.CreateComment(ctx, testSellerPeer, request.Slug, commentInput("after expiry"))
	require.ErrorIs(t, err, database.ErrBuyRequestExpired)
}
