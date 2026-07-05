package orders

import (
	"testing"

	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"github.com/stretchr/testify/require"
)

func TestCalculateSignedListingHash(t *testing.T) {
	listing := &pb.SignedListing{
		Listing:   &pb.Listing{Slug: "deal-listing"},
		Signature: []byte("signed"),
	}

	first, err := CalculateSignedListingHash(listing)
	require.NoError(t, err)
	require.NotEmpty(t, first)

	second, err := CalculateSignedListingHash(listing)
	require.NoError(t, err)
	require.Equal(t, first, second)

	_, err = CalculateSignedListingHash(nil)
	require.Error(t, err)
}
