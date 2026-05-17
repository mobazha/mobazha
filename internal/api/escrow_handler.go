//go:build !private_distribution

package api

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
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

const (
	// UTXOPaymentWindowDuration is the user-facing payment window for UTXO
	// external wallet payments. Aligned with the 15-minute rate lock window
	// (EXCHANGE_RATE_DESIGN.md §13) and industry standard (BitPay, BTCPay).
	// The backend address monitor runs independently for 24h as a safety net
	// (see payment_app_service_utxo.go AddressMonitorDuration).
	UTXOPaymentWindowDuration = 15 * time.Minute
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
	Amount         uint64                `json:"amount,string"`  // 金额（satoshi）
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
	PaidAmount        uint64 `json:"paidAmount,omitempty,string"` // 已支付金额
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

// ManagedEscrowPaymentInfoResponse is the payment setup response for ManagedEscrow EVM orders.
//
// ManagedEscrow uses address-monitored funding: the buyer transfers native ETH (or ERC-20)
// directly to the predicted ManagedEscrow address. No client-side signing is required
// during setup — signatures are collected at action time (Confirm / Cancel / Complete).
//
// Phase PS B2: replaces the legacy EVMPaymentInfoResponse for ManagedEscrow adapter results.
// The legacy instructions endpoint continues to serve V1 EVM / Solana orders.
type ManagedEscrowPaymentInfoResponse struct {
	PaymentType    string                `json:"paymentType"`         // always "managed_escrow_address_monitored"
	PaymentMethod  pb.PaymentSent_Method `json:"paymentMethod"`       // CANCELABLE or MODERATED
	PaymentAddress string                `json:"paymentAddress"`      // predicted ManagedEscrow address (hex)
	Amount         uint64                `json:"amount,string"`       // amount in wei (or token minimal units)
	Coin           string                `json:"coin"`                // canonical coin type
	ExpiresAt      time.Time             `json:"expiresAt"`           // funding window deadline
	Moderator      string                `json:"moderator,omitempty"` // peerID (MODERATED only)
}

// ============================================================================
// 主处理函数
// ============================================================================

// handleGetOrderPaymentInstructions 获取订单支付指令
// 通过 ChainEscrow 分发，根据 PaymentModel 格式化响应
//
// Phase PS / B5: unified read model lives at GET /v1/orders/{orderID}/payment-session.
// Crypto funding can also be provisioned via POST .../payment-session; this legacy JSON
// body route remains for richer client-signed payloads and specialised responses (QR, URIs).
func (g *Gateway) handleGetOrderPaymentInstructions(w http.ResponseWriter, r *http.Request) {
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
	walletSvc := getWalletService(r)

	order, err := orderSvc.GetOrder(params.OrderID)
	if err != nil {
		log.Warningf("Failed to get order %s for payment instructions: %v", params.OrderID, err)
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

	if err := models.ValidateRefundAddress(params.CoinType, params.RefundAddress); err != nil {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, err.Error())
		return
	}

	// Server-side amount computation: orderOpen.Amount is the finalized total
	// in pricingCoin's smallest unit (calculated at OrderOpen time). For cross-
	// currency payments, convert that total to the payment currency. For same-
	// currency, use directly. UTXO adapters compute amount internally.
	orderAmount := iwallet.NewAmount(orderOpen.Amount)
	pricingCoin := strings.ToUpper(orderOpen.PricingCoin)
	paymentCoinCode, err := params.CoinType.PricingCurrencyCode()
	if err != nil {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, fmt.Sprintf("invalid coinType: %v", err))
		return
	}
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
			log.Warningf("Failed to convert payment amount from %s to %s: %v", pricingCoin, paymentCoinCode, err)
			responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "Failed to convert payment amount")
			return
		}
		params.Amount = converted.Uint64()
	} else {
		params.Amount = orderAmount.Uint64()
	}

	result, err := walletSvc.GeneratePaymentInstructions(r.Context(), params)
	if err != nil {
		if result != nil && result.PaymentData != nil {
			if paymentData := result.PaymentData; paymentData != nil {
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
		log.Warningf("Failed to generate payment instructions for order %s: %v", params.OrderID, err)
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "Failed to generate payment instructions")
		return
	}

	if err := orderSvc.SetOrderRefundAddressForPayment(r.Context(), params.OrderID, params.CoinType, params.RefundAddress); err != nil {
		if errors.Is(err, coreiface.ErrBadRequest) || errors.Is(err, models.ErrRefundAddressRequired) || errors.Is(err, models.ErrRefundAddressInvalid) {
			responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, err.Error())
			return
		}
		log.Warningf("SetOrderRefundAddressForPayment failed for order %s: %v", params.OrderID, err)
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "Failed to save refund address")
		return
	}

	switch result.PaymentModel {
	case payment.PaymentModelMonitored:
		// Phase PS B2: ManagedEscrow EVM uses address-monitored funding (no Script/ScriptHash).
		// UTXO chains still use the existing formatMonitoredPaymentResponse path.
		if result.IsManagedEscrowOrder {
			g.formatManagedEscrowPaymentResponse(w, result)
		} else {
			g.formatMonitoredPaymentResponse(w, params, result)
		}
	case payment.PaymentModelClientSigned:
		g.formatClientSignedPaymentResponse(w, result)
	default:
		log.Warningf("Unsupported payment model %s for order %s", result.PaymentModel, params.OrderID)
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "Unsupported payment configuration")
	}
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

