package api

import (
	"testing"

	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcessJSONListingVariants_PrunesSingleValueOptions(t *testing.T) {
	g := &Gateway{}
	listing := &pb.Listing{Item: &pb.Listing_Item{}}

	g.processJSONListingVariants(listing, []JSONVariantInput{
		{
			Selections: map[string]string{
				"Color": "Black",
				"Size":  "M",
			},
			ProductID: "sku-m",
		},
		{
			Selections: map[string]string{
				"Color": "Black",
				"Size":  "L",
			},
			ProductID: "sku-l",
		},
	})

	require.Len(t, listing.Item.Options, 1)
	assert.Equal(t, "Size", listing.Item.Options[0].Name)
	require.Len(t, listing.Item.Options[0].Variants, 2)
	require.Len(t, listing.Item.Skus, 2)
	for _, sku := range listing.Item.Skus {
		require.Len(t, sku.Selections, 1)
		assert.Equal(t, "Size", sku.Selections[0].Option)
		assert.NotEqual(t, "Color", sku.Selections[0].Option)
	}
}

func TestProcessListingVariants_DropsDegenerateSingleOption(t *testing.T) {
	g := &Gateway{}
	listing := &pb.Listing{Item: &pb.Listing_Item{}}

	g.processListingVariants(listing, "Dad Hat", []VariantData{
		{
			ProductTitle: "Dad Hat",
			ProductID:    "sku-black",
			Selections: []Selection{
				{Option: "Color", Variant: "Black"},
			},
		},
	})

	assert.Empty(t, listing.Item.Options)
	require.Len(t, listing.Item.Skus, 1)
	assert.Empty(t, listing.Item.Skus[0].Selections)
}
