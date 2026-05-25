//go:build !private_distribution

package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	responsePkg "github.com/mobazha/mobazha3.0/pkg/response"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// ============================================================================
// 响应结构定义
// ============================================================================

// RWATokenPaymentInfoResponse RWA Token 支付响应
type RWATokenPaymentInfoResponse struct {
	BuyerAddress  string `json:"buyerAddress"`
	VendorAddress string `json:"vendorAddress"`
}

// ============================================================================
// 主处理函数
// ============================================================================

// handleGetRWATokenPaymentInfoRequest returns the buyer/vendor identity
// addresses needed by RWA token purchase flows.
func (g *Gateway) handleGetRWATokenPaymentInfoRequest(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "orderID")
	if orderID == "" {
		ErrorResponse(w, http.StatusBadRequest, "missing orderID")
		return
	}

	var params models.InitializeEscrowData
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, err.Error())
		return
	}
	params.OrderID = orderID

	normalizedCoin, err := iwallet.NormalizePaymentCoinIngress(string(params.CoinType))
	if err != nil {
		responsePkg.Error(
			w,
			http.StatusBadRequest,
			responsePkg.CodeBadRequest,
			fmt.Sprintf("invalid coinType: %v", err),
		)
		return
	}
	params.CoinType = normalizedCoin

	orderSvc := getOrderService(r)
	order, err := orderSvc.GetOrder(params.OrderID)
	if err != nil {
		log.Warningf("Failed to get order %s for RWA token payment info: %v", params.OrderID, err)
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "Order not found or unavailable")
		return
	}
	orderOpen, err := order.OrderOpenMessage()
	if err != nil {
		log.Warningf("Failed to parse order open message for %s: %v", params.OrderID, err)
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "Failed to load order details")
		return
	}
	if len(orderOpen.Listings) > 0 && orderOpen.Listings[0].Listing.Metadata.ContractType == pb.Listing_Metadata_RWA_TOKEN {
		coinInfo, err := params.CoinType.CoinInfo()
		if err != nil {
			log.Warningf("Invalid coin type %s for RWA token payment: %v", params.CoinType, err)
			responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "Unsupported coin type")
			return
		}
		g.handleGetRWATokenPaymentInfo(w, r, orderSvc, params, coinInfo)
		return
	}

	responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest,
		"order is not an RWA token listing")
}

// ============================================================================
// 场景处理函数
// ============================================================================

// handleGetRWATokenPaymentInfo 处理 RWA Token 支付（特殊产品类型，不走 ChainEscrow）
func (g *Gateway) handleGetRWATokenPaymentInfo(w http.ResponseWriter, r *http.Request, orderSvc contracts.OrderService, params models.InitializeEscrowData, coinInfo iwallet.CoinInfo) {
	if !coinInfo.IsEthTypeChain() {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "RWA Token only supports EVM chains")
		return
	}

	orderInfo, err := orderSvc.GetOrderInfo(models.OrderID(params.OrderID), params.CoinType)
	if err != nil {
		log.Warningf("Failed to get RWA order info for %s: %v", params.OrderID, err)
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "Failed to load order information")
		return
	}

	response := RWATokenPaymentInfoResponse{
		BuyerAddress:  orderInfo.BuyerAddress,
		VendorAddress: orderInfo.VendorAddress,
	}
	responsePkg.Success(w, response)
}
