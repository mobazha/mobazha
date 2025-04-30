package api

import (
	"encoding/json"
	"net/http"

	"github.com/gagliardetto/solana-go"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/models"
)

// InitializeSolEscrow 初始化SOL托管
func (g *Gateway) handleInitializeSolEscrow(w http.ResponseWriter, r *http.Request) {
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

// ReleaseSolEscrow 释放SOL
func (g *Gateway) handleReleaseSolEscrow(w http.ResponseWriter, r *http.Request) {
	type ReleaseSolRequest struct {
		OrderID   models.OrderID   `json:"orderID"`
		Initiator solana.PublicKey `json:"initiator"`
	}

	var params ReleaseSolRequest
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	// 使用 EscrowClient 释放 SOL 托管
	instructions, err := node.ReleaseSolEscrow(r.Context(), params.OrderID, params.Initiator)
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

// InitializeSPLTokenEscrow 初始化SPL Token托管
func (g *Gateway) handleInitializeSPLTokenEscrow(w http.ResponseWriter, r *http.Request) {
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

// ReleaseSPLTokenEscrow 释放SPL Token
func (g *Gateway) handleReleaseSPLTokenEscrow(w http.ResponseWriter, r *http.Request) {
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
