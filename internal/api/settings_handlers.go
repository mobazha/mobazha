//go:build !private_distribution

package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/mobazha/mobazha3.0/internal/wallet"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/mobazha/mobazha3.0/pkg/response"
)

func (g *Gateway) handleGETExchangeRates(w http.ResponseWriter, r *http.Request) {
	exchange := getExchangeRateService(r)

	currencyCode := chi.URLParam(r, "currencyCode")

	var base models.CurrencyCode
	if currencyCode != "" {
		def, err := models.CurrencyDefinitions.Lookup(currencyCode)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		base = *def.CurrencyCode()
	} else {
		base = wallet.ReserveCurrency
	}

	forceRefresh := r.URL.Query().Get("refresh") == "true"
	rates, err := exchange.GetAllRates(base, forceRefresh)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	sanitizedJSONResponse(w, rates)
}

func (g *Gateway) handlePOSTPublish(w http.ResponseWriter, r *http.Request) {
	node := getNodeService(r)

	node.Publish(nil)

	sanitizedStringResponse(w, "{}")
}

func (g *Gateway) handlePOSTPurgeCache(w http.ResponseWriter, r *http.Request) {
	node := getNodeService(r)

	node.Publish(nil)

	sanitizedStringResponse(w, "{}")
}

func (g *Gateway) handleGETPeers(w http.ResponseWriter, r *http.Request) {
	node, ok := getCoreIface(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Not available in SaaS mode")
		return
	}

	peers := node.PeerHost().Network().Peers()

	var ret []string
	for _, p := range peers {
		ret = append(ret, p.String())
	}

	sanitizedJSONResponse(w, ret)
}
