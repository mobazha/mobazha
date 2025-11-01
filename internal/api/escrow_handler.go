package api

import (
	"encoding/json"
	"net/http"

	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// handleGetOrderPaymentInstructions 获取初始化托管的指令
func (g *Gateway) handleGetOrderPaymentInstructions(w http.ResponseWriter, r *http.Request) {
	var params models.InitializeEscrowData
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	// ⭐ 从订单数据中获取 ContractType，而不依赖前端传参
	order, err := node.GetOrder(params.OrderID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	orderOpen, err := order.OrderOpenMessage()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 检查订单的第一个商品的 ContractType 是否为 RWA_TOKEN
	isRwaToken := false
	if len(orderOpen.Listings) > 0 {
		isRwaToken = orderOpen.Listings[0].Listing.Metadata.ContractType == pb.Listing_Metadata_RWA_TOKEN
	}

	// 检查是否为 RWA Token 合约商品
	if isRwaToken {
		coinInfo, err := params.CoinType.CoinInfo()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if coinInfo.Chain == iwallet.ChainEthereum {
			orderInfo, err := node.GetOrderInfo(models.OrderID(params.OrderID), params.CoinType)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			response := struct {
				BuyerAddress  string `json:"buyerAddress"`
				VendorAddress string `json:"vendorAddress"`
			}{
				BuyerAddress:  orderInfo.BuyerAddress,
				VendorAddress: orderInfo.VendorAddress,
			}
			json.NewEncoder(w).Encode(response)
			return
		} else {
			http.Error(w, "Unsupported coin type", http.StatusBadRequest)
			return
		}
	}

	// 使用 EscrowProcessor 构建初始化托管指令
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
