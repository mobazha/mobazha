package distribution

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewAIHTTPPolicy_DisabledFailsClosed(t *testing.T) {
	policy := NewAIHTTPPolicy(false, true, true, true)

	assert.False(t, policy.AIHTTPEnabled())
	assert.False(t, policy.AllowsRemoteAIEndpoints())
	assert.False(t, policy.AllowsPlatformAIFallback())
	assert.False(t, policy.AllowsAgentWorkspace())
}

func TestNewAIHTTPPolicy_PreservesEnabledCapabilities(t *testing.T) {
	policy := NewAIHTTPPolicy(true, true, false, true)

	assert.True(t, policy.AIHTTPEnabled())
	assert.True(t, policy.AllowsRemoteAIEndpoints())
	assert.False(t, policy.AllowsPlatformAIFallback())
	assert.True(t, policy.AllowsAgentWorkspace())
}
