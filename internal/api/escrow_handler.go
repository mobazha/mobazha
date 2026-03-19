package api

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/mobazha/mobazha3.0/internal/chains/utxo"
	wallet "github.com/mobazha/mobazha3.0/internal/wallet"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	responsePkg "github.com/mobazha/mobazha3.0/pkg/response"
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
// 通过 PaymentStrategy 分发，根据 PaymentModel 格式化响应
func (g *Gateway) handleGetOrderPaymentInstructions(w http.ResponseWriter, r *http.Request) {
	orderID := mux.Vars(r)["orderID"]
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

	orderSvc := getOrderService(r)
	walletSvc := getWalletService(r)

	order, err := orderSvc.GetOrder(params.OrderID)
	if err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, err.Error())
		return
	}
	orderOpen, err := order.OrderOpenMessage()
	if err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, err.Error())
		return
	}
	if len(orderOpen.Listings) > 0 && orderOpen.Listings[0].Listing.Metadata.ContractType == pb.Listing_Metadata_RWA_TOKEN {
		coinInfo, err := params.CoinType.CoinInfo()
		if err != nil {
			responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, err.Error())
			return
		}
		g.handleGetRWATokenPaymentInfo(w, r, orderSvc, params, coinInfo)
		return
	}

	// Server-side amount computation: orderOpen.Amount is the finalized total
	// in pricingCoin's smallest unit (calculated at OrderOpen time). For cross-
	// currency payments, convert that total to the payment currency. For same-
	// currency, use directly. UTXO adapters compute amount internally.
	orderAmount := iwallet.NewAmount(orderOpen.Amount)
	pricingCoin := strings.ToUpper(orderOpen.PricingCoin)
	paymentCoinCode := strings.ToUpper(params.CoinType.CurrencyCode())
	if pricingCoin != "" && pricingCoin != paymentCoinCode {
		pricingCurrency, err := models.CurrencyDefinitions.Lookup(pricingCoin)
		if err != nil {
			responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest,
				fmt.Sprintf("unknown pricing currency: %s", pricingCoin))
			return
		}
		paymentCurrency, err := models.CurrencyDefinitions.Lookup(paymentCoinCode)
		if err != nil {
			responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest,
				fmt.Sprintf("unknown payment currency: %s", paymentCoinCode))
			return
		}
		ci, ok := getCoreIface(r)
		if !ok {
			responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "exchange rates unavailable")
			return
		}
		converted, err := wallet.ConvertCurrencyAmount(
			&models.CurrencyValue{Amount: orderAmount, Currency: pricingCurrency},
			paymentCurrency,
			ci.ExchangeRates(),
		)
		if err != nil {
			responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError,
				fmt.Sprintf("convert payment amount: %v", err))
			return
		}
		params.Amount = converted.Uint64()
	} else {
		params.Amount = orderAmount.Uint64()
	}

	result, err := walletSvc.GeneratePaymentInstructions(r.Context(), params)
	if err != nil {
		if result != nil && result.PaymentData != nil {
			if paymentData, ok := result.PaymentData.(*models.PaymentData); ok && paymentData != nil {
				if errors.Is(err, coreiface.ErrCoinSwitchRequiresConfirmation) {
					response := UTXOPaymentInfoResponse{
						PaymentType:       "external_wallet",
						HasPartialPayment: paymentData.HasPartialPayment,
						PaidAmount:        paymentData.PaidAmount,
						PaidCoin:          paymentData.PaidCoin,
						PaidAddress:       paymentData.PaidAddress,
					}
					responsePkg.StatusWithData(w, http.StatusConflict, response)
					return
				}
			}
		}
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, err.Error())
		return
	}

	switch result.PaymentModel {
	case payment.PaymentModelMonitored:
		g.formatMonitoredPaymentResponse(w, params, result)
	case payment.PaymentModelClientSigned:
		g.formatClientSignedPaymentResponse(w, result)
	default:
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, fmt.Sprintf("unsupported payment model: %s", result.PaymentModel))
	}
}

// ============================================================================
// 场景处理函数
// ============================================================================

// handleGetRWATokenPaymentInfo 处理 RWA Token 支付（特殊产品类型，不走 PaymentStrategy）
func (g *Gateway) handleGetRWATokenPaymentInfo(w http.ResponseWriter, r *http.Request, orderSvc contracts.OrderService, params models.InitializeEscrowData, coinInfo iwallet.CoinInfo) {
	if !coinInfo.IsEthTypeChain() {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "RWA Token only supports EVM chains")
		return
	}

	orderInfo, err := orderSvc.GetOrderInfo(models.OrderID(params.OrderID), params.CoinType)
	if err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, err.Error())
		return
	}

	response := RWATokenPaymentInfoResponse{
		BuyerAddress:  orderInfo.BuyerAddress,
		VendorAddress: orderInfo.VendorAddress,
	}
	responsePkg.Success(w, response)
}

// formatMonitoredPaymentResponse formats the response for Monitored (UTXO) payments.
func (g *Gateway) formatMonitoredPaymentResponse(w http.ResponseWriter, params models.InitializeEscrowData, result *payment.PaymentSetupResult) {
	paymentData, ok := result.PaymentData.(*models.PaymentData)
	if !ok || paymentData == nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "invalid payment data for monitored chain")
		return
	}

	coinInfo, err := params.CoinType.CoinInfo()
	if err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, err.Error())
		return
	}

	scriptPubKey, err := hex.DecodeString(paymentData.Script)
	if err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, fmt.Sprintf("Failed to decode script: %v", err))
		return
	}

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
	responsePkg.Success(w, response)
}

// formatClientSignedPaymentResponse formats the response for ClientSigned (EVM/Solana) payments.
func (g *Gateway) formatClientSignedPaymentResponse(w http.ResponseWriter, result *payment.PaymentSetupResult) {
	paymentData, ok := result.PaymentData.(*models.PaymentData)
	if !ok || paymentData == nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "invalid payment data for client-signed chain")
		return
	}

	response := EVMPaymentInfoResponse{
		PaymentData:   paymentData,
		EscrowAccount: result.EscrowAddr,
		Instructions:  result.Instructions,
	}
	responsePkg.Success(w, response)
}
