package models

import (
	"errors"

	"github.com/ipfs/go-cid"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
)

// RatingIndex is a list of RatingInfo objects. It is saved
// in the public data directory.
type RatingIndex []RatingInfo

// AddRating adds a new rating to the index. If the cid already exists, it
// will not be added to the index.
func (ri *RatingIndex) AddRating(rating *pb.Rating, cid cid.Cid) error {
	var (
		ratingInfo RatingInfo
		index      = -1
	)
	for i, info := range *ri {
		if info.Slug == rating.VendorSig.Slug {
			ratingInfo = info
			index = i
			break
		}
	}
	for _, id := range ratingInfo.Ratings {
		if id == cid.String() {
			return nil // Already exists
		}
	}
	if rating.VendorSig == nil {
		return errors.New("vendor sig is nil")
	}

	total := float64(ratingInfo.Count) * ratingInfo.Average
	total += float64(rating.Overall)
	newAverage := total / float64(ratingInfo.Count+1)

	ratingInfo.Average = newAverage
	ratingInfo.Count++
	ratingInfo.Slug = rating.VendorSig.Slug
	ratingInfo.Ratings = append(ratingInfo.Ratings, cid.String())
	if index == -1 {
		*ri = append(*ri, ratingInfo)
	} else {
		(*ri)[index] = ratingInfo
	}
	return nil
}

// GetRatingCIDs returns the rating CIDs for the rating with the given slug.
func (ri *RatingIndex) GetRatingCIDs(slug string) ([]cid.Cid, error) {
	var cids []cid.Cid
	for _, r := range *ri {
		if r.Slug == slug {
			for _, idStr := range r.Ratings {
				id, err := cid.Decode(idStr)
				if err != nil {
					return nil, err
				}
				cids = append(cids, id)
			}
			return cids, nil
		}
	}
	return nil, nil
}

// RatingInfo stores info about the ratings for each listing.
type RatingInfo struct {
	Slug    string   `json:"slug"`
	Count   uint64   `json:"count"`
	Average float64  `json:"average"`
	Ratings []string `json:"ratings"`
}

// Rating holds the review information for a listing.
type Rating struct {
	Overall     uint8    `json:"overall"`
	Review      string   `json:"review"`
	ImageHashes []string `json:"imageHashes,omitempty"`
}
