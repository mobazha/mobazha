package models

import (
	"time"

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
	OrderID     string `json:"orderID"`
	Method      string `json:"method"`
	Moderator   string `json:"moderator"`
	Amount      string `json:"amount"`
	Chaincode   string `json:"chaincode"`
	FromAddress string `json:"fromAddress"`
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
	ModeratorKey       []byte    `json:"moderatorKey"`
	Coin               string    `json:"coin"`
	EscrowReleaseFee   string    `json:"escrowReleaseFee"`
	PlatformAmount     string    `json:"platformAmount"`
	PlatformAddr       string    `json:"platformAddr"`
	PlatformRewardAddr string    `json:"platformRewardAddr"`
	RefundAddress      string    `json:"refundAddress"`
	Timestamp          time.Time `json:"timestamp"`
	TransactionID      string    `json:"transactionID"`
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
		Value: iwallet.NewAmount(p.Amount),
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
