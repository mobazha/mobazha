// Package paymentplugin defines the public, versioned metadata boundary for
// independently distributed payment extensions.
package paymentplugin

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	// ManifestSchemaVersion is the current plugin distribution schema.
	ManifestSchemaVersion = 1
	// APIVersionV1 is the first supported out-of-process protocol version.
	APIVersionV1 = "payment.mobazha.io/v1"
)

// Capability is a negotiated payment plugin operation.
type Capability string

const (
	CapabilityChainMetadata    Capability = "chain.metadata"
	CapabilityAddressValidate  Capability = "address.validate"
	CapabilityAddressDerive    Capability = "address.derive"
	CapabilityPaymentSetup     Capability = "payment.setup"
	CapabilityPaymentObserve   Capability = "payment.observe"
	CapabilityPaymentVerify    Capability = "payment.verify"
	CapabilityTransactionBuild Capability = "transaction.build"
	CapabilityTransactionSign  Capability = "transaction.signing-payloads"
	CapabilityTransactionSend  Capability = "transaction.broadcast"
	CapabilityFeeEstimate      Capability = "fee.estimate"
	CapabilitySettlement       Capability = "settlement.execute"
)

var (
	pluginIDPattern   = regexp.MustCompile(`^[a-z0-9]+(?:[.-][a-z0-9]+)+$`)
	versionPattern    = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:[-+][0-9A-Za-z.-]+)?$`)
	chainIDPattern    = regexp.MustCompile(`^[A-Z][A-Z0-9_-]{1,31}$`)
	assetPattern      = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9:._/-]{0,127}$`)
	spdxLikePattern   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.+-]{0,63}$`)
	knownCapabilities = map[Capability]struct{}{
		CapabilityChainMetadata:    {},
		CapabilityAddressValidate:  {},
		CapabilityAddressDerive:    {},
		CapabilityPaymentSetup:     {},
		CapabilityPaymentObserve:   {},
		CapabilityPaymentVerify:    {},
		CapabilityTransactionBuild: {},
		CapabilityTransactionSign:  {},
		CapabilityTransactionSend:  {},
		CapabilityFeeEstimate:      {},
		CapabilitySettlement:       {},
	}
)

// Manifest declares plugin identity, protocol compatibility, capabilities,
// and least-privilege permissions. Capabilities contains required operations;
// OptionalCapabilities may be omitted by either side during negotiation.
type Manifest struct {
	SchemaVersion        int          `json:"schemaVersion" yaml:"schemaVersion"`
	ID                   string       `json:"id" yaml:"id"`
	Name                 string       `json:"name" yaml:"name"`
	Version              string       `json:"version" yaml:"version"`
	APIVersion           string       `json:"apiVersion" yaml:"apiVersion"`
	License              string       `json:"license" yaml:"license"`
	Chains               []Chain      `json:"chains" yaml:"chains"`
	Capabilities         []Capability `json:"capabilities" yaml:"capabilities"`
	OptionalCapabilities []Capability `json:"optionalCapabilities,omitempty" yaml:"optionalCapabilities,omitempty"`
	Permissions          Permissions  `json:"permissions" yaml:"permissions"`
}

// Chain declares a canonical chain identifier and the assets served by it.
type Chain struct {
	ChainID string   `json:"chainId" yaml:"chainId"`
	Assets  []string `json:"assets" yaml:"assets"`
}

// Permissions declares external network and Core signing access requested by
// a plugin. Absence means no permission.
type Permissions struct {
	Network []string            `json:"network,omitempty" yaml:"network,omitempty"`
	Signing []SigningPermission `json:"signing,omitempty" yaml:"signing,omitempty"`
}

// SigningPermission restricts a plugin to a reviewed algorithm and purpose.
type SigningPermission struct {
	Algorithm string `json:"algorithm" yaml:"algorithm"`
	Purpose   string `json:"purpose" yaml:"purpose"`
}

// ParseManifest decodes and validates a plugin.yaml payload.
func ParseManifest(data []byte) (Manifest, error) {
	var manifest Manifest
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode payment plugin manifest: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return Manifest{}, fmt.Errorf("decode payment plugin manifest: multiple YAML documents are not allowed")
		}
		return Manifest{}, fmt.Errorf("decode payment plugin manifest: %w", err)
	}
	if err := manifest.Validate(); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

// Validate enforces protocol, identity, capability, and permission shape.
func (m Manifest) Validate() error {
	if m.SchemaVersion != ManifestSchemaVersion {
		return fmt.Errorf("payment plugin manifest schemaVersion must be %d", ManifestSchemaVersion)
	}
	if !pluginIDPattern.MatchString(m.ID) {
		return fmt.Errorf("invalid payment plugin id %q", m.ID)
	}
	if strings.TrimSpace(m.Name) == "" {
		return fmt.Errorf("payment plugin %q: name is required", m.ID)
	}
	if !versionPattern.MatchString(m.Version) {
		return fmt.Errorf("payment plugin %q: invalid semantic version %q", m.ID, m.Version)
	}
	if m.APIVersion != APIVersionV1 {
		return fmt.Errorf("payment plugin %q: unsupported apiVersion %q", m.ID, m.APIVersion)
	}
	if !spdxLikePattern.MatchString(m.License) {
		return fmt.Errorf("payment plugin %q: invalid license identifier %q", m.ID, m.License)
	}
	if err := validateChains(m.Chains); err != nil {
		return fmt.Errorf("payment plugin %q: %w", m.ID, err)
	}
	if err := validateCapabilities(m.Capabilities, m.OptionalCapabilities); err != nil {
		return fmt.Errorf("payment plugin %q: %w", m.ID, err)
	}
	if err := validatePermissions(m.Permissions); err != nil {
		return fmt.Errorf("payment plugin %q: %w", m.ID, err)
	}
	return nil
}

func validateChains(chains []Chain) error {
	if len(chains) == 0 {
		return fmt.Errorf("at least one chain is required")
	}
	seenChains := make(map[string]struct{}, len(chains))
	for _, chain := range chains {
		if !chainIDPattern.MatchString(chain.ChainID) {
			return fmt.Errorf("invalid chainId %q", chain.ChainID)
		}
		if _, duplicate := seenChains[chain.ChainID]; duplicate {
			return fmt.Errorf("duplicate chainId %q", chain.ChainID)
		}
		seenChains[chain.ChainID] = struct{}{}
		if len(chain.Assets) == 0 {
			return fmt.Errorf("chain %q must declare at least one asset", chain.ChainID)
		}
		seenAssets := make(map[string]struct{}, len(chain.Assets))
		for _, asset := range chain.Assets {
			if !assetPattern.MatchString(asset) {
				return fmt.Errorf("chain %q has invalid asset %q", chain.ChainID, asset)
			}
			if _, duplicate := seenAssets[asset]; duplicate {
				return fmt.Errorf("chain %q has duplicate asset %q", chain.ChainID, asset)
			}
			seenAssets[asset] = struct{}{}
		}
	}
	return nil
}

func validateCapabilities(required, optional []Capability) error {
	if len(required) == 0 {
		return fmt.Errorf("at least one required capability is required")
	}
	seen := make(map[Capability]struct{}, len(required)+len(optional))
	for _, capability := range append(append([]Capability{}, required...), optional...) {
		if _, known := knownCapabilities[capability]; !known {
			return fmt.Errorf("unknown capability %q", capability)
		}
		if _, duplicate := seen[capability]; duplicate {
			return fmt.Errorf("duplicate capability %q", capability)
		}
		seen[capability] = struct{}{}
	}
	return nil
}

func validatePermissions(permissions Permissions) error {
	seenNetwork := make(map[string]struct{}, len(permissions.Network))
	for _, endpoint := range permissions.Network {
		if err := validateNetworkPermission(endpoint); err != nil {
			return err
		}
		if _, duplicate := seenNetwork[endpoint]; duplicate {
			return fmt.Errorf("duplicate network permission %q", endpoint)
		}
		seenNetwork[endpoint] = struct{}{}
	}
	seenSigning := make(map[string]struct{}, len(permissions.Signing))
	for _, permission := range permissions.Signing {
		algorithm := strings.ToLower(strings.TrimSpace(permission.Algorithm))
		purpose := strings.ToLower(strings.TrimSpace(permission.Purpose))
		if algorithm != "secp256k1" && algorithm != "ed25519" {
			return fmt.Errorf("unsupported signing algorithm %q", permission.Algorithm)
		}
		if purpose != "transaction" && purpose != "message" {
			return fmt.Errorf("unsupported signing purpose %q", permission.Purpose)
		}
		key := algorithm + ":" + purpose
		if _, duplicate := seenSigning[key]; duplicate {
			return fmt.Errorf("duplicate signing permission %q", key)
		}
		seenSigning[key] = struct{}{}
	}
	return nil
}

func validateNetworkPermission(endpoint string) error {
	if !strings.HasPrefix(endpoint, "tcp:") {
		return fmt.Errorf("network permission %q must use tcp:host:port", endpoint)
	}
	hostPort := strings.TrimPrefix(endpoint, "tcp:")
	host, portText, err := net.SplitHostPort(hostPort)
	if err != nil {
		return fmt.Errorf("invalid network permission %q: %w", endpoint, err)
	}
	if strings.TrimSpace(host) == "" || strings.ContainsAny(host, "*/@") {
		return fmt.Errorf("invalid network permission host %q", host)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("invalid network permission port %q", portText)
	}
	return nil
}
