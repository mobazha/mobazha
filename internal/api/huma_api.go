// Package api — huma_api.go
//
// AH-1.4: Establishes the huma v2 + humamux base wiring for the Node
// business API (/v1/*). Mirrors the hosting huma scaffold (AH-1.2/1.3)
// with adaptations for the Node auth model (Basic Auth / JWT / API Token).
//
// Architectural choices (shared with hosting, locked in AH-1.2):
//   - Shared gorilla/mux router. huma operations register directly on
//     the existing V1 mux.
//   - OpenAPI 3.1 spec served at /v1/openapi.json.
//   - Per-route auth via huma.Operation.Security + auth bridge middleware.
package api

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humamux"
	"github.com/gorilla/mux"
)

const (
	nodeHumaAPIVersion     = "1.0.0"
	nodeHumaAPITitle       = "Mobazha Node API"
	nodeHumaAPIDescription = "OpenAPI 3.1 contract for the Node business domain " +
		"(/v1/*). Generated from typed Go handlers via huma v2."
)

// Security scheme names referenced in huma.Operation.Security.
const (
	// SecuritySchemeBasicAuth is standalone admin Basic Auth.
	SecuritySchemeBasicAuth = "basicAuth"
	// SecuritySchemeBearerJWT is Casdoor Bearer JWT (SaaS proxy / Mini App).
	SecuritySchemeBearerJWT = "bearerJWT"
	// SecuritySchemeAPIToken is mbz_<id>_<secret> API token.
	SecuritySchemeAPIToken = "apiToken"
	// SecuritySchemeNodeAuth is the unified node auth scheme covering
	// Basic Auth, Bearer JWT, and API token — used in Operation.Security.
	SecuritySchemeNodeAuth = "nodeAuth"
)

// registerHumaAPI installs the huma adapter onto the V1 router and
// registers all huma-managed operations.
func (g *Gateway) registerHumaAPI(r *mux.Router) huma.API {
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

	api := humamux.New(r, cfg)

	g.installNodeHumaMiddlewares(api)

	g.registerNodeHumaSmokeRoutes(api)
	g.registerNodeHumaWalletOperations(api)
	g.registerNodeHumaChatOperations(api)

	return api
}

// HumaNodePingOutput is the smoke-test response.
type HumaNodePingOutput struct {
	Body struct {
		Message    string    `json:"message" example:"pong" doc:"Static greeting."`
		ServerTime time.Time `json:"serverTime" doc:"Server time at request handling."`
	}
}

func (g *Gateway) registerNodeHumaSmokeRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "node-huma-ping",
		Method:      http.MethodGet,
		Path:        "/v1/system/huma-ping",
		Summary:     "Huma pipeline smoke test",
		Description: "Returns a static greeting and server time.",
		Tags:        []string{"system"},
	}, func(ctx context.Context, _ *struct{}) (*HumaNodePingOutput, error) {
		out := &HumaNodePingOutput{}
		out.Body.Message = "pong"
		out.Body.ServerTime = time.Now().UTC()
		return out, nil
	})
}
