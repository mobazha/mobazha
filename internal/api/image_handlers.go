package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/ipfs/go-cid"
	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/models"
)

func (g *Gateway) handleGETImage(w http.ResponseWriter, r *http.Request) {
	imageIDStr := mux.Vars(r)["imageID"]

	id, cerr := cid.Decode(imageIDStr)
	if cerr != nil {
		ErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("invalid image id: %s", cerr.Error()))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*45)
	defer cancel()

	node := getMediaService(r)

	reader, contentType, err := node.GetMedia(ctx, id)
	if errors.Is(err, coreiface.ErrNotFound) {
		ErrorResponse(w, http.StatusNotFound, err.Error())
		return
	} else if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Cache-Control", "public, max-age=29030400, immutable")
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	} else {
		w.Header().Del("Content-Type")
	}
	http.ServeContent(w, r, id.String(), time.Now(), reader)
}

func (g *Gateway) handleGETAvatar(w http.ResponseWriter, r *http.Request) {
	peerIDStr := mux.Vars(r)["peerID"]
	sizeStr := mux.Vars(r)["size"]

	pid, cerr := peer.Decode(peerIDStr)
	if cerr != nil {
		ErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("invalid peer id: %s", cerr.Error()))
		return
	}

	useCache, _ := strconv.ParseBool(r.URL.Query().Get("usecache"))

	node := getMediaService(r)
	reader, err := node.GetAvatar(r.Context(), pid, models.ImageSize(sizeStr), useCache)
	if errors.Is(err, coreiface.ErrNotFound) {
		ErrorResponse(w, http.StatusNotFound, err.Error())
		return
	} else if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	http.ServeContent(w, r, peerIDStr, time.Now(), reader)
}

func (g *Gateway) handleGETHeader(w http.ResponseWriter, r *http.Request) {
	peerIDStr := mux.Vars(r)["peerID"]
	sizeStr := mux.Vars(r)["size"]

	pid, cerr := peer.Decode(peerIDStr)
	if cerr != nil {
		ErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("invalid peer id: %s", cerr.Error()))
		return
	}

	useCache, _ := strconv.ParseBool(r.URL.Query().Get("usecache"))

	node := getMediaService(r)
	reader, err := node.GetHeader(r.Context(), pid, models.ImageSize(sizeStr), useCache)
	if errors.Is(err, coreiface.ErrNotFound) {
		ErrorResponse(w, http.StatusNotFound, err.Error())
		return
	} else if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	http.ServeContent(w, r, peerIDStr, time.Now(), reader)
}

func (g *Gateway) handlePOSTAvatar(w http.ResponseWriter, r *http.Request) {
	type ImgData struct {
		Avatar string `json:"avatar"`
	}
	decoder := json.NewDecoder(r.Body)
	data := new(ImgData)
	if err := decoder.Decode(&data); err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	node := getMediaService(r)
	hashes, err := node.SetAvatarImage(data.Avatar, nil)
	if errors.Is(err, coreiface.ErrBadRequest) {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	} else if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	sanitizedJSONResponse(w, hashes)
}

func (g *Gateway) handlePOSTHeader(w http.ResponseWriter, r *http.Request) {
	type ImgData struct {
		Header string `json:"header"`
	}
	decoder := json.NewDecoder(r.Body)
	data := new(ImgData)
	if err := decoder.Decode(&data); err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	node := getMediaService(r)
	hashes, err := node.SetHeaderImage(data.Header, nil)
	if errors.Is(err, coreiface.ErrBadRequest) {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	} else if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	sanitizedJSONResponse(w, hashes)
}

func (g *Gateway) handlePOSTImages(w http.ResponseWriter, r *http.Request) {
	type ImgData struct {
		Image    string `json:"image"`
		Filename string `json:"filename"`
	}
	var images []ImgData
	if err := json.NewDecoder(r.Body).Decode(&images); err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	node := getMediaService(r)

	var imgs []models.FileHash

	for _, img := range images {
		hash, err := node.SetImage(img.Image, img.Filename)
		if errors.Is(err, coreiface.ErrBadRequest) {
			ErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		} else if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		imgs = append(imgs, hash)
	}

	sanitizedJSONResponse(w, imgs)
}

func (g *Gateway) handlePOSTProductImage(w http.ResponseWriter, r *http.Request) {
	type ImgData struct {
		Image    string `json:"image"`
		Filename string `json:"filename"`
	}
	var images []ImgData
	if err := json.NewDecoder(r.Body).Decode(&images); err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	node := getMediaService(r)

	var imgs []models.ImageHashes

	for _, img := range images {
		hashes, err := node.SetProductImage(img.Image, img.Filename)
		if errors.Is(err, coreiface.ErrBadRequest) {
			ErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		} else if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		imgs = append(imgs, hashes)
	}

	sanitizedJSONResponse(w, imgs)
}
