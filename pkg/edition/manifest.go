// Package edition defines the public distribution policy that narrows
// recognized backend capabilities into capabilities an edition may expose.
package edition

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

const (
	// CommunityName identifies the public Community Edition distribution.
	CommunityName = "community"
	// FullName identifies an unrestricted private or commercial composition.
	FullName = "full"
	// ManifestSchemaVersion is the current edition manifest schema.
	ManifestSchemaVersion = 1
)

var editionNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)

//go:embed manifests/community.json
var communityManifestJSON []byte

// Manifest is the machine-readable boundary for a Mobazha distribution.
// It is a positive allowlist: omitted capabilities are unavailable even when
// their identifiers or legacy adapters remain recognizable by Core.
type Manifest struct {
	SchemaVersion           int             `json:"schemaVersion"`
	Edition                 string          `json:"edition"`
	License                 string          `json:"license"`
	PaymentPluginSDKLicense string          `json:"paymentPluginSdkLicense"`
	Payment                 PaymentManifest `json:"payment"`
	DeploymentTargets       []string        `json:"deploymentTargets"`
	Zcash                   ZcashManifest   `json:"zcash"`
}

// PaymentManifest lists payment chains and rails enabled by an edition.
type PaymentManifest struct {
	Chains []string `json:"chains"`
	Rails  []string `json:"rails"`
}

// ZcashManifest records the address-family boundary for Zcash.
type ZcashManifest struct {
	TransparentOnly   bool   `json:"transparentOnly"`
	DefaultDerivation string `json:"defaultDerivation"`
}

// ParseManifest decodes and validates an edition manifest.
func ParseManifest(data []byte) (Manifest, error) {
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode edition manifest: %w", err)
	}
	if err := manifest.Validate(); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

// CommunityManifest returns a validated copy of the embedded Community
// Edition manifest used by production capability policy.
func CommunityManifest() (Manifest, error) {
	return ParseManifest(communityManifestJSON)
}

// Validate checks the shape and positive-allowlist invariants of a manifest.
func (m Manifest) Validate() error {
	if m.SchemaVersion != ManifestSchemaVersion {
		return fmt.Errorf("edition manifest schemaVersion must be %d", ManifestSchemaVersion)
	}
	if !editionNamePattern.MatchString(m.Edition) {
		return fmt.Errorf("invalid edition name %q", m.Edition)
	}
	if strings.TrimSpace(m.License) == "" {
		return fmt.Errorf("edition %q: license is required", m.Edition)
	}
	if strings.TrimSpace(m.PaymentPluginSDKLicense) == "" {
		return fmt.Errorf("edition %q: paymentPluginSdkLicense is required", m.Edition)
	}
	if err := validateUniqueTokens("payment.chains", m.Payment.Chains, true); err != nil {
		return err
	}
	if err := validateUniqueTokens("payment.rails", m.Payment.Rails, false); err != nil {
		return err
	}
	if err := validateUniqueTokens("deploymentTargets", m.DeploymentTargets, false); err != nil {
		return err
	}
	if contains(m.Payment.Chains, "ZEC") {
		if !m.Zcash.TransparentOnly || m.Zcash.DefaultDerivation != "transparent" {
			return fmt.Errorf("edition %q: ZEC requires transparent-only derivation", m.Edition)
		}
	}
	return nil
}

func validateUniqueTokens(field string, values []string, uppercase bool) error {
	if len(values) == 0 {
		return fmt.Errorf("%s must contain at least one value", field)
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return fmt.Errorf("%s contains an empty value", field)
		}
		if uppercase && trimmed != strings.ToUpper(trimmed) {
			return fmt.Errorf("%s value %q must be uppercase", field, value)
		}
		if _, exists := seen[trimmed]; exists {
			return fmt.Errorf("%s contains duplicate value %q", field, trimmed)
		}
		seen[trimmed] = struct{}{}
	}
	return nil
}

func contains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
