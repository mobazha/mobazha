package models

import (
	"testing"
)

func TestPayoutRatio_Validate(t *testing.T) {
	tests := []struct {
		name    string
		ratio   PayoutRatio
		wantErr bool
	}{
		{"50/50 split", PayoutRatio{Buyer: 50, Vendor: 50}, false},
		{"100/0 buyer gets all", PayoutRatio{Buyer: 100, Vendor: 0}, false},
		{"0/100 vendor gets all", PayoutRatio{Buyer: 0, Vendor: 100}, false},
		{"33.33/66.67 float precision", PayoutRatio{Buyer: 33.33, Vendor: 66.67}, false},
		{"49.99/50.01 near-even", PayoutRatio{Buyer: 49.99, Vendor: 50.01}, false},

		{"0/0 reject zero total", PayoutRatio{Buyer: 0, Vendor: 0}, true},
		{"200/-100 reject negative vendor", PayoutRatio{Buyer: 200, Vendor: -100}, true},
		{"-50/150 reject negative buyer", PayoutRatio{Buyer: -50, Vendor: 150}, true},
		{"50/60 reject over 100", PayoutRatio{Buyer: 50, Vendor: 60}, true},
		{"50/40 reject under 100", PayoutRatio{Buyer: 50, Vendor: 40}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ratio.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("PayoutRatio{%v, %v}.Validate() error = %v, wantErr %v",
					tt.ratio.Buyer, tt.ratio.Vendor, err, tt.wantErr)
			}
		})
	}
}
