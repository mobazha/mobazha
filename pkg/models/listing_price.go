package models

import (
	"math/big"
	"strings"

	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
)

// ListingPriceSnapshot captures how a listing should be priced on storefront surfaces.
// DisplayAmount is the buyer-facing price (min explicit SKU price, else base).
// BaseAmount is item.price (fallback when SKU price is empty).
type ListingPriceSnapshot struct {
	DisplayAmount string
	BaseAmount    string
	MaxAmount     string
	HasRange      bool
	UsesSkuPrice  bool
}

// ResolveListingPriceSnapshot derives storefront pricing from item.price and SKU prices.
func ResolveListingPriceSnapshot(item *pb.Listing_Item) ListingPriceSnapshot {
	if item == nil {
		return ListingPriceSnapshot{
			DisplayAmount: "0",
			BaseAmount:    "0",
			MaxAmount:     "0",
		}
	}

	base := strings.TrimSpace(item.GetPrice())
	if base == "" {
		base = "0"
	}

	explicit := collectExplicitSkuPrices(item.GetSkus())
	if len(explicit) == 0 {
		return ListingPriceSnapshot{
			DisplayAmount: base,
			BaseAmount:    base,
			MaxAmount:     base,
		}
	}

	minPrice, maxPrice := minMaxBigInt(explicit)
	return ListingPriceSnapshot{
		DisplayAmount: minPrice.String(),
		BaseAmount:    base,
		MaxAmount:     maxPrice.String(),
		HasRange:      minPrice.Cmp(maxPrice) != 0,
		UsesSkuPrice:  true,
	}
}

func collectExplicitSkuPrices(skus []*pb.Listing_Item_Sku) []*big.Int {
	prices := make([]*big.Int, 0, len(skus))
	for _, sku := range skus {
		if sku == nil {
			continue
		}
		raw := strings.TrimSpace(sku.GetPrice())
		if raw == "" {
			continue
		}
		if val, ok := new(big.Int).SetString(raw, 10); ok {
			prices = append(prices, val)
		}
	}
	return prices
}

func minMaxBigInt(values []*big.Int) (*big.Int, *big.Int) {
	min := new(big.Int).Set(values[0])
	max := new(big.Int).Set(values[0])
	for _, v := range values[1:] {
		if v.Cmp(min) < 0 {
			min.Set(v)
		}
		if v.Cmp(max) > 0 {
			max.Set(v)
		}
	}
	return min, max
}
