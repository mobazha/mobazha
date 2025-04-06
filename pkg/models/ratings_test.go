package models

import (
	"testing"

	"github.com/ipfs/go-cid"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
)

func TestRatingIndex_AddRating(t *testing.T) {
	ri := RatingIndex{}

	slug := "asdf"
	id, err := cid.Decode("QmcUDmZK8PsPYWw5FRHKNZFjszm2K6e68BQSTpnJYUsML7")
	if err != nil {
		t.Fatal(err)
	}
	err = ri.AddRating(&pb.Rating{
		Overall: 5,
		VendorSig: &pb.RatingSignature{
			Slug: slug,
		},
	}, id)
	if err != nil {
		t.Fatal(err)
	}

	exists := false
	for _, lm := range ri {
		if lm.Slug == slug {
			exists = true
			break
		}
	}
	if !exists {
		t.Error("Failed to add rating info to index")
	}

	id2, err := cid.Decode("QmYvc9UpxAvEqabMkKzFzbnJvW8KquzfMByapUyCBnoMRx")
	if err != nil {
		t.Fatal(err)
	}
	err = ri.AddRating(&pb.Rating{
		Overall: 4,
		VendorSig: &pb.RatingSignature{
			Slug: slug,
		},
	}, id2)
	if err != nil {
		t.Fatal(err)
	}

	var rating RatingInfo
	for _, r := range ri {
		if r.Slug == slug {
			rating = r
		}
	}

	if rating.Slug != slug {
		t.Errorf("Expected slug %s got %s", slug, rating.Slug)
	}
	if rating.Average != 4.5 {
		t.Errorf("Expected average %f got %f", 4.5, rating.Average)
	}
	if rating.Count != 2 {
		t.Errorf("Expected count %d got %d", 2, rating.Count)
	}
	if len(rating.Ratings) != 2 {
		t.Errorf("Expected len ratings %d got %d", 2, len(rating.Ratings))
	}
	if rating.Ratings[0] != id.String() {
		t.Errorf("Expected id %s, got %s", id.String(), rating.Ratings[0])
	}
	if rating.Ratings[1] != id2.String() {
		t.Errorf("Expected id %s, got %s", id2.String(), rating.Ratings[1])
	}
}
