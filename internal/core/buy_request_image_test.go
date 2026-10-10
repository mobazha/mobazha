package core

import (
	"context"
	"testing"

	"github.com/mobazha/mobazha/internal/database"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func imageSet(suffix string) models.ImageHashes {
	return models.ImageHashes{
		Tiny:     "QmTiny" + suffix,
		Small:    "QmSmall" + suffix,
		Medium:   "QmMedium" + suffix,
		Large:    "QmLarge" + suffix,
		Original: "QmOriginal" + suffix,
		Filename: suffix + ".jpg",
	}
}

func TestBuyRequestImagesRoundTrip(t *testing.T) {
	svc := newBuyRequestService(t)
	ctx := context.Background()

	in := defaultCreateInput()
	in.Images = []models.ImageHashes{imageSet("a"), imageSet("b")}

	created, err := svc.Create(ctx, testBuyerPeer, in)
	require.NoError(t, err)
	require.Len(t, created.Images, 2)
	assert.Equal(t, "QmOriginala", created.Images[0].Original)

	// Read back through the store: the hash sets survive the JSON column and are
	// hydrated for every caller.
	detail, _, err := svc.Get(ctx, created.Slug)
	require.NoError(t, err)
	require.Len(t, detail.Images, 2)
	assert.Equal(t, "QmOriginalb", detail.Images[1].Original)

	board, _, err := svc.List(ctx, contracts.BuyRequestFilter{})
	require.NoError(t, err)
	require.Len(t, board, 1)
	require.Len(t, board[0].Images, 2)
}

func TestBuyRequestImagesNormalization(t *testing.T) {
	svc := newBuyRequestService(t)
	ctx := context.Background()

	padded := defaultCreateInput()
	padded.Images = []models.ImageHashes{
		{}, // no hash at all: carries no information
		{Original: "  QmOriginala  ", Filename: " a.jpg "},
	}
	created, err := svc.Create(ctx, testBuyerPeer, padded)
	require.NoError(t, err)
	require.Len(t, created.Images, 1, "an entry with no hash is dropped, not rejected")
	assert.Equal(t, "QmOriginala", created.Images[0].Original, "hashes are trimmed")
	assert.Equal(t, "a.jpg", created.Images[0].Filename)

	tooMany := defaultCreateInput()
	tooMany.Title = "too many pictures"
	for i := 0; i <= models.MaxBuyRequestImages; i++ {
		tooMany.Images = append(tooMany.Images, imageSet(string(rune('a'+i))))
	}
	_, err = svc.Create(ctx, testBuyerPeer, tooMany)
	require.ErrorIs(t, err, database.ErrBuyRequestInvalid,
		"exceeding the cap is an error, not a silent truncation")

	// Exactly at the cap is fine — the bound is inclusive.
	atCap := defaultCreateInput()
	atCap.Title = "exactly at the cap"
	for i := 0; i < models.MaxBuyRequestImages; i++ {
		atCap.Images = append(atCap.Images, imageSet(string(rune('a'+i))))
	}
	full, err := svc.Create(ctx, testBuyerPeer, atCap)
	require.NoError(t, err)
	assert.Len(t, full.Images, models.MaxBuyRequestImages)
}

func TestBuyRequestWithoutImagesStaysEmpty(t *testing.T) {
	svc := newBuyRequestService(t)
	ctx := context.Background()

	created, err := svc.Create(ctx, testBuyerPeer, defaultCreateInput())
	require.NoError(t, err)
	assert.Empty(t, created.Images)
	assert.Equal(t, "[]", created.ImagesJSON)

	detail, _, err := svc.Get(ctx, created.Slug)
	require.NoError(t, err)
	assert.Empty(t, detail.Images)
}
