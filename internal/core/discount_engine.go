package core

import (
	"context"
	"math/big"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/models"
)

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

// DiscountEngine calculates applicable discounts for a checkout.
type DiscountEngine struct {
	svc             contracts.DiscountService
	store           contracts.DiscountStore
	collectionStore contracts.CollectionStore
}

func NewDiscountEngine(svc contracts.DiscountService, store contracts.DiscountStore, collectionStore contracts.CollectionStore) *DiscountEngine {
	return &DiscountEngine{svc: svc, store: store, collectionStore: collectionStore}
}

// Calculate evaluates all applicable discounts for the given context and returns
// the optimal set considering stacking rules.
func (e *DiscountEngine) Calculate(ctx context.Context, dc DiscountContext) (*DiscountResult, error) {
	result := &DiscountResult{
		DiscountsTotal: big.NewInt(0),
	}

	var candidates []discountCandidate

	// 1. Resolve discount codes → associated discounts
	for _, code := range dc.DiscountCodes {
		vr, err := e.svc.ValidateCode(ctx, code, dc.CustomerPeerID)
		if err != nil {
			continue
		}
		if !vr.Valid || vr.Discount == nil {
			continue
		}
		candidates = append(candidates, discountCandidate{
			discount: vr.Discount,
			code:     vr.Code,
			codeStr:  code,
			auto:     false,
		})
	}

	// 2. Load active automatic discounts
	autoDiscounts, err := e.store.GetApplicableDiscounts(ctx, dc.ProductIDs)
	if err == nil {
		for i := range autoDiscounts {
			candidates = append(candidates, discountCandidate{
				discount: &autoDiscounts[i],
				auto:     true,
			})
		}
	}

	// 3. Filter by conditions
	now := time.Now()
	var valid []discountCandidate
	for _, c := range candidates {
		d := c.discount
		if d.StartsAt.After(now) {
			continue
		}
		if d.EndsAt != nil && !d.EndsAt.IsZero() && d.EndsAt.Before(now) {
			continue
		}
		if d.UsageLimit > 0 && d.UsageCount >= d.UsageLimit {
			continue
		}
		if !e.checkMinPurchase(d, dc) {
			continue
		}
		if !e.checkProductScope(ctx, d, dc.ProductIDs) {
			continue
		}
		valid = append(valid, c)
	}

	// 4. Calculate discount amounts and resolve stacking
	var productDiscounts, orderDiscounts, shippingDiscounts []resolvedDiscount

	for _, c := range valid {
		amount, isShipping := e.calculateAmount(c.discount, dc)
		rd := resolvedDiscount{
			candidate: c,
			amount:    amount,
		}
		if isShipping {
			shippingDiscounts = append(shippingDiscounts, rd)
		} else if c.discount.Scope == models.DiscountScopeProduct {
			productDiscounts = append(productDiscounts, rd)
		} else {
			orderDiscounts = append(orderDiscounts, rd)
		}
	}

	// 5. Apply stacking rules: pick best within each category if not combinable
	selected := e.resolveStacking(productDiscounts, orderDiscounts, shippingDiscounts)

	// 6. Build result
	totalDiscount := big.NewInt(0)
	for _, rd := range selected {
		c := rd.candidate
		codeID := ""
		codeStr := ""
		if c.code != nil {
			codeID = c.code.ID
			codeStr = c.codeStr
		}

		amountStr := new(big.Int).Neg(rd.amount).String()

		result.AppliedDiscounts = append(result.AppliedDiscounts, AppliedDiscount{
			DiscountID: c.discount.ID,
			CodeID:     codeID,
			Title:      c.discount.Title,
			Code:       codeStr,
			ValueType:  string(c.discount.ValueType),
			Value:      c.discount.Value,
			Amount:     amountStr,
			Auto:       c.auto,
		})

		totalDiscount.Add(totalDiscount, rd.amount)

		if c.discount.ValueType == models.DiscountValueFreeShipping {
			result.ShippingDiscount = true
		}
	}

	// Floor total discount to subtotal (discount cannot exceed subtotal)
	if totalDiscount.Cmp(dc.SubTotal) > 0 {
		totalDiscount.Set(dc.SubTotal)
	}

	result.DiscountsTotal = new(big.Int).Neg(totalDiscount)
	return result, nil
}

type discountCandidate struct {
	discount *models.Discount
	code     *models.DiscountCode
	codeStr  string
	auto     bool
}

type resolvedDiscount struct {
	candidate discountCandidate
	amount    *big.Int
}

