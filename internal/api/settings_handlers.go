package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/gorilla/mux"
	"github.com/mobazha/mobazha3.0/internal/version"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/models"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

type nodeConfig struct {
	PeerID  string   `json:"peerID"`
	Testnet bool     `json:"testnet"`
	Tor     bool     `json:"tor"`
	Wallets []string `json:"wallets"`
}

func (g *Gateway) handleGETConfig(w http.ResponseWriter, r *http.Request) {
	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	ret := nodeConfig{
		PeerID:  node.Identity().String(),
		Testnet: node.UsingTestnet(),
		Tor:     node.UsingTorMode(),
	}

	for currency := range node.Multiwallet() {
		ret.Wallets = append(ret.Wallets, currency.CurrencyCode())
	}

	sanitizedJSONResponse(w, &ret)
}

func (g *Gateway) handlePutUserPreferences(w http.ResponseWriter, r *http.Request) {
	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	currentPrefs, err := node.GetPreferences()
	if err != nil && !errors.Is(err, coreiface.ErrNotFound) {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	var prefs models.UserPreferences
	if err == nil {
		prefsBytes, _ := json.Marshal(currentPrefs)
		request, _ := io.ReadAll(r.Body)
		patch, err := jsonpatch.MergePatch(prefsBytes, request)
		if err != nil {
			ErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}

		if err = json.Unmarshal(patch, &prefs); err != nil {
			ErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
	} else {
		decoder := json.NewDecoder(r.Body)

		if err := decoder.Decode(&prefs); err != nil {
			ErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	err = node.SavePreferences(&prefs, nil)
	if errors.Is(err, coreiface.ErrBadRequest) {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	} else if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	sanitizedJSONResponse(w, struct{}{})
}

func (g *Gateway) handleGetUserPreferences(w http.ResponseWriter, r *http.Request) {
	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	prefs, err := node.GetPreferences()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	prefs.UserAgent = version.UserAgent()
	sanitizedJSONResponse(w, prefs)
}

func (g *Gateway) handleGETExchangeRates(w http.ResponseWriter, r *http.Request) {
	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	currencyCode := mux.Vars(r)["currencyCode"]

	var base models.CurrencyCode
	if currencyCode != "" {
		def, err := models.CurrencyDefinitions.Lookup(currencyCode)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		base = *def.CurrencyCode()
	} else {
		base = models.CurrencyCode(iwallet.CtBitcoin)
	}
	rates, err := node.ExchangeRates().GetAllRates(base, false)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	sanitizedJSONResponse(w, rates)
}

func (g *Gateway) handlePOSTBulkUpdateCurrency(w http.ResponseWriter, r *http.Request) {
	// Do nothing. Listing中不再添加acceptedCurrencies，通过下单时卖家设置的货币来决定
	sanitizedStringResponse(w, `{"success": "true"}`)
}

func (g *Gateway) handlePOSTPublish(w http.ResponseWriter, r *http.Request) {
	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	node.Publish(nil)

	sanitizedStringResponse(w, "{}")
}

func (g *Gateway) handlePOSTPurgeCache(w http.ResponseWriter, r *http.Request) {
	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	ctx := context.Background()
	ch, err := node.IPFSNode().Blockstore.AllKeysChan(ctx)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	for id := range ch {
		if err := node.IPFSNode().Blockstore.DeleteBlock(ctx, id); err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	// Republish to IPNS
	node.Publish(nil)

	sanitizedStringResponse(w, "{}")
}

func (g *Gateway) handlePOSTShutdown(w http.ResponseWriter, r *http.Request) {
	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	node.Stop(true)
	os.Exit(1)

	sanitizedStringResponse(w, "{}")
}

func (g *Gateway) handleGETPeers(w http.ResponseWriter, r *http.Request) {
	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	peers := node.IPFSNode().PeerHost.Network().Peers()

	var ret []string
	for _, p := range peers {
		ret = append(ret, p.String())
	}

	sanitizedJSONResponse(w, ret)
}
