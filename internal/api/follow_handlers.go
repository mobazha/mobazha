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

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	var (
		followers models.Followers
		err       error
	)
	if peerIDStr == "" || peerIDStr == node.Identity().String() {
		followers, err = node.GetMyFollowers()
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
		followers, err = node.GetFollowers(r.Context(), pid, useCache)
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

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	var (
		following models.Following
		err       error
	)
	if peerIDStr == "" || peerIDStr == node.Identity().String() {
		following, err = node.GetMyFollowing()
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
		following, err = node.GetFollowing(r.Context(), pid, useCache)
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

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)
	ret, err := node.FollowsMe(pid)
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
	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)
	err = node.FollowNode(pid, nil)
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
	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)
	err = node.UnfollowNode(pid, nil)
	if errors.Is(err, coreiface.ErrBadRequest) {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	} else if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	sanitizedStringResponse(w, `{"success": "true"}`)
}
