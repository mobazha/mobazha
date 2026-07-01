package api

import (
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/distribution"
	"github.com/mobazha/mobazha3.0/pkg/edition"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveAIHTTPPolicy_CommunityAllowsBYOKWithoutPlatformFallback(t *testing.T) {
	policy := resolveAIHTTPPolicy(nil, edition.DefaultPolicy())

	assert.True(t, policy.AIHTTPEnabled())
	assert.True(t, policy.AllowsRemoteAIEndpoints())
	assert.False(t, policy.AllowsPlatformAIFallback())
	assert.True(t, policy.AllowsAgentWorkspace())
}

func TestResolveAIHTTPPolicy_CommercialAllowsPlatformFallback(t *testing.T) {
	commercial, err := edition.ResolvePolicy("commercial")
	require.NoError(t, err)

	policy := resolveAIHTTPPolicy(nil, commercial)

	assert.True(t, policy.AIHTTPEnabled())
	assert.True(t, policy.AllowsRemoteAIEndpoints())
	assert.True(t, policy.AllowsPlatformAIFallback())
}

func TestResolveAIHTTPPolicy_ExplicitPolicyOverridesEdition(t *testing.T) {
	explicit := distribution.NewAIHTTPPolicy(true, false, false, false)
	commercial, err := edition.ResolvePolicy("commercial")
	require.NoError(t, err)

	policy := resolveAIHTTPPolicy(explicit, commercial)

	assert.False(t, policy.AllowsRemoteAIEndpoints())
	assert.False(t, policy.AllowsPlatformAIFallback())
	assert.False(t, policy.AllowsAgentWorkspace())
}

func TestResolveAIHTTPPolicy_ExplicitPolicyCannotWidenEdition(t *testing.T) {
	restrictive, err := edition.NewPolicy(edition.Manifest{
		SchemaVersion:           edition.ManifestSchemaVersion,
		Edition:                 "restricted-test",
		License:                 "MPL-2.0",
		PaymentPluginSDKLicense: "Apache-2.0",
		Payment: edition.PaymentManifest{
			Chains: []string{"BTC"},
			Rails:  []string{"utxo_transparent"},
		},
		DeploymentTargets: []string{"standalone"},
	})
	require.NoError(t, err)
	explicit := distribution.NewAIHTTPPolicy(true, true, true, true)

	policy := resolveAIHTTPPolicy(explicit, restrictive)

	assert.False(t, policy.AIHTTPEnabled())
	assert.False(t, policy.AllowsRemoteAIEndpoints())
	assert.False(t, policy.AllowsPlatformAIFallback())
	assert.False(t, policy.AllowsAgentWorkspace())
}
