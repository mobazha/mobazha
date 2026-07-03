package api

import (
	"testing"

	"github.com/mobazha/mobazha/pkg/models"
	"github.com/mobazha/mobazha/pkg/storefront"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

// makeIndex builds a ListingIndex populated with integer prices so the
// test assertions stay arithmetic-simple and avoid big.Int edge cases.
func makeIndex(prices ...int64) models.ListingIndex {
	out := make(models.ListingIndex, 0, len(prices))
	for i, p := range prices {
		out = append(out, models.ListingMetadata{
			Slug: string(rune('a' + i)),
			Price: models.CurrencyValue{
				Amount: iwallet.NewAmount(p),
			},
		})
	}
	return out
}

// TestApplyStorefrontPriceRuleToIndex_Nil — a nil / zero rule leaves the
// base prices untouched. Covers the fast-path used by main-host traffic.
func TestApplyStorefrontPriceRuleToIndex_Nil(t *testing.T) {
	index := makeIndex(1000, 2000)
	applyStorefrontPriceRuleToIndex(index, nil)
	if index[0].Price.Amount.Cmp(iwallet.NewAmount(1000)) != 0 {
		t.Fatalf("nil rule changed price[0]: got %s", index[0].Price.Amount.String())
	}
	if index[1].Price.Amount.Cmp(iwallet.NewAmount(2000)) != 0 {
		t.Fatalf("nil rule changed price[1]: got %s", index[1].Price.Amount.String())
	}

	applyStorefrontPriceRuleToIndex(index, &storefront.PriceRule{Type: storefront.PriceRuleTypeFlatDiscount, ValuePct: 0})
	if index[0].Price.Amount.Cmp(iwallet.NewAmount(1000)) != 0 {
		t.Fatalf("zero-value rule changed price[0]: got %s", index[0].Price.Amount.String())
	}
}

// TestApplyStorefrontPriceRuleToIndex_FlatDiscount — a 15% discount on
// 10_000 minor units should produce 8_500 (10000 × 8500/10000 = 8500).
// Verifies the percent → basis-point conversion and big.Int arithmetic
// work end-to-end through the handler helper.
func TestApplyStorefrontPriceRuleToIndex_FlatDiscount(t *testing.T) {
	index := makeIndex(10000, 100)
	rule := &storefront.PriceRule{Type: storefront.PriceRuleTypeFlatDiscount, ValuePct: 15}
	applyStorefrontPriceRuleToIndex(index, rule)

	if got := index[0].Price.Amount; got.Cmp(iwallet.NewAmount(8500)) != 0 {
		t.Fatalf("15%% discount of 10000: got %s, want 8500", got.String())
	}
	// 100 × 8500 / 10000 = 85
	if got := index[1].Price.Amount; got.Cmp(iwallet.NewAmount(85)) != 0 {
		t.Fatalf("15%% discount of 100: got %s, want 85", got.String())
	}
}

// TestApplyStorefrontPriceRuleToIndex_FlatMarkup — a 20% markup on 1_000
// gives 1_200. Also verifies the markup branch chooses addition rather
// than subtraction.
func TestApplyStorefrontPriceRuleToIndex_FlatMarkup(t *testing.T) {
	index := makeIndex(1000)
	rule := &storefront.PriceRule{Type: storefront.PriceRuleTypeFlatMarkup, ValuePct: 20}
	applyStorefrontPriceRuleToIndex(index, rule)

	if got := index[0].Price.Amount; got.Cmp(iwallet.NewAmount(1200)) != 0 {
		t.Fatalf("20%% markup of 1000: got %s, want 1200", got.String())
	}
}

// TestApplyStorefrontPriceRuleToIndex_FixedSurcharge — a +50 surcharge
// should add exactly 50 to every listing. Differs from percentage rules
// because the operand is absolute not relative.
func TestApplyStorefrontPriceRuleToIndex_FixedSurcharge(t *testing.T) {
	index := makeIndex(100, 200)
	rule := &storefront.PriceRule{Type: storefront.PriceRuleTypeFixedSurcharge, AmountMinor: 50}
	applyStorefrontPriceRuleToIndex(index, rule)

	if got := index[0].Price.Amount; got.Cmp(iwallet.NewAmount(150)) != 0 {
		t.Fatalf("+50 surcharge of 100: got %s, want 150", got.String())
	}
	if got := index[1].Price.Amount; got.Cmp(iwallet.NewAmount(250)) != 0 {
		t.Fatalf("+50 surcharge of 200: got %s, want 250", got.String())
	}
}

// TestApplyStorefrontPriceRuleToIndex_EmptyIndex — no-op on an empty
// listing index. Guards the bounds check inside the helper loop.
func TestApplyStorefrontPriceRuleToIndex_EmptyIndex(t *testing.T) {
	var index models.ListingIndex
	rule := &storefront.PriceRule{Type: storefront.PriceRuleTypeFlatDiscount, ValuePct: 10}
	applyStorefrontPriceRuleToIndex(index, rule)
	if len(index) != 0 {
		t.Fatalf("expected empty index to stay empty, got %d entries", len(index))
	}
}

// TestApplyStorefrontPriceRuleToIndex_PreservesCurrency — a rule must
// only mutate Amount; the currency pointer and any future divisibility
// metadata must stay intact so the client still renders "$X.YZ" not
// just "X bare minor units".
func TestApplyStorefrontPriceRuleToIndex_PreservesCurrency(t *testing.T) {
	usd := &models.Currency{Code: "USD", Divisibility: 2}
	index := models.ListingIndex{
		models.ListingMetadata{
			Slug: "a",
			Price: models.CurrencyValue{
				Amount:   iwallet.NewAmount(10000),
				Currency: usd,
			},
		},
	}
	rule := &storefront.PriceRule{Type: storefront.PriceRuleTypeFlatDiscount, ValuePct: 10}
	applyStorefrontPriceRuleToIndex(index, rule)

	if index[0].Price.Currency == nil || index[0].Price.Currency.Code != "USD" {
		t.Fatalf("expected USD currency preserved, got %+v", index[0].Price.Currency)
	}
	if got := index[0].Price.Amount; got.Cmp(iwallet.NewAmount(9000)) != 0 {
		t.Fatalf("expected 9000 after 10%% discount, got %s", got.String())
	}
}
