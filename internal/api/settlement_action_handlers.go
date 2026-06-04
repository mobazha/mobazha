//go:build !private_distribution

package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"

	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	responsePkg "github.com/mobazha/mobazha3.0/pkg/response"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// handlePOSTOrderSettlementAction executes the default backend settlement
// surface via ChainEscrowV2.
//
// POST /v1/orders/{orderID}/settlement-actions/{action}
//
// Path action (case-insensitive): "confirm" | "cancel" | "complete" | "dispute-release".
//
// Body JSON (optional, camelCase):
//   - payoutAddress — vendor payout (confirm) or buyer refund override (cancel).
//
// This is the primary settlement entrypoint for backend-submitted routes such
// as ManagedEscrow-backed EVM. Client-signed legacy chains stay on the instructions
// endpoints.
func (g *Gateway) handlePOSTOrderSettlementAction(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "orderID")
	if orderID == "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "orderID is required")
		return
	}

	action, err := payment.ParseSettlementActionPath(chi.URLParam(r, "action"))
	if err != nil {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation, err.Error())
		return
	}

	orderSvc := getOrderService(r)
	if orderSvc == nil {
		responsePkg.Error(w, http.StatusServiceUnavailable, responsePkg.CodeServiceUnavail,
			"order service not available")
		return
	}

	var body struct {
		PayoutAddress string `json:"payoutAddress"`
	}

	if r.Body != nil {
		dec := json.NewDecoder(r.Body)
		if err := dec.Decode(&body); err != nil && !errors.Is(err, io.EOF) {
			responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, err.Error())
			return
		}
		var trailing json.RawMessage
		if err := dec.Decode(&trailing); err != nil && !errors.Is(err, io.EOF) {
			responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, err.Error())
			return
		}
		if len(trailing) > 0 {
			responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest,
				"unexpected trailing JSON in request body")
			return
		}
	}

	result, coinType, err := orderSvc.ExecuteSettlementAction(
		r.Context(),
		action,
		models.OrderID(orderID),
		body.PayoutAddress,
	)
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			responsePkg.Error(w, http.StatusNotFound, responsePkg.CodeNotFound, "order not found")
		case errors.Is(err, coreiface.ErrBadRequest):
			responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, err.Error())
		default:
			responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, err.Error())
		}
		return
	}

	paymentChain := ""
	if coinType != "" {
		if coinInfo, ierr := iwallet.CoinInfoFromCoinType(coinType); ierr == nil {
			paymentChain = string(coinInfo.Chain)
		}
	}

	resp := struct {
		Mode         string `json:"mode"`
		ActionID     string `json:"actionId,omitempty"`
		EscrowAddr   string `json:"escrowAddr,omitempty"`
		TxHash       string `json:"txHash,omitempty"`
		PaymentChain string `json:"paymentChain,omitempty"`
		PaymentCoin  string `json:"paymentCoin,omitempty"`
	}{
		PaymentCoin:  string(coinType),
		PaymentChain: paymentChain,
	}

	if result != nil {
		resp.Mode = string(result.Mode)
		resp.ActionID = result.ActionID
		resp.EscrowAddr = result.EscrowAddr
		resp.TxHash = result.SubmittedTxHash
	}

	status := http.StatusOK
	if result != nil && result.Mode == payment.ActionModeSubmitted {
		status = http.StatusAccepted
	}
	responsePkg.StatusWithData(w, status, resp)
}

// handleGETOrderSettlementActionStatus returns the latest known status for a
// settlement action created via POST /settlement-actions/{action}.
//
// GET /v1/orders/{orderID}/settlement-actions/{action}/status?actionId=...
func (g *Gateway) handleGETOrderSettlementActionStatus(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "orderID")
	if orderID == "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "orderID is required")
		return
	}

	action, err := payment.ParseSettlementActionPath(chi.URLParam(r, "action"))
	if err != nil {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation, err.Error())
		return
	}

	actionID := strings.TrimSpace(r.URL.Query().Get("actionId"))
	if actionID == "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "actionId is required")
		return
	}

	orderSvc := getOrderService(r)
	if orderSvc == nil {
		responsePkg.Error(w, http.StatusServiceUnavailable, responsePkg.CodeServiceUnavail,
			"order service not available")
		return
	}

	status, coinType, err := orderSvc.GetSettlementActionStatus(r.Context(), action, models.OrderID(orderID), actionID)
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound), errors.Is(err, coreiface.ErrNotFound), errors.Is(err, payment.ErrActionNotFound):
			responsePkg.Error(w, http.StatusNotFound, responsePkg.CodeNotFound, err.Error())
		case errors.Is(err, coreiface.ErrBadRequest), errors.Is(err, payment.ErrUnsupportedAction):
			responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, err.Error())
		default:
			responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, err.Error())
		}
		return
	}

	paymentChain := ""
	if coinType != "" {
		if coinInfo, ierr := iwallet.CoinInfoFromCoinType(coinType); ierr == nil {
			paymentChain = string(coinInfo.Chain)
		}
	}

	resp := struct {
		ActionID         string `json:"actionId"`
		State            string `json:"state"`
		TxHash           string `json:"txHash,omitempty"`
		Confirmations    int    `json:"confirmations,omitempty"`
		LastError        string `json:"lastError,omitempty"`
		RelayTaskID      string `json:"relayTaskId,omitempty"`
		OrderID          string `json:"orderId,omitempty"`
		SettlementAction string `json:"settlementAction,omitempty"`
		PaymentChain     string `json:"paymentChain,omitempty"`
		PaymentCoin      string `json:"paymentCoin,omitempty"`
	}{
		ActionID:     actionID,
		PaymentCoin:  string(coinType),
		PaymentChain: paymentChain,
	}
	if status != nil {
		resp.State = status.State
		resp.TxHash = status.TxHash
		resp.Confirmations = status.Confirmations
		resp.LastError = status.LastError
		resp.RelayTaskID = status.RelayTaskID
		resp.OrderID = status.OrderID
		resp.SettlementAction = status.SettlementAction
	}
	responsePkg.Success(w, resp)
}
