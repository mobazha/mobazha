package core

import (
	"context"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/internal/wallet"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"google.golang.org/protobuf/proto"
)

// freeShippingDiscountValueType matches mbzpb.Order.AppliedDiscount.valueType
// for free-shipping discounts. Those reduce shipping fees, not item revenue,
// so they are excluded from the margin gate's revenue prorate.
const freeShippingDiscountValueType = "free_shipping"

// marginSafetyPct is the maximum percentage of retail that supplier
// cost+shipping may consume before auto-fulfillment is rejected.
const marginSafetyPct uint64 = 80

// parseUSDDollarsToCents parses a USD dollar amount string ("4", "4.5", "4.50",
// "0.99") into integer cents. Returns (cents, true) on a clean parse with at
// most 2 fractional digits. Returns (0, false) for empty input or any
// malformed value (e.g. "abc", "1.234", "-1", multiple decimal points).
//
// Used by margin gate shipping rate parsing. The previous implementation used
// strings.ReplaceAll(s, ".", "") which silently mis-parsed "4" as 4 cents and
// "4.5" as 45 cents — both off by 10x or 100x.
func parseUSDDollarsToCents(s string) (uint64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	// Reject negatives and explicit signs.
	if s[0] == '-' || s[0] == '+' {
		return 0, false
	}

	dot := strings.IndexByte(s, '.')
	var dollars, cents uint64
	if dot < 0 {
		// Integer dollars: "4" → 400 cents.
		v, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return 0, false
		}
		dollars = v
	} else {
		// "X.YY" format. Reject more than one decimal point.
		intPart := s[:dot]
		fracPart := s[dot+1:]
		if strings.IndexByte(fracPart, '.') >= 0 {
			return 0, false
		}
		if len(fracPart) == 0 || len(fracPart) > 2 {
			return 0, false
		}
		// Empty integer part ("." or ".5") — accept ".5" as 50 cents.
		if intPart != "" {
			v, err := strconv.ParseUint(intPart, 10, 64)
			if err != nil {
				return 0, false
			}
			dollars = v
		}
		// Pad fractional to 2 digits: "5" → "50", "50" → "50".
		if len(fracPart) == 1 {
			fracPart = fracPart + "0"
		}
		v, err := strconv.ParseUint(fracPart, 10, 64)
		if err != nil {
			return 0, false
		}
		cents = v
	}
	// Overflow check: dollars * 100 must not wrap.
	if dollars > (^uint64(0))/100 {
		return 0, false
	}
	return dollars*100 + cents, true
}

