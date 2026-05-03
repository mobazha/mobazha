//go:build !private_distribution

package api

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strconv"

	"github.com/go-chi/chi/v5"
	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/internal/orders/utils"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/mobazha/mobazha3.0/pkg/posts/pb"
	"google.golang.org/protobuf/encoding/protojson"
)

// Post a post
func (g *Gateway) handlePOSTPost(w http.ResponseWriter, r *http.Request) {
	post := new(pb.Post)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("error reading request body: %s", err.Error()))
		return
	}
	if err := protojson.Unmarshal(body, post); err != nil {
		ErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("error unmarshaling post: %s", err.Error()))
		return
	}

	social := getSocialService(r)

	// If the post already exists in path, tell them to use PUT
	if post.Slug != "" && social.PostExist(post.Slug) {
		ErrorResponse(w, http.StatusConflict, "post exists. use PUT to update")
		return
	}

	err = social.AddPost(post, nil)
	if errors.Is(err, coreiface.ErrBadRequest) {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	} else if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	sanitizedStringResponse(w, fmt.Sprintf(`{"slug": "%s"}`, post.Slug))
}

// PUT a post
func (g *Gateway) handlePUTPost(w http.ResponseWriter, r *http.Request) {
	post := new(pb.Post)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("error reading request body: %s", err.Error()))
		return
	}
	if err := protojson.Unmarshal(body, post); err != nil {
		ErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("error unmarshaling post: %s", err.Error()))
		return
	}

	social := getSocialService(r)

	if !social.PostExist(post.Slug) {
		ErrorResponse(w, http.StatusConflict, "post does not exist. use POST to create")
		return
	}

	err = social.AddPost(post, nil)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	sanitizedStringResponse(w, `{}`)
}

// DELETE a post
func (g *Gateway) handleDELETEPost(w http.ResponseWriter, r *http.Request) {
	_, slug := path.Split(r.URL.Path)

	social := getSocialService(r)

	err := social.DeletePost(slug, nil)
	if errors.Is(err, coreiface.ErrNotFound) {
		ErrorResponse(w, http.StatusNotFound, err.Error())
		return
	} else if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	sanitizedStringResponse(w, `{}`)
}

// GET a list of posts (self or peer)
func (g *Gateway) handleGETPosts(w http.ResponseWriter, r *http.Request) {
	peerIDStr := chi.URLParam(r, "peerID")
	var (
		index []models.PostData
		err   error
	)

	social := getSocialService(r)
	if peerIDStr == "" || peerIDStr == getIdentityService(r).Identity().String() {
		index, err = social.GetMyPosts()
	} else {
		pid, perr := peer.Decode(peerIDStr)
		if perr != nil {
			ErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("invalid peer id: %s", perr.Error()))
			return
		}
		useCache, _ := strconv.ParseBool(r.URL.Query().Get("usecache"))

		index, err = social.GetPosts(r.Context(), pid, useCache)
	}
	if errors.Is(err, coreiface.ErrNotFound) {
		ErrorResponse(w, http.StatusNotFound, err.Error())
		return
	} else if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	sanitizedJSONResponse(w, index)
}

// GET a post (self)
func (g *Gateway) handleGETMyPost(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	social := getSocialService(r)

	signedPost, err := social.GetMyPostBySlug(slug)

	if errors.Is(err, coreiface.ErrNotFound) {
		ErrorResponse(w, http.StatusNotFound, err.Error())
		return
	} else if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	sanitizedProtobufResponse(w, signedPost)
}

// GET a post (peer)
func (g *Gateway) handleGETPost(w http.ResponseWriter, r *http.Request) {
	peerIDStr := chi.URLParam(r, "peerID")
	slug := chi.URLParam(r, "slug")

	social := getSocialService(r)

	var (
		post *pb.SignedPost
		err  error
	)
	if peerIDStr != "" && slug != "" { // Query by peerID/slug
		pid, perr := peer.Decode(peerIDStr)
		if perr != nil {
			ErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("invalid peer id: %s", perr.Error()))
			return
		}
		useCache, _ := strconv.ParseBool(r.URL.Query().Get("usecache"))
		post, err = social.GetPostBySlug(r.Context(), pid, slug, useCache)
	}

	if errors.Is(err, coreiface.ErrNotFound) {
		ErrorResponse(w, http.StatusNotFound, err.Error())
		return
	} else if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	sanitizedProtobufResponse(w, post)
}

func (g *Gateway) handlePOSTSignMessage(w http.ResponseWriter, r *http.Request) {
	type signRequest struct {
		Content string `json:"content"`
	}
	var (
		req signRequest
		err = json.NewDecoder(r.Body).Decode(&req)
	)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	identity := getIdentityService(r)

	sig, pubKey, err := identity.SignMessage([]byte(req.Content))
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	sanitizedStringResponse(w, fmt.Sprintf(`{"signature": "%s","pubkey":"%s","peerId":"%s"}`,
		hex.EncodeToString(sig),
		hex.EncodeToString(pubKey),
		identity.Identity().String()))
}

func (g *Gateway) handlePOSTVerifyMessage(w http.ResponseWriter, r *http.Request) {
	type ciphertext struct {
		Content   string `json:"content"`
		Signature string `json:"signature"`
		Pubkey    string `json:"pubkey"`
		PeerId    string `json:"peerId"`
	}
	var msg ciphertext
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&msg)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	keyBytes, err := hex.DecodeString(msg.Pubkey)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	sigBytes, err := hex.DecodeString(msg.Signature)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	peerID, err := verifyPayload([]byte(msg.Content), sigBytes, keyBytes)
	if err != nil {
		sanitizedStringResponse(w, `{"error":"VERIFICATION_FAILED"}`)
		return
	}

	if peerID != msg.PeerId {
		sanitizedStringResponse(w, `{"error":"PEER_ID_PUBKEY_MISMATCH"}`)
		return
	}
	sanitizedStringResponse(w, fmt.Sprintf(`{"error":"","peerId":"%s"}`, msg.PeerId))
}

func (g *Gateway) handlePOSTHashMessage(w http.ResponseWriter, r *http.Request) {
	type hashRequest struct {
		Content string `json:"content"`
	}
	var (
		req hashRequest
		err = json.NewDecoder(r.Body).Decode(&req)
	)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	messageHash, err := utils.MultihashSha256([]byte(req.Content))
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	sanitizedStringResponse(w, fmt.Sprintf(`{"hash": "%s"}`,
		messageHash.B58String()))
}
