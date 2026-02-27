package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/mobazha/mobazha3.0/pkg/models"
	responsePkg "github.com/mobazha/mobazha3.0/pkg/response"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

func (g *Gateway) handlePOSTOpenDispute(w http.ResponseWriter, r *http.Request) {
	orderID := mux.Vars(r)["orderID"]
	if orderID == "" {
		ErrorResponse(w, http.StatusBadRequest, "missing orderID")
		return
	}

	type dispute struct {
		Claim string `json:"claim"`
	}
	decoder := json.NewDecoder(r.Body)
	var d dispute
	err := decoder.Decode(&d)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	node := getOrderService(r)

	done := make(chan struct{})
	err = node.OpenDispute(models.OrderID(orderID), d.Claim, done)
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
	orderID := mux.Vars(r)["orderID"]
	if orderID == "" {
		ErrorResponse(w, http.StatusBadRequest, "missing orderID")
		return
	}

	type disputeParams struct {
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

	node := getOrderService(r)

	done := make(chan struct{})
	err = node.CloseDispute(models.OrderID(orderID), d.BuyerPercentage, d.VendorPercentage, d.Resolution, done)
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
	orderID := mux.Vars(r)["orderID"]
	if orderID == "" {
		ErrorResponse(w, http.StatusBadRequest, "missing orderID")
		return
	}

	type release struct {
		TxID string `json:"txID"`
	}
	decoder := json.NewDecoder(r.Body)
	var rel release
	err := decoder.Decode(&rel)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	node := getOrderService(r)
	done := make(chan struct{})
	err = node.ReleaseFunds(models.OrderID(orderID), iwallet.TransactionID(rel.TxID), done)
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
	orderID := mux.Vars(r)["orderID"]
	if orderID == "" {
		ErrorResponse(w, http.StatusBadRequest, "missing orderID")
		return
	}

	done := make(chan struct{})
	node := getOrderService(r)
	err := node.ReleaseFundsAfterTimeout(models.OrderID(orderID), done)
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
	orderID := mux.Vars(r)["orderID"]
	if orderID == "" {
		ErrorResponse(w, http.StatusBadRequest, "missing orderID")
		return
	}

	type RequestParams struct {
		InitiatorAddress string `json:"initiatorAddress"`
	}
	decoder := json.NewDecoder(r.Body)
	var params RequestParams
	err := decoder.Decode(&params)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	node := getOrderService(r)
	coinType, instructions, err := node.GetReleaseFundsInstructions(models.OrderID(orderID), params.InitiatorAddress)
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
		Instructions    any               `json:"instructions,omitempty"`
		PaymentChain    iwallet.ChainType `json:"paymentChain"`
	}

	// 返回响应
	// For UTXO chains, instructions is nil - frontend should call /v1/disputes/{orderID}/release directly
	response := InstructionsResponse{
		HasInstructions: instructions != nil,
		Instructions:    instructions,
		PaymentChain:    chainType,
	}
	responsePkg.Success(w, response)
}
