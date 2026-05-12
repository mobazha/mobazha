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
