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

	"github.com/mobazha/mobazha3.0/internal/core/coreiface"
	"github.com/mobazha/mobazha3.0/internal/models"
	"github.com/gorilla/mux"
	"github.com/ipfs/go-cid"
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

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	reader, err := node.GetFile(ctx, id)
	if errors.Is(err, coreiface.ErrNotFound) {
		ErrorResponse(w, http.StatusNotFound, err.Error())
		return
	} else if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Cache-Control", "public, max-age=29030400, immutable")
	w.Header().Del("Content-Type")
	http.ServeContent(w, r, id.String(), time.Now(), reader)
}

func (g *Gateway) handlePOSTFile(w http.ResponseWriter, r *http.Request) {
	// parse input, type multipart/form-data
	r.Body = http.MaxBytesReader(w, r.Body, 15<<20) // limit your max input length! 15M

	// retrieve the file from form data
	file, header, err := r.FormFile("file")
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer file.Close()

	buf := bytes.NewBuffer(nil)
	if _, err := io.Copy(buf, file); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	fileType := r.FormValue("type")
	var fileHash models.FileHash
	if fileType == "introVideo" {
		if !filetype.IsVideo(buf.Bytes()) {
			ErrorResponse(w, http.StatusBadRequest, "Not video file")
		}
		fileHash, err = node.AddIntroVideo(buf.Bytes(), header.Filename)
	} else {
		fileHash, err = node.AddFile(buf.Bytes(), header.Filename)
	}
	if errors.Is(err, coreiface.ErrBadRequest) {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	} else if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	sanitizedJSONResponse(w, fileHash)
}