// resolveSKUEconomics returns (supplierCostCents, retailCents, ok, reason) for
// the SKU the buyer actually selected.
//
// The supplier cost comes from the PRIVATE per-variant cost map stored in
// `SyncedProductMapping.Metadata` (loaded by loadVariantCostMap). The retail
// price comes from the PUBLIC `Skus[i].Price` field. Critically, supplier
// wholesale cost is NEVER read from `Skus[i].CompareAtPrice` — that's the
// public strike-through "original price" rendered to buyers (see Listing.proto
// and unified useProductDetail).
//
// Resolution order:
//
//  1. Match buyer's option selections to a SKU (matchesSKUSelections). Read
//     the SKU's `ProductID` (= catalog variant ID written by
//     buildListingFromCatalog), then look up cost in the private map.
//     Retail = SKU.Price.
//  2. Fallback for single-SKU listings only: use
//     `SyncedProductMapping.SupplierCost` + `Listing.Item.Price` snapshot.
//     Multi-SKU listings must NOT fall back to the listing-level snapshot
//     because it stores only the cheapest variant's cost and would
//     underestimate margin for higher-cost selections.
func resolveSKUEconomics(
	listing *pb.Listing,
	orderItem *pb.OrderOpen_Item,
	spm *models.SyncedProductMapping,
) (cost uint64, retail uint64, ok bool, reason string) {
	if listing == nil || listing.Item == nil {
		return 0, 0, false, "listing.Item missing"
	}
	skus := listing.Item.Skus

	// Step 1: locate the SKU the buyer chose.
	var matched *pb.Listing_Item_Sku
	if orderItem != nil {
		buyerSelections := make(map[string]string, len(orderItem.Options))
		for _, opt := range orderItem.Options {
			buyerSelections[opt.Name] = opt.Value
		}
		if len(buyerSelections) == 0 && len(skus) == 1 {
			matched = skus[0]
		} else {
			for _, sku := range skus {
				if matchesSKUSelections(sku, buyerSelections) {
					matched = sku
					break
				}
			}
		}
	} else if len(skus) == 1 {
		matched = skus[0]
	}

	// Step 2: per-variant cost from PRIVATE metadata, retail from public SKU.Price.
	if matched != nil && matched.Price != "" {
		costMap := loadVariantCostMap(spm)
		variantID := matched.GetProductID()
		if c, found := costMap[variantID]; found && c > 0 {
			r, errR := strconv.ParseUint(matched.Price, 10, 64)
			if errR == nil && r > 0 {
				return c, r, true, ""
			}
		}
	}

	// Step 3: single-SKU fallback to listing-level snapshot.
	if len(skus) <= 1 && spm != nil && spm.SupplierCost != "" && listing.Item.Price != "" {
		c, errC := strconv.ParseUint(spm.SupplierCost, 10, 64)
		r, errR := strconv.ParseUint(listing.Item.Price, 10, 64)
		if errC == nil && errR == nil && c > 0 && r > 0 {
			return c, r, true, ""
		}
	}

	if matched == nil && len(skus) > 1 {
		return 0, 0, false, "could not resolve buyer-selected SKU from order options"
	}
	if matched != nil {
		return 0, 0, false, fmt.Sprintf(
			"variant %q has no supplier cost in SyncedProductMapping.Metadata (re-import or wait for FF-3 quote API)",
			matched.GetProductID())
	}
	return 0, 0, false, "no SKU economics and no usable import-time snapshot"
}

// supplyMarginInputs is the resolved data required to evaluate the margin gate.
// Built once per (order, retry attempt).
type supplyMarginInputs struct {
	oo         *pb.OrderOpen
	providerID string
	recipient  contracts.FulfillmentRecipient
	items      []contracts.FulfillmentItem
}

