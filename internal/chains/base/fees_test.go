package base

import (
	"testing"

	"github.com/jarcoal/httpmock"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

func TestHardCodedFeeProvider_GetFee(t *testing.T) {
	tests := []struct {
		feeLevel iwallet.FeeLevel
		expected iwallet.Amount
	}{
		{
			feeLevel: iwallet.FlPriority,
			expected: iwallet.NewAmount(50),
		},
		{
			feeLevel: iwallet.FlNormal,
			expected: iwallet.NewAmount(40),
		},
		{
			feeLevel: iwallet.FlEconomic,
			expected: iwallet.NewAmount(30),
		},
		{
			feeLevel: iwallet.FLSuperEconomic,
			expected: iwallet.NewAmount(20),
		},
		{
			feeLevel: iwallet.FeeLevel(100),
			expected: iwallet.NewAmount(100),
		},
	}

	fp := NewHardCodedFeeProvider(iwallet.NewAmount(50), iwallet.NewAmount(40), iwallet.NewAmount(30), iwallet.NewAmount(20))

	for i, test := range tests {
		amt, err := fp.GetFee(test.feeLevel)
		if err != nil {
			t.Fatal(err)
		}
		if amt.Cmp(test.expected) != 0 {
			t.Errorf("Test %d: expected %s, got %s", i, test.expected, amt)
		}
	}
}

func TestAPIFeeProvider_GetFee(t *testing.T) {
	tests := []struct {
		feeLevel iwallet.FeeLevel
		expected iwallet.Amount
	}{
		{
			feeLevel: iwallet.FlPriority,
			expected: iwallet.NewAmount(153),
		},
		{
			feeLevel: iwallet.FlNormal,
			expected: iwallet.NewAmount(102),
		},
		{
			feeLevel: iwallet.FlEconomic,
			expected: iwallet.NewAmount(61),
		},
		{
			feeLevel: iwallet.FLSuperEconomic,
			expected: iwallet.NewAmount(30),
		},
		{
			feeLevel: iwallet.FeeLevel(100),
			expected: iwallet.NewAmount(100),
		},
	}

	url := "https://ticker.openbazaar.org/api"
	httpmock.RegisterResponder("GET", url,
		httpmock.NewStringResponder(200, `{"priority":153,"normal":102,"economic":61,"superEconomic":30}`))

	httpmock.Activate()
	defer httpmock.Deactivate()

	fp := NewAPIFeeProvider(url, iwallet.NewAmount(200))

	for i, test := range tests {
		amt, err := fp.GetFee(test.feeLevel)
		if err != nil {
			t.Fatal(err)
		}
		if amt.Cmp(test.expected) != 0 {
			t.Errorf("Test %d: expected %s, got %s", i, test.expected, amt)
		}
	}
}

func TestExchangeRateFeeProvider_GetFee(t *testing.T) {
	tests := []struct {
		feeLevel iwallet.FeeLevel
		expected iwallet.Amount
	}{
		{
			feeLevel: iwallet.FlPriority,
			expected: iwallet.NewAmount(90),
		},
		{
			feeLevel: iwallet.FlNormal,
			expected: iwallet.NewAmount(54),
		},
		{
			feeLevel: iwallet.FlEconomic,
			expected: iwallet.NewAmount(18),
		},
		{
			feeLevel: iwallet.FLSuperEconomic,
			expected: iwallet.NewAmount(3),
		},
		{
			feeLevel: iwallet.FeeLevel(100),
			expected: iwallet.NewAmount(100),
		},
	}

	url := "https://mobazha.info/api/ticker"
	erp := NewDefaultExchangeRateProvider(url)

	fp := NewExchangeRateFeeProvider(iwallet.CtZCash, 8, erp, 1500, iwallet.NewAmount(200), 5, 3, 1.5, 1)

	for i, test := range tests {
		amt, err := fp.GetFee(test.feeLevel)
		if err != nil {
			t.Fatal(err)
		}
		if amt.Cmp(test.expected) != 0 {
			t.Errorf("Test %d: expected %s, got %s", i, test.expected, amt)
		}
	}
}
