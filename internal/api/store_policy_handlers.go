package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha/internal/database"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/core/coreiface"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/mobazha/mobazha/pkg/response"
)

func getStorePolicyService(r *http.Request) (contracts.StorePolicyService, bool) {
	provider, ok := getNodeService(r).(contracts.StorePolicyProvider)
	if !ok {
		return nil, false
	}
	svc := provider.StorePolicy()
	if svc == nil {
		return nil, false
	}
	return svc, true
}

func storePolicyErrorResponse(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, database.ErrStorePolicyConflict):
		response.Error(w, http.StatusConflict, response.CodeConflict, "Store policy revision conflict")
	case errors.Is(err, coreiface.ErrBadRequest):
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, err.Error())
	default:
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "Internal server error")
	}
}

func (g *Gateway) handleGetStorePolicy(w http.ResponseWriter, r *http.Request) {
	svc, ok := getStorePolicyService(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Store policy not available")
		return
	}
	policy, err := svc.GetPolicy(r.Context())
	if err != nil {
		storePolicyErrorResponse(w, err)
		return
	}
	response.Success(w, policy)
}

func (g *Gateway) handlePutStorePolicyModerators(w http.ResponseWriter, r *http.Request) {
	svc, ok := getStorePolicyService(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Store policy not available")
		return
	}
	var req models.StorePolicyModeratorsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "Invalid request body")
		return
	}
	policy, err := svc.ReplaceModerators(r.Context(), req.ExpectedRevision, req.Moderators)
	if err != nil {
		storePolicyErrorResponse(w, err)
		return
	}
	response.Success(w, policy)
}

func (g *Gateway) handleGetStorePolicyModerators(w http.ResponseWriter, r *http.Request) {
	svc, ok := getStorePolicyService(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Store policy not available")
		return
	}
	policy, err := svc.GetPolicy(r.Context())
	if err != nil {
		storePolicyErrorResponse(w, err)
		return
	}
	response.Success(w, policy.Moderators)
}

func (g *Gateway) handlePostStorePolicyModerator(w http.ResponseWriter, r *http.Request) {
	svc, ok := getStorePolicyService(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Store policy not available")
		return
	}
	var req models.StorePolicyModeratorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "Invalid request body")
		return
	}
	policy, err := svc.UpsertModerator(r.Context(), req.ExpectedRevision, models.StorePolicyModeratorInput{
		PeerID:   req.PeerID,
		Enabled:  req.Enabled,
		Position: req.Position,
	})
	if err != nil {
		storePolicyErrorResponse(w, err)
		return
	}
	response.Success(w, policy)
}

func (g *Gateway) handleDeleteStorePolicyModerator(w http.ResponseWriter, r *http.Request) {
	svc, ok := getStorePolicyService(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Store policy not available")
		return
	}
	var req models.StorePolicyDeleteModeratorRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	policy, err := svc.RemoveModerator(r.Context(), req.ExpectedRevision, chi.URLParam(r, "peerID"))
	if err != nil {
		storePolicyErrorResponse(w, err)
		return
	}
	response.Success(w, policy)
}

func (g *Gateway) handleGetPublishedStorePolicy(w http.ResponseWriter, r *http.Request) {
	peerIDStr := chi.URLParam(r, "peerID")
	pid, err := peer.Decode(peerIDStr)
	if err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "Invalid store peer ID")
		return
	}
	if pid != getIdentityService(r).Identity() {
		response.Error(w, http.StatusNotFound, response.CodeNotFound, "Store policy not found")
		return
	}

	svc, ok := getStorePolicyService(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Store policy not available")
		return
	}
	policy, err := svc.GetPublishedPolicy(r.Context())
	if err != nil {
		storePolicyErrorResponse(w, err)
		return
	}
	response.Success(w, policy)
}
