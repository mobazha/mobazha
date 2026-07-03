package edition

import (
	"fmt"
	"strings"
	"sync/atomic"
)

const (
	paymentKindCrypto      = "crypto"
	paymentFlowAddress     = "address-transfer"
	addressModeTransparent = "transparent"
	communityUTXORail      = "utxo_transparent"

	// CapabilityFiatPayments enables provider-backed fiat payment routes and
	// services in a distribution composition.
	CapabilityFiatPayments = "payment.fiat"
	// CapabilityPlatformIntegration enables optional connections and reverse
	// proxying from a standalone node to the Mobazha Hosting control plane.
	CapabilityPlatformIntegration = "platform.integration"
)

// PaymentMethod is the provider-neutral capability shape evaluated by an
// edition policy. It deliberately mirrors the public runtime projection
// without importing API or frontend packages.
type PaymentMethod struct {
	ID          string
	Kind        string
	Flow        string
	AddressMode string
}

// Policy narrows recognized capabilities to those enabled by a distribution.
// It is not a feature flag and must be applied before public projection.
type Policy interface {
	Name() string
	AllowsCapability(string) bool
	AllowsPaymentMethod(PaymentMethod) bool
	FilterPaymentMethods([]PaymentMethod) []PaymentMethod
}

type policy struct {
	name          string
	unrestricted  bool
	allowedChains map[string]struct{}
	allowedRails  map[string]struct{}
	capabilities  map[string]struct{}
	deniedChains  map[string]struct{}
	zec           ZcashManifest
}

type currentPolicyHolder struct {
	policy Policy
}

var currentPolicy atomic.Pointer[currentPolicyHolder]

func init() {
	currentPolicy.Store(&currentPolicyHolder{policy: defaultPolicy()})
}

// ResolvePolicy returns the policy for a configured edition. An empty name is
// the public composition default and therefore resolves to Community. Private
// compositions must opt in to full capabilities explicitly.
func ResolvePolicy(name string) (Policy, error) {
	switch normalized := strings.ToLower(strings.TrimSpace(name)); normalized {
	case "", CommunityName:
		manifest, err := CommunityManifest()
		if err != nil {
			return nil, err
		}
		return NewPolicy(manifest)
	case FullName, "commercial", "sovereign":
		return unrestrictedPolicy(normalized), nil
	default:
		return nil, fmt.Errorf("unknown Mobazha edition %q", name)
	}
}

// ConfigureCurrentPolicy sets the process-wide distribution policy used by
// payment ingress. A Mobazha process has one edition composition; tenant and
// seller settings may narrow it further but can never widen it.
func ConfigureCurrentPolicy(name string) error {
	resolved, err := ResolvePolicy(name)
	if err != nil {
		return err
	}
	currentPolicy.Store(&currentPolicyHolder{policy: resolved})
	return nil
}

// CurrentPolicy returns the process-wide policy. It always returns a non-nil
// policy and fails closed to Community until a private composition root
// explicitly selects its distribution.
func CurrentPolicy() Policy {
	holder := currentPolicy.Load()
	if holder == nil || holder.policy == nil {
		return defaultPolicy()
	}
	return holder.policy
}

// DefaultPolicy returns the fail-closed policy for the public Mobazha
// composition. Callers should prefer this semantic default over depending on
// a concrete distribution name.
func DefaultPolicy() Policy {
	return defaultPolicy()
}

func defaultPolicy() Policy {
	manifest, err := CommunityManifest()
	if err != nil {
		return &policy{name: CommunityName}
	}
	resolved, err := NewPolicy(manifest)
	if err != nil {
		return &policy{name: CommunityName}
	}
	return resolved
}

// NewPolicy constructs a restrictive positive-allowlist policy from a
// validated manifest.
func NewPolicy(manifest Manifest) (Policy, error) {
	if err := manifest.Validate(); err != nil {
		return nil, err
	}
	p := &policy{
		name:          manifest.Edition,
		allowedChains: make(map[string]struct{}, len(manifest.Payment.Chains)),
		allowedRails:  make(map[string]struct{}, len(manifest.Payment.Rails)),
		capabilities:  make(map[string]struct{}, len(manifest.Capabilities)),
		zec:           manifest.Zcash,
	}
	for _, chain := range manifest.Payment.Chains {
		p.allowedChains[chain] = struct{}{}
	}
	for _, rail := range manifest.Payment.Rails {
		p.allowedRails[rail] = struct{}{}
	}
	for _, capability := range manifest.Capabilities {
		p.capabilities[strings.ToLower(strings.TrimSpace(capability))] = struct{}{}
	}
	return p, nil
}

func unrestrictedPolicy(name string) Policy {
	// Zcash was not part of the pre-distribution commercial composition. Keep
	// that concrete capability disabled until a composition explicitly supplies
	// a policy which enables it. This is a capability decision, not a profile
	// name check in business code.
	return &policy{
		name:         name,
		unrestricted: true,
		deniedChains: map[string]struct{}{"ZEC": {}},
	}
}

func (p *policy) Name() string {
	return p.name
}

// AllowsCapability evaluates a provider-neutral distribution capability.
// Missing policy or missing declarations fail closed; unrestricted legacy and
// commercial compositions preserve their existing behavior.
func (p *policy) AllowsCapability(capability string) bool {
	if p == nil {
		return false
	}
	if p.unrestricted {
		return true
	}
	_, ok := p.capabilities[strings.ToLower(strings.TrimSpace(capability))]
	return ok
}

func (p *policy) AllowsPaymentMethod(method PaymentMethod) bool {
	if p == nil {
		return false
	}
	kind := strings.ToLower(strings.TrimSpace(method.Kind))
	id := strings.ToUpper(strings.TrimSpace(method.ID))
	if kind == paymentKindCrypto {
		if _, denied := p.deniedChains[id]; denied {
			return false
		}
	}
	if p.unrestricted {
		return true
	}
	if kind != paymentKindCrypto {
		return false
	}
	if _, allowed := p.allowedChains[id]; !allowed {
		return false
	}
	if _, allowed := p.allowedRails[communityUTXORail]; !allowed {
		return false
	}
	if strings.TrimSpace(method.Flow) != paymentFlowAddress {
		return false
	}
	if id == "ZEC" && p.zec.TransparentOnly {
		return strings.TrimSpace(method.AddressMode) == addressModeTransparent
	}
	return strings.TrimSpace(method.AddressMode) == ""
}

func (p *policy) FilterPaymentMethods(methods []PaymentMethod) []PaymentMethod {
	filtered := make([]PaymentMethod, 0, len(methods))
	for _, method := range methods {
		if p.AllowsPaymentMethod(method) {
			filtered = append(filtered, method)
		}
	}
	return filtered
}
