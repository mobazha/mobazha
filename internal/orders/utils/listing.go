package utils

import (
	"fmt"

	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
)

// ExtractListing will return the listing with the given hash from the provided
// slice of listings if it exists.
func ExtractListing(hash string, listings []*pb.SignedListing) (*pb.Listing, error) {
	for _, sl := range listings {
		mh, err := HashListing(sl)
		if err != nil {
			return nil, err
		}
		if mh.B58String() == hash {
			return sl.Listing, nil
		}
	}
	return nil, fmt.Errorf("listing %s not found in order", hash)
}
