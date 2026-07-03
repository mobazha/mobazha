package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/models"
	responsePkg "github.com/mobazha/mobazha/pkg/response"
)

func getWishlistService(r *http.Request) contracts.WishlistService {
	return getNodeService(r).Wishlist()
}

func (g *Gateway) handleGETWishlists(w http.ResponseWriter, r *http.Request) {
	svc := getWishlistService(r)
	if svc == nil {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "Wishlist not available")
		return
	}
	items, err := svc.GetWishlist()
	if err != nil {
		log.Warningf("Failed to get wishlist: %v", err)
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "Failed to load wishlist")
		return
	}
	if items == nil {
		items = make([]models.WishlistItem, 0)
	}
	responsePkg.Success(w, items)
}

func (g *Gateway) handlePOSTWishlist(w http.ResponseWriter, r *http.Request) {
	svc := getWishlistService(r)
	if svc == nil {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "Wishlist not available")
		return
	}

	var req struct {
		PeerID    string `json:"peerID"`
		Slug      string `json:"slug"`
		Title     string `json:"title"`
		Thumbnail string `json:"thumbnail"`
		Price     string `json:"price"`
		Currency  string `json:"currency"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "Invalid request body")
		return
	}
	if req.PeerID == "" || req.Slug == "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation, "peerID and slug are required")
		return
	}

	item := models.WishlistItem{
		VendorPeerID: req.PeerID,
		Slug:         req.Slug,
		Title:        req.Title,
		Thumbnail:    req.Thumbnail,
		Price:        req.Price,
		Currency:     req.Currency,
	}

	created, err := svc.AddToWishlist(item)
	if err != nil {
		if errors.Is(err, contracts.ErrWishlistFull) {
			responsePkg.Error(w, http.StatusConflict, responsePkg.CodeConflict, err.Error())
			return
		}
		log.Warningf("Failed to add to wishlist: %v", err)
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "Failed to add to wishlist")
		return
	}
	responsePkg.Created(w, created)
}

func (g *Gateway) handleDELETEWishlist(w http.ResponseWriter, r *http.Request) {
	svc := getWishlistService(r)
	if svc == nil {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "Wishlist not available")
		return
	}

	peerID := chi.URLParam(r, "peerID")
	slug := chi.URLParam(r, "slug")
	if peerID == "" || slug == "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation, "peerID and slug are required")
		return
	}

	if err := svc.RemoveFromWishlist(peerID, slug); err != nil {
		log.Warningf("Failed to remove from wishlist: %v", err)
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "Failed to remove from wishlist")
		return
	}
	responsePkg.NoContent(w)
}