func (e *DiscountEngine) checkMinPurchase(d *models.Discount, dc DiscountContext) bool {
	switch d.MinPurchaseType {
	case models.DiscountMinPurchaseAmount:
		if d.MinAmount == nil {
			return true
		}
		minAmt := big.NewInt(0)
		if dc.ConvertAmount != nil {
			converted, err := dc.ConvertAmount(*d.MinAmount, d.Currency)
			if err == nil {
				minAmt = converted
			}
		} else {
			minAmt.SetString(*d.MinAmount, 10)
		}
		return dc.SubTotal.Cmp(minAmt) >= 0
	case models.DiscountMinPurchaseQuantity:
		if d.MinQuantity == nil {
			return true
		}
		return dc.ItemQuantity >= *d.MinQuantity
	default:
		return true
	}
}

func (e *DiscountEngine) checkProductScope(ctx context.Context, d *models.Discount, productIDs []string) bool {
	switch d.AppliesTo {
	case models.DiscountAppliesToAll:
		return true
	case models.DiscountAppliesToSpecificProducts:
		return discountHasOverlap(d.ProductIDs, productIDs)
	case models.DiscountAppliesToSpecificCollections:
		if e.collectionStore == nil || len(d.CollectionIDs) == 0 {
			return false
		}
		for _, pid := range productIDs {
			found, err := e.collectionStore.IsProductInCollections(ctx, d.CollectionIDs, pid)
			if err == nil && found {
				return true
			}
		}
		return false
	default:
		return true
	}
}

func (e *DiscountEngine) calculateAmount(d *models.Discount, dc DiscountContext) (*big.Int, bool) {
	switch d.ValueType {
	case models.DiscountValuePercentage:
		pct, ok := new(big.Int).SetString(d.Value, 10)
		if !ok {
			return big.NewInt(0), false
		}
		amount := new(big.Int).Mul(dc.SubTotal, pct)
		amount.Div(amount, big.NewInt(100))

		if d.MaxDiscountAmount != nil && *d.MaxDiscountAmount != "" {
			maxAmt := big.NewInt(0)
			if dc.ConvertAmount != nil {
				converted, err := dc.ConvertAmount(*d.MaxDiscountAmount, d.Currency)
				if err == nil {
					maxAmt = converted
				}
			} else {
				maxAmt.SetString(*d.MaxDiscountAmount, 10)
			}
			if maxAmt.Sign() > 0 && amount.Cmp(maxAmt) > 0 {
				amount.Set(maxAmt)
			}
		}
		return amount, false

	case models.DiscountValueFixed:
		amount := big.NewInt(0)
		if dc.ConvertAmount != nil {
			converted, err := dc.ConvertAmount(d.Value, d.Currency)
			if err == nil {
				amount = converted
			}
		} else {
			amount.SetString(d.Value, 10)
		}
		if amount.Cmp(dc.SubTotal) > 0 {
			amount.Set(dc.SubTotal)
		}
		return amount, false

	case models.DiscountValueFreeShipping:
		return big.NewInt(0), true

	default:
		return big.NewInt(0), false
	}
}

// resolveStacking applies combination rules: if two discounts in different categories
// are not mutually combinable, keep the one with the largest discount amount.
func (e *DiscountEngine) resolveStacking(product, order, shipping []resolvedDiscount) []resolvedDiscount {
	bestProduct := pickBest(product)
	bestOrder := pickBest(order)
	bestShipping := pickBest(shipping)

	var result []resolvedDiscount

	if bestProduct != nil && bestOrder != nil {
		pCombinesWithOrder := bestProduct.candidate.discount.CombinesWithOrder
		oCombinesWithProduct := bestOrder.candidate.discount.CombinesWithProduct

		if pCombinesWithOrder && oCombinesWithProduct {
			result = append(result, *bestProduct, *bestOrder)
		} else {
			if bestProduct.amount.Cmp(bestOrder.amount) >= 0 {
				result = append(result, *bestProduct)
			} else {
				result = append(result, *bestOrder)
			}
		}
	} else if bestProduct != nil {
		result = append(result, *bestProduct)
	} else if bestOrder != nil {
		result = append(result, *bestOrder)
	}

	if bestShipping != nil {
		canCombine := true
		for _, r := range result {
			if !r.candidate.discount.CombinesWithShipping {
				canCombine = false
				break
			}
		}
		if canCombine {
			result = append(result, *bestShipping)
		}
	}

	return result
}

func discountHasOverlap(a, b []string) bool {
	set := make(map[string]struct{}, len(a))
	for _, v := range a {
		set[v] = struct{}{}
	}
	for _, v := range b {
		if _, ok := set[v]; ok {
			return true
		}
	}
	return false
}

func pickBest(discounts []resolvedDiscount) *resolvedDiscount {
	if len(discounts) == 0 {
		return nil
	}
	best := &discounts[0]
	for i := 1; i < len(discounts); i++ {
		if discounts[i].amount.Cmp(best.amount) > 0 {
			best = &discounts[i]
		}
	}
	return best
}
