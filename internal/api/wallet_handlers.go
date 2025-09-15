package api

import (
	"net/http"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/models"
)

func (g *Gateway) handleGETCurrencies(w http.ResponseWriter, r *http.Request) {
	sanitizedJSONResponse(w, models.CurrencyDefinitions)
}

func (g *Gateway) handleGETMnemonic(w http.ResponseWriter, r *http.Request) {
	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	ret, err := node.GetMnemonic()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	sanitizedJSONResponse(w, struct {
		Mnemonic string `json:"mnemonic"`
	}{
		Mnemonic: ret,
	})
}

func (g *Gateway) handleGETSystemInfo(w http.ResponseWriter, r *http.Request) {
	sanitizedJSONResponse(w, struct {
		Timestamp int64 `json:"timestamp"`
	}{
		Timestamp: time.Now().Unix(),
	})
}

func (g *Gateway) handlePOSTSpend(w http.ResponseWriter, r *http.Request) {
}
