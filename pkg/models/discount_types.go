package models

import "math/big"

// DiscountContext holds all inputs needed to calculate applicable discounts.
type DiscountContext struct {
	DiscountCodes   []string
	ProductIDs      []string
	CustomerPeerID  string
	PaymentCurrency string
	SubTotal        *big.Int
	ItemQuantity    int
	// ConvertAmount converts a value in the discount's currency to the payment currency.
	// Returns the converted amount as *big.Int in the smallest unit.
	ConvertAmount func(amount string, fromCurrency string) (*big.Int, error)
}

// AppliedDiscount represents a single discount that has been applied.
type AppliedDiscount struct {
	DiscountID string `json:"discountID"`
	CodeID     string `json:"codeID,omitempty"`
	Title      string `json:"title"`
	Code       string `json:"code,omitempty"`
	ValueType  string `json:"valueType"`
	Value      string `json:"value"`
	Amount     string `json:"amount"`
	Auto       bool   `json:"auto,omitempty"`
}

// DiscountResult holds the output of discount calculation.
type DiscountResult struct {
	AppliedDiscounts []AppliedDiscount `json:"appliedDiscounts,omitempty"`
	DiscountsTotal   *big.Int          `json:"discountsTotal"`
	ShippingDiscount bool              `json:"shippingDiscount"`
}
