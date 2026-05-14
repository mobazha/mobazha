package repo

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const brandConfigFilename = "brand.yaml"

// BrandConfig holds white-label branding overrides loaded from
// <dataDir>/brand.yaml. When absent, the node uses Mobazha defaults.
// Partners (e.g. "Example Market") ship a pre-filled brand.yaml
// alongside the binary or Docker image.
type BrandConfig struct {
	Brand BrandFields `yaml:"brand"`
}

// BrandFields contains the actual branding values exposed to CLI,
// OpenAPI docs, and the frontend via /runtime-config.js.
type BrandFields struct {
	// Name is the product name shown in CLI banners, page titles, and
	// OpenAPI docs (e.g. "Example Market").
	Name string `yaml:"name" json:"name"`

	// ShortName is an abbreviated form for constrained UI (e.g. "AGM").
	ShortName string `yaml:"shortName,omitempty" json:"shortName,omitempty"`

	// Tagline is a one-line product description.
	Tagline string `yaml:"tagline,omitempty" json:"tagline,omitempty"`

	// Description is a longer product description for meta tags / docs.
	Description string `yaml:"description,omitempty" json:"description,omitempty"`

	// PrimaryColor is the CSS hex colour for the main brand accent
	// (e.g. "#7B2FBE"). Injected as a CSS custom property.
	PrimaryColor string `yaml:"primaryColor,omitempty" json:"primaryColor,omitempty"`

	// AccentColor is a secondary brand colour.
	AccentColor string `yaml:"accentColor,omitempty" json:"accentColor,omitempty"`

	// LogoURL is a path (relative to the frontend override dir) or
	// absolute URL pointing to the brand logo (e.g. "/brand/logo.svg").
	LogoURL string `yaml:"logoUrl,omitempty" json:"logoUrl,omitempty"`

	// FaviconURL overrides the default favicon.
	FaviconURL string `yaml:"faviconUrl,omitempty" json:"faviconUrl,omitempty"`

	// PrivacyNotice is a short text displayed in the storefront footer
	// (e.g. "This store operates on I2P. No data leaves the network.").
	PrivacyNotice string `yaml:"privacyNotice,omitempty" json:"privacyNotice,omitempty"`

	// HidePoweredBy, when true, removes the "Powered by Mobazha"
	// attribution from the storefront footer.
	HidePoweredBy bool `yaml:"hidePoweredBy,omitempty" json:"hidePoweredBy,omitempty"`

	// Network controls UI visibility of network/node-pool features for
	// white-label deployments (PrivateDistribution / Example / Example Market).
	// All flags default to false (hide) so partners get the locked-down
	// baseline by default; opt-in is explicit per partner.
	//
	// Protocol-layer decisions (i2pProxy/torProxy fallback) are NOT
	// configured here — those live in the system layer. Brand only
	// controls what the user sees and can edit. See
	// PRIVATE_DISTRIBUTION_EXTERNAL_PAYMENTD_NETWORK_DESIGN.md §3.2 for rationale.
	Network NetworkFields `yaml:"network,omitempty" json:"network,omitempty"`
}

// NetworkFields gates the network/node-pool UI surface for white-label
// builds. Setting all fields to false yields the most locked-down UX
// (Example baseline) — users see neither node lists nor diagnostics.
// Each flag is independent so partners can opt in to advanced visibility
// piecemeal without unlocking custom-node entry.
type NetworkFields struct {
	// AllowUserCustomNode lets the user paste their own external_paymentd RPC
	// address into Settings → Network. Off by default — Example-style
	// devices ship a curated pool only. Independent of ShowNodePoolUI:
	// a partner may show the curated pool (read-only) without allowing
	// custom entries.
	AllowUserCustomNode bool `yaml:"allowUserCustomNode,omitempty" json:"allowUserCustomNode,omitempty"`

	// ShowAdvancedDiagnostics surfaces per-node latency, height-lag,
	// success/fail streak, and source (embedded / user / p2p-discovered)
	// in the UI. Off by default — partners that want a "magic, just
	// works" UX hide all of this.
	ShowAdvancedDiagnostics bool `yaml:"showAdvancedDiagnostics,omitempty" json:"showAdvancedDiagnostics,omitempty"`

	// ShowNodePoolUI exposes Settings → Network → ExternalPayment Nodes (the
	// node pool management page). Off by default — without this the
	// user has no per-node visibility; the system rotates silently.
	ShowNodePoolUI bool `yaml:"showNodePoolUI,omitempty" json:"showNodePoolUI,omitempty"`

	// AllowDiscoverToggle exposes the on/off switch for Tier 3 P2P
	// self-discovery (`get_peer_list` polling). Off by default —
	// discovery runs silently per the build's compiled defaults. Only
	// general-purpose PrivateDistributions (not white-label) typically expose this.
	AllowDiscoverToggle bool `yaml:"allowDiscoverToggle,omitempty" json:"allowDiscoverToggle,omitempty"`
}

// LoadBrandConfig reads <dataDir>/brand.yaml and returns the parsed
// config. If the file does not exist it returns nil (no error) — the
// caller should fall back to Mobazha defaults.
func LoadBrandConfig(dataDir string) (*BrandFields, error) {
	p := filepath.Join(dataDir, brandConfigFilename)
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var cfg BrandConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.Brand.Name == "" {
		return nil, nil
	}
	return &cfg.Brand, nil
}

// DefaultBrandName returns the default product name when no brand.yaml
// is present. Overridable via ldflags at build time.
func DefaultBrandName() string {
	return defaultProductName
}
