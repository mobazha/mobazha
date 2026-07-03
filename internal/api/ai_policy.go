package api

import (
	"github.com/mobazha/mobazha/pkg/distribution"
	"github.com/mobazha/mobazha/pkg/edition"
)

func resolveAIHTTPPolicy(explicit distribution.AIHTTPPolicy, policy edition.Policy) distribution.AIHTTPPolicy {
	if policy == nil {
		policy = edition.DefaultPolicy()
	}
	enabled := policy.AllowsCapability(distribution.CapabilityAI)
	if explicit != nil {
		return distribution.NewAIHTTPPolicy(
			enabled && explicit.AIHTTPEnabled(),
			enabled && explicit.AllowsRemoteAIEndpoints(),
			policy.AllowsCapability(distribution.CapabilityAIPlatform) && explicit.AllowsPlatformAIFallback(),
			enabled && explicit.AllowsAgentWorkspace(),
		)
	}
	return distribution.NewAIHTTPPolicy(
		enabled,
		enabled,
		policy.AllowsCapability(distribution.CapabilityAIPlatform),
		enabled,
	)
}

func (g *Gateway) activeAIHTTPPolicy() distribution.AIHTTPPolicy {
	if g != nil && g.aiHTTPPolicy != nil {
		return g.aiHTTPPolicy
	}
	if g != nil {
		return resolveAIHTTPPolicy(nil, g.editionPolicy)
	}
	return resolveAIHTTPPolicy(nil, nil)
}
