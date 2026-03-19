package tron

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHexToBase58_ValidAddress(t *testing.T) {
	// Known TRON address pair (genesis account)
	hexAddr := "410000000000000000000000000000000000000000"
	b58, err := HexToBase58(hexAddr)
	require.NoError(t, err)
	assert.NotEmpty(t, b58)
	assert.True(t, b58[0] == 'T' || b58[0] >= '1')

	// Round-trip
	gotHex, err := Base58ToHex(b58)
	require.NoError(t, err)
	assert.Equal(t, hexAddr, gotHex)
}

func TestHexToBase58_TronGridExample(t *testing.T) {
	// TRX foundation address
	hexAddr := "41a614f803b6fd780986a42c78ec9c7f77e6ded13c"
	b58, err := HexToBase58(hexAddr)
	require.NoError(t, err)
	assert.True(t, len(b58) > 0)

	gotHex, err := Base58ToHex(b58)
	require.NoError(t, err)
	assert.Equal(t, hexAddr, gotHex)
}

func TestHexToBase58_InvalidInputs(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"too short", "4100"},
		{"wrong prefix", "4200000000000000000000000000000000000000000"},
		{"not hex", "41zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"},
		{"empty", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := HexToBase58(tt.input)
			assert.Error(t, err)
		})
	}
}

func TestBase58ToHex_InvalidInputs(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"too short", "T1234"},
		{"invalid base58 chars", "T0OIl"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Base58ToHex(tt.input)
			assert.Error(t, err)
		})
	}
}

func TestValidateAddress(t *testing.T) {
	hexAddr := "41a614f803b6fd780986a42c78ec9c7f77e6ded13c"
	b58, err := HexToBase58(hexAddr)
	require.NoError(t, err)

	assert.True(t, ValidateAddress(b58))
	assert.False(t, ValidateAddress("NotAnAddress"))
	assert.False(t, ValidateAddress(""))
}

func TestRoundTrip_MultipleAddresses(t *testing.T) {
	hexAddrs := []string{
		"410000000000000000000000000000000000000000",
		"41a614f803b6fd780986a42c78ec9c7f77e6ded13c",
		"41ffffffffffffffffffffffffffffffffffffffff",
	}
	for _, h := range hexAddrs {
		b58, err := HexToBase58(h)
		require.NoError(t, err, "HexToBase58 failed for %s", h)

		gotHex, err := Base58ToHex(b58)
		require.NoError(t, err, "Base58ToHex failed for %s", b58)
		assert.Equal(t, h, gotHex)
	}
}