// evaluateMarginGate runs the unified margin protection logic shared by
// handleOrderFunded (first-attempt) and processRetry (retry worker).
//
// On pass returns (true, FailureReasonNone, "").
// On reject returns (false, reason, msg) — caller marks the mapping with the
// returned reason+msg. The function is read-only: it does NOT write to the DB
// or call providers other than EstimateShipping.
//
// Resolution sequence:
//  1. Per-listing: resolve buyer-selected SKU economics (cost from PRIVATE
//     metadata, retail from public SKU.Price). See resolveSKUEconomics.
//  2. Sum supply-chain (sc) retail and ALL-listings retail across the order.
//  3. If OrderOpen carries AppliedDiscounts, prorate every non-shipping
//     discount across listings by retail share, then subtract the
//     supply-chain share from `scRetail` to get effective revenue. This is
//     critical: an order with $20 SKU price + 30% discount produces $14 of
//     real revenue, and the gate must compare cost+shipping to $14, not $20,
//     or thin-margin orders slip through.
//  4. Estimate shipping (cheapest rate), require every rate to parse.
//  5. Reject when (cost + shipping) * 100 ≥ effectiveRevenue * marginSafetyPct.
//
// Rejection reasons:
//   - manual_action_required: cannot resolve buyer's SKU, SKU lacks economics,
//     shipping estimate failed, any rate string is unparseable, or the order
//     carries an unrecognized discount value type we cannot prorate safely
//   - margin_protection_failed: cost+shipping ≥ marginSafetyPct of effective
//     revenue (post-discount)
func (s *SupplyChainAppService) evaluateMarginGate(
	ctx context.Context,
	in supplyMarginInputs,
) (bool, contracts.FailureReason, string) {
	if in.oo == nil {
		return false, contracts.FailureReasonValidationFailed, "missing OrderOpen"
	}

	// scRetail = supply-chain listings only. allRetail spans every listing in
	// the order (including ones not managed by supply chain) and is the
	// denominator for prorating discounts.
	//
	// IMPORTANT — currency unit invariant:
	//   * Listing.Item.Price / SKU.Price are in the LISTING's PricingCurrency
	//     smallest unit (typically USD cents — Printful catalog is USD,
	//     divisibility=2).
	//   * OrderOpen.AppliedDiscounts[i].amount is in the PAYMENT currency
	//     smallest unit (orders.proto:30 — could be BTC satoshis, ETH wei,
	//     MCK, etc. for crypto orders), which differs from listing currency
	//     when buyer pays a token for a USD-priced item.
	//
	// We compare scRetail (listing units) with the discount amounts directly
	// in proratedSCDiscount, so we MUST verify the units actually match. When
	// PricingCoin diverges from listing currency and exchangeRates is available,
	// Pass 2 converts discount amounts via wallet.ConvertFiatAmount. When
	// exchangeRates is nil, we fail closed (manual_action_required).
	var totalSupplierCost, scRetail, allRetail uint64
	var scListingCurrency string

	// Pass 1: per-listing economics.
	for i, li := range in.oo.Listings {
		if li == nil || li.Listing == nil || li.Listing.Item == nil {
			continue
		}
		slug := li.Listing.GetSlug()
		if slug == "" {
			continue
		}

		// Locate the buyer's chosen SKU + qty for ALL listings (used for
		// allRetail denominator), regardless of supply-chain status.
		var orderItem *pb.OrderOpen_Item
		if i < len(in.oo.Items) {
			orderItem = in.oo.Items[i]
		}
		qty := uint64(1)
		if orderItem != nil {
			if q, qErr := strconv.Atoi(orderItem.Quantity); qErr == nil && q > 0 {
				qty = uint64(q)
			}
		}
		anyRetail := publicSKURetailCents(li.Listing, orderItem)
		allRetail += anyRetail * qty

		// Now check if it's supply-chain-managed.
		var spm models.SyncedProductMapping
		if findErr := s.db.View(func(tx database.Tx) error {
			return tx.Read().Where("listing_slug = ?", slug).First(&spm).Error
		}); findErr != nil {
			// Not supply-chain-managed — counted in allRetail but not in
			// scRetail/totalSupplierCost. Mixed-order safety is enforced by
			// the caller.
			continue
		}

		// Capture & verify listing currency consistency across all SC items.
		listingCurrency := strings.ToUpper(strings.TrimSpace(li.Listing.GetMetadata().GetPricingCurrency().GetCode()))
		if listingCurrency == "" {
			return false, contracts.FailureReasonManualActionRequired,
				fmt.Sprintf("listing %q has no PricingCurrency.Code — cannot validate discount unit", slug)
		}
		if scListingCurrency == "" {
			scListingCurrency = listingCurrency
		} else if scListingCurrency != listingCurrency {
			return false, contracts.FailureReasonManualActionRequired,
				fmt.Sprintf("supply-chain listings span multiple currencies (%s vs %s) — cannot prorate discount safely",
					scListingCurrency, listingCurrency)
		}

		costCents, retailCents, ok, why := resolveSKUEconomics(li.Listing, orderItem, &spm)
		if !ok {
			return false, contracts.FailureReasonManualActionRequired,
				fmt.Sprintf("listing %q: %s", slug, why)
		}
		totalSupplierCost += costCents * qty
		scRetail += retailCents * qty
	}

	if scRetail == 0 {
		return false, contracts.FailureReasonManualActionRequired,
			"no supply-chain-managed listings or zero retail total — refusing to ship"
	}

	// Pass 2: prorate post-discount revenue onto the supply-chain portion.
	// Discount amount lives in OrderOpen.PricingCoin's smallest unit, which
	// must match the listing currency for direct subtraction to be safe.
	// When currencies differ and ExchangeRates is available, we convert each
	// discount amount from PricingCoin → scListingCurrency before prorate.
	pricingCoin := strings.ToUpper(strings.TrimSpace(in.oo.GetPricingCoin()))
	discountsForProrate := in.oo.AppliedDiscounts
	if len(in.oo.AppliedDiscounts) > 0 && pricingCoin != "" && pricingCoin != scListingCurrency {
		if s.exchangeRates == nil {
			return false, contracts.FailureReasonManualActionRequired,
				fmt.Sprintf("order pays in %s but supply-chain listings price in %s; "+
					"discount amounts cannot be prorated without exchange rate (auto-ship blocked, "+
					"manual review required)",
					pricingCoin, scListingCurrency)
		}
		converted, convErr := s.convertDiscountsToListingCurrency(in.oo.AppliedDiscounts, pricingCoin, scListingCurrency)
		if convErr != nil {
			return false, contracts.FailureReasonManualActionRequired,
				fmt.Sprintf("cross-currency discount conversion failed (%s→%s): %v",
					pricingCoin, scListingCurrency, convErr)
		}
		discountsForProrate = converted
	}
	scDiscount, drErr := proratedSCDiscount(discountsForProrate, scRetail, allRetail)
	if drErr != nil {
		return false, contracts.FailureReasonManualActionRequired, drErr.Error()
	}
	effectiveRevenue := uint64(0)
	if scRetail > scDiscount {
		effectiveRevenue = scRetail - scDiscount
	}
	if effectiveRevenue == 0 {
		// Discount fully consumed (or exceeded) supply-chain retail. Auto-ship
		// would lose the entire supplier cost.
		return false, contracts.FailureReasonMarginProtectionFailed,
			fmt.Sprintf("post-discount revenue is zero (scRetail=%d, scDiscount=%d)",
				scRetail, scDiscount)
	}

	// Shipping: must succeed AND every returned rate must parse cleanly.
	estimates, estErr := s.EstimateShipping(ctx, in.providerID, contracts.ShippingEstimateParams{
		Recipient: in.recipient,
		Items:     in.items,
	})
	if estErr != nil {
		return false, contracts.FailureReasonManualActionRequired,
			fmt.Sprintf("shipping estimate failed: %v", estErr)
	}
	if len(estimates) == 0 {
		return false, contracts.FailureReasonManualActionRequired,
			"supplier returned no shipping estimates"
	}

	// Shipping unit invariant: each rate's Currency should equal the supply-chain
	// listing currency. When they differ and ExchangeRates is available, we
	// convert the rate amount. Provider adapters MUST set
	// ShippingEstimate.Currency explicitly — empty values fail closed.
	var minShippingCents uint64
	firstSet := false
	for _, est := range estimates {
		rateCurrency := strings.ToUpper(strings.TrimSpace(est.Currency))
		if rateCurrency == "" {
			return false, contracts.FailureReasonManualActionRequired,
				fmt.Sprintf("shipping estimate %q has no Currency set — provider must declare it explicitly", est.ID)
		}
		// Decimal-to-smallest-unit conversion. Listing divisibility is fixed
		// at 2 in buildListingFromCatalog (Printful catalog), so 2 fractional
		// digits is correct for USD/EUR/CNY etc.
		rateCents, ok := parseUSDDollarsToCents(est.Rate)
		if !ok {
			return false, contracts.FailureReasonManualActionRequired,
				fmt.Sprintf("shipping rate %q (option %q, currency %s) is not a parseable decimal amount",
					est.Rate, est.ID, rateCurrency)
		}
		if rateCurrency != scListingCurrency {
			if s.exchangeRates == nil {
				return false, contracts.FailureReasonManualActionRequired,
					fmt.Sprintf("shipping estimate %q quotes %s but supply-chain listings price in %s; "+
						"cannot add to supplier cost without exchange rate (auto-ship blocked, manual review required)",
						est.ID, rateCurrency, scListingCurrency)
			}
			converted, convErr := wallet.ConvertFiatAmount(int64(rateCents), rateCurrency, scListingCurrency, s.exchangeRates)
			if convErr != nil {
				return false, contracts.FailureReasonManualActionRequired,
					fmt.Sprintf("shipping estimate %q: cross-currency conversion failed (%s→%s): %v",
						est.ID, rateCurrency, scListingCurrency, convErr)
			}
			if converted < 0 {
				converted = 0
			}
			rateCents = uint64(converted)
		}
		if !firstSet || rateCents < minShippingCents {
			minShippingCents = rateCents
			firstSet = true
		}
	}
	if !firstSet {
		return false, contracts.FailureReasonManualActionRequired,
			"could not determine cheapest shipping rate"
	}

	totalCost := totalSupplierCost + minShippingCents
	// Use uint128-safe big.Int multiplication: pathological inputs (1B+ cents)
	// could overflow uint64 when multiplied by 100 / marginSafetyPct.
	lhs := new(big.Int).Mul(big.NewInt(int64(totalCost)), big.NewInt(100))
	rhs := new(big.Int).Mul(big.NewInt(int64(effectiveRevenue)), big.NewInt(int64(marginSafetyPct)))
	if lhs.Cmp(rhs) >= 0 {
		reason := fmt.Sprintf("supplier cost+shipping (%d cents) >= %d%% of effective revenue (%d cents post-discount, scRetail=%d, scDiscount=%d)",
			totalCost, marginSafetyPct, effectiveRevenue, scRetail, scDiscount)
		return false, contracts.FailureReasonMarginProtectionFailed, reason
	}

	logger.LogInfoWithIDf(log, s.nodeID,
		"SupplyChain margin gate OK: providerID=%s effectiveRevenue=%d (scRetail=%d scDiscount=%d) totalCost=%d (cost=%d shipping=%d) safety=%d%%",
		in.providerID, effectiveRevenue, scRetail, scDiscount,
		totalCost, totalSupplierCost, minShippingCents, marginSafetyPct)
	return true, contracts.FailureReasonNone, ""
}

