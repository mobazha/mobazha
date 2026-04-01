package models

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"time"

	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// PurchaseItemOption is the item option selection.
type PurchaseItemOption struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// PurchaseShippingOption is the shipping option selection.
type PurchaseShippingOption struct {
	Name    string `json:"name"`
	Service string `json:"service"`
	ZoneId  string `json:"zoneId,omitempty"` // ShippingZone.id（可选，精确匹配优先）
	RateId  string `json:"rateId,omitempty"` // ShippingRate.id（可选，精确匹配优先）
}

// PurchaseItem is information about the item in the purchase.
type PurchaseItem struct {
	ListingHash      string                 `json:"listingHash"`
	Quantity         string                 `json:"quantity"`
	Options          []PurchaseItemOption   `json:"options"`
	Shipping         PurchaseShippingOption `json:"shipping"`
	Memo             string                 `json:"memo"`
	PaymentAddress   string                 `json:"paymentAddress"`
	OptionalFeatures []string               `json:"optionalFeatures"`
}

// ShoppingCartItem is information about the item in the shopping cart.
// Listing hash for a listing will change everytime when updating the listing.  When
// doing a real purchase, we need the exact same listing hash, for the consistent listing info.
// However, for shopping cart, we just want to track the up-to-date listing of the same slug.
type ShoppingCartItem struct {
	Slug             string                 `json:"slug"`
	Quantity         string                 `json:"quantity"`
	Options          []PurchaseItemOption   `json:"options"`
	Shipping         PurchaseShippingOption `json:"shipping"`
	Memo             string                 `json:"memo"`
	Checked          bool                   `json:"checked"`
	OptionalFeatures []string               `json:"optionalFeatures"`
}

func (item *ShoppingCartItem) IsSamePurchaseItem(secondItem *ShoppingCartItem) bool {
	// check slug
	if item.Slug != secondItem.Slug {
		return false
	}

	// check options
	if len(item.Options) != len(secondItem.Options) {
		return false
	}
	for _, option1 := range item.Options {
		for _, option2 := range secondItem.Options {
			if option1.Name == option2.Name {
				if option1.Value != option2.Value {
					return false
				}
			}
		}
	}

	// check optional features
	if len(item.OptionalFeatures) != len(secondItem.OptionalFeatures) {
		return false
	}
	featureMap := make(map[string]int)
	for _, feature := range item.OptionalFeatures {
		featureMap[feature]++
	}
	for _, feature := range secondItem.OptionalFeatures {
		if count, ok := featureMap[feature]; !ok || count == 0 {
			return false
		}
		featureMap[feature]--
	}

	return true
}

// Purchase contains all the information needed by the node to
// execute a purchase.
type Purchase struct {
	ShipTo               string         `json:"shipTo"`
	Address              string         `json:"address"`
	City                 string         `json:"city"`
	State                string         `json:"state"`
	PostalCode           string         `json:"postalCode"`
	CountryCode          string         `json:"countryCode"`
	AddressNotes         string         `json:"addressNotes"`
	Items                []PurchaseItem `json:"items"`
	AlternateContactInfo string         `json:"alternateContactInfo"`
	PricingCoin          string         `json:"pricingCoin"`
	DiscountCodes        []string       `json:"discountCodes,omitempty"`
}

type PaymentData struct {
	OrderID          string                `json:"orderID"`
	TransactionID    string                `json:"transactionID"`
	Coin             iwallet.CoinType      `json:"coin"`
	Method           pb.PaymentSent_Method `json:"method"`
	ProviderID       string                `json:"providerID,omitempty"`
	ContractAddress  string                `json:"contractAddress"`
	PayerAddress     string                `json:"payerAddress"`
	Moderator        string                `json:"moderator"`
	ModeratorAddress string                `json:"moderatorAddress"`
	Amount           uint64                `json:"amount,string"`
	/*
		id := make([]byte, 36)
		copy(id[:32], prevHash[:])
		copy(id[32:], index)
		reference: (legacy) previously from blockbook buildTransaction(), now via UTXOChainClient
	*/
	FromID             []byte    `json:"fromID"` // 36 bytes, derived from PayerAddress
	ToAddress          string    `json:"toAddress"`
	ToID               []byte    `json:"toID"` // 36 bytes
	Script             string    `json:"script"`
	UnlockHours        uint32    `json:"unlockHours"`
	EscrowReleaseFee   string    `json:"escrowReleaseFee"`
	PlatformAmount     string    `json:"platformAmount"`
	PlatformAddr       string    `json:"platformAddr"`
	PlatformRewardAddr string    `json:"platformRewardAddr"`
	RefundAddress      string    `json:"refundAddress"`
	Timestamp          time.Time `json:"timestamp"`
	// 新增支付方式信息
	PaymentMethod struct {
		Type  string `json:"type"`  // 支付方式类型
		Brand string `json:"brand"` // 卡品牌（如果是信用卡支付）
		Last4 string `json:"last4"` // 卡号后四位（如果是信用卡支付）
	} `json:"paymentMethod"`
	// 新增收据信息
	ReceiptInfo struct {
		URL    string `json:"url"`    // 收据URL
		Number string `json:"number"` // 收据编号
	} `json:"receiptInfo"`
	// 支付代币地址（通用字段，适用于所有 Token 支付）
	PaymentTokenAddress string `json:"paymentTokenAddress,omitempty"` // 支付代币合约地址（ETH为零地址）

	BuyerReceiveAddress string `json:"buyerReceiveAddress,omitempty"` // 买家接收 Token 的地址（支持多链地址格式）

	// RWA 原子交换相关
	ApprovalTxHash string `json:"approvalTxHash,omitempty"` // 买家 approve 交易哈希（RWA 原子交换模式）

	// RWA 双模式交易相关
	RwaTradeMode         int    `json:"rwaTradeMode,omitempty"`         // 0: 即时交易, 1: 确认交易
	RwaOrderCompleted    bool   `json:"rwaOrderCompleted,omitempty"`    // 链上订单是否已完成（即时模式为 true）
	SellerReceiveAddress string `json:"sellerReceiveAddress,omitempty"` // 卖家收款地址（即时交易模式使用）

	// 币种切换检测相关字段
	HasPartialPayment bool   `json:"hasPartialPayment,omitempty"` // 是否已有部分支付（用于币种切换时提示）
	PaidAmount        uint64 `json:"paidAmount,omitempty,string"`  // 已支付金额
	PaidCoin          string `json:"paidCoin,omitempty"`          // 已支付的币种
	PaidAddress       string `json:"paidAddress,omitempty"`       // 已支付的地址

	// 部分支付信息（用于多次支付场景）
	RemainingAmount uint64 `json:"remainingAmount,omitempty,string"` // 剩余待支付金额
}

