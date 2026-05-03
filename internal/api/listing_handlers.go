package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/ipfs/go-cid"
	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/pkg/request"
	"github.com/mobazha/mobazha3.0/pkg/storefront"
	"google.golang.org/protobuf/encoding/protojson"
)

var (
	ErrGlobalBannedPeerID = errors.New("the peer ID is globally banned")

	unmarshaler = protojson.UnmarshalOptions{
		DiscardUnknown: true,
	}
)

// extractRequestContext extracts group context from HTTP headers and creates a request.Context
func extractRequestContext(r *http.Request) *request.Context {
	groupPlatform := r.Header.Get("X-Group-Platform")
	groupChatID := r.Header.Get("X-Group-ChatID")

	if groupPlatform != "" && groupChatID != "" {
		return request.NewContext().WithGroupContext(groupPlatform, groupChatID)
	}
	return nil
}

func (g *Gateway) handleGETListing(w http.ResponseWriter, r *http.Request) {
	listingIDStr := chi.URLParam(r, "listingID")
	peerIDStr := chi.URLParam(r, "peerID")
	slug := chi.URLParam(r, "slug")

	ls := getListingService(r)
	is := getIdentityService(r)
	reqCtx := extractRequestContext(r)

	var (
		listing *pb.SignedListing
		err     error
	)
	if listingIDStr != "" { // Query by CID
		id, cerr := cid.Decode(listingIDStr)
		if cerr == nil {
			listing, err = ls.GetListingByCID(r.Context(), id, reqCtx)
		} else {
			listing, err = ls.GetMyListingBySlug(listingIDStr)
		}

		if err == nil && listing != nil && listing.Listing != nil && listing.Listing.VendorID != nil {
			pid, _ := peer.Decode(listing.Listing.VendorID.PeerID)
			if is.IsGlobalBanned(pid) {
				err = ErrGlobalBannedPeerID
			}
		}
	} else if peerIDStr == is.Identity().String() {
		listing, err = ls.GetMyListingBySlug(slug)
	} else if peerIDStr != "" && slug != "" { // Query by peerID/slug
		pid, perr := peer.Decode(peerIDStr)
		if perr != nil {
			ErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("invalid peer id: %s", perr.Error()))
			return
		}
		if is.IsGlobalBanned(pid) {
			err = ErrGlobalBannedPeerID
		} else {
			useCache, _ := strconv.ParseBool(r.URL.Query().Get("usecache"))
			listing, err = ls.GetListingBySlug(r.Context(), pid, slug, reqCtx, useCache)
		}
	} else {
		ErrorResponse(w, http.StatusBadRequest, "")
		return
	}

	if errors.Is(err, coreiface.ErrNotFound) {
		ErrorResponse(w, http.StatusNotFound, err.Error())
		return
	} else if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	sanitizedProtobufResponse(w, listing)
}

func (g *Gateway) handleGETMyListing(w http.ResponseWriter, r *http.Request) {
	slugOrCid := chi.URLParam(r, "slugOrCID")

	var (
		slug    string
		listing *pb.SignedListing
		err     error
	)
	cid, cerr := cid.Decode(slugOrCid)
	if cerr != nil {
		slug = slugOrCid
	}

	ls := getListingService(r)

	if slug != "" {
		listing, err = ls.GetMyListingBySlug(slug)
	} else {
		listing, err = ls.GetMyListingByCID(cid)
		w.Header().Set("Cache-Control", "public, max-age=29030400, immutable")
	}

	if errors.Is(err, coreiface.ErrNotFound) {
		ErrorResponse(w, http.StatusNotFound, err.Error())
		return
	} else if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	sanitizedProtobufResponse(w, listing)
}