// convertDiscountsToListingCurrency clones the applied discounts and converts
// each non-shipping amount from pricingCoin to scListingCurrency using the
// node's exchange rate provider. The returned slice can be passed directly to
// proratedSCDiscount which expects amounts in the listing currency's smallest
// unit (cents). Conversion errors are surfaced immediately — we never silently
// fall back to unconverted amounts.
func (s *SupplyChainAppService) convertDiscountsToListingCurrency(
	discounts []*pb.OrderOpen_AppliedDiscount,
	pricingCoin, scListingCurrency string,
) ([]*pb.OrderOpen_AppliedDiscount, error) {
	result := make([]*pb.OrderOpen_AppliedDiscount, 0, len(discounts))
	for _, d := range discounts {
		if d == nil {
			continue
		}
		if strings.EqualFold(d.GetValueType(), freeShippingDiscountValueType) {
			result = append(result, d)
			continue
		}
		raw := strings.TrimSpace(d.GetAmount())
		if raw == "" {
			result = append(result, d)
			continue
		}

		neg := false
		numStr := raw
		if numStr[0] == '-' {
			neg = true
			numStr = numStr[1:]
		}
		v, err := strconv.ParseInt(numStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("discount %q has unparseable amount %q", d.GetDiscountID(), raw)
		}

		converted, convErr := wallet.ConvertFiatAmount(v, pricingCoin, scListingCurrency, s.exchangeRates)
		if convErr != nil {
			return nil, fmt.Errorf("discount %q conversion %s→%s: %w",
				d.GetDiscountID(), pricingCoin, scListingCurrency, convErr)
		}

		sign := ""
		if neg {
			sign = "-"
		}
		// proto.Clone deep-copies without copying the embedded protoimpl.MessageState mutex.
		clone := proto.Clone(d).(*pb.OrderOpen_AppliedDiscount)
		clone.Amount = fmt.Sprintf("%s%d", sign, converted)
		result = append(result, clone)
	}
	return result, nil
}

