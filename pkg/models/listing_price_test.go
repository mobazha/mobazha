package models

import (
	"testing"

	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
)

func TestResolveListingPriceSnapshot_NoSkus_UsesBase(t *testing.T) {
	snap := ResolveListingPriceSnapshot(&pb.Listing_Item{Price: "5"})
	if snap.DisplayAmount != "5" || snap.BaseAmount != "5" || snap.MaxAmount != "5" {
		t.Fatalf("expected base-only snapshot, got %+v", snap)
	}
	if snap.HasRange || snap.UsesSkuPrice {
		t.Fatalf("expected no range/sku usage, got %+v", snap)
	}
}

func TestResolveListingPriceSnapshot_ExplicitSkuPrices_UseMinMax(t *testing.T) {
	item := &pb.Listing_Item{
		Price: "5",
		Skus: []*pb.Listing_Item_Sku{
			{Price: "7275"},
			{Price: "7725"},
			{Price: ""},
		},
	}
	snap := ResolveListingPriceSnapshot(item)
	if snap.DisplayAmount != "7275" || snap.BaseAmount != "5" || snap.MaxAmount != "7725" {
		t.Fatalf("unexpected snapshot: %+v", snap)
	}
	if !snap.HasRange || !snap.UsesSkuPrice {
		t.Fatalf("expected range + sku usage, got %+v", snap)
	}
}

func TestResolveListingPriceSnapshot_AllSkuPricesSame_NoRange(t *testing.T) {
	item := &pb.Listing_Item{
		Price: "100",
		Skus: []*pb.Listing_Item_Sku{
			{Price: "7275"},
			{Price: "7275"},
		},
	}
	snap := ResolveListingPriceSnapshot(item)
	if snap.DisplayAmount != "7275" || snap.MaxAmount != "7275" || snap.HasRange {
		t.Fatalf("expected flat sku price, got %+v", snap)
	}
}
