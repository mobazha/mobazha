package orders

import (
	"errors"
	"fmt"

	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"github.com/multiformats/go-multihash"
	"google.golang.org/protobuf/proto"
)

// CalculateSignedListingHash returns the multihash stored in
// OrderOpen.items[].listingHash for an embedded signed listing. This order-item
// identity is intentionally distinct from the catalog CID used to retrieve the
// listing and lets external order orchestrators validate the signed boundary
// without importing Core internals.
func CalculateSignedListingHash(listing *pb.SignedListing) (string, error) {
	if listing == nil {
		return "", errors.New("signed listing is required")
	}
	serialized, err := proto.Marshal(listing)
	if err != nil {
		return "", fmt.Errorf("marshal signed listing: %w", err)
	}
	hash, err := multihash.Sum(serialized, multihash.SHA2_256, -1)
	if err != nil {
		return "", fmt.Errorf("hash signed listing: %w", err)
	}
	return hash.B58String(), nil
}