func (g *Gateway) handleGETListingIndex(w http.ResponseWriter, r *http.Request) {
	peerIDStr := chi.URLParam(r, "peerID")
	useCache, _ := strconv.ParseBool(r.URL.Query().Get("usecache"))
	var (
		listingIndex models.ListingIndex
		ratingIndex  models.RatingIndex
		ratingErr    error
		listingErr   error
	)

	ls := getListingService(r)
	is := getIdentityService(r)
	ss := getSocialService(r)
	reqCtx := extractRequestContext(r)

	// MS-Phase-2a · MS2a.2c — capture storefront filter (if any) so we can
	// scope the listing result to the storefront's product selection after
	// the full index is loaded. isLocalPeer controls whether collection
	// filtering is applied: collection membership lives in the local DB and
	// is only meaningful for the node's own listings.
	sfFilter := StorefrontFilterFromContext(r.Context())
	isLocalPeer := peerIDStr == "" || peerIDStr == is.Identity().String()

	if isLocalPeer {
		listingIndex, listingErr = ls.GetMyListings()

		if listingErr == nil && ss != nil {
			ratingIndex, ratingErr = ss.GetMyRatings()
		}
	} else {
		pid, err := peer.Decode(peerIDStr)
		if err != nil {
			ErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("invalid peer id: %s", err.Error()))
			return
		}

		if is.IsGlobalBanned(pid) {
			listingErr = ErrGlobalBannedPeerID
		} else {
			listingIndex, listingErr = ls.GetListings(r.Context(), pid, reqCtx, useCache)

			if listingErr == nil && ss != nil {
				ratingIndex, ratingErr = ss.GetRatings(r.Context(), pid, reqCtx, useCache)
			}
		}
	}

	if listingErr != nil && !errors.Is(listingErr, coreiface.ErrNotFound) {
		ErrorResponse(w, http.StatusInternalServerError, listingErr.Error())
		return
	}

	// MS-Phase-2a · MS2a.2c — apply storefront collection filter before the
	// rating enrichment pass so we don't waste work annotating listings
	// that will be dropped. Collection filtering runs only for local-peer
	// listings because CollectionService membership lookups hit the local
	// DB. Tag include/exclude filtering is deferred to TD-033.
	if isLocalPeer && sfFilter != nil && len(sfFilter.CollectionIDs) > 0 && len(listingIndex) > 0 {
		filtered, filterErr := filterListingsByCollections(r, listingIndex, sfFilter.CollectionIDs)
		if filterErr != nil {
			ErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("storefront collection filter: %s", filterErr.Error()))
			return
		}
		listingIndex = filtered
	}

	if ratingErr == nil {
		ratings := make(map[string]models.RatingInfo)
		for _, r := range ratingIndex {
			ratings[r.Slug] = r
		}

		for idx, listing := range listingIndex {
			if rating, ok := ratings[listing.Slug]; ok {
				listing.AverageRating = float32(rating.Average)
				listing.RatingCount = uint32(rating.Count)

				listingIndex[idx] = listing
			}
		}
	}

	// MS-Phase-2a · MS2a.5 — apply storefront price rule to the list-view
	// DTOs. Runs last (after collection filter + rating enrichment) so the
	// adjustment only happens on listings that will actually be rendered.
	// Applies to any storefront-scoped request, including cross-peer views
	// on the SaaS Gateway — the rule is a storefront-owner-defined
	// transform, not dependent on whether the data was served locally.
	if rule := StorefrontPriceRuleFromContext(r.Context()); rule != nil && len(listingIndex) > 0 {
		applyStorefrontPriceRuleToIndex(listingIndex, rule)
	}

	sanitizedJSONResponse(w, listingIndex)
}

// applyStorefrontPriceRuleToIndex mutates listingIndex in place, replacing
// each entry's Price.Amount with the rule-adjusted value. The currency and
// divisibility remain untouched because the rule operates on minor units
// of the listing's native currency — no cross-currency conversion happens
// here.
//
// Kept private to listing_handlers.go because the only caller is the
// index handler. Profiles / search follow-ups that need the same behavior
// should call rule.ApplyAmount() directly on their DTOs.
func applyStorefrontPriceRuleToIndex(index models.ListingIndex, rule *storefront.PriceRule) {
	if rule == nil || rule.IsZero() {
		return
	}
	for idx := range index {
		base := index[idx].Price.Amount
		index[idx].Price.Amount = rule.ApplyAmount(base)
	}
}

