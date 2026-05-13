package api

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
)

// Security scheme names referenced in huma.Operation.Security.
const (
	SecuritySchemeBasicAuth = "basicAuth"
	SecuritySchemeBearerJWT = "bearerJWT"
	SecuritySchemeAPIToken  = "apiToken"
	SecuritySchemeNodeAuth  = "nodeAuth"
)

// nodeAuthSecurity is the standard security requirement for owner-only routes.
// It allows any one of: Basic Auth, Bearer JWT, or API token.
var nodeAuthSecurity = []map[string][]string{
	{SecuritySchemeBasicAuth: {}},
	{SecuritySchemeBearerJWT: {}},
	{SecuritySchemeAPIToken: {}},
}

// adminOnlyAuthSecurity is used for first-run / lifecycle-critical
// operations that must NOT be reachable via mbz_ API tokens — e.g. the
// admin password setup, the EXTERNAL_PAYMENT wallet setup wizard, and other "operator
// at the keyboard" actions. Excluding apiToken keeps OpenAPI honest:
// machines that follow the spec won't waste a round-trip trying tokens
// that the scope middleware would deny-by-default anyway.
var adminOnlyAuthSecurity = []map[string][]string{
	{SecuritySchemeBasicAuth: {}},
	{SecuritySchemeBearerJWT: {}},
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
