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

func TestResolvePolicyEmptyDefaultsToCommunity(t *testing.T) {
	policy, err := ResolvePolicy("")
	require.NoError(t, err)
	require.Equal(t, CommunityName, policy.Name())
	require.False(t, policy.AllowsCapability(CapabilityFiatPayments))
}

func TestFullPolicyPreservesRecognizedMethods(t *testing.T) {
	policy, err := ResolvePolicy(FullName)
	require.NoError(t, err)

	methods := []PaymentMethod{{ID: "SOL", Kind: "crypto", Flow: "external-wallet"}}
	assert.Equal(t, methods, policy.FilterPaymentMethods(methods))
}

func TestPolicyBehaviorDoesNotDependOnEditionName(t *testing.T) {
	manifest, err := CommunityManifest()
	require.NoError(t, err)
	manifest.Edition = "self-hosted"

	policy, err := NewPolicy(manifest)
	require.NoError(t, err)
	require.True(t, policy.AllowsPaymentMethod(PaymentMethod{
		ID: "BTC", Kind: "crypto", Flow: "address-transfer",
	}))
	require.False(t, policy.AllowsPaymentMethod(PaymentMethod{
		ID: "ETH", Kind: "crypto", Flow: "external-wallet",
	}))
	require.False(t, policy.AllowsPaymentMethod(PaymentMethod{
		ID: "stripe", Kind: "fiat", Flow: "provider-session",
	}))
	require.False(t, policy.AllowsCapability(CapabilityFiatPayments))
	require.False(t, policy.AllowsCapability(CapabilityPlatformIntegration))
	require.True(t, policy.AllowsCapability("ai"))
	require.True(t, policy.AllowsCapability("analytics"))
	require.True(t, policy.AllowsCapability("fulfillment"))
	require.True(t, policy.AllowsCapability("webhooks"))
}

func TestFullPolicyUsesConcretePaymentCapabilities(t *testing.T) {
	policy, err := ResolvePolicy(FullName)
	require.NoError(t, err)
	require.True(t, policy.AllowsPaymentMethod(PaymentMethod{
		ID: "SOL", Kind: "crypto", Flow: "external-wallet",
	}))
	require.True(t, policy.AllowsPaymentMethod(PaymentMethod{
		ID: "stripe", Kind: "fiat", Flow: "provider-session",
	}))
	require.False(t, policy.AllowsPaymentMethod(PaymentMethod{
		ID: "ZEC", Kind: "crypto", Flow: "address-transfer", AddressMode: "transparent",
	}))
	require.True(t, policy.AllowsCapability(CapabilityFiatPayments))
	require.True(t, policy.AllowsCapability(CapabilityPlatformIntegration))
}

func TestPolicyAllowsManifestCapabilityIndependentOfEditionName(t *testing.T) {
	manifest, err := CommunityManifest()
	require.NoError(t, err)
	manifest.Edition = "self-hosted"
	manifest.Capabilities = []string{CapabilityPlatformIntegration}

	policy, err := NewPolicy(manifest)
	require.NoError(t, err)
	require.True(t, policy.AllowsCapability(CapabilityPlatformIntegration))
	require.False(t, policy.AllowsCapability(CapabilityFiatPayments))
}
