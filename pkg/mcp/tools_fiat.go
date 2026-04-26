package mcp

import (
	"context"
	"fmt"
	"net/url"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func fiatToolRegistrars(bf BridgeFactory) []ToolRegistrar {
	return []ToolRegistrar{
		{
			Name: "fiat_get_providers",
			Tool: gomcp.NewTool("fiat_get_providers",
				gomcp.WithDescription(
					"List configured fiat payment providers (e.g., Stripe, PayPal) with their status. "+
						"Returns provider ID, display name, enabled state, and supported payment methods. "+
						"Use when asked about accepted fiat payment options or payment provider setup.",
				),
			),
			Handler: makeFiatGetProviders(bf),
		},
		{
			Name: "fiat_get_provider_config",
			Tool: gomcp.NewTool("fiat_get_provider_config",
				gomcp.WithDescription(
					"Get detailed configuration for a specific fiat payment provider. "+
						"Returns onboarding status, webhook setup, and provider-specific settings. "+
						"Use when asked about a specific payment provider's configuration or setup status.",
				),
				gomcp.WithString("provider_id",
					gomcp.Required(),
					gomcp.Description("The provider ID (e.g., 'stripe', 'paypal')"),
				),
			),
			Handler: makeFiatGetProviderConfig(bf),
		},
	}
}

func makeFiatGetProviders(bf BridgeFactory) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		bridge := bf(req)
		code, body, err := bridge.Call(ctx, "GET", "/v1/fiat/providers", nil, nil)
		return HandleBridgeResult(code, body, err)
	}
}

func makeFiatGetProviderConfig(bf BridgeFactory) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		providerID := req.GetString("provider_id", "")
		if providerID == "" {
			return gomcp.NewToolResultError("provider_id is required"), nil
		}
		bridge := bf(req)
		path := fmt.Sprintf("/v1/fiat/%s/config", url.PathEscape(providerID))
		code, body, err := bridge.Call(ctx, "GET", path, nil, nil)
		return HandleBridgeResult(code, body, err)
	}
}
