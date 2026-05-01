package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/ipfs/go-cid"
	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/models"
)

func (g *Gateway) handleGETImage(w http.ResponseWriter, r *http.Request) {
	imageIDStr := chi.URLParam(r, "imageID")

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
	peerIDStr := chi.URLParam(r, "peerID")
	sizeStr := chi.URLParam(r, "size")

	pid, cerr := peer.Decode(peerIDStr)
	if cerr != nil {
		ErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("invalid peer id: %s", cerr.Error()))
		return
	}

	useCache, _ := strconv.ParseBool(r.URL.Query().Get("usecache"))

	node := getMediaService(r)
	reader, err := node.GetProfileMedia(r.Context(), pid, contracts.SlotAvatar, models.ImageSize(sizeStr), useCache)
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
	peerIDStr := chi.URLParam(r, "peerID")
	sizeStr := chi.URLParam(r, "size")

	pid, cerr := peer.Decode(peerIDStr)
	if cerr != nil {
		ErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("invalid peer id: %s", cerr.Error()))
		return
	}

	useCache, _ := strconv.ParseBool(r.URL.Query().Get("usecache"))

	node := getMediaService(r)
	reader, err := node.GetProfileMedia(r.Context(), pid, contracts.SlotHeader, models.ImageSize(sizeStr), useCache)
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
		if handleMaxBytesError(w, err) {
			return
		}
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	imgBytes, err := base64.StdEncoding.DecodeString(data.Avatar)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("invalid base64: %s", err.Error()))
		return
	}
	if len(imgBytes) == 0 {
		ErrorResponse(w, http.StatusBadRequest, "avatar image data is empty")
		return
	}

	node := getMediaService(r)
	result, err := node.SetProfileMedia(r.Context(), contracts.SlotAvatar, imgBytes)
	if errors.Is(err, coreiface.ErrBadRequest) {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	} else if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	sanitizedJSONResponse(w, result.Hashes)
}

func (g *Gateway) handlePOSTHeader(w http.ResponseWriter, r *http.Request) {
	type ImgData struct {
		Header string `json:"header"`
	}
	decoder := json.NewDecoder(r.Body)
	data := new(ImgData)
	if err := decoder.Decode(&data); err != nil {
		if handleMaxBytesError(w, err) {
			return
		}
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	imgBytes, err := base64.StdEncoding.DecodeString(data.Header)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("invalid base64: %s", err.Error()))
		return
	}
	if len(imgBytes) == 0 {
		ErrorResponse(w, http.StatusBadRequest, "header image data is empty")
		return
	}

	node := getMediaService(r)
	result, err := node.SetProfileMedia(r.Context(), contracts.SlotHeader, imgBytes)
	if errors.Is(err, coreiface.ErrBadRequest) {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	} else if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	sanitizedJSONResponse(w, result.Hashes)
}

func (g *Gateway) handlePOSTImages(w http.ResponseWriter, r *http.Request) {
	type ImgData struct {
		Image    string `json:"image"`
		Filename string `json:"filename"`
	}
	var images []ImgData
	if err := json.NewDecoder(r.Body).Decode(&images); err != nil {
		if handleMaxBytesError(w, err) {
			return
		}
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	node := getMediaService(r)

	var imgs []models.FileHash

	for _, img := range images {
		imgBytes, err := base64.StdEncoding.DecodeString(img.Image)
		if err != nil {
			ErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("invalid base64 for %s: %s", img.Filename, err.Error()))
			return
		}
		if len(imgBytes) == 0 {
			ErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("image data is empty for %s", img.Filename))
			return
		}
		result, err := node.UploadMedia(r.Context(), imgBytes, img.Filename, contracts.UploadOpts{})
		if errors.Is(err, coreiface.ErrBadRequest) {
			ErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		} else if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		imgs = append(imgs, models.FileHash{Hash: result.Hash, Name: result.Filename})
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
		if handleMaxBytesError(w, err) {
			return
		}
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	node := getMediaService(r)

	var imgs []models.ImageHashes

	for _, img := range images {
		imgBytes, err := base64.StdEncoding.DecodeString(img.Image)
		if err != nil {
			ErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("invalid base64 for %s: %s", img.Filename, err.Error()))
			return
		}
		if len(imgBytes) == 0 {
			ErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("image data is empty for %s", img.Filename))
			return
		}
		result, err := node.UploadMedia(r.Context(), imgBytes, img.Filename, contracts.UploadOpts{Variants: true})
		if errors.Is(err, coreiface.ErrBadRequest) {
			ErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		} else if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		if result.Hashes != nil {
			imgs = append(imgs, *result.Hashes)
		}
	}

	sanitizedJSONResponse(w, imgs)
}
