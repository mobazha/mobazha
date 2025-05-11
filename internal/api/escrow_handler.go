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
	paymentData, escrowAccount, instructions, err := node.InitializeSolEscrow(
		r.Context(),
		params,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	type InitializeSolResponse struct {
		PaymentData   *models.PaymentData  `json:"paymentData"`
		EscrowAccount solana.PublicKey     `json:"escrowAccount"`
		Instructions  []solana.Instruction `json:"instructions"`
	}

	// 返回响应
	response := InitializeSolResponse{
		PaymentData:   paymentData,
		EscrowAccount: escrowAccount,
		Instructions:  instructions,
	}
	json.NewEncoder(w).Encode(response)
}

// GetReleaseCancelableEscrowInstructions 获取释放可取消的SOL托管的指令
func (g *Gateway) handleGetReleaseCancelableEscrowInstructions(w http.ResponseWriter, r *http.Request) {
	type ReleaseSolRequest struct {
		OrderID   models.OrderID   `json:"orderID"`
		Initiator solana.PublicKey `json:"initiator"`
		Receiver  solana.PublicKey `json:"receiver"`
	}

	var params ReleaseSolRequest
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	// 使用 EscrowClient 释放 SOL 托管
	instructions, err := node.GetCancelableSOLEscrowReleaseInstructions(params.OrderID, params.Initiator, params.Receiver)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	type ReleaseSolResponse struct {
		Instructions []solana.Instruction `json:"instructions"`
	}

	// 返回响应
	response := ReleaseSolResponse{
		Instructions: instructions,
	}
	json.NewEncoder(w).Encode(response)
}

// GetInitializeSPLTokenInstructions 获取初始化SPL Token托管的指令
func (g *Gateway) handleGetInitializeSPLTokenInstructions(w http.ResponseWriter, r *http.Request) {
	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)
	var params models.InitializeSPLTokenData
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 使用 EscrowClient 初始化 SPL Token 托管
	paymentData, escrowAccount, escrowTokenAccount, instructions, err := node.InitializeSPLTokenEscrow(
		r.Context(),
		params,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	type InitializeSPLTokenResponse struct {
		PaymentData        *models.PaymentData  `json:"paymentData"`
		EscrowAccount      solana.PublicKey     `json:"escrowAccount"`
		EscrowTokenAccount solana.PublicKey     `json:"escrowTokenAccount"`
		Instructions       []solana.Instruction `json:"instructions"`
	}

	// 返回响应
	response := InitializeSPLTokenResponse{
		PaymentData:        paymentData,
		EscrowAccount:      escrowAccount,
		EscrowTokenAccount: escrowTokenAccount,
		Instructions:       instructions,
	}
	json.NewEncoder(w).Encode(response)
}

// GetReleaseSPLTokenInstructions 获取释放SPL Token的指令
func (g *Gateway) handleGetReleaseSPLTokenInstructions(w http.ResponseWriter, r *http.Request) {
	type ReleaseSPLTokenRequest struct {
		OrderID   models.OrderID   `json:"orderID"`
		Initiator solana.PublicKey `json:"initiator"`
	}
	var params ReleaseSPLTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	// 使用 EscrowClient 释放 SPL Token 托管
	instructions, err := node.ReleaseSPLTokenEscrow(r.Context(), params.OrderID, params.Initiator)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	type ReleaseSPLTokenResponse struct {
		Instructions []solana.Instruction `json:"instructions"`
	}

	// 返回响应
	response := ReleaseSPLTokenResponse{
		Instructions: instructions,
	}
	json.NewEncoder(w).Encode(response)
}
