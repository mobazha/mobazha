package orders

import (
	"testing"

	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAllocateAffiliateDiscount_ConservesDiscountedMerchandise(t *testing.T) {
	tests := []struct {
		name     string
		lines    []string
		discount string
		want     []string
		wantErr  bool
	}{
		{name: "pro rata with deterministic remainder", lines: []string{"100", "200", "700"}, discount: "-101", want: []string{"90", "180", "629"}},
		{name: "free line remains zero", lines: []string{"0", "100"}, discount: "-10", want: []string{"0", "90"}},
		{name: "discount exceeds merchandise", lines: []string{"100"}, discount: "-101", wantErr: true},
		{name: "discount cannot target zero subtotal", lines: []string{"0"}, discount: "-1", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := make([]iwallet.Amount, 0, len(tt.lines))
			for _, line := range tt.lines {
				lines = append(lines, iwallet.NewAmount(line))
			}
			got, err := allocateAffiliateDiscount(lines, iwallet.NewAmount(tt.discount))
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			actual := make([]string, 0, len(got))
			for _, amount := range got {
				actual = append(actual, amount.String())
			}
			assert.Equal(t, tt.want, actual)
		})
	}
}
