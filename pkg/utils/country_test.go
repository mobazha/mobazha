package utils

import "testing"

import "github.com/stretchr/testify/require"

func TestShippingRegionMatchesCountry_AuthoritativeGroups(t *testing.T) {
	tests := []struct {
		name    string
		region  string
		country string
		want    bool
	}{
		{name: "exact", region: "US", country: "US", want: true},
		{name: "worldwide", region: "ALL", country: "JP", want: true},
		{name: "continent", region: "NORTH_AMERICA", country: "US", want: true},
		{name: "wrong continent", region: "EUROPE", country: "US", want: false},
		{name: "invalid country", region: "ALL", country: "United States", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, ShippingRegionMatchesCountry(tt.region, tt.country))
		})
	}
}
