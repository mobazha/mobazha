package api

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/ipfs/go-cid"
	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/pkg/request"
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
	listingIDStr := mux.Vars(r)["listingID"]
	peerIDStr := mux.Vars(r)["peerID"]
	slug := mux.Vars(r)["slug"]

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
	slugOrCid := mux.Vars(r)["slugOrCID"]

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
	peerIDStr := mux.Vars(r)["peerID"]
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

	if peerIDStr == "" || peerIDStr == is.Identity().String() {
		listingIndex, listingErr = ls.GetMyListings()

		if listingErr == nil {
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

			if listingErr == nil {
				ratingIndex, ratingErr = ss.GetRatings(r.Context(), pid, reqCtx, useCache)
			}
		}
	}

	if listingErr != nil && !errors.Is(listingErr, coreiface.ErrNotFound) {
		ErrorResponse(w, http.StatusInternalServerError, listingErr.Error())
		return
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

	sanitizedJSONResponse(w, listingIndex)
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
		} else if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
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
		} else if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	sanitizedJSONResponse(w, &struct {
		Slug string `json:"slug"`
	}{
		Slug: listing.Slug,
	})
}

func (g *Gateway) handleDELETEListing(w http.ResponseWriter, r *http.Request) {
	slug := mux.Vars(r)["slug"]

	ls := getListingService(r)

	if err := ls.DeleteListing(slug, nil); err != nil {
		if errors.Is(err, coreiface.ErrNotFound) {
			ErrorResponse(w, http.StatusNotFound, err.Error())
			return
		} else if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	sanitizedJSONResponse(w, struct{}{})
}
