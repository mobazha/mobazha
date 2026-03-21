package models

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

func newCurrency(code string) *Currency {
	if code == "" {
		code = "BTC"
	}
	return &Currency{
		Name:         fmt.Sprintf("%scoin", code),
		Code:         CurrencyCode(code),
		Divisibility: 8,
		CurrencyType: CurrencyTypeCrypto,
	}
}

func TestCurrencyDefinitionsAreEqual(t *testing.T) {
	var (
		validDef                 = newCurrency("BTC")
		matchingDef              = newCurrency("BTC")
		differentCodeDef         = newCurrency("ETH")
		differentTypeDef         = newCurrency("BTC")
		differentDivisibilityDef = newCurrency("BTC")
		differentNameDef         = newCurrency("BTC")
		examples                 = []struct {
			value    *Currency
			other    *Currency
			expected bool
		}{
			{ // currency and divisibility matching should be equal
				value:    validDef,
				other:    matchingDef,
				expected: true,
			},
			{ // different names should be true
				value:    validDef,
				other:    differentNameDef,
				expected: true,
			},
			{ // nils should be false
				value:    nil,
				other:    nil,
				expected: false,
			},
			{ // different code should be false
				value:    validDef,
				other:    differentCodeDef,
				expected: false,
			},
			{ // different divisibility should be false
				value:    validDef,
				other:    differentDivisibilityDef,
				expected: false,
			},
			{ // different type should be false
				value:    validDef,
				other:    differentTypeDef,
				expected: false,
			},
		}
	)
	differentDivisibilityDef.Divisibility = 10
	differentNameDef.Name = "Something else"
	differentTypeDef.CurrencyType = "invalid"

	for _, c := range examples {
		if c.value.Equal(c.other) != c.expected {
			if c.expected {
				t.Errorf("expected values to be equal but was not")
			} else {
				t.Errorf("expected values to NOT be equal but was")
			}
			t.Logf("\tvalue name: %s code: %s divisibility: %d type: %s", c.value.Name, c.value.Code, c.value.Divisibility, c.value.CurrencyType)
			t.Logf("\tother name: %s code: %s divisibility: %d type: %s", c.other.Name, c.other.Code, c.other.Divisibility, c.other.CurrencyType)
		}
	}
}

func TestCurrencyDictionaryLookup(t *testing.T) {
	var (
		expected = newCurrency("ABC")
		dict     = CurrencyDictionary{
			expected.Code.String(): expected,
		}

		examples = []struct {
			lookup      string
			expected    *Currency
			expectedErr error
		}{
			{ // upcase lookup
				lookup:      "ABC",
				expected:    expected,
				expectedErr: nil,
			},
			{ // lowercase lookup
				lookup:      "abc",
				expected:    expected,
				expectedErr: nil,
			},
			{ // TABC no longer auto-maps to ABC
				lookup:      "TABC",
				expected:    nil,
				expectedErr: ErrCurrencyDefinitionUndefined,
			},
			{ // undefined key
				lookup:      "FAIL",
				expected:    nil,
				expectedErr: ErrCurrencyDefinitionUndefined,
			},
		}
	)

	for _, e := range examples {
		var def, err = dict.Lookup(e.lookup)
		if err != nil {
			if e.expectedErr != nil {
				if err != e.expectedErr {
					t.Errorf("expected err to be (%s), but was (%s)", e.expectedErr.Error(), err.Error())
					t.Logf("\tlookup: %s", e.lookup)
				}
				continue
			}
			t.Errorf("unexpected error: %s", err.Error())
			t.Logf("\tlookup: %s", e.lookup)
			continue
		}

		if !e.expected.Equal(def) {
			t.Errorf("expected (%s) but got (%s)", e.expected, def)
			t.Logf("\tlookup: %s", e.lookup)
		}
	}
}

func TestCurrencyDictionaryLookup_FiatCompoundCodes(t *testing.T) {
	usd := &Currency{Name: "US Dollar", Code: "USD", CurrencyType: CurrencyTypeFiat, Divisibility: 2}
	eur := &Currency{Name: "Euro", Code: "EUR", CurrencyType: CurrencyTypeFiat, Divisibility: 2}
	dict := CurrencyDictionary{
		"USD": usd,
		"EUR": eur,
	}

	tests := []struct {
		code        string
		wantCode    CurrencyCode
		wantDiv     uint
		wantErr     bool
	}{
		{"fiat:stripe:USD", "USD", 2, false},
		{"fiat:paypal:USD", "USD", 2, false},
		{"fiat:stripe:EUR", "EUR", 2, false},
		{"FIAT:STRIPE:USD", "USD", 2, false},
		{"Fiat:Stripe:usd", "USD", 2, false},
		{"fiat:USD", "USD", 2, false},   // 2-segment format
		{"fiat:EUR", "EUR", 2, false},   // 2-segment format
		{"FIAT:USD", "USD", 2, false},   // 2-segment uppercase
		{"fiat:stripe:INVALID", "", 0, true},
		{"fiat:INVALID", "", 0, true},   // 2-segment unknown currency
	}

	for _, tt := range tests {
		def, err := dict.Lookup(tt.code)
		if tt.wantErr {
			if err == nil {
				t.Errorf("Lookup(%q): expected error, got %v", tt.code, def)
			}
			continue
		}
		if err != nil {
			t.Errorf("Lookup(%q): unexpected error: %v", tt.code, err)
			continue
		}
		if def.Code != tt.wantCode {
			t.Errorf("Lookup(%q): code = %s, want %s", tt.code, def.Code, tt.wantCode)
		}
		if def.Divisibility != tt.wantDiv {
			t.Errorf("Lookup(%q): divisibility = %d, want %d", tt.code, def.Divisibility, tt.wantDiv)
		}
	}
}

