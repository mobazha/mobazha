package storefront

import (
	"testing"

	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

func TestPriceRuleValidate(t *testing.T) {
	cases := []struct {
		name    string
		rule    PriceRule
		wantErr bool
	}{
		{"flat_discount ok", PriceRule{Type: PriceRuleTypeFlatDiscount, ValuePct: 15}, false},
		{"flat_discount 100% boundary", PriceRule{Type: PriceRuleTypeFlatDiscount, ValuePct: 100}, false},
		{"flat_discount >100%", PriceRule{Type: PriceRuleTypeFlatDiscount, ValuePct: 101}, true},
		{"flat_markup negative", PriceRule{Type: PriceRuleTypeFlatMarkup, ValuePct: -5}, true},
		{"flat_markup 500%", PriceRule{Type: PriceRuleTypeFlatMarkup, ValuePct: 500}, false},
		{"fixed_surcharge negative (fixed discount)", PriceRule{Type: PriceRuleTypeFixedSurcharge, AmountMinor: -100}, false},
		{"unknown type", PriceRule{Type: "mystery", ValuePct: 1}, true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := tc.rule.Validate()
			if (err != nil) != tc.wantErr {
				t.Fatalf("Validate() err = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestPriceRuleIsZero(t *testing.T) {
	cases := []struct {
		name string
		rule *PriceRule
		want bool
	}{
		{"nil", nil, true},
		{"empty struct", &PriceRule{}, true},
		{"flat_discount 0 pct", &PriceRule{Type: PriceRuleTypeFlatDiscount, ValuePct: 0}, true},
		{"flat_discount 15 pct", &PriceRule{Type: PriceRuleTypeFlatDiscount, ValuePct: 15}, false},
		{"flat_markup 10 pct", &PriceRule{Type: PriceRuleTypeFlatMarkup, ValuePct: 10}, false},
		{"fixed_surcharge 0", &PriceRule{Type: PriceRuleTypeFixedSurcharge, AmountMinor: 0}, true},
		{"fixed_surcharge 100", &PriceRule{Type: PriceRuleTypeFixedSurcharge, AmountMinor: 100}, false},
		{"unknown type acts identity", &PriceRule{Type: "unknown", ValuePct: 50}, true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.rule.IsZero(); got != tc.want {
				t.Fatalf("IsZero() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestPriceRuleApplyAmount(t *testing.T) {
	cases := []struct {
		name string
		rule *PriceRule
		base string
		want string
	}{
		{"nil rule identity", nil, "10000", "10000"},
		{"zero rule identity", &PriceRule{Type: PriceRuleTypeFlatDiscount}, "10000", "10000"},
		{"flat_discount 15%", &PriceRule{Type: PriceRuleTypeFlatDiscount, ValuePct: 15}, "10000", "8500"},
		{"flat_discount 15.5% rounds to bp", &PriceRule{Type: PriceRuleTypeFlatDiscount, ValuePct: 15.5}, "10000", "8450"},
		{"flat_discount 100% clamps to 0", &PriceRule{Type: PriceRuleTypeFlatDiscount, ValuePct: 100}, "4900", "0"},
		{"flat_markup 10%", &PriceRule{Type: PriceRuleTypeFlatMarkup, ValuePct: 10}, "10000", "11000"},
		{"flat_markup 500%", &PriceRule{Type: PriceRuleTypeFlatMarkup, ValuePct: 500}, "1000", "6000"},
		{"fixed_surcharge +50", &PriceRule{Type: PriceRuleTypeFixedSurcharge, AmountMinor: 50}, "10000", "10050"},
		{"fixed_surcharge -200 fixed discount", &PriceRule{Type: PriceRuleTypeFixedSurcharge, AmountMinor: -200}, "10000", "9800"},
		{"fixed_surcharge over-discount clamps to 0", &PriceRule{Type: PriceRuleTypeFixedSurcharge, AmountMinor: -500}, "100", "0"},
		{"large amount preserves precision", &PriceRule{Type: PriceRuleTypeFlatDiscount, ValuePct: 10}, "1000000000000000000000000", "900000000000000000000000"},
		{"invalid rule acts as identity", &PriceRule{Type: "unknown", ValuePct: 50}, "10000", "10000"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			base := iwallet.NewAmount(tc.base)
			got := tc.rule.ApplyAmount(base)
			if got.String() != tc.want {
				t.Fatalf("ApplyAmount(%s) = %s, want %s", tc.base, got.String(), tc.want)
			}
		})
	}
}

func TestParsePriceRule(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		wantErr  bool
		wantNil  bool
		wantType string
	}{
		{"empty string identity", "", false, true, ""},
		{"whitespace identity", "   ", false, true, ""},
		{"valid flat_discount", `{"type":"flat_discount","value_pct":15}`, false, false, PriceRuleTypeFlatDiscount},
		{"valid fixed_surcharge", `{"type":"fixed_surcharge","amount_minor":100}`, false, false, PriceRuleTypeFixedSurcharge},
		{"zero-valued treated as nil", `{"type":"flat_discount","value_pct":0}`, false, true, ""},
		{"malformed JSON", `{type:flat_discount}`, true, true, ""},
		{"unknown type rejected", `{"type":"mystery","value_pct":1}`, true, true, ""},
		{"negative pct rejected", `{"type":"flat_markup","value_pct":-5}`, true, true, ""},
		{"> 100 discount rejected", `{"type":"flat_discount","value_pct":150}`, true, true, ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParsePriceRule(tc.input)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ParsePriceRule() err = %v, wantErr %v", err, tc.wantErr)
			}
			if tc.wantNil {
				if got != nil {
					t.Fatalf("ParsePriceRule() = %+v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("ParsePriceRule() returned nil, want rule")
			}
			if got.Type != tc.wantType {
				t.Fatalf("ParsePriceRule() type = %q, want %q", got.Type, tc.wantType)
			}
		})
	}
}

func TestPriceRuleRoundtrip(t *testing.T) {
	rule := &PriceRule{Type: PriceRuleTypeFlatDiscount, ValuePct: 15.5}
	encoded, err := rule.Marshal()
	if err != nil {
		t.Fatalf("Marshal() err = %v", err)
	}
	if encoded == "" {
		t.Fatalf("Marshal() returned empty for non-zero rule")
	}
	back, err := ParsePriceRule(encoded)
	if err != nil {
		t.Fatalf("ParsePriceRule(%s) err = %v", encoded, err)
	}
	if back == nil || back.Type != rule.Type || back.ValuePct != rule.ValuePct {
		t.Fatalf("roundtrip mismatch: got %+v, want %+v", back, rule)
	}
}

func TestPriceRuleMarshalZero(t *testing.T) {
	var nilRule *PriceRule
	s, err := nilRule.Marshal()
	if err != nil || s != "" {
		t.Fatalf("nil Marshal() = %q, %v; want \"\", nil", s, err)
	}
	zero := &PriceRule{Type: PriceRuleTypeFlatDiscount, ValuePct: 0}
	s, err = zero.Marshal()
	if err != nil || s != "" {
		t.Fatalf("zero Marshal() = %q, %v; want \"\", nil", s, err)
	}
}
