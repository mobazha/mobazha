package utxo

import "testing"

func TestEstimateP2WSHMultisigSpendVSize(t *testing.T) {
	tests := []struct {
		name      string
		threshold int
		nOuts     int
		expected  int
	}{
		{name: "1-of-2 one input one output", threshold: 1, nOuts: 1, expected: 123},
		{name: "2-of-3 one input two outputs", threshold: 2, nOuts: 2, expected: 184},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EstimateP2WSHMultisigSpendVSize(1, tt.threshold, tt.nOuts)
			if got != tt.expected {
				t.Fatalf("EstimateP2WSHMultisigSpendVSize() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestEstimateP2WSHMultisigSpendVSize_MultipleInputs(t *testing.T) {
	got := EstimateP2WSHMultisigSpendVSize(2, 2, 2)
	if got != 289 {
		t.Fatalf("EstimateP2WSHMultisigSpendVSize() = %d, want %d", got, 289)
	}
}

func TestEstimateP2SHMultisigSpendSize(t *testing.T) {
	tests := []struct {
		name     string
		estimate func() int
		expected int
	}{
		{
			name:     "BCH Schnorr 1-of-2",
			estimate: func() int { return EstimateP2SHSchnorrMultisigSpendSize(1, 1, 1, 0) },
			expected: 224,
		},
		{
			name:     "BCH Schnorr 2-of-3",
			estimate: func() int { return EstimateP2SHSchnorrMultisigSpendSize(1, 2, 2, 0) },
			expected: 358,
		},
		{
			name:     "ZEC ECDSA 1-of-2",
			estimate: func() int { return EstimateP2SHECDSAMultisigSpendSize(1, 1, 1, 15) },
			expected: 247,
		},
		{
			name:     "ZEC ECDSA 2-of-3",
			estimate: func() int { return EstimateP2SHECDSAMultisigSpendSize(1, 2, 2, 15) },
			expected: 391,
		},
		{
			name:     "BCH Schnorr 2-of-3 multiple inputs",
			estimate: func() int { return EstimateP2SHSchnorrMultisigSpendSize(2, 2, 2, 0) },
			expected: 638,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.estimate()
			if got != tt.expected {
				t.Fatalf("estimate = %d, want %d", got, tt.expected)
			}
		})
	}
}
