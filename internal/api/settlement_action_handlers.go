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

// handlePOSTOrderSettlementAction executes a unified settlement intent via ChainEscrowV2.
//
// POST /v1/orders/{orderID}/settlement-actions/{action}
//
// Path action (case-insensitive): "confirm" | "cancel".
//
// Body JSON (optional, camelCase):
//   - payoutAddress — vendor payout (confirm) or buyer refund override (cancel).
//
// Phase PS — aligns with UNIFIED_PAYMENT_SESSION_ARCHITECTURE §7.2 minimal surface.
func (g *Gateway) handlePOSTOrderSettlementAction(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "orderID")
	if orderID == "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "orderID is required")
		return
	}

	action := strings.ToLower(strings.TrimSpace(chi.URLParam(r, "action")))
	if action != "confirm" && action != "cancel" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation,
			`action must be "confirm" or "cancel"`)
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

	responsePkg.Success(w, resp)
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

	action := strings.ToLower(strings.TrimSpace(chi.URLParam(r, "action")))
	if action != "confirm" && action != "cancel" && action != "complete" && action != "dispute_release" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation,
			`action must be "confirm", "cancel", "complete", or "dispute_release"`)
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
