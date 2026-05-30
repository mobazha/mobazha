package electrum

import "testing"

func TestFeeBTCPerKBToSatPerVB(t *testing.T) {
	tests := []struct {
		name string
		fee  float64
		want uint64
	}{
		{name: "relay fee one sat per byte", fee: 0.00001, want: 1},
		{name: "rounds up fractional relay fee", fee: 0.00002232, want: 3},
		{name: "minimum one sat per byte", fee: 0, want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := feeBTCPerKBToSatPerVB(tt.fee); got != tt.want {
				t.Fatalf("feeBTCPerKBToSatPerVB(%f) = %d, want %d", tt.fee, got, tt.want)
			}
		})
	}
}
