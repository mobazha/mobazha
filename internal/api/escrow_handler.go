package api

import (
	"encoding/json"
	"net/http"

	"github.com/gagliardetto/solana-go"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/models"
)

// handleGetOrderPaymentInstructions 获取初始化SOL托管的指令
func (g *Gateway) handleGetOrderPaymentInstructions(w http.ResponseWriter, r *http.Request) {
	var params models.InitializeSolEscrowData
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	// 使用 EscrowClient 初始化 SOL 托管
	paymentData, escrowAccount, escrowTokenAccount, instructions, err := node.BuildInitSolEscrowInstructions(
		r.Context(),
		params,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	type InitializeSolResponse struct {
		PaymentData        *models.PaymentData  `json:"paymentData"`
		EscrowAccount      solana.PublicKey     `json:"escrowAccount"`
		EscrowTokenAccount solana.PublicKey     `json:"escrowTokenAccount"`
		Instructions       []solana.Instruction `json:"instructions"`
	}

	// 返回响应
	response := InitializeSolResponse{
		PaymentData:        paymentData,
		EscrowAccount:      escrowAccount,
		EscrowTokenAccount: escrowTokenAccount,
		Instructions:       instructions,
	}
	json.NewEncoder(w).Encode(response)
}
