package api

import (
	"encoding/json"
	"net/http"

	"github.com/mobazha/mobazha3.0/internal/core/coreiface"
	"github.com/mobazha/mobazha3.0/internal/models"
	"github.com/gorilla/mux"
	peer "github.com/libp2p/go-libp2p/core/peer"
)

func (g *Gateway) handleGETCartsItemsCount(w http.ResponseWriter, r *http.Request) {
	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)
	count, _ := node.GetCartsTotalItemsCount()
	sanitizedJSONResponse(w, count)
}

func (g *Gateway) handleGETCarts(w http.ResponseWriter, r *http.Request) {
	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)
	carts, err := node.GetCarts()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	sanitizedJSONResponse(w, carts)
}

func (g *Gateway) handleClearCarts(w http.ResponseWriter, r *http.Request) {
	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)
	err := node.ClearAllCarts()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	sanitizedStringResponse(w, `{"success": "true"}`)
}

func (g *Gateway) handleAddToCart(w http.ResponseWriter, r *http.Request) {
	vendorIDStr := mux.Vars(r)["peerID"]
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

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)
	err = node.AddToCart(pid, cartItem)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	sanitizedStringResponse(w, `{"success": "true"}`)
}

func (g *Gateway) handleRemoveCartItem(w http.ResponseWriter, r *http.Request) {
	vendorIDStr := mux.Vars(r)["peerID"]
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

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)
	err = node.RemoveCartItem(pid, cartItem)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	sanitizedStringResponse(w, `{"success": "true"}`)
}
