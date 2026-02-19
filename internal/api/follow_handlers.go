package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/models"
)

func (g *Gateway) handleGETFollowers(w http.ResponseWriter, r *http.Request) {
	peerIDStr := mux.Vars(r)["peerID"]

	useCache, _ := strconv.ParseBool(r.URL.Query().Get("usecache"))

	social := getSocialService(r)
	reqCtx := extractRequestContext(r)

	var (
		followers models.Followers
		err       error
	)
	if peerIDStr == "" || peerIDStr == getIdentityService(r).Identity().String() {
		followers, err = social.GetMyFollowers()
		if errors.Is(err, coreiface.ErrNotFound) {
			ErrorResponse(w, http.StatusNotFound, err.Error())
			return
		} else if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	} else {
		pid, err := peer.Decode(peerIDStr)
		if err != nil {
			ErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		followers, err = social.GetFollowers(r.Context(), pid, reqCtx, useCache)
		if errors.Is(err, coreiface.ErrNotFound) {
			ErrorResponse(w, http.StatusNotFound, err.Error())
			return
		} else if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	if followers == nil {
		followers = models.Followers{}
	}
	sanitizedJSONResponse(w, followers)
}

func (g *Gateway) handleGETFollowing(w http.ResponseWriter, r *http.Request) {
	peerIDStr := mux.Vars(r)["peerID"]

	useCache, _ := strconv.ParseBool(r.URL.Query().Get("usecache"))

	social := getSocialService(r)
	reqCtx := extractRequestContext(r)

	var (
		following models.Following
		err       error
	)
	if peerIDStr == "" || peerIDStr == getIdentityService(r).Identity().String() {
		following, err = social.GetMyFollowing()
		if errors.Is(err, coreiface.ErrNotFound) {
			ErrorResponse(w, http.StatusNotFound, err.Error())
			return
		} else if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	} else {
		pid, err := peer.Decode(peerIDStr)
		if err != nil {
			ErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		following, err = social.GetFollowing(r.Context(), pid, reqCtx, useCache)
		if errors.Is(err, coreiface.ErrNotFound) {
			ErrorResponse(w, http.StatusNotFound, err.Error())
			return
		} else if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	if following == nil {
		following = models.Following{}
	}
	sanitizedJSONResponse(w, following)
}

func (g *Gateway) handleGETFollowsMe(w http.ResponseWriter, r *http.Request) {
	peerIDStr := mux.Vars(r)["peerID"]
	pid, err := peer.Decode(peerIDStr)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	social := getSocialService(r)
	ret, err := social.FollowsMe(pid)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	sanitizedJSONResponse(w, struct {
		FollowsMe bool `json:"followsMe"`
	}{
		FollowsMe: ret,
	})
}

func (g *Gateway) handlePOSTFollow(w http.ResponseWriter, r *http.Request) {
	peerIDStr := mux.Vars(r)["peerID"]
	pid, err := peer.Decode(peerIDStr)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	social := getSocialService(r)
	err = social.FollowNode(pid, nil)
	if errors.Is(err, coreiface.ErrBadRequest) {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	} else if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	sanitizedStringResponse(w, `{"success": "true"}`)
}

func (g *Gateway) handlePOSTUnFollow(w http.ResponseWriter, r *http.Request) {
	peerIDStr := mux.Vars(r)["peerID"]
	pid, err := peer.Decode(peerIDStr)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	social := getSocialService(r)
	err = social.UnfollowNode(pid, nil)
	if errors.Is(err, coreiface.ErrBadRequest) {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	} else if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	sanitizedStringResponse(w, `{"success": "true"}`)
}
