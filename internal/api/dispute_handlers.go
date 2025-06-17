package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/models"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

func (g *Gateway) handlePOSTOpenDispute(w http.ResponseWriter, r *http.Request) {
	type dispute struct {
		OrderID string `json:"orderID"`
		Claim   string `json:"claim"`
	}
	decoder := json.NewDecoder(r.Body)
	var d dispute
	err := decoder.Decode(&d)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	done := make(chan struct{})
	err = node.OpenDispute(models.OrderID(d.OrderID), d.Claim, done)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	select {
	case <-done:
		sanitizedStringResponse(w, `{}`)
		return
	case <-time.After(time.Second * 10):
		ErrorResponse(w, http.StatusInternalServerError, "timeout waiting on channel")
		return
	}
}

func (g *Gateway) handlePOSTCloseDispute(w http.ResponseWriter, r *http.Request) {
	type disputeParams struct {
		OrderID          string  `json:"orderID"`
		Resolution       string  `json:"resolution"`
		BuyerPercentage  float32 `json:"buyerPercentage"`
		VendorPercentage float32 `json:"vendorPercentage"`
	}
	decoder := json.NewDecoder(r.Body)
	var d disputeParams
	err := decoder.Decode(&d)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	done := make(chan struct{})
	err = node.CloseDispute(models.OrderID(d.OrderID), d.BuyerPercentage, d.VendorPercentage, d.Resolution, done)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	select {
	case <-done:
		sanitizedStringResponse(w, `{}`)
		return
	case <-time.After(time.Second * 10):
		ErrorResponse(w, http.StatusInternalServerError, "timeout waiting on channel")
		return
	}
}

func (g *Gateway) handlePOSTReleaseFunds(w http.ResponseWriter, r *http.Request) {
	type release struct {
		OrderID string `json:"orderID"`
		TxID    string `json:"txID"`
	}
	decoder := json.NewDecoder(r.Body)
	var rel release
	err := decoder.Decode(&rel)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)
	done := make(chan struct{})
	err = node.ReleaseFunds(models.OrderID(rel.OrderID), iwallet.TransactionID(rel.TxID), done)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	select {
	case <-done:
		sanitizedStringResponse(w, `{}`)
		return
	case <-time.After(time.Second * 10):
		ErrorResponse(w, http.StatusInternalServerError, "timeout waiting on channel")
		return
	}
}

func (g *Gateway) handlePOSTReleaseEscrow(w http.ResponseWriter, r *http.Request) {
	type release struct {
		OrderID string `json:"orderID"`
	}
	decoder := json.NewDecoder(r.Body)
	var rel release
	err := decoder.Decode(&rel)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	done := make(chan struct{})
	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)
	err = node.ReleaseFundsAfterTimeout(models.OrderID(rel.OrderID), done)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	select {
	case <-done:
		sanitizedStringResponse(w, `{}`)
		return
	case <-time.After(time.Second * 10):
		ErrorResponse(w, http.StatusInternalServerError, "timeout waiting on channel")
		return
	}
}

func (g *Gateway) handleGETReleaseFundsInstructions(w http.ResponseWriter, r *http.Request) {
	type RequestParams struct {
		OrderID          string `json:"orderID"`
		InitiatorAddress string `json:"initiatorAddress"`
	}
	decoder := json.NewDecoder(r.Body)
	var params RequestParams
	err := decoder.Decode(&params)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)
	coinType, instructions, err := node.GetReleaseFundsInstructions(models.OrderID(params.OrderID), params.InitiatorAddress)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	coinInfo, err := iwallet.CoinInfoFromCoinType(coinType)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	chainType := coinInfo.Chain

	type InstructionsResponse struct {
		HasInstructions bool              `json:"hasInstructions"`
		Instructions    any               `json:"instructions"`
		PaymentChain    iwallet.ChainType `json:"paymentChain"`
	}

	// 返回响应
	response := InstructionsResponse{
		HasInstructions: true,
		Instructions:    instructions,
		PaymentChain:    chainType,
	}
	json.NewEncoder(w).Encode(response)
}