func mustNewCurrencyValue(t *testing.T, amount, currencyCode string) *CurrencyValue {
	var (
		def = newCurrency(currencyCode)
		c   = NewCurrencyValue(amount, def)
	)
	return c
}

func TestCurrencyValueMarshalsToJSON(t *testing.T) {
	var (
		examples = []struct {
			value    string
			currency *Currency
		}{
			{ // valid currency value
				value:    "123456789012345678",
				currency: newCurrency("ABC"),
			},
			{ // valid currency value large enough to overflow primative ints
				value:    "123456789012345678901234567890",
				currency: newCurrency("BCD"),
			},
		}
	)

	for _, e := range examples {
		var (
			example = NewCurrencyValue(e.value, e.currency)
			actual  *CurrencyValue
		)

		j, err := json.Marshal(example)
		if err != nil {
			t.Errorf("marshaling %s: %s", example.String(), err)
			continue
		}

		if err := json.Unmarshal(j, &actual); err != nil {
			t.Errorf("unmarhsaling %s, %s", example.String(), err)
			continue
		}

		if !actual.Equal(example) {
			t.Errorf("expected %s and %s to be equal, but was not", example.String(), actual.String())
		}
	}
}

func TestCurrencyValuesAreEqual(t *testing.T) {
	var examples = []struct {
		value    *CurrencyValue
		other    *CurrencyValue
		expected bool
	}{
		{ // value and currency matching should be equal
			value: &CurrencyValue{
				Amount:   iwallet.NewAmount("1"),
				Currency: newCurrency("BTC"),
			},
			other: &CurrencyValue{
				Amount:   iwallet.NewAmount("1"),
				Currency: newCurrency("BTC"),
			},
			expected: true,
		},
		{ // nils should not be equal
			value:    nil,
			other:    nil,
			expected: false,
		},
		{ // nil should not match with a value
			value: nil,
			other: &CurrencyValue{
				Amount:   iwallet.NewAmount("1"),
				Currency: newCurrency("BTC"),
			},
			expected: false,
		},
		{ // value should not match with nil
			value: &CurrencyValue{
				Amount:   iwallet.NewAmount("1"),
				Currency: newCurrency("BTC"),
			},
			other:    nil,
			expected: false,
		},
		{ // value difference
			value: &CurrencyValue{
				Amount:   iwallet.NewAmount("1"),
				Currency: newCurrency("BTC"),
			},
			other: &CurrencyValue{
				Amount:   iwallet.NewAmount("2"),
				Currency: newCurrency("BTC"),
			},
			expected: false,
		},
		{ // currency code difference
			value: &CurrencyValue{
				Amount:   iwallet.NewAmount("1"),
				Currency: newCurrency("BTC"),
			},
			other: &CurrencyValue{
				Amount:   iwallet.NewAmount("1"),
				Currency: newCurrency("ETH"),
			},
			expected: false,
		},
		{ // currency code missing
			value: &CurrencyValue{
				Amount:   iwallet.NewAmount("1"),
				Currency: nil,
			},
			other: &CurrencyValue{
				Amount:   iwallet.NewAmount("1"),
				Currency: newCurrency("ETH"),
			},
			expected: false,
		},
	}

	for _, c := range examples {
		if c.value.Equal(c.other) != c.expected {
			if c.expected {
				t.Errorf("expected %s to equal %s but did not", c.value.String(), c.other.String())
			} else {
				t.Errorf("expected %s to not equal %s but did", c.value.String(), c.other.String())
			}
		}
	}
}

