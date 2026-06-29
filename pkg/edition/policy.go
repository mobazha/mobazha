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
	AllowsPaymentMethod(PaymentMethod) bool
	FilterPaymentMethods([]PaymentMethod) []PaymentMethod
}

type policy struct {
	name          string
	unrestricted  bool
	allowedChains map[string]struct{}
	allowedRails  map[string]struct{}
	zec           ZcashManifest
}

type currentPolicyHolder struct {
	policy Policy
}

var currentPolicy atomic.Pointer[currentPolicyHolder]

func init() {
	currentPolicy.Store(&currentPolicyHolder{policy: unrestrictedPolicy(FullName)})
}

// ResolvePolicy returns the policy for a configured edition. Empty, full,
// commercial, and private_distribution names preserve the private composition's existing
// unrestricted behavior. Explicit unknown names fail startup.
func ResolvePolicy(name string) (Policy, error) {
	switch normalized := strings.ToLower(strings.TrimSpace(name)); normalized {
	case "", FullName, "commercial", "private_distribution":
		if normalized == "" {
			normalized = FullName
		}
		return unrestrictedPolicy(normalized), nil
	case CommunityName:
		manifest, err := CommunityManifest()
		if err != nil {
			return nil, err
		}
		return NewPolicy(manifest)
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
// policy and defaults to the backwards-compatible full composition until the
// application composition root configures an edition.
func CurrentPolicy() Policy {
	holder := currentPolicy.Load()
	if holder == nil || holder.policy == nil {
		return unrestrictedPolicy(FullName)
	}
	return holder.policy
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
		zec:           manifest.Zcash,
	}
	for _, chain := range manifest.Payment.Chains {
		p.allowedChains[chain] = struct{}{}
	}
	for _, rail := range manifest.Payment.Rails {
		p.allowedRails[rail] = struct{}{}
	}
	return p, nil
}

func unrestrictedPolicy(name string) Policy {
	return &policy{name: name, unrestricted: true}
}

func (p *policy) Name() string {
	return p.name
}

func (p *policy) AllowsPaymentMethod(method PaymentMethod) bool {
	if p == nil || p.unrestricted {
		return true
	}
	if strings.ToLower(strings.TrimSpace(method.Kind)) != paymentKindCrypto {
		return false
	}
	id := strings.ToUpper(strings.TrimSpace(method.ID))
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
