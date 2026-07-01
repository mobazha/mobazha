package distribution

const (
	// CapabilityAI enables the public AI settings and generation surface.
	CapabilityAI = "ai"
	// CapabilityAIPlatform enables distribution-provided AI fallback routes.
	CapabilityAIPlatform = "ai.platform"
)

// AIHTTPPolicy defines which AI HTTP capabilities a distribution composes.
// It is evaluated before routes are registered and before provider
// configuration is accepted. Feature flags may narrow these capabilities at
// runtime but cannot widen them.
type AIHTTPPolicy interface {
	AIHTTPEnabled() bool
	AllowsRemoteAIEndpoints() bool
	AllowsPlatformAIFallback() bool
	AllowsAgentWorkspace() bool
}

// StaticAIHTTPPolicy is an immutable AI HTTP capability policy suitable for
// composition roots and tests.
type StaticAIHTTPPolicy struct {
	enabled               bool
	remoteEndpoints       bool
	platformFallback      bool
	agentWorkspaceEnabled bool
}

// NewAIHTTPPolicy creates an immutable AI HTTP capability policy.
func NewAIHTTPPolicy(enabled, remoteEndpoints, platformFallback, agentWorkspace bool) StaticAIHTTPPolicy {
	return StaticAIHTTPPolicy{
		enabled:               enabled,
		remoteEndpoints:       enabled && remoteEndpoints,
		platformFallback:      enabled && platformFallback,
		agentWorkspaceEnabled: enabled && agentWorkspace,
	}
}

// AIHTTPEnabled reports whether the base AI HTTP surface is composed.
func (p StaticAIHTTPPolicy) AIHTTPEnabled() bool { return p.enabled }

// AllowsRemoteAIEndpoints reports whether operator-provided remote AI endpoints are accepted.
func (p StaticAIHTTPPolicy) AllowsRemoteAIEndpoints() bool { return p.remoteEndpoints }

// AllowsPlatformAIFallback reports whether distribution-provided AI routes may be selected.
func (p StaticAIHTTPPolicy) AllowsPlatformAIFallback() bool {
	return p.platformFallback
}

// AllowsAgentWorkspace reports whether the seller Agent workspace routes are composed.
func (p StaticAIHTTPPolicy) AllowsAgentWorkspace() bool { return p.agentWorkspaceEnabled }

var _ AIHTTPPolicy = StaticAIHTTPPolicy{}
