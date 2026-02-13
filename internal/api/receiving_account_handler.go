package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/mobazha/mobazha3.0/pkg/models"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// GetReceivingAccounts 获取用户的收款账户信息
func (g *Gateway) GetReceivingAccounts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		ErrorResponse(w, http.StatusMethodNotAllowed, "只允许GET请求")
		return
	}

	node := getNodeService(r)

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

	node := getNodeService(r)

	type ReceivingAccountParams struct {
		ID             int               `json:"id"`
		Name           string            `json:"name"`             // 账户名称
		ChainType      iwallet.ChainType `json:"chainType"`        // 区块链网络类型
		Address        string            `json:"address"`          // 用户的收款钱包地址
		ActiveTokens   []string          `json:"activeTokens"`     // 已激活的代币列表
		InactiveTokens []string          `json:"inactiveTokens"`   // 未激活的代币列表
		Source         string            `json:"source,omitempty"` // 来源
		IsActive       bool              `json:"isActive"`         // 是否激活
		Status         string            `json:"status,omitempty"` // 账户状态, 对于Stripe, 则是StripeStatus
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
		Source:    req.Source,
		IsActive:  req.IsActive,
		Status:    req.Status, // 对于Stripe, 则是StripeStatus
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

// DeleteReceivingAccount 删除收款账户
func (g *Gateway) DeleteReceivingAccount(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		ErrorResponse(w, http.StatusMethodNotAllowed, "只允许DELETE请求")
		return
	}

	node := getNodeService(r)

	// 从URL路径获取账户ID
	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, "无效的账户ID")
		return
	}

	// 删除账户
	err = node.DeleteReceivingAccount(id)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	// 返回成功响应
	resp := struct {
		Success bool `json:"success"`
	}{
		Success: true,
	}

	sanitizedJSONResponse(w, resp)
}
