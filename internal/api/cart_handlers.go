package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha/pkg/models"
)

func (g *Gateway) handleGETCartsItemsCount(w http.ResponseWriter, r *http.Request) {
	svc := getShoppingCartService(r)
	count, _ := svc.GetCartsTotalItemsCount()
	sanitizedJSONResponse(w, count)
}

func (g *Gateway) handleGETCarts(w http.ResponseWriter, r *http.Request) {
	svc := getShoppingCartService(r)
	carts, err := svc.GetCarts()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	sanitizedJSONResponse(w, carts)
}

func (g *Gateway) handleClearCarts(w http.ResponseWriter, r *http.Request) {
	svc := getShoppingCartService(r)
	err := svc.ClearAllCarts()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	sanitizedStringResponse(w, `{"success": "true"}`)
}

func (g *Gateway) handleAddToCart(w http.ResponseWriter, r *http.Request) {
	vendorIDStr := chi.URLParam(r, "peerID")
	pid, err := peer.Decode(vendorIDStr)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	decoder := json.NewDecoder(r.Body)
	var cartItem models.ShoppingCartItem
	err = decoder.Decode(&cartItem)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, wrapError(err))
		return
	}

	svc := getShoppingCartService(r)
	err = svc.AddToCart(pid, cartItem)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	sanitizedStringResponse(w, `{"success": "true"}`)
}

func (g *Gateway) handleRemoveCartItem(w http.ResponseWriter, r *http.Request) {
	vendorIDStr := chi.URLParam(r, "peerID")
	pid, err := peer.Decode(vendorIDStr)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	decoder := json.NewDecoder(r.Body)
	var cartItem models.ShoppingCartItem
	err = decoder.Decode(&cartItem)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, wrapError(err))
		return
	}

	svc := getShoppingCartService(r)
	err = svc.RemoveCartItem(pid, cartItem)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	sanitizedStringResponse(w, `{"success": "true"}`)
}
