package models

import (
	"strings"
	"testing"

	"github.com/ipfs/go-cid"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
)

func TestListingIndex_UpdateListing(t *testing.T) {
	li := ListingIndex{}

	slug := "asdf"
	li.UpdateListing(ListingMetadata{
		Slug:  slug,
		Title: "abc",
	})

	exists := false
	for _, lm := range li {
		if lm.Slug == slug {
			exists = true
			break
		}
	}
	if !exists {
		t.Error("Failed to add listing metadata to index")
	}

	newTitle := "123"
	li.UpdateListing(ListingMetadata{
		Slug:  slug,
		Title: newTitle,
	})

	exists = false
	for _, lm := range li {
		if lm.Slug == slug {
			if lm.Title != newTitle {
				t.Error("Title failed to update")
			}
			exists = true
			break
		}
	}
	if !exists {
		t.Error("Failed to add listing metadata to index")
	}

}

func TestListingIndex_GetListingSlug(t *testing.T) {
	li := ListingIndex{}

	slug := "asdf"
	li.UpdateListing(ListingMetadata{
		Slug: slug,
		CID:  "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
	})

	c, err := cid.Decode("QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub")
	if err != nil {
		t.Fatal(err)
	}
	ret, err := li.GetListingSlug(c)
	if err != nil {
		t.Fatal(err)
	}

	if ret != slug {
		t.Errorf("Returned incorrect slug. Expected %s, got %s", slug, ret)
	}
}

func TestListingIndex_GetListingCID(t *testing.T) {
	li := ListingIndex{}

	slug := "asdf"
	li.UpdateListing(ListingMetadata{
		Slug: slug,
		CID:  "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
	})

	c, err := cid.Decode("QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub")
	if err != nil {
		t.Fatal(err)
	}
	ret, err := li.GetListingCID(slug)
	if err != nil {
		t.Fatal(err)
	}

	if ret != c {
		t.Errorf("Returned incorrect slug. Expected %s, got %s", slug, ret)
	}
}

func TestListingIndex_Count(t *testing.T) {
	li := ListingIndex{}

	slug := "asdf"
	li.UpdateListing(ListingMetadata{
		Slug: slug,
		CID:  "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
	})

	ret := li.Count()
	if ret != 1 {
		t.Errorf("Returned incorrect count. Expected %d, got %d", 1, ret)
	}

}

func TestListingIndex_DeleteListing(t *testing.T) {
	li := ListingIndex{}

	slug := "asdf"
	li.UpdateListing(ListingMetadata{
		Slug: slug,
		CID:  "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
	})

	li.DeleteListing(slug)

	ret := li.Count()
	if ret != 0 {
		t.Errorf("Returned incorrect count. Expected %d, got %d", 0, ret)
	}
}

func TestNewListingMetadataFromListing(t *testing.T) {
	c, err := cid.Decode("QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub")
	if err != nil {
		t.Fatal(err)
	}

	listing := &pb.Listing{
		Slug: "abc",
		Item: &pb.Listing_Item{
			Title: strings.Repeat("s", ShortDescriptionLength+1),
			Price: "1000",
			Images: []*pb.Image{
				{
					Tiny:   "aaa",
					Small:  "bbb",
					Medium: "ccc",
				},
			},
		},
		Metadata: &pb.Listing_Metadata{
			PricingCurrency: &pb.Currency{
				Code: "BTC",
			},
		},
		ShippingProfile: &pb.ShippingProfile{
			ProfileID: "test-profile",
			Name:      "Test Shipping",
			LocationGroups: []*pb.LocationGroup{
				{
					Id: "lg-1",
					Zones: []*pb.ShippingZone{
						{
							Id:      "zone-al",
							Name:    "AL Shipping",
							Regions: []string{"AL"},
							Rates: []*pb.ShippingRate{
								{Id: "rate-1", Name: "asdf", Price: "0", Currency: "USD"},
							},
						},
					},
				},
			},
		},
	}

	ret, err := NewListingMetadataFromListing(listing, c)
	if err != nil {
		t.Fatal(err)
	}

	if len(ret.ShipsTo) != 1 {
		t.Errorf("Returned incorrect shipping regions. Expected %d, got %d", 1, len(ret.ShipsTo))
	}

	if len(ret.FreeShipping) != 1 {
		t.Errorf("Returned incorrect shipping regions. Expected %d, got %d", 1, len(ret.FreeShipping))
	}
}

func TestNewListingMetadataFromListing_NoImages(t *testing.T) {
	c, err := cid.Decode("QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub")
	if err != nil {
		t.Fatal(err)
	}
	listing := &pb.Listing{
		Slug: "image-pending-draft",
		Item: &pb.Listing_Item{
			Title: "Image Pending Draft",
			Price: "4500",
		},
		Metadata: &pb.Listing_Metadata{
			PricingCurrency: &pb.Currency{Code: "USD", Divisibility: 2},
		},
		Status: ListingStatusDraft,
	}

	metadata, err := NewListingMetadataFromListing(listing, c)
	if err != nil {
		t.Fatalf("build metadata for image-less draft: %v", err)
	}
	if metadata.Thumbnail != (ListingThumbnail{}) {
		t.Fatalf("image-less draft thumbnail = %#v, want empty", metadata.Thumbnail)
	}
}
