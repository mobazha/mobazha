// Package api — huma_api.go
//
// AH-1.4: Establishes the huma v2 + humachi base wiring for the Node
// business API (/v1/*). Mirrors the hosting huma scaffold (AH-1.2/1.3)
// with adaptations for the Node auth model (Basic Auth / JWT / API Token).
//
// Architectural choices (shared with hosting, locked in AH-1.2):
//   - Shared chi router. huma operations register directly on
//     the existing V1 chi mux.
//   - OpenAPI 3.1 spec served at /v1/openapi.json.
//   - Per-route auth via huma.Operation.Security + auth bridge middleware.
package api

import (
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/mobazha/mobazha3.0/pkg/distribution"
	"github.com/mobazha/mobazha3.0/pkg/edition"
)

const (
	nodeHumaAPIVersion     = "1.0.0"
	nodeHumaAPITitle       = "Mobazha Node API"
	nodeHumaAPIDescription = "OpenAPI 3.1 contract for the Node business domain " +
		"(/v1/*). Generated from typed Go handlers via huma v2."
)

// registerHumaAPI installs the huma adapter onto the V1 router and
// registers all huma-managed operations.
func (g *Gateway) registerHumaAPI(r chi.Router) huma.API {
	apiTitle := nodeHumaAPITitle
	apiDescription := nodeHumaAPIDescription
	serverDescription := "Node gateway"
	if g.restrictedProductSurface() {
		if g.config.Brand != nil && g.config.Brand.Name != "" {
			apiTitle = g.config.Brand.Name + " API"
		}
		apiDescription = "Local-first sovereign commerce node API."
		serverDescription = "Sovereign node"
	}
	cfg := huma.DefaultConfig(apiTitle, nodeHumaAPIVersion)
	cfg.Info.Description = apiDescription

	cfg.OpenAPIPath = "/v1/openapi"
	cfg.DocsPath = "/v1/docs"
	cfg.SchemasPath = "/v1/schemas"

	cfg.Servers = []*huma.Server{{URL: "/", Description: serverDescription}}

	if cfg.Components == nil {
		cfg.Components = &huma.Components{}
	}
	if cfg.Components.SecuritySchemes == nil {
		cfg.Components.SecuritySchemes = map[string]*huma.SecurityScheme{}
	}
	cfg.Components.SecuritySchemes[SecuritySchemeBasicAuth] = &huma.SecurityScheme{
		Type:        "http",
		Scheme:      "basic",
		Description: "Standalone admin password via HTTP Basic Auth.",
	}
	cfg.Components.SecuritySchemes[SecuritySchemeBearerJWT] = &huma.SecurityScheme{
		Type:         "http",
		Scheme:       "bearer",
		BearerFormat: "JWT",
		Description:  "Casdoor JWT issued by the SaaS platform (Mini App / proxy).",
	}
	cfg.Components.SecuritySchemes[SecuritySchemeAPIToken] = &huma.SecurityScheme{
		Type:         "http",
		Scheme:       "bearer",
		BearerFormat: "mbz_<id>_<secret>",
		Description:  "Scoped API token (standalone). Prefix: mbz_.",
	}
	cfg.Components.SecuritySchemes[SecuritySchemeNodeAuth] = &huma.SecurityScheme{
		Type:        "http",
		Scheme:      "bearer",
		Description: "Node authentication: Basic Auth (standalone admin), Bearer JWT (SaaS proxy), or Bearer mbz_ API token.",
	}

	installNodeHumaEnvelope(&cfg)

	api := humachi.New(r, cfg)

	g.installNodeHumaMiddlewares(api)
	if g.restrictedProductSurface() {
		g.registerRestrictedHumaOperations(api)
		g.registerTrustedHumaModules(api)
		return api
	}

	// Public storefront routes — always registered, even in PublicOnly mode.
	g.registerNodeHumaSmokeRoutes(api)
	g.registerNodeHumaListingPublicOperations(api)
	g.registerNodeHumaMediaPublicOperations(api)
	g.registerNodeHumaProfilePublicOperations(api)
	g.registerNodeHumaDiscountPublicOperations(api)
	g.registerNodeHumaCollectionPublicOperations(api)
	g.registerNodeHumaStorePolicyPublicOperations(api)
	g.registerNodeHumaSystemPublicOperations(api)
	g.registerNodeHumaMiscPublicOperations(api)
	g.registerNodeHumaSocialPublicOperations(api)
	g.registerNodeHumaOrderPublicOperations(api)
	if g.editionPolicy != nil && g.editionPolicy.AllowsCapability(edition.CapabilityFiatPayments) {
		g.registerNodeHumaFiatPublicOperations(api)
	}
	g.registerNodeHumaFulfillmentPublicOperations(api)
	g.registerNodeHumaSettingsPublicOperations(api)
	g.registerNodeHumaAuthPublicOperations(api)

	// Buyer portal and license validation are public endpoints. Guest buyer
	// access uses an independent buyerPortalToken; license validation uses the
	// submitted license key as its capability. Keep these available in
	// PublicOnly mode so buyers can access purchased digital goods.
	g.registerNodeHumaDigitalOperations(api)

	// Admin/seller routes — suppressed in PublicOnly (--publicgateway) mode.
	if !g.config.PublicOnly {
		// Admin parts of mixed domains (public parts already registered above).
		g.registerNodeHumaListingAdminOperations(api)
		g.registerNodeHumaMediaAdminOperations(api)
		g.registerNodeHumaProfileAdminOperations(api)
		g.registerNodeHumaDiscountAdminOperations(api)
		g.registerNodeHumaCollectionAdminOperations(api)
		g.registerNodeHumaStorePolicyAdminOperations(api)
		g.registerNodeHumaSystemAdminOperations(api)
		g.registerNodeHumaMiscAdminOperations(api)
		g.registerNodeHumaSocialAdminOperations(api)
		g.registerNodeHumaOrderAdminOperations(api)
		if g.editionPolicy != nil && g.editionPolicy.AllowsCapability(edition.CapabilityFiatPayments) {
			g.registerNodeHumaFiatAdminOperations(api)
		}
		g.registerNodeHumaFulfillmentAdminOperations(api)
		g.registerNodeHumaSettingsAdminOperations(api)
		g.registerNodeHumaAuthAdminOperations(api)
		// Pure admin domains.
		g.registerNodeHumaWalletOperations(api)
		g.registerNodeHumaChatOperations(api)
		g.registerNodeHumaDisputeOperations(api)
		g.registerNodeHumaCartOperations(api)
		g.registerNodeHumaNotificationOperations(api)
		g.registerNodeHumaWebhookOperations(api)
		g.registerAIHTTPCapabilities(api)
		g.registerNodeHumaShippingOperations(api)
		g.registerNodeHumaSellerDigitalOperations(api)
	}
	g.registerTrustedHumaModules(api)

	return api
}

