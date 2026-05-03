//go:build !private_distribution

package api

import (
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/mobazha/mobazha3.0/internal/wallet"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/mobazha/mobazha3.0/pkg/response"
)

type nodeConfig struct {
	PeerID  string   `json:"peerID"`
	Testnet bool     `json:"testnet"`
	Tor     bool     `json:"tor"`
	Wallets []string `json:"wallets"`
}

func (g *Gateway) handleGETConfig(w http.ResponseWriter, r *http.Request) {
	identity := getIdentityService(r)

	ret := nodeConfig{
		PeerID:  identity.Identity().String(),
		Testnet: identity.UsingTestnet(),
	}

	// Tor and Wallets are only available on full nodes (CoreIface).
	// In SaaS mode these default to false/empty, which is correct.
	if ci, ok := getCoreIface(r); ok {
		ret.Tor = ci.UsingTorMode()
		if mw := ci.Multiwallet(); mw != nil {
			for _, chain := range mw.SupportedChains() {
				ret.Wallets = append(ret.Wallets, chain.String())
			}
		}
	}

	sanitizedJSONResponse(w, &ret)
}

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

func (g *Gateway) handlePOSTShutdown(w http.ResponseWriter, r *http.Request) {
	node, ok := getCoreIface(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Not available in SaaS mode")
		return
	}

	node.Stop(true)
	os.Exit(1)

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
