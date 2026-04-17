// Package storefront — shared Node-side types for MS-Phase-2a storefront
// routing. Kept under pkg/ so hosting and any future consumer can depend
// on the same wire format.
//
// This package only carries value objects (no behavior coupling to
// MobazhaNode). Runtime application lives in internal/api handlers.
package storefront

import (
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"strings"

	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// PriceRule applies a deterministic transform over a listing's base price
// for requests routed to a specific storefront (MS-Phase-2a · MS2a.5).
//
// The wire format mirrors hosting's db.StorefrontPriceRule so the two
// sides can serialize/deserialize via a single JSON blob carried in the
// X-Storefront-Price-Rule request header.
//
// Semantics:
//   - flat_discount:   new = base × (10000 - ValuePct·100) / 10000
//   - flat_markup:     new = base × (10000 + ValuePct·100) / 10000
//   - fixed_surcharge: new = base + AmountMinor
//
// Math is done in basis points on big.Int to preserve precision across
// the 256-bit range used by ERC-20 / native-token minor units. A rule
// that would take the price below zero is clamped to zero rather than
// rejected — the adjustment is intended for display (list view, SEO
// price cards), not for settlement, so silently clamping is safer than
// producing a nonsense "negative price" render.
type PriceRule struct {
	// Type is one of: "flat_discount", "flat_markup", "fixed_surcharge".
	Type string `json:"type"`

	// ValuePct is the percentage adjustment in percentage points
	// (e.g. 15 = 15%). Used by flat_discount / flat_markup; ignored
	// otherwise. Must be finite and non-negative.
	ValuePct float64 `json:"value_pct,omitempty"`

	// AmountMinor is used by fixed_surcharge in minor units of the
	// listing's currency (e.g. cents for USD, wei for ETH). Can be
	// negative to express a fixed discount as well.
	AmountMinor int64 `json:"amount_minor,omitempty"`
}

// Rule type constants — kept as string consts so mistyped JSON payloads
// are rejected by Validate rather than silently no-oping at apply time.
const (
	PriceRuleTypeFlatDiscount   = "flat_discount"
	PriceRuleTypeFlatMarkup     = "flat_markup"
	PriceRuleTypeFixedSurcharge = "fixed_surcharge"
)

// IsZero reports whether the rule would produce the identity transform
// (base price unchanged). Callers use this to short-circuit wiring work
// when an operator left the rule at its default.
func (r *PriceRule) IsZero() bool {
	if r == nil {
		return true
	}
	switch r.Type {
	case PriceRuleTypeFlatDiscount, PriceRuleTypeFlatMarkup:
		return r.ValuePct == 0
	case PriceRuleTypeFixedSurcharge:
		return r.AmountMinor == 0
	default:
		// Unknown types act as identity to preserve forward compat —
		// a newer hosting version can introduce a rule type without
		// breaking older nodes. Validate() flags these for operators.
		return true
	}
}

// Validate returns an error when the rule cannot be meaningfully applied.
// nil rules are valid (identity transform). Callers should treat a
// Validate failure as "skip this rule" rather than 500 — a misconfigured
// rule must not take down listing endpoints.
func (r *PriceRule) Validate() error {
	if r == nil {
		return nil
	}
	switch r.Type {
	case PriceRuleTypeFlatDiscount, PriceRuleTypeFlatMarkup:
		if math.IsNaN(r.ValuePct) || math.IsInf(r.ValuePct, 0) {
			return fmt.Errorf("storefront price rule %q: value_pct must be finite", r.Type)
		}
		if r.ValuePct < 0 {
			return fmt.Errorf("storefront price rule %q: value_pct must be non-negative", r.Type)
		}
		// A discount ≥100% would wipe the price to zero. Allow 100 but
		// cap anything higher at apply time via the clamp logic.
		if r.Type == PriceRuleTypeFlatDiscount && r.ValuePct > 100 {
			return fmt.Errorf("storefront price rule flat_discount: value_pct must be ≤ 100")
		}
	case PriceRuleTypeFixedSurcharge:
		// AmountMinor already bounded by int64; nothing to validate.
	default:
		return fmt.Errorf("storefront price rule: unknown type %q", r.Type)
	}
	return nil
}

// ApplyAmount returns base transformed by the rule. A nil receiver or an
// unknown/invalid type returns base unchanged so callers can chain this
// safely without pre-checking.
func (r *PriceRule) ApplyAmount(base iwallet.Amount) iwallet.Amount {
	if r.IsZero() {
		return base
	}
	if err := r.Validate(); err != nil {
		// Invalid rule acts as identity. Handler logs the error once
		// at middleware parse time; no need to re-log per listing.
		return base
	}

	switch r.Type {
	case PriceRuleTypeFlatDiscount:
		return applyBasisPoints(base, percentToBasisPoints(r.ValuePct), false)
	case PriceRuleTypeFlatMarkup:
		return applyBasisPoints(base, percentToBasisPoints(r.ValuePct), true)
	case PriceRuleTypeFixedSurcharge:
		return applyFixedSurcharge(base, r.AmountMinor)
	}
	return base
}

// percentToBasisPoints converts a float percentage (e.g. 15.5) to an
// integer basis-point multiplier (e.g. 1550). Rounds half-away-from-zero
// so 15.5% → 1550 bp rather than the banker's-rounding 1550. This avoids
// the "15% discount shows as 14.99%" class of bug in the UI.
func percentToBasisPoints(pct float64) int64 {
	return int64(math.Round(pct * 100))
}

// applyBasisPoints computes base × (10000 ± bp) / 10000 using big.Int.
// When markup is false, the rule subtracts bp (discount); when true, it
// adds bp. A discount that would push the result below zero clamps to 0.
func applyBasisPoints(base iwallet.Amount, bp int64, markup bool) iwallet.Amount {
	if bp == 0 {
		return base
	}
	const denom = int64(10000)
	multiplier := denom - bp
	if markup {
		multiplier = denom + bp
	}
	// Clamp: discount > 100% would be negative; treat as zero.
	if multiplier <= 0 {
		return iwallet.NewAmount(0)
	}

	baseInt := (*big.Int)(&base)
	num := new(big.Int).Mul(baseInt, big.NewInt(multiplier))
	out := new(big.Int).Quo(num, big.NewInt(denom))
	return iwallet.NewAmount(out)
}

// applyFixedSurcharge adds (or subtracts, when amountMinor is negative)
// amountMinor to/from the base amount. Clamps the result to zero rather
// than returning a negative price.
func applyFixedSurcharge(base iwallet.Amount, amountMinor int64) iwallet.Amount {
	if amountMinor == 0 {
		return base
	}
	baseInt := (*big.Int)(&base)
	out := new(big.Int).Add(baseInt, big.NewInt(amountMinor))
	if out.Sign() < 0 {
		return iwallet.NewAmount(0)
	}
	return iwallet.NewAmount(out)
}

// ParsePriceRule decodes the compact JSON carried in the request header.
// Empty / whitespace input returns (nil, nil) so callers can treat the
// header as "rule absent, apply identity". A malformed JSON blob returns
// an error — the middleware logs it and proceeds without a rule so a
// hosting bug cannot take down listing endpoints.
func ParsePriceRule(raw string) (*PriceRule, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var r PriceRule
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		return nil, fmt.Errorf("price rule: decode JSON: %w", err)
	}
	if err := r.Validate(); err != nil {
		return nil, err
	}
	if r.IsZero() {
		return nil, nil
	}
	return &r, nil
}

// Marshal returns the compact JSON form used in the X-Storefront-Price-Rule
// header. Exposed for hosting-side injection and for unit tests. Returns
// an empty string for nil or zero-valued rules so the caller can skip
// setting the header entirely (identity transform is the default).
func (r *PriceRule) Marshal() (string, error) {
	if r.IsZero() {
		return "", nil
	}
	b, err := json.Marshal(r)
	if err != nil {
		return "", fmt.Errorf("price rule: encode JSON: %w", err)
	}
	return string(b), nil
}
