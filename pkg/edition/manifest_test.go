package edition

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommunityManifestMatchesDistributionManifest(t *testing.T) {
	embedded, err := CommunityManifest()
	require.NoError(t, err)

	distributionPath := filepath.Join("..", "..", "config", "editions", "community.json")
	data, err := os.ReadFile(distributionPath)
	require.NoError(t, err)
	distribution, err := ParseManifest(data)
	require.NoError(t, err)

	assert.Equal(t, embedded, distribution)
}

func TestCommunityPolicyFiltersNonCommunityPaymentMethods(t *testing.T) {
	policy, err := ResolvePolicy(CommunityName)
	require.NoError(t, err)

	methods := []PaymentMethod{
		{ID: "BTC", Kind: "crypto", Flow: "address-transfer"},
		{ID: "BCH", Kind: "crypto", Flow: "address-transfer"},
		{ID: "LTC", Kind: "crypto", Flow: "address-transfer"},
		{ID: "ZEC", Kind: "crypto", Flow: "address-transfer", AddressMode: "transparent"},
		{ID: "ZEC", Kind: "crypto", Flow: "address-transfer", AddressMode: "shielded"},
		{ID: "ETH", Kind: "crypto", Flow: "external-wallet"},
		{ID: "EXTERNAL_PAYMENT", Kind: "crypto", Flow: "address-transfer"},
		{ID: "stripe", Kind: "fiat", Flow: "provider-session"},
	}

	assert.Equal(t, methods[:4], policy.FilterPaymentMethods(methods))
}

func TestResolvePolicyRejectsUnknownExplicitEdition(t *testing.T) {
	_, err := ResolvePolicy("commuinty")
	require.ErrorContains(t, err, "unknown Mobazha edition")
}

func TestFullPolicyPreservesRecognizedMethods(t *testing.T) {
	policy, err := ResolvePolicy(FullName)
	require.NoError(t, err)

	methods := []PaymentMethod{{ID: "SOL", Kind: "crypto", Flow: "external-wallet"}}
	assert.Equal(t, methods, policy.FilterPaymentMethods(methods))
}
