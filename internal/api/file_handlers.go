package api

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/h2non/filetype"

	"github.com/gorilla/mux"
	"github.com/ipfs/go-cid"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
)

func (g *Gateway) handleGETFile(w http.ResponseWriter, r *http.Request) {
	fileIDStr := mux.Vars(r)["fileID"]

	id, cerr := cid.Decode(fileIDStr)
	if cerr != nil {
		ErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("invalid file id: %s", cerr.Error()))
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

func (g *Gateway) handlePOSTFile(w http.ResponseWriter, r *http.Request) {
	// Set max possible limit at HTTP layer; fine-grained checks in service layer.
	r.Body = http.MaxBytesReader(w, r.Body, 50<<20)

	file, header, err := r.FormFile("file")
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			ErrorResponse(w, http.StatusRequestEntityTooLarge, "file exceeds maximum upload size")
			return
		}
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	defer file.Close()

	buf := bytes.NewBuffer(nil)
	if _, err := io.Copy(buf, file); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			ErrorResponse(w, http.StatusRequestEntityTooLarge, "file exceeds maximum upload size")
			return
		}
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	node := getMediaService(r)
	data := buf.Bytes()

	fileType := r.FormValue("type")
	opts := contracts.UploadOpts{}
	if fileType == "introVideo" {
		if !filetype.IsVideo(data) {
			ErrorResponse(w, http.StatusBadRequest, "Not video file")
			return
		}
		opts.MaxBytes = 50 << 20
	}

	result, err := node.UploadMedia(r.Context(), data, header.Filename, opts)
	if errors.Is(err, coreiface.ErrBadRequest) {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	} else if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	sanitizedJSONResponse(w, result)
}
