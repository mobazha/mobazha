//go:build !private_distribution

package api

import (
	"encoding/json"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
)

// BuildOpenAPISpec returns the complete OpenAPI 3.1 JSON for the Node
// business API. It creates a temporary router and zero Gateway, registers
// all operations, and serializes the spec.
//
// This function does NOT depend on any runtime state (DB, wallet, P2P).
// Handler closures capture a zero Gateway but are never invoked.
func BuildOpenAPISpec() []byte {
	r := chi.NewRouter()
	g := &Gateway{}

	cfg := huma.DefaultConfig(nodeHumaAPITitle, nodeHumaAPIVersion)
	cfg.Info.Description = nodeHumaAPIDescription

	cfg.OpenAPIPath = "/v1/openapi"
	cfg.DocsPath = "/v1/docs"
	cfg.SchemasPath = "/v1/schemas"

	cfg.Servers = []*huma.Server{{URL: "/", Description: "Node gateway"}}

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

	// Public operations.
	g.registerNodeHumaSmokeRoutes(api)
	g.registerNodeHumaListingPublicOperations(api)
	g.registerNodeHumaMediaPublicOperations(api)
	g.registerNodeHumaProfilePublicOperations(api)
	g.registerNodeHumaDiscountPublicOperations(api)
	g.registerNodeHumaCollectionPublicOperations(api)
	g.registerNodeHumaSystemPublicOperations(api)
	g.registerNodeHumaMiscPublicOperations(api)
	g.registerNodeHumaSocialPublicOperations(api)
	g.registerNodeHumaOrderPublicOperations(api)
	g.registerNodeHumaFiatPublicOperations(api)
	g.registerNodeHumaFulfillmentPublicOperations(api)
	g.registerNodeHumaSettingsPublicOperations(api)
	g.registerNodeHumaAuthPublicOperations(api)
	// Admin operations.
	g.registerNodeHumaListingAdminOperations(api)
	g.registerNodeHumaMediaAdminOperations(api)
	g.registerNodeHumaProfileAdminOperations(api)
	g.registerNodeHumaDiscountAdminOperations(api)
	g.registerNodeHumaCollectionAdminOperations(api)
	g.registerNodeHumaSystemAdminOperations(api)
	g.registerNodeHumaMiscAdminOperations(api)
	g.registerNodeHumaSocialAdminOperations(api)
	g.registerNodeHumaOrderAdminOperations(api)
	g.registerNodeHumaFiatAdminOperations(api)
	g.registerNodeHumaFulfillmentAdminOperations(api)
	g.registerNodeHumaSettingsAdminOperations(api)
	g.registerNodeHumaAuthAdminOperations(api)
	g.registerNodeHumaWalletOperations(api)
	g.registerNodeHumaChatOperations(api)
	g.registerNodeHumaDisputeOperations(api)
	g.registerNodeHumaCartOperations(api)
	g.registerNodeHumaNotificationOperations(api)
	g.registerNodeHumaWebhookOperations(api)
	g.registerNodeHumaAIOperations(api)
	g.registerNodeHumaShippingOperations(api)
	g.registerNodeHumaDigitalOperations(api)
	g.registerNodeHumaSellerDigitalOperations(api)

	spec, err := json.MarshalIndent(api.OpenAPI(), "", "  ")
	if err != nil {
		panic("failed to serialize OpenAPI spec: " + err.Error())
	}
	return spec
}
