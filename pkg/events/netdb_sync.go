package events

import (
	"encoding/json"

	opb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
)

// ProfileChanged signals that the local profile has been modified
// (name, about, avatar, header, etc.). NetDBSyncService re-reads
// the profile from DB and pushes to the search service.
type ProfileChanged struct{}

// ListingChanged signals that a single listing was created or updated.
type ListingChanged struct {
	Slug string
}

// ListingDeleted signals that a listing was removed.
type ListingDeleted struct {
	Cid string
}

// ListingsReindexed signals that multiple listings were bulk-updated
// and the full index + all listings should be re-pushed.
type ListingsReindexed struct{}

// FollowingChanged signals that the local following list changed.
type FollowingChanged struct{}

// FollowersChanged signals that the local followers list changed.
type FollowersChanged struct{}

// CollectionsChanged signals that store collections were modified.
type CollectionsChanged struct{}

// DiscountsChanged signals that store discounts were modified.
type DiscountsChanged struct{}

// StorefrontChanged signals that the storefront branding config was saved.
type StorefrontChanged struct {
	Config json.RawMessage
}

// RatingsChanged signals that new ratings were received (via ORDER_COMPLETE).
// Ratings carry the original protobuf data because they come from a P2P
// message rather than a simple DB query.
type RatingsChanged struct {
	Ratings []*opb.Rating
}
