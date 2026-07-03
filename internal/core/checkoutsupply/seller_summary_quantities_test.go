package checkoutsupply

import (
	"testing"

	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"github.com/stretchr/testify/require"
)

func TestListingSkuOnHandQuantity_SumsTrackedSkus(t *testing.T) {
	sl := listingWithSkus("shirt",
		&pb.Listing_Item_Sku{Quantity: "2"},
		&pb.Listing_Item_Sku{Quantity: "5"},
		&pb.Listing_Item_Sku{Quantity: "-1"},
	)
	require.EqualValues(t, 7, listingSkuOnHandQuantity(sl))
}

func TestListingSkuOnHandQuantity_ReturnsNegativeWhenUntracked(t *testing.T) {
	sl := listingWithSkus("digital", &pb.Listing_Item_Sku{Quantity: "-1"})
	require.EqualValues(t, -1, listingSkuOnHandQuantity(sl))
}
