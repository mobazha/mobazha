// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package orders

import (
	"fmt"
	"math/big"

	"github.com/mobazha/mobazha/internal/orders/utils"
	"github.com/mobazha/mobazha/internal/wallet"
	"github.com/mobazha/mobazha/pkg/models"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

// CalculateOrderNetMerchandiseLines derives the seller-authoritative line
// amounts used for affiliate commission. It excludes tax, shipping, provider
// fees, and free-shipping discounts. Non-shipping order discounts are allocated
// deterministically in item order so the result sums to discounted merchandise.
func CalculateOrderNetMerchandiseLines(order *pb.OrderOpen, erp wallet.ExchangeRateQuerier) ([]iwallet.Amount, error) {
	if order == nil || len(order.Items) == 0 {
		return nil, fmt.Errorf("order must contain items")
	}
	paymentCurrency, err := models.CurrencyDefinitions.Lookup(order.PricingCoin)
	if err != nil {
		return nil, fmt.Errorf("lookup pricing currency: %w", err)
	}
	lines := make([]iwallet.Amount, len(order.Items))
	for index, item := range order.Items {
		amount, err := calculateOrderMerchandiseLine(item, order.Listings, paymentCurrency, erp)
		if err != nil {
			return nil, fmt.Errorf("calculate merchandise for item %d: %w", index, err)
		}
		lines[index] = amount
	}
	discountTotal := iwallet.NewAmount(0)
	for _, discount := range order.AppliedDiscounts {
		if discount.GetValueType() == "free_shipping" || discount.GetAmount() == "" {
			continue
		}
		discountTotal = discountTotal.Add(iwallet.NewAmount(discount.GetAmount()))
	}
	return allocateAffiliateDiscount(lines, discountTotal)
}

func calculateOrderMerchandiseLine(item *pb.OrderOpen_Item, listings []*pb.SignedListing, paymentCurrency *models.Currency, erp wallet.ExchangeRateQuerier) (iwallet.Amount, error) {
	if item == nil {
		return iwallet.Amount{}, fmt.Errorf("order item is required")
	}
	listing, err := utils.ExtractListing(item.GetListingHash(), listings)
	if err != nil {
		return iwallet.Amount{}, err
	}
	if listing.GetMetadata() == nil || listing.GetItem() == nil {
		return iwallet.Amount{}, fmt.Errorf("listing metadata and item are required")
	}
	quantity := iwallet.NewAmount(item.GetQuantity())
	if listing.GetMetadata().GetContractType() == pb.Listing_Metadata_RWA_TOKEN {
		if err := validateRwaTokenQuantity(item.GetQuantity()); err != nil {
			return iwallet.Amount{}, err
		}
		quantity = iwallet.NewAmount(1)
	} else if quantity.Cmp(iwallet.NewAmount(0)) <= 0 {
		return iwallet.Amount{}, fmt.Errorf("quantity must be a positive integer")
	}
	pricingCurrency, err := models.CurrencyDefinitions.Lookup(listing.GetMetadata().GetPricingCurrency().GetCode())
	if err != nil {
		return iwallet.Amount{}, fmt.Errorf("lookup listing pricing currency: %w", err)
	}
	var itemAmount iwallet.Amount
	switch {
	case listing.GetMetadata().GetContractType() == pb.Listing_Metadata_RWA_TOKEN:
		itemAmount, err = calculateRwaTokenItemTotal(listing, item, pricingCurrency, paymentCurrency, erp)
	case listing.GetMetadata().GetFormat() == pb.Listing_Metadata_MARKET_PRICE:
		cryptoCurrency, lookupErr := models.CurrencyDefinitions.Lookup(listing.GetItem().GetCryptoListingCurrencyCode())
		if lookupErr != nil {
			return iwallet.Amount{}, fmt.Errorf("lookup market listing currency: %w", lookupErr)
		}
		itemAmount, err = wallet.ConvertCurrencyAmount(models.NewCurrencyValue(item.GetQuantity(), cryptoCurrency), paymentCurrency, erp)
		if err == nil {
			value, _ := new(big.Float).SetString(itemAmount.String())
			value.Mul(value, big.NewFloat(float64(listing.GetItem().GetCryptoListingPriceModifier()/100)))
			modifier, _ := value.Int(nil)
			itemAmount = itemAmount.Add(iwallet.NewAmount(modifier))
			quantity = iwallet.NewAmount(1)
		}
	default:
		itemAmount, err = wallet.ConvertCurrencyAmount(models.NewCurrencyValue(listing.GetItem().GetPrice(), pricingCurrency), paymentCurrency, erp)
	}
	if err != nil {
		return iwallet.Amount{}, err
	}
	if listing.GetMetadata().GetContractType() != pb.Listing_Metadata_RWA_TOKEN {
		sku, err := getSelectedSku(listing, item.GetOptions())
		if err != nil {
			return iwallet.Amount{}, err
		}
		if sku.GetPrice() != "" && iwallet.NewAmount(sku.GetPrice()).Cmp(iwallet.NewAmount(0)) > 0 {
			itemAmount, err = wallet.ConvertCurrencyAmount(models.NewCurrencyValue(sku.GetPrice(), pricingCurrency), paymentCurrency, erp)
			if err != nil {
				return iwallet.Amount{}, err
			}
		}
		for _, feature := range getSelectedOptionalFeatures(listing, item.GetOptionalFeatures()) {
			if feature.GetSurcharge() == "" {
				continue
			}
			surcharge, convertErr := wallet.ConvertCurrencyAmount(models.NewCurrencyValue(feature.GetSurcharge(), pricingCurrency), paymentCurrency, erp)
			if convertErr != nil {
				return iwallet.Amount{}, convertErr
			}
			itemAmount = itemAmount.Add(surcharge)
		}
	}
	return itemAmount.Mul(quantity), nil
}

func allocateAffiliateDiscount(lines []iwallet.Amount, discount iwallet.Amount) ([]iwallet.Amount, error) {
	if len(lines) == 0 {
		return nil, fmt.Errorf("merchandise lines are required")
	}
	total := iwallet.NewAmount(0)
	for _, line := range lines {
		if line.Cmp(iwallet.NewAmount(0)) < 0 {
			return nil, fmt.Errorf("merchandise line cannot be negative")
		}
		total = total.Add(line)
	}
	if total.Cmp(iwallet.NewAmount(0)) == 0 {
		if discount.Cmp(iwallet.NewAmount(0)) != 0 {
			return nil, fmt.Errorf("discount cannot be allocated to zero merchandise")
		}
		return append([]iwallet.Amount(nil), lines...), nil
	}
	result := make([]iwallet.Amount, len(lines))
	remaining := iwallet.NewAmount(discount.String())
	for index, line := range lines {
		share := remaining
		if index < len(lines)-1 {
			product := new(big.Int).Mul((*big.Int)(&discount), (*big.Int)(&line))
			share = iwallet.NewAmount(new(big.Int).Quo(product, (*big.Int)(&total)))
			remaining = remaining.Sub(share)
		}
		result[index] = line.Add(share)
		if result[index].Cmp(iwallet.NewAmount(0)) < 0 {
			return nil, fmt.Errorf("discount exceeds merchandise for line %d", index)
		}
	}
	return result, nil
}