func (g *Gateway) restrictedProductSurface() bool {
	return g != nil && g.config != nil && g.config.ProductSurfacePolicy != nil &&
		g.config.ProductSurfacePolicy.CoreAPISurface() == distribution.CoreAPISurfaceRestricted
}

// registerRestrictedHumaOperations is the explicit, fail-closed contract for
// local-first distributions. Private modules can extend this surface through
// TrustedHumaModules, but unselected Open Core domains are never registered.
func (g *Gateway) registerRestrictedHumaOperations(api huma.API) {
	g.registerNodeHumaSmokeRoutes(api)
	g.registerNodeHumaListingPublicOperations(api)
	g.registerNodeHumaMediaPublicOperations(api)
	g.registerNodeHumaProfilePublicOperations(api)
	g.registerNodeHumaDiscountPublicOperations(api)
	g.registerNodeHumaCollectionPublicOperations(api)
	g.registerNodeHumaStorePolicyPublicOperations(api)
	g.registerNodeHumaSystemPublicOperations(api)
	g.registerNodeHumaSettingsPublicOperations(api)
	g.registerNodeHumaAuthPublicOperations(api)
	g.registerGuestOrderQuotePublic(api)
	g.registerGuestOrderPostPublic(api)
	g.registerGuestOrderGetPublic(api)
	g.registerPaymentMethodsGet(api)
	g.registerPGPKeyGet(api)
	g.registerNodeHumaMiscPublicOperations(api)
	g.registerNodeHumaDigitalOperations(api)

	if g.config.PublicOnly {
		return
	}
	g.registerNodeHumaListingAdminOperations(api)
	g.registerNodeHumaMediaAdminOperations(api)
	g.registerNodeHumaProfileAdminOperations(api)
	g.registerNodeHumaDiscountAdminOperations(api)
	g.registerNodeHumaCollectionAdminOperations(api)
	g.registerNodeHumaStorePolicyAdminOperations(api)
	g.registerNodeHumaSystemAdminOperations(api)
	g.registerNodeHumaSettingsAdminOperations(api)
	g.registerNodeHumaAuthAdminOperations(api)
	g.registerNodeHumaShippingOperations(api)
	g.registerNodeHumaCartOperations(api)
	g.registerGuestOrdersListAuth(api)
	g.registerGuestOrderShip(api)
	g.registerGuestOrderComplete(api)
	g.registerGuestOrderAdminDetail(api)
	g.registerPGPKeyPut(api)
	g.registerPGPKeyDelete(api)
	g.registerNodeHumaReceivingAccountOperations(api)
	g.registerNodeHumaNotificationCoreOperations(api)
	g.registerNodeHumaSellerDigitalOperations(api)
	g.registerAIHTTPCapabilities(api)
}
