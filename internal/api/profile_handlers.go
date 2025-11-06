package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/gorilla/mux"
	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/models"
)

func (g *Gateway) handleGETProfile(w http.ResponseWriter, r *http.Request) {
	peerIDStr := mux.Vars(r)["peerID"]

	useCache, _ := strconv.ParseBool(r.URL.Query().Get("usecache"))

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	var (
		profile *models.Profile
		err     error
	)
	if peerIDStr == "" || peerIDStr == node.Identity().String() {
		profile, err = node.GetMyProfile()
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
		reqCtx := extractRequestContext(r)
		profile, err = node.GetProfile(r.Context(), pid, reqCtx, useCache)
		if errors.Is(err, coreiface.ErrNotFound) {
			ErrorResponse(w, http.StatusNotFound, err.Error())
			return
		} else if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	sanitizedJSONResponse(w, profile)
}

func (g *Gateway) handlePOSTProfile(w http.ResponseWriter, r *http.Request) {
	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	peerIDStr := mux.Vars(r)["peerID"]
	if peerIDStr != "" && peerIDStr != node.Identity().String() {
		ErrorResponse(w, http.StatusConflict, "profile id doesn't match with local")
		return
	}

	if _, err := node.GetMyProfile(); !errors.Is(err, coreiface.ErrNotFound) {
		ErrorResponse(w, http.StatusConflict, "profile exists. use PUT to update")
		return
	}

	var profile models.Profile
	if err := json.NewDecoder(r.Body).Decode(&profile); err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := node.SetProfile(&profile, nil); err != nil {
		if errors.Is(err, coreiface.ErrBadRequest) {
			ErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		} else if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	sanitizedJSONResponse(w, struct{}{})
}

func (g *Gateway) handlePUTProfile(w http.ResponseWriter, r *http.Request) {
	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	peerIDStr := mux.Vars(r)["peerID"]
	if peerIDStr != "" && peerIDStr != node.Identity().String() {
		ErrorResponse(w, http.StatusBadRequest, "profile id doesn't match with local")
		return
	}

	myProfile, err := node.GetMyProfile()
	if errors.Is(err, coreiface.ErrNotFound) {
		ErrorResponse(w, http.StatusConflict, "profile does not exists. use POST to create")
		return
	}
	profileBytes, _ := json.Marshal(myProfile)

	request, _ := io.ReadAll(r.Body)
	patch, err := jsonpatch.MergePatch(profileBytes, request)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	var profile models.Profile
	if json.Unmarshal(patch, &profile); err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := node.SetProfile(&profile, nil); err != nil {
		if errors.Is(err, coreiface.ErrBadRequest) {
			ErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		} else if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	sanitizedJSONResponse(w, struct{}{})
}

func (g *Gateway) handlePOSTFetchProfiles(w http.ResponseWriter, r *http.Request) {
	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	// useCache, _ := strconv.ParseBool(r.URL.Query().Get("usecache"))
	useCache := false
	async, _ := strconv.ParseBool(r.URL.Query().Get("async"))

	var peerIDs []string
	if err := json.NewDecoder(r.Body).Decode(&peerIDs); err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	type profileWithAsyncID struct {
		ID      string         `json:"id"`
		PeerID  string         `json:"peerID"`
		Profile models.Profile `json:"profile"`
	}

	type profileError struct {
		ID     string `json:"id"`
		PeerID string `json:"peerID"`
		Error  string `json:"error"`
	}

	var (
		profiles     = make([]profileWithAsyncID, 0, len(peerIDs))
		responseChan = make(chan interface{}, 8)
		wg           sync.WaitGroup
	)

	wg.Add(len(peerIDs))
	go func() {
		for _, peerIDStr := range peerIDs {
			pid, err := peer.Decode(peerIDStr)
			if err != nil {
				responseChan <- profileError{
					PeerID: peerIDStr,
					Error:  err.Error(),
				}
				wg.Done()
				continue
			}
			go func(p peer.ID) {
				defer wg.Done()
				ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
				defer cancel()

				profile, err := node.GetProfile(ctx, p, nil, useCache)
				if err != nil {
					responseChan <- profileError{
						PeerID: p.String(),
						Error:  err.Error(),
					}
					return
				}
				responseChan <- profileWithAsyncID{
					PeerID:  p.String(),
					Profile: *profile,
				}
			}(pid)
		}
		wg.Wait()
		close(responseChan)
	}()

	if !async {
		for i := range responseChan {
			switch p := i.(type) {
			case profileWithAsyncID:
				profiles = append(profiles, p)
			}
		}
		sanitizedJSONResponse(w, profiles)
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
				case profileWithAsyncID:
					p.ID = asyncID
					g.NotifyWebsockets(node.GetNodeID())(p)
				case profileError:
					p.ID = asyncID
					g.NotifyWebsockets(node.GetNodeID())(p)
				}
			}
		}()
	}
}

