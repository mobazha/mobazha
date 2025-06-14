package api

import (
	"encoding/json"
	"net/http"

	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/models"
)

// handleGetOrderPaymentInstructions 获取初始化SOL托管的指令
func (g *Gateway) handleGetOrderPaymentInstructions(w http.ResponseWriter, r *http.Request) {
	var params models.InitializeEscrowData
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	// 使用 EscrowClient 初始化 SOL 托管
	paymentData, escrowAccount, instructions, err := node.BuildInitEscrowInstructions(
		r.Context(),
		params,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	type InitializeEscrowResponse struct {
		PaymentData   *models.PaymentData `json:"paymentData"`
		EscrowAccount string              `json:"escrowAccount"`
		Instructions  any                 `json:"instructions"`
	}

	// 返回响应
	response := InitializeEscrowResponse{
		PaymentData:   paymentData,
		EscrowAccount: escrowAccount.String(),
		Instructions:  instructions,
	}
	json.NewEncoder(w).Encode(response)
}