// formatMonitoredPaymentResponse formats the response for Monitored (UTXO) payments.
func (g *Gateway) formatMonitoredPaymentResponse(w http.ResponseWriter, params models.InitializeEscrowData, result *payment.PaymentSetupResult) {
	paymentData := result.PaymentData
	if paymentData == nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "invalid payment data for monitored chain")
		return
	}

	coinInfo, err := params.CoinType.CoinInfo()
	if err != nil {
		log.Warningf("Failed to get coin info for %s: %v", params.CoinType, err)
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "Failed to process coin type")
		return
	}

	scriptPubKey, err := hex.DecodeString(paymentData.Script)
	if err != nil {
		log.Warningf("Failed to decode payment script: %v", err)
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "Failed to process payment script")
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
		ExpiresAt:      time.Now().Add(UTXOPaymentWindowDuration),
	}
	responsePkg.Success(w, response)
}

// formatClientSignedPaymentResponse formats the response for ClientSigned (EVM/Solana) payments.
func (g *Gateway) formatClientSignedPaymentResponse(w http.ResponseWriter, result *payment.PaymentSetupResult) {
	paymentData := result.PaymentData
	if paymentData == nil {
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

// formatManagedEscrowPaymentResponse formats the response for ManagedEscrow EVM address-monitored payments.
//
// Unlike UTXO monitored (which returns Script/ScriptHash for Electrum) and V1 EVM
// client-signed (which returns contract call Instructions), ManagedEscrow only needs the
// predicted address and amount — the buyer just transfers ETH to it.
//
// Phase PS B2.
func (g *Gateway) formatManagedEscrowPaymentResponse(w http.ResponseWriter, result *payment.PaymentSetupResult) {
	paymentData := result.PaymentData
	if paymentData == nil || paymentData.ToAddress == "" {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "invalid payment data for ManagedEscrow order: missing address")
		return
	}

	response := ManagedEscrowPaymentInfoResponse{
		PaymentType:    "managed_escrow_address_monitored",
		PaymentMethod:  paymentData.Method,
		PaymentAddress: paymentData.ToAddress,
		Amount:         paymentData.Amount,
		Coin:           string(paymentData.Coin),
		ExpiresAt:      time.Now().Add(UTXOPaymentWindowDuration),
		Moderator:      paymentData.Moderator,
	}
	responsePkg.Success(w, response)
}
