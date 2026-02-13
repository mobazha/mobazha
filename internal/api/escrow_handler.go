package api

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/mobazha/mobazha3.0/internal/multiwallet/utxo"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// ============================================================================
// 响应结构定义
// ============================================================================

// UTXOPaymentInfoResponse 外部钱包支付响应（UTXO 链）
type UTXOPaymentInfoResponse struct {
	PaymentType    string                `json:"paymentType"`
	PaymentMethod  pb.PaymentSent_Method `json:"paymentMethod"`  // CANCELABLE/MODERATED
	PaymentAddress string                `json:"paymentAddress"` // 支付地址
	PaymentURI     string                `json:"paymentURI"`     // BIP21 URI
	Amount         uint64                `json:"amount"`         // 金额（satoshi）
	Coin           string                `json:"coin"`           // 币种
	ChainType      iwallet.ChainType     `json:"chainType"`      // 链类型
	QRCodeData     string                `json:"qrCodeData"`     // 二维码数据
	ScriptHash     string                `json:"scriptHash"`     // Electrum scripthash
	Script         string                `json:"script"`         // 赎回脚本（多签需要）
	Moderator      string                `json:"moderator"`      // 仲裁者（MODERATED）
	UnlockHours    uint32                `json:"unlockHours"`    // 托管超时时间（MODERATED）
	ExpiresAt      time.Time             `json:"expiresAt"`      // 过期时间

	// 币种切换检测相关字段
	HasPartialPayment bool   `json:"hasPartialPayment,omitempty"` // 是否已有部分支付
	PaidAmount        uint64 `json:"paidAmount,omitempty"`        // 已支付金额
	PaidCoin          string `json:"paidCoin,omitempty"`          // 已支付的币种
	PaidAddress       string `json:"paidAddress,omitempty"`       // 已支付的地址
}

// RWATokenPaymentInfoResponse RWA Token 支付响应
type RWATokenPaymentInfoResponse struct {
	BuyerAddress  string `json:"buyerAddress"`
	VendorAddress string `json:"vendorAddress"`
}

// EVMPaymentInfoResponse 智能合约托管响应（EVM/Solana）
type EVMPaymentInfoResponse struct {
	PaymentData   *models.PaymentData `json:"paymentData"`
	EscrowAccount string              `json:"escrowAccount"`
	Instructions  any                 `json:"instructions"`
}

// ============================================================================
// 主处理函数
// ============================================================================

// handleGetOrderPaymentInstructions 获取订单支付指令
// 根据商品类型和支付币种，返回不同的支付指令
func (g *Gateway) handleGetOrderPaymentInstructions(w http.ResponseWriter, r *http.Request) {
	var params models.InitializeEscrowData
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	node := getNodeService(r)

	// 获取订单信息
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

	// 获取币种信息
	coinInfo, err := params.CoinType.CoinInfo()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 检查商品类型
	isRwaToken := false
	if len(orderOpen.Listings) > 0 {
		isRwaToken = orderOpen.Listings[0].Listing.Metadata.ContractType == pb.Listing_Metadata_RWA_TOKEN
	}

	// 根据场景分发处理
	switch {
	case isRwaToken:
		g.handleGetRWATokenPaymentInfo(w, r, node, params, coinInfo)
	case coinInfo.Chain.IsUTXOChain():
		g.handleGetUTXOPaymentInfo(w, r, node, params, coinInfo)
	default:
		g.handleGetEVMPaymentInfo(w, r, node, params)
	}
}

// ============================================================================
// 场景处理函数
// ============================================================================

// handleGetRWATokenPaymentInfo 处理 RWA Token 支付
func (g *Gateway) handleGetRWATokenPaymentInfo(w http.ResponseWriter, r *http.Request, node contracts.NodeService, params models.InitializeEscrowData, coinInfo iwallet.CoinInfo) {
	if !coinInfo.IsEthTypeChain() {
		http.Error(w, "RWA Token only supports EVM chains", http.StatusBadRequest)
		return
	}

	orderInfo, err := node.GetOrderInfo(models.OrderID(params.OrderID), params.CoinType)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := RWATokenPaymentInfoResponse{
		BuyerAddress:  orderInfo.BuyerAddress,
		VendorAddress: orderInfo.VendorAddress,
	}
	json.NewEncoder(w).Encode(response)
}

// handleGetUTXOPaymentInfo 处理 UTXO 链外部钱包支付（BTC/LTC/BCH/ZEC）
func (g *Gateway) handleGetUTXOPaymentInfo(w http.ResponseWriter, r *http.Request, node contracts.NodeService, params models.InitializeEscrowData, coinInfo iwallet.CoinInfo) {
	// 获取支付信息
	// 支付方式由 Moderator 决定：
	// - 无 Moderator → CANCELABLE（1-of-2 多签，买家可取消，卖家可确认）
	// - 有 Moderator → MODERATED（2-of-3 多签）
	paymentData, err := node.GetUTXOPaymentInfo(
		r.Context(),
		params.OrderID,
		params.Moderator,
		params.CoinType,
	)
	if err != nil {
		// 检查是否是币种切换需要确认的错误
		if errors.Is(err, coreiface.ErrCoinSwitchRequiresConfirmation) && paymentData != nil {
			// 返回需要确认的响应
			response := UTXOPaymentInfoResponse{
				PaymentType:       "external_wallet",
				HasPartialPayment: paymentData.HasPartialPayment,
				PaidAmount:        paymentData.PaidAmount,
				PaidCoin:          paymentData.PaidCoin,
				PaidAddress:       paymentData.PaidAddress,
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict) // 409 Conflict
			json.NewEncoder(w).Encode(response)
			return
		}
		http.Error(w, fmt.Sprintf("Failed to get UTXO payment info: %v", err), http.StatusInternalServerError)
		return
	}

	// 解析 script 计算 scripthash
	scriptPubKey, err := hex.DecodeString(paymentData.Script)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to decode script: %v", err), http.StatusInternalServerError)
		return
	}

	// 生成支付 URI 和 scripthash
	amountInCoin := float64(paymentData.Amount) / 1e8
	paymentURI := utxo.GeneratePaymentURI(coinInfo.Chain, paymentData.ToAddress, amountInCoin)
	scriptHash := utxo.AddressToScriptHash(scriptPubKey)

	response := UTXOPaymentInfoResponse{
		PaymentType:    "external_wallet",
		PaymentMethod:  paymentData.Method,
		PaymentAddress: paymentData.ToAddress,
		PaymentURI:     paymentURI,
		Amount:         paymentData.Amount,
		Coin:           string(params.CoinType),
		ChainType:      coinInfo.Chain,
		QRCodeData:     paymentURI,
		ScriptHash:     scriptHash,
		Script:         paymentData.Script,
		Moderator:      paymentData.Moderator,
		UnlockHours:    paymentData.UnlockHours,
		ExpiresAt:      time.Now().Add(24 * time.Hour),
	}
	json.NewEncoder(w).Encode(response)
}

// handleGetEVMPaymentInfo 处理智能合约托管支付（EVM/Solana）
func (g *Gateway) handleGetEVMPaymentInfo(w http.ResponseWriter, r *http.Request, node contracts.NodeService, params models.InitializeEscrowData) {
	paymentData, escrowAccount, instructions, err := node.BuildInitEscrowInstructions(
		r.Context(),
		params,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := EVMPaymentInfoResponse{
		PaymentData:   paymentData,
		EscrowAccount: escrowAccount.String(),
		Instructions:  instructions,
	}
	json.NewEncoder(w).Encode(response)
}
