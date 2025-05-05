package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/models"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// GetReceivingAccounts 获取用户的收款账户信息
func (g *Gateway) GetReceivingAccounts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		ErrorResponse(w, http.StatusMethodNotAllowed, "只允许GET请求")
		return
	}

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	// 从数据库获取用户的收款账户信息
	receivingAccounts, err := node.GetReceivingAccounts()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	// 返回收款账户信息
	resp := struct {
		ReceivingAccounts []models.ReceivingAccount `json:"receivingAccounts"`
	}{
		ReceivingAccounts: receivingAccounts,
	}

	sanitizedJSONResponse(w, resp)
}

// handleReceivingAccountRequest 处理收款账户的添加和更新请求
func (g *Gateway) handleReceivingAccountRequest(w http.ResponseWriter, r *http.Request, isUpdate bool) {
	// 检查请求方法
	expectedMethod := http.MethodPost
	if isUpdate {
		expectedMethod = http.MethodPut
	}
	if r.Method != expectedMethod {
		ErrorResponse(w, http.StatusMethodNotAllowed, fmt.Sprintf("只允许%s请求", expectedMethod))
		return
	}

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	type ReceivingAccountParams struct {
		ID             int               `json:"id"`
		Name           string            `json:"name"`             // 账户名称
		ChainType      iwallet.ChainType `json:"chainType"`        // 区块链网络类型
		Address        string            `json:"address"`          // 用户的收款钱包地址
		ActiveTokens   []string          `json:"activeTokens"`     // 已激活的代币列表
		InactiveTokens []string          `json:"inactiveTokens"`   // 未激活的代币列表
		Email          string            `json:"email,omitempty"`  // 对于Stripe/Paypal，使用Email
		Source         string            `json:"source,omitempty"` // 来源
		IsActive       bool              `json:"isActive"`         // 是否激活
	}

	// 解析请求体
	var req ReceivingAccountParams
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	account := &models.ReceivingAccount{
		ID:        req.ID,
		Name:      req.Name,
		ChainType: req.ChainType,
		Address:   req.Address,
		Email:     req.Email,
		Source:    req.Source,
		IsActive:  req.IsActive,
	}

	// 设置激活和未激活的代币列表
	if err := account.SetActiveTokens(req.ActiveTokens); err != nil {
		ErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("设置激活代币失败: %v", err))
		return
	}
	if err := account.SetInactiveTokens(req.InactiveTokens); err != nil {
		ErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("设置未激活代币失败: %v", err))
		return
	}

	if isUpdate {
		account, err = node.UpdateReceivingAccount(account)
	} else {
		account, err = node.AddReceivingAccount(account)
	}
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	// 返回成功响应
	resp := struct {
		Success bool                     `json:"success"`
		Account *models.ReceivingAccount `json:"account"`
	}{
		Success: true,
		Account: account,
	}

	sanitizedJSONResponse(w, resp)
}

// AddReceivingAccount 添加新的收款账户
func (g *Gateway) AddReceivingAccount(w http.ResponseWriter, r *http.Request) {
	g.handleReceivingAccountRequest(w, r, false)
}

// UpdateReceivingAccount 更新用户的收款账户信息
func (g *Gateway) UpdateReceivingAccount(w http.ResponseWriter, r *http.Request) {
	g.handleReceivingAccountRequest(w, r, true)
}

// GetStripeConnectURL 获取Stripe OAuth连接URL
func (g *Gateway) GetStripeConnectURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		ErrorResponse(w, http.StatusMethodNotAllowed, "只允许GET请求")
		return
	}

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	// 获取Stripe连接URL
	// 这里需要使用Stripe API生成OAuth URL
	// 实际实现中需要使用Stripe SDK
	stripeConnectURL, err := node.GetStripeConnectURL()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	// 返回URL
	resp := struct {
		URL string `json:"url"`
	}{
		URL: stripeConnectURL,
	}

	sanitizedJSONResponse(w, resp)
}