func (g *Gateway) handleSetModerator(w http.ResponseWriter, r *http.Request) {
	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)
	var moderatorInfo models.ModeratorInfo
	if err := json.NewDecoder(r.Body).Decode(&moderatorInfo); err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	// Save self as moderator
	done := make(chan struct{})
	err := node.SetSelfAsModerator(r.Context(), &moderatorInfo, done)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	select {
	case <-done:
		sanitizedStringResponse(w, `{}`)
		return
	case <-time.After(time.Second * 10):
		ErrorResponse(w, http.StatusInternalServerError, "timeout waiting on channel")
		return
	}
}

func (g *Gateway) handleUnsetModerator(w http.ResponseWriter, r *http.Request) {
	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	done := make(chan struct{})
	err := node.RemoveSelfAsModerator(r.Context(), done)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	select {
	case <-done:
		sanitizedStringResponse(w, `{}`)
		return
	case <-time.After(time.Second * 10):
		ErrorResponse(w, http.StatusInternalServerError, "timeout waiting on channel")
		return
	}
}

func (g *Gateway) handleGetModerators(w http.ResponseWriter, r *http.Request) {
	async, _ := strconv.ParseBool(r.URL.Query().Get("async"))
	include := r.URL.Query().Get("include")
	// useCache, _ := strconv.ParseBool(r.URL.Query().Get("usecache"))
	useCache := false

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	if async {
		id := r.URL.Query().Get("asyncID")
		if id == "" {
			return
		}

		type resp struct {
			ID string `json:"id"`
		}
		response := resp{id}
		w.WriteHeader(http.StatusAccepted)
		sanitizedJSONResponse(w, response)
		go func() {
			found := make(map[string]bool)
			foundMu := sync.Mutex{}

			notifyModInfo := func(pidStr string) {
				pid, err := peer.Decode(pidStr)
				if err != nil {
					return
				}

				// Check and set the peer in `found` with locking
				foundMu.Lock()
				if found[pidStr] {
					foundMu.Unlock()
					return
				}
				found[pidStr] = true
				foundMu.Unlock()

				if strings.ToLower(include) == "profile" {
					ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
					defer cancel()
					profile, err := node.GetProfile(ctx, pid, nil, useCache)
					if err != nil {
						return
					}

					type PeerAndProfileWithID struct {
						Id      string          `json:"id,omitempty"`
						PeerId  string          `json:"peerId,omitempty"`
						Profile *models.Profile `json:"profile,omitempty"`
					}
					resp := PeerAndProfileWithID{Id: id, PeerId: pidStr, Profile: profile}
					g.NotifyWebsockets(node.GetNodeID())(resp)
				} else {
					type wsResp struct {
						ID     string `json:"id"`
						PeerID string `json:"peerId"`
					}
					resp := wsResp{id, pidStr}
					g.NotifyWebsockets(node.GetNodeID())(resp)
				}
			}

			for _, mod := range node.GetVerifiedModerators(context.Background()) {
				go notifyModInfo(mod.String())
			}

			for mod := range node.GetModeratorsAsync(context.Background()) {
				go notifyModInfo(mod.String())
			}
		}()
	} else {
		moderatorIDs := node.GetModerators(context.Background())

		verifiedModIDs := node.GetVerifiedModerators(context.Background())
		moderatorIDs = append(moderatorIDs, verifiedModIDs...)

		if strings.ToLower(include) == "profile" {
			var profiles []*models.Profile
			for _, pid := range moderatorIDs {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
				defer cancel()
				profile, err := node.GetProfile(ctx, pid, nil, useCache)
				if err != nil {
					continue
				}
				profiles = append(profiles, profile)
			}
			sanitizedJSONResponse(w, profiles)
		} else {
			var ids []string
			for _, id := range moderatorIDs {
				ids = append(ids, id.String())
			}
			sanitizedJSONResponse(w, ids)
		}
	}
}

func (g *Gateway) handleBlockNode(w http.ResponseWriter, r *http.Request) {
	_, peerID := path.Split(r.URL.Path)

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	_, err := node.BlockNode(peerID)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	sanitizedStringResponse(w, `{}`)
}

func (g *Gateway) handleUnBlockNode(w http.ResponseWriter, r *http.Request) {
	_, peerID := path.Split(r.URL.Path)

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	_, err := node.UnblockNode(peerID)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	sanitizedStringResponse(w, `{}`)
}