// publicSKURetailCents returns the retail cents for a buyer-selected SKU
// (or the listing-level Item.Price when no SKU resolves). Used to compute
// allRetail for discount prorate. Returns 0 on parse failure (treat the
// listing as zero-revenue rather than aborting the whole gate, since
// non-supply-chain listings might legitimately have weird shapes).
func publicSKURetailCents(listing *pb.Listing, orderItem *pb.OrderOpen_Item) uint64 {
	if listing == nil || listing.Item == nil {
		return 0
	}
	skus := listing.Item.Skus
	var matched *pb.Listing_Item_Sku
	if orderItem != nil && len(skus) > 0 {
		buyerSelections := make(map[string]string, len(orderItem.Options))
		for _, opt := range orderItem.Options {
			buyerSelections[opt.Name] = opt.Value
		}
		if len(buyerSelections) == 0 && len(skus) == 1 {
			matched = skus[0]
		} else {
			for _, sku := range skus {
				if matchesSKUSelections(sku, buyerSelections) {
					matched = sku
					break
				}
			}
		}
	} else if len(skus) == 1 {
		matched = skus[0]
	}
	if matched != nil && matched.Price != "" {
		v, err := strconv.ParseUint(matched.Price, 10, 64)
		if err == nil {
			return v
		}
	}
	if listing.Item.Price != "" {
		v, err := strconv.ParseUint(listing.Item.Price, 10, 64)
		if err == nil {
			return v
		}
	}
	return 0
}

