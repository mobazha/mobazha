package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha/pkg/core/coreiface"
	"github.com/mobazha/mobazha/pkg/models"
	"google.golang.org/protobuf/encoding/protojson"
)

func (g *Gateway) handleGETMyRatingIndex(w http.ResponseWriter, r *http.Request) {
	social := getSocialService(r)

	index, err := social.GetMyRatings()
	if errors.Is(err, coreiface.ErrNotFound) {
		emptyRatingInfo := models.RatingInfo{}
		sanitizedJSONResponse(w, emptyRatingInfo)
		return
	} else if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	ratingsRet := mergeRatings(&index)

	sanitizedJSONResponse(w, ratingsRet)
}

func (g *Gateway) handleGETPeerRatingsBySlug(w http.ResponseWriter, r *http.Request) {
	peerIDStr := chi.URLParam(r, "peerID")
	slug := chi.URLParam(r, "slug")

	pid, perr := peer.Decode(peerIDStr)
	if perr != nil {
		ErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("invalid peer id: %s", perr.Error()))
		return
	}
	useCache, _ := strconv.ParseBool(r.URL.Query().Get("usecache"))

	social := getSocialService(r)

	var index models.RatingIndex
	var err error
	if peerIDStr == getIdentityService(r).Identity().String() {
		index, err = social.GetMyRatings()
	} else {
		reqCtx := extractRequestContext(r)
		index, err = social.GetRatings(r.Context(), pid, reqCtx, useCache)
	}

	if errors.Is(err, coreiface.ErrNotFound) {
		emptyRatingInfo := models.RatingInfo{}
		sanitizedJSONResponse(w, emptyRatingInfo)
		return
	} else if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	var ratingInfo models.RatingInfo
	for _, r := range index {
		if r.Slug == slug {
			ratingInfo = r
			break
		}
	}

	sanitizedJSONResponse(w, ratingInfo)
}

func (g *Gateway) handleGETRatingIndex(w http.ResponseWriter, r *http.Request) {
	peerIDOrSlug := chi.URLParam(r, "peerIDOrSlug")
	var slug string
	pid, perr := peer.Decode(peerIDOrSlug)
	if perr != nil {
		log.Infof("Decode peerID failed, use as slug: %s, %e", peerIDOrSlug, perr)
		slug = peerIDOrSlug
	}

	social := getSocialService(r)

	var (
		index models.RatingIndex
		err   error
	)

	if pid.String() == "" || pid.String() == getIdentityService(r).Identity().String() {
		index, err = social.GetMyRatings()
	} else {
		useCache, _ := strconv.ParseBool(r.URL.Query().Get("usecache"))
		reqCtx := extractRequestContext(r)
		index, err = social.GetRatings(r.Context(), pid, reqCtx, useCache)
	}

	if errors.Is(err, coreiface.ErrNotFound) {
		emptyRatingInfo := models.RatingInfo{}
		sanitizedJSONResponse(w, emptyRatingInfo)
		return
	} else if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	if pid.String() != "" {
		ratingInfo := mergeRatings(&index)
		sanitizedJSONResponse(w, ratingInfo)
		return
	}

	var ratingInfo models.RatingInfo
	for _, r := range index {
		if r.Slug == slug {
			ratingInfo = r
		}
	}

	sanitizedJSONResponse(w, ratingInfo)
}

func mergeRatings(index *models.RatingIndex) models.RatingInfo {
	ratingsRet := models.RatingInfo{}
	total := float64(0)
	count := uint64(0)
	for _, r := range *index {
		total += r.Average * float64(r.Count)
		count += r.Count
		ratingsRet.Ratings = append(ratingsRet.Ratings, r.Ratings...)
	}
	ratingsRet.Count = count
	if count > 0 {
		ratingsRet.Average = total / float64(count)
	}
	return ratingsRet
}

func (g *Gateway) handleGETRating(w http.ResponseWriter, r *http.Request) {
	ratingIDStr := chi.URLParam(r, "ratingID")

	id, cerr := cid.Decode(ratingIDStr)
	if cerr != nil {
		ErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("invalid rating id: %s", cerr.Error()))
		return
	}

	social := getSocialService(r)

	rating, err := social.GetRating(r.Context(), id)

	if errors.Is(err, coreiface.ErrNotFound) {
		ErrorResponse(w, http.StatusNotFound, err.Error())
		return
	} else if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=29030400, immutable")

	sanitizedProtobufResponse(w, rating)
}

func (g *Gateway) handlePOSTFetchRatings(w http.ResponseWriter, r *http.Request) {
	async, _ := strconv.ParseBool(r.URL.Query().Get("async"))

	var ratingIDs []string
	if err := json.NewDecoder(r.Body).Decode(&ratingIDs); err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	type ratingWithAsyncID struct {
		ID     string          `json:"id"`
		Rating json.RawMessage `json:"rating"`
	}

	type ratingError struct {
		ID       string `json:"id"`
		RatingID string `json:"ratingID"`
		Error    string `json:"error"`
	}

	inputOrder := make(map[string]int, len(ratingIDs))
	for i, id := range ratingIDs {
		inputOrder[id] = i
	}

	var (
		ratings      = make([]ratingWithAsyncID, 0, len(ratingIDs))
		responseChan = make(chan interface{}, 8)
		wg           sync.WaitGroup
		marshaler    = protojson.MarshalOptions{Indent: "    "}
	)

	social := getSocialService(r)
	nodeID := getIdentityService(r).GetNodeID()

	wg.Add(len(ratingIDs))
	go func() {
		for _, ratingID := range ratingIDs {
			rid, err := cid.Decode(ratingID)
			if err != nil {
				responseChan <- ratingError{
					RatingID: ratingID,
					Error:    err.Error(),
				}
				wg.Done()
				continue
			}
			go func(id cid.Cid) {
				defer wg.Done()
				rating, err := social.GetRating(r.Context(), id)
				if err != nil {
					responseChan <- ratingError{
						RatingID: id.String(),
						Error:    err.Error(),
					}
					return
				}
				ratingJSON := marshaler.Format(rating)
				responseChan <- ratingWithAsyncID{
					ID:     id.String(),
					Rating: []byte(ratingJSON),
				}
			}(rid)
		}
		wg.Wait()
		close(responseChan)
	}()

	if !async {
		for i := range responseChan {
			switch p := i.(type) {
			case ratingWithAsyncID:
				ratings = append(ratings, p)
			}
		}
		sort.Slice(ratings, func(i, j int) bool {
			return inputOrder[ratings[i].ID] < inputOrder[ratings[j].ID]
		})
		sanitizedJSONResponse(w, ratings)
	} else {
		asyncID := r.URL.Query().Get("asyncID")
		if asyncID == "" {
			r := make([]byte, 20)
			rand.Read(r)
			asyncID = hex.EncodeToString(r)
		}
		w.WriteHeader(http.StatusAccepted)
		sanitizedJSONResponse(w, struct {
			ID string `json:"id"`
		}{ID: asyncID})

		go func() {
			for i := range responseChan {
				switch p := i.(type) {
				case ratingWithAsyncID:
					p.ID = asyncID
					g.NotifyWebsockets(nodeID)(p)
				case ratingError:
					p.ID = asyncID
					g.NotifyWebsockets(nodeID)(p)
				}
			}
		}()
	}
}
