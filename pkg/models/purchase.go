package models

import (
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
}

// PurchaseItem is information about the item in the purchase.
type PurchaseItem struct {
	ListingHash      string                 `json:"listingHash"`
	Quantity         string                 `json:"quantity"`
	Options          []PurchaseItemOption   `json:"options"`
	Shipping         PurchaseShippingOption `json:"shipping"`
	Memo             string                 `json:"memo"`
	Coupons          []string               `json:"coupons"`
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
}

type PaymentData struct {
	OrderID          string                `json:"orderID"`
	TransactionID    string                `json:"transactionID"`
	Coin             iwallet.CoinType      `json:"coin"`
	Method           pb.PaymentSent_Method `json:"method"`
	ContractAddress  string                `json:"contractAddress"`
	PayerAddress     string                `json:"payerAddress"`
	Moderator        string                `json:"moderator"`
	ModeratorAddress string                `json:"moderatorAddress"`
	Amount           uint64                `json:"amount"`
	FromAddress      string                `json:"fromAddress"`
	/*
		id := make([]byte, 36)
		copy(id[:32], prevHash[:])
		copy(id[32:], index)
		reference: internal/multiwallet/client/blockbook -> buildTransaction()
	*/
	FromID             []byte    `json:"fromID"` // 36 bytes
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
}

func (p *PaymentData) BuildTransaction() iwallet.Transaction {
	tx := iwallet.Transaction{
		ID: iwallet.TransactionID(p.TransactionID),
		From: []iwallet.SpendInfo{
			{
				ID:      p.FromID[:36],
				Address: iwallet.NewAddress(p.FromAddress, iwallet.CoinType(p.Coin)),
				Amount:  iwallet.NewAmount(p.Amount),
			},
		},
		To: []iwallet.SpendInfo{
			{
				ID:      p.ToID[:36],
				Address: iwallet.NewAddress(p.ToAddress, iwallet.CoinType(p.Coin)),
				Amount:  iwallet.NewAmount(p.Amount),
			},
		},
		Value:     iwallet.NewAmount(p.Amount),
		Timestamp: p.Timestamp,
	}
	return tx
}

// OrderTotals represents a breakdown of the various charges of the order.
type OrderTotals struct {
	Subtotal  iwallet.Amount `json:"subtotal"`
	Shipping  iwallet.Amount `json:"shipping"`
	Discounts iwallet.Amount `json:"discounts"`
	Taxes     iwallet.Amount `json:"taxes"`
	Total     iwallet.Amount `json:"total"`
}

type StoreCart struct {
	VendorID string             `json:"vendorID"`
	Items    []ShoppingCartItem `json:"items"`
}

type StoreCartRecord struct {
	VendorID string `gorm:"primaryKey"`
	Items    []byte
}