// proratedSCDiscount returns the supply-chain portion of all non-shipping
// discounts applied to the order, computed as:
//
//	sum_over_discounts |discount.amount| * scRetail / allRetail   (rounded UP)
//
// Rounding up is the safe direction: it gives the supply-chain side a larger
// discount allocation, lowering effective revenue and tightening the gate.
//
// Returns an error when an applied discount carries an unparseable amount or
// the order's allRetail is zero (cannot prorate).
//
// free_shipping discounts reduce shipping fees, not item revenue, so they
// are skipped here. Their full amount is implicitly excluded from
// effectiveRevenue (we don't subtract shipping from revenue at all — shipping
// cost is added on the cost side instead).
func proratedSCDiscount(applied []*pb.OrderOpen_AppliedDiscount, scRetail, allRetail uint64) (uint64, error) {
	if len(applied) == 0 || scRetail == 0 {
		return 0, nil
	}
	if allRetail == 0 {
		return 0, fmt.Errorf("allRetail is zero but order carries %d applied discounts; cannot prorate", len(applied))
	}

	totalAbsDiscount := big.NewInt(0)
	for _, d := range applied {
		if d == nil {
			continue
		}
		if strings.EqualFold(d.GetValueType(), freeShippingDiscountValueType) {
			continue
		}
		raw := strings.TrimSpace(d.GetAmount())
		if raw == "" {
			continue
		}
		// AppliedDiscount.amount is a NEGATIVE integer in payment currency
		// smallest unit (per orders.proto). Strip the leading sign and parse.
		neg := false
		if raw[0] == '-' {
			neg = true
			raw = raw[1:]
		}
		v, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("applied discount %q has unparseable amount %q (value type %q)",
				d.GetDiscountID(), d.GetAmount(), d.GetValueType())
		}
		_ = neg // stored as positive total below; sign was just a sanity check.
		totalAbsDiscount.Add(totalAbsDiscount, new(big.Int).SetUint64(v))
	}
	if totalAbsDiscount.Sign() == 0 {
		return 0, nil
	}

	// scShare = ceil(totalAbsDiscount * scRetail / allRetail)
	num := new(big.Int).Mul(totalAbsDiscount, new(big.Int).SetUint64(scRetail))
	denom := new(big.Int).SetUint64(allRetail)
	q, r := new(big.Int).QuoRem(num, denom, new(big.Int))
	if r.Sign() != 0 {
		q.Add(q, big.NewInt(1))
	}
	if !q.IsUint64() {
		return 0, fmt.Errorf("prorated discount overflow")
	}
	return q.Uint64(), nil
}
