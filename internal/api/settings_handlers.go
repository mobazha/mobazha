package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/mobazha/mobazha3.0/internal/core/coreiface"
	"github.com/mobazha/mobazha3.0/internal/models"
	iwallet "github.com/mobazha/mobazha3.0/internal/multiwallet/wallet-interface"
	pb "github.com/mobazha/mobazha3.0/internal/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/internal/version"
	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/gorilla/mux"
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
		base = iwallet.CtBitcoin
	}
	rates, err := node.ExchangeRates().GetAllRates(base, false)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	sanitizedJSONResponse(w, rates)
}

func (g *Gateway) handlePOSTBulkUpdateCurrency(w http.ResponseWriter, r *http.Request) {
	type BulkUpdateRequest struct {
		Currencies []string `json:"currencies"`
	}

	var bulkUpdate BulkUpdateRequest
	err := json.NewDecoder(r.Body).Decode(&bulkUpdate)
	if err != nil {
		ErrorResponse(w, 400, err.Error())
		return
	}

	// Check for no currencies selected
	if len(bulkUpdate.Currencies) == 0 {
		sanitizedStringResponse(w, `{"success": "false", "reason":"No currencies specified"}`)
		return
	}

	log.Info("Updating currencies for all listings to: ", bulkUpdate.Currencies)

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	done := make(chan struct{})
	err = node.UpdateAllListings(func(listing *pb.Listing) (bool, error) {
		listing.Metadata.AcceptedCurrencies = bulkUpdate.Currencies
		return true, nil
	}, done)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	select {
	case <-done:
		sanitizedStringResponse(w, `{"success": "true"}`)
		return
	case <-time.After(time.Second * 300):
		ErrorResponse(w, http.StatusInternalServerError, "timeout waiting on channel")
		return
	}
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