// filterListingsByCollections keeps only listings whose slug belongs to at
// least one of the given collectionIDs. Returns the filtered slice (may be
// empty) or an error from the collection service. When no collection
// service is registered on the request (edge case — older wiring paths),
// we return the input unchanged rather than failing the request hard.
//
// MS-Phase-2a · MS2a.2c.
func filterListingsByCollections(r *http.Request, index models.ListingIndex, collectionIDs []string) (models.ListingIndex, error) {
	cs, ok := getCollectionService(r)
	if !ok || cs == nil {
		return index, nil
	}
	return filterListingsByCollectionsWithService(r.Context(), cs, index, collectionIDs)
}

// filterListingsByCollectionsWithService is the pure core of collection
// filtering — same semantics as filterListingsByCollections but takes the
// CollectionService directly so it can be unit-tested without wiring up
// a full NodeService mock.
func filterListingsByCollectionsWithService(
	ctx context.Context,
	cs contracts.CollectionService,
	index models.ListingIndex,
	collectionIDs []string,
) (models.ListingIndex, error) {
	if cs == nil || len(collectionIDs) == 0 {
		return index, nil
	}
	kept := make(models.ListingIndex, 0, len(index))
	for _, l := range index {
		ok, err := cs.IsProductInCollections(ctx, collectionIDs, l.Slug)
		if err != nil {
			return nil, err
		}
		if ok {
			kept = append(kept, l)
		}
	}
	return kept, nil
}

func (g *Gateway) handlePOSTListing(w http.ResponseWriter, r *http.Request) {
	listing := new(pb.Listing)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("error reading request body: %s", err.Error()))
		return
	}

	if err := unmarshaler.Unmarshal(body, listing); err != nil {
		ErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("error unmarshaling listing: %s", err.Error()))
		return
	}

	ls := getListingService(r)

	if _, err := ls.GetMyListingBySlug(listing.Slug); !errors.Is(err, coreiface.ErrNotFound) {
		ErrorResponse(w, http.StatusConflict, "listing exists. use PUT to update")
		return
	}

	if err := ls.SaveListing(listing, nil); err != nil {
		if errors.Is(err, coreiface.ErrBadRequest) {
			ErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	sanitizedJSONResponse(w, &struct {
		Slug string `json:"slug"`
	}{
		Slug: listing.Slug,
	})
}

func (g *Gateway) handlePUTListing(w http.ResponseWriter, r *http.Request) {
	listing := new(pb.Listing)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("error reading request body: %s", err.Error()))
		return
	}

	if err := unmarshaler.Unmarshal(body, listing); err != nil {
		ErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("error unmarshaling listing: %s", err.Error()))
		return
	}

	ls := getListingService(r)

	if _, err := ls.GetMyListingBySlug(listing.Slug); errors.Is(err, coreiface.ErrNotFound) {
		ErrorResponse(w, http.StatusConflict, "listing does not exist. use POST to create")
		return
	}

	if err := ls.SaveListing(listing, nil); err != nil {
		if errors.Is(err, coreiface.ErrBadRequest) {
			ErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	sanitizedJSONResponse(w, &struct {
		Slug string `json:"slug"`
	}{
		Slug: listing.Slug,
	})
}

func (g *Gateway) handleDELETEListing(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	ls := getListingService(r)

	if err := ls.DeleteListing(slug, nil); err != nil {
		if errors.Is(err, coreiface.ErrNotFound) {
			ErrorResponse(w, http.StatusNotFound, err.Error())
			return
		}
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	sanitizedJSONResponse(w, struct{}{})
}
