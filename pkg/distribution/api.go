package distribution

import "github.com/danielgtaylor/huma/v2"

// TrustedHumaRegistration is the narrow in-process API surface available to
// first-party distribution modules. The gateway still owns authentication,
// middleware, envelopes, listener lifecycle, and OpenAPI publication.
type TrustedHumaRegistration struct {
	API               huma.API
	NodeAuthSecurity  []map[string][]string
	AdminOnlySecurity []map[string][]string
}

// TrustedHumaModule registers first-party routes into the Core-owned gateway.
// It is a build-time composition contract, not the third-party plugin API.
// Implementations must use the supplied security requirements and must not
// replace gateway middleware or global OpenAPI configuration.
type TrustedHumaModule interface {
	RegisterTrustedHuma(registration TrustedHumaRegistration)
}