// EnsureTransactionFields populates TransactionID and FromID on PaymentData
// if they are empty. Call this before BuildTransaction to guarantee non-empty values.
func (p *PaymentData) EnsureTransactionFields() error {
	if p.TransactionID == "" {
		txidBytes := make([]byte, 32)
		if _, err := rand.Read(txidBytes); err != nil {
			return fmt.Errorf("generate transaction ID: %w", err)
		}
		p.TransactionID = hex.EncodeToString(txidBytes)
	}

	if len(p.FromID) == 0 {
		prevBytes := make([]byte, 36)
		if _, err := rand.Read(prevBytes); err != nil {
			return fmt.Errorf("generate FromID: %w", err)
		}
		p.FromID = prevBytes
	}

	return nil
}

func (p *PaymentData) BuildTransaction() (iwallet.Transaction, error) {
	if p.TransactionID == "" {
		return iwallet.Transaction{}, fmt.Errorf("TransactionID is required: call EnsureTransactionFields first")
	}
	if len(p.FromID) == 0 {
		return iwallet.Transaction{}, fmt.Errorf("FromID is required: call EnsureTransactionFields first")
	}

	// 安全获取 FromID，避免切片越界
	var fromID []byte
	if len(p.FromID) >= 36 {
		fromID = p.FromID[:36]
	} else {
		fromID = p.FromID
	}

	// 安全获取 ToID，避免切片越界
	var toID []byte
	if len(p.ToID) >= 36 {
		toID = p.ToID[:36]
	} else if len(p.ToID) > 0 {
		toID = p.ToID
	}

	txID := iwallet.TransactionID(p.TransactionID)

	// Auto-generate ToID (outpoint) if not provided
	// Format: 32 bytes txid + 4 bytes output index (big-endian)
	if len(toID) == 0 {
		txidBytes, err := hex.DecodeString(string(txID))
		if err == nil && len(txidBytes) >= 32 {
			idx := make([]byte, 4)
			binary.BigEndian.PutUint32(idx, 0)
			toID = append(txidBytes[:32], idx...)
		}
	}

	tx := iwallet.Transaction{
		ID: txID,
		From: []iwallet.SpendInfo{
			{
				ID:      fromID,
				Address: iwallet.NewAddress(p.PayerAddress, iwallet.CoinType(p.Coin)),
				Amount:  iwallet.NewAmount(p.Amount),
			},
		},
		To: []iwallet.SpendInfo{
			{
				ID:      toID,
				Address: iwallet.NewAddress(p.ToAddress, iwallet.CoinType(p.Coin)),
				Amount:  iwallet.NewAmount(p.Amount),
			},
		},
		Value:     iwallet.NewAmount(p.Amount),
		Timestamp: p.Timestamp,
	}
	return tx, nil
}

// DiscountDetail describes a single applied discount for API responses.
type DiscountDetail struct {
	DiscountID string `json:"discountID"`
	Title      string `json:"title"`
	Code       string `json:"code,omitempty"`
	ValueType  string `json:"valueType"`
	Value      string `json:"value"`
	Amount     string `json:"amount"`
	Auto       bool   `json:"auto,omitempty"`
}

// OrderTotals represents a breakdown of the various charges of the order.
type OrderTotals struct {
	Subtotal        iwallet.Amount   `json:"subtotal"`
	Shipping        iwallet.Amount   `json:"shipping"`
	Discounts       iwallet.Amount   `json:"discounts"`
	Taxes           iwallet.Amount   `json:"taxes"`
	Total           iwallet.Amount   `json:"total"`
	DiscountDetails []DiscountDetail `json:"discountDetails,omitempty"`
}

type StoreCart struct {
	VendorID string             `json:"vendorID"`
	Items    []ShoppingCartItem `json:"items"`
}

// StoreCartRecord stores a shopping cart keyed by vendor.
// In multi-tenant shared DB mode, the composite primary key
// (TenantID, VendorID) ensures different tenants' carts for the
// same vendor don't conflict.
type StoreCartRecord struct {
	TenantID string `gorm:"column:tenant_id;primaryKey;default:''" json:"-"`
	VendorID string `gorm:"primaryKey"`
	Items    []byte
}
