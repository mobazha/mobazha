package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/mobazha/mobazha3.0/pkg/response"
)

func getCollectionService(r *http.Request) (contracts.CollectionService, bool) {
	cp, ok := getNodeService(r).(contracts.CollectionProvider)
	if !ok {
		return nil, false
	}
	svc := cp.Collection()
	if svc == nil {
		return nil, false
	}
	return svc, true
}

func collectionErrorResponse(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, database.ErrCollectionNotFound):
		response.Error(w, http.StatusNotFound, response.CodeNotFound, "Collection not found")
	case errors.Is(err, database.ErrCollectionTitleRequired),
		errors.Is(err, database.ErrCollectionProductRequired):
		response.Error(w, http.StatusBadRequest, response.CodeValidation, err.Error())
	case errors.Is(err, database.ErrCollectionMaxReached),
		errors.Is(err, database.ErrCollectionProductMaxExceeded),
		errors.Is(err, database.ErrDuplicateCollectionProduct):
		response.Error(w, http.StatusConflict, response.CodeConflict, err.Error())
	default:
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "Internal server error")
	}
}

func (g *Gateway) handleCreateCollection(w http.ResponseWriter, r *http.Request) {
	svc, ok := getCollectionService(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Collections not available")
		return
	}

	var c models.Collection
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "Invalid request body")
		return
	}

	if err := svc.CreateCollection(r.Context(), &c); err != nil {
		collectionErrorResponse(w, err)
		return
	}
	response.Created(w, c)
}

func (g *Gateway) handleListCollections(w http.ResponseWriter, r *http.Request) {
	svc, ok := getCollectionService(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Collections not available")
		return
	}

	page := intQueryParam(r, "page", 1)
	pageSize := intQueryParam(r, "pageSize", 20)
	publishedOnly := r.URL.Query().Get("publishedOnly") == "true"

	collections, total, err := svc.ListCollections(r.Context(), page, pageSize, publishedOnly)
	if err != nil {
		collectionErrorResponse(w, err)
		return
	}
	response.List(w, collections, response.Meta{
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	})
}

func (g *Gateway) handleGetCollection(w http.ResponseWriter, r *http.Request) {
	svc, ok := getCollectionService(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Collections not available")
		return
	}

	id := chi.URLParam(r, "collectionID")
	c, err := svc.GetCollection(r.Context(), id)
	if err != nil {
		collectionErrorResponse(w, err)
		return
	}
	response.Success(w, c)
}

func (g *Gateway) handleUpdateCollection(w http.ResponseWriter, r *http.Request) {
	svc, ok := getCollectionService(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Collections not available")
		return
	}

	id := chi.URLParam(r, "collectionID")
	existing, err := svc.GetCollection(r.Context(), id)
	if err != nil {
		collectionErrorResponse(w, err)
		return
	}

	var patch struct {
		Title       *string                     `json:"title"`
		Description *string                     `json:"description"`
		Image       *string                     `json:"image"`
		SortOrder   *models.CollectionSortOrder `json:"sortOrder"`
		Published   *bool                       `json:"published"`
	}
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "Invalid request body")
		return
	}

	if patch.Title != nil {
		existing.Title = *patch.Title
	}
	if patch.Description != nil {
		existing.Description = *patch.Description
	}
	if patch.Image != nil {
		existing.Image = *patch.Image
	}
	if patch.SortOrder != nil {
		existing.SortOrder = *patch.SortOrder
	}
	if patch.Published != nil {
		existing.Published = *patch.Published
	}

	if err := svc.UpdateCollection(r.Context(), existing); err != nil {
		collectionErrorResponse(w, err)
		return
	}
	response.Success(w, existing)
}

func (g *Gateway) handleDeleteCollection(w http.ResponseWriter, r *http.Request) {
	svc, ok := getCollectionService(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Collections not available")
		return
	}

	id := chi.URLParam(r, "collectionID")
	if err := svc.DeleteCollection(r.Context(), id); err != nil {
		collectionErrorResponse(w, err)
		return
	}
	response.NoContent(w)
}

func (g *Gateway) handleAddCollectionProducts(w http.ResponseWriter, r *http.Request) {
	svc, ok := getCollectionService(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Collections not available")
		return
	}

	collectionID := chi.URLParam(r, "collectionID")
	var req struct {
		Slugs []string `json:"slugs"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Slugs) == 0 {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "slugs array is required")
		return
	}

	if err := svc.AddProducts(r.Context(), collectionID, req.Slugs); err != nil {
		collectionErrorResponse(w, err)
		return
	}
	response.NoContent(w)
}

func (g *Gateway) handleRemoveCollectionProduct(w http.ResponseWriter, r *http.Request) {
	svc, ok := getCollectionService(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Collections not available")
		return
	}

	collectionID := chi.URLParam(r, "collectionID")
	slug := chi.URLParam(r, "slug")

	if err := svc.RemoveProduct(r.Context(), collectionID, slug); err != nil {
		collectionErrorResponse(w, err)
		return
	}
	response.NoContent(w)
}

func (g *Gateway) handleReorderCollectionProducts(w http.ResponseWriter, r *http.Request) {
	svc, ok := getCollectionService(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Collections not available")
		return
	}

	collectionID := chi.URLParam(r, "collectionID")
	var req struct {
		Slugs []string `json:"slugs"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Slugs) == 0 {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "ordered slugs array is required")
		return
	}

	if err := svc.ReorderProducts(r.Context(), collectionID, req.Slugs); err != nil {
		collectionErrorResponse(w, err)
		return
	}
	response.NoContent(w)
}

func (g *Gateway) handleGetCollectionPublic(w http.ResponseWriter, r *http.Request) {
	svc, ok := getCollectionService(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Collections not available")
		return
	}

	id := chi.URLParam(r, "collectionID")
	c, err := svc.GetCollection(r.Context(), id)
	if err != nil || !c.Published {
		response.Error(w, http.StatusNotFound, response.CodeNotFound, "Collection not found")
		return
	}
	response.Success(w, c)
}

func (g *Gateway) handleListCollectionsPublic(w http.ResponseWriter, r *http.Request) {
	svc, ok := getCollectionService(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Collections not available")
		return
	}

	page := intQueryParam(r, "page", 1)
	pageSize := intQueryParam(r, "pageSize", 20)

	collections, total, err := svc.ListCollections(r.Context(), page, pageSize, true)
	if err != nil {
		collectionErrorResponse(w, err)
		return
	}
	response.List(w, collections, response.Meta{
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	})
}
