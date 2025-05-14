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
	"google.golang.org/protobuf/encoding/protojson"
)

var (
	ErrGlobalBannedPeerID = errors.New("the peer ID is globally banned")

	unmarshaler = protojson.UnmarshalOptions{
		DiscardUnknown: true,
	}
)

func (g *Gateway) handleGETListing(w http.ResponseWriter, r *http.Request) {
	listingIDStr := mux.Vars(r)["listingID"]
	peerIDStr := mux.Vars(r)["peerID"]
	slug := mux.Vars(r)["slug"]

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	var (
		listing *pb.SignedListing
		err     error
	)
	if listingIDStr != "" { // Query by CID
		id, cerr := cid.Decode(listingIDStr)
		if cerr == nil {
			listing, err = node.GetListingByCID(r.Context(), id)
		} else {
			listing, err = node.GetMyListingBySlug(listingIDStr)
		}

		if err == nil && listing != nil {
			pid, _ := peer.Decode(listing.Listing.VendorID.PeerID)
			if node.IsGlobalBanned(pid) {
				err = ErrGlobalBannedPeerID
			}
		}
	} else if peerIDStr == node.Identity().String() {
		listing, err = node.GetMyListingBySlug(slug)
	} else if peerIDStr != "" && slug != "" { // Query by peerID/slug{
		pid, perr := peer.Decode(peerIDStr)
		if perr != nil {
			ErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("invalid peer id: %s", perr.Error()))
			return
		}
		if node.IsGlobalBanned(pid) {
			err = ErrGlobalBannedPeerID
		} else {
			useCache, _ := strconv.ParseBool(r.URL.Query().Get("usecache"))
			listing, err = node.GetListingBySlug(r.Context(), pid, slug, useCache)
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

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	if slug != "" {
		listing, err = node.GetMyListingBySlug(slug)
	} else {
		listing, err = node.GetMyListingByCID(cid)
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

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	if peerIDStr == "" || peerIDStr == node.Identity().String() {
		listingIndex, listingErr = node.GetMyListings()

		if listingErr == nil {
			ratingIndex, ratingErr = node.GetMyRatings()
		}
	} else {
		pid, err := peer.Decode(peerIDStr)
		if err != nil {
			ErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("invalid peer id: %s", err.Error()))
			return
		}

		if node.IsGlobalBanned(pid) {
			listingErr = ErrGlobalBannedPeerID
		} else {
			listingIndex, listingErr = node.GetListings(r.Context(), pid, useCache)

			if listingErr == nil {
				ratingIndex, ratingErr = node.GetRatings(r.Context(), pid, useCache)
			}
		}
	}

	if errors.Is(listingErr, coreiface.ErrNotFound) {
		ErrorResponse(w, http.StatusNotFound, listingErr.Error())
		return
	} else if listingErr != nil {
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

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	if _, err := node.GetMyListingBySlug(listing.Slug); !errors.Is(err, coreiface.ErrNotFound) {
		ErrorResponse(w, http.StatusConflict, "listing exists. use PUT to update")
		return
	}

	if err := node.SaveListing(listing, nil); err != nil {
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

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	if _, err := node.GetMyListingBySlug(listing.Slug); errors.Is(err, coreiface.ErrNotFound) {
		ErrorResponse(w, http.StatusConflict, "listing does not exist. use POST to create")
		return
	}

	if err := node.SaveListing(listing, nil); err != nil {
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

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	if err := node.DeleteListing(slug, nil); err != nil {
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