func TestCurrencyValuesConvertCorrectly(t *testing.T) {
	var (
		zeroRateErr = "rate must be greater than zero"
		invalidErr  = "cannot convert invalid value"

		examples = []struct {
			value        *CurrencyValue
			convertTo    *Currency
			exchangeRate float64
			expected     *CurrencyValue
			expectedErr  *string
		}{
			{ // errors when definition is nil
				value:        NewCurrencyValue("0", newCurrency("BTC")),
				convertTo:    nil,
				exchangeRate: 0.99999,
				expected:     nil,
				expectedErr:  &invalidErr,
			},
			{ // errors zero rate
				value:        NewCurrencyValue("123", newCurrency("BTC")),
				convertTo:    newCurrency("BCH"),
				exchangeRate: 0,
				expected:     nil,
				expectedErr:  &zeroRateErr,
			},
			{ // errors negative rate
				value:        NewCurrencyValue("123", newCurrency("BTC")),
				convertTo:    newCurrency("BCH"),
				exchangeRate: -0.1,
				expected:     nil,
				expectedErr:  &zeroRateErr,
			},
			{ // rounds down
				value:        NewCurrencyValue("1", newCurrency("BTC")),
				convertTo:    newCurrency("BCH"),
				exchangeRate: 0.9,
				expected:     NewCurrencyValue("0", newCurrency("BCH")),
				expectedErr:  nil,
			},
			{ // handles negative values
				value:        NewCurrencyValue("-100", newCurrency("BTC")),
				convertTo:    newCurrency("BCH"),
				exchangeRate: 0.123,
				expected:     NewCurrencyValue("-12", newCurrency("BCH")),
				expectedErr:  nil,
			},
			{ // handles zero
				value:        NewCurrencyValue("0", newCurrency("BTC")),
				convertTo:    newCurrency("BCH"),
				exchangeRate: 0.99999,
				expected:     NewCurrencyValue("0", newCurrency("BCH")),
				expectedErr:  nil,
			},
			{ // handles invalid value
				value: &CurrencyValue{
					Amount:   iwallet.NewAmount("1000"),
					Currency: nil,
				},
				convertTo:    newCurrency("BTC"),
				exchangeRate: 0.5,
				expected:     nil,
				expectedErr:  &invalidErr,
			},
			{ // handles conversions between different divisibility
				value: &CurrencyValue{
					Amount: iwallet.NewAmount("1000"),
					Currency: &Currency{
						Name:         "United States Dollar",
						Code:         "USD",
						Divisibility: 2,
						CurrencyType: CurrencyTypeFiat,
					},
				},
				convertTo: &Currency{
					Name:         "SimpleCoin",
					Code:         "SPC",
					Divisibility: 6,
					CurrencyType: CurrencyTypeFiat,
				},
				exchangeRate: 0.5,
				expected: &CurrencyValue{
					Amount: iwallet.NewAmount("5000000"),
					Currency: &Currency{
						Name:         "SimpleCoin",
						Code:         "SPC",
						Divisibility: 6,
						CurrencyType: CurrencyTypeFiat,
					},
				},
				expectedErr: nil,
			},
			{ // handles conversions between different divisibility w/ exchange rate > 1
				value: &CurrencyValue{
					Amount: iwallet.NewAmount("1000"),
					Currency: &Currency{
						Name:         "United States Dollar",
						Code:         "USD",
						Divisibility: 2,
						CurrencyType: CurrencyTypeFiat,
					},
				},
				convertTo: &Currency{
					Name:         "SimpleCoin",
					Code:         "SPC",
					Divisibility: 8,
					CurrencyType: CurrencyTypeFiat,
				},
				exchangeRate: 0.002455796,
				expected: &CurrencyValue{
					Amount: iwallet.NewAmount("2455796"),
					Currency: &Currency{
						Name:         "SimpleCoin",
						Code:         "SPC",
						Divisibility: 8,
						CurrencyType: CurrencyTypeFiat,
					},
				},
				expectedErr: nil,
			},
			{ // handles conversions between different
				// divisibility w inverse rate
				value: &CurrencyValue{
					Amount: iwallet.NewAmount("1000000"),
					Currency: &Currency{
						Name:         "SimpleCoin",
						Code:         "SPC",
						Divisibility: 6,
						CurrencyType: CurrencyTypeFiat,
					},
				},
				convertTo: &Currency{
					Name:         "United States Dollar",
					Code:         "USD",
					Divisibility: 2,
					CurrencyType: CurrencyTypeFiat,
				},
				exchangeRate: 2,
				expected: &CurrencyValue{
					Amount: iwallet.NewAmount("200"),
					Currency: &Currency{
						Name:         "United States Dollar",
						Code:         "USD",
						Divisibility: 2,
						CurrencyType: CurrencyTypeFiat,
					},
				},
				expectedErr: nil,
			},
		}
	)

	for _, e := range examples {
		actual, err := e.value.ConvertTo(e.convertTo, e.exchangeRate)
		if err != nil {
			if e.expectedErr != nil && !strings.Contains(err.Error(), *e.expectedErr) {
				t.Errorf("expected value (%s) to error with (%s) but returned: %s", e.value, *e.expectedErr, err.Error())
			}
			continue
		} else {
			if e.expectedErr != nil {
				t.Errorf("expected error (%s) but produced none", *e.expectedErr)
				t.Logf("\tfor value: (%s) convertTo: (%s) rate: (%f)", e.value, e.convertTo, e.exchangeRate)
			}
		}

		if !actual.Equal(e.expected) {
			t.Errorf("expected converted value to be %s, but was %s", e.expected, actual)
			t.Logf("\tfor value: (%s) convertTo: (%s) rate: (%f)", e.value, e.convertTo, e.exchangeRate)
			continue
		}
	}
}
