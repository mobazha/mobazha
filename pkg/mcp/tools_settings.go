package mcp

import (
	"context"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func settingsToolRegistrars(bf BridgeFactory) []ToolRegistrar {
	return []ToolRegistrar{
		{
			Name: "settings_get_storefront",
			Tool: gomcp.NewTool("settings_get_storefront",
				gomcp.WithDescription(
					"Get storefront configuration including accepted currencies, shipping options, "+
						"return policy, terms of service, and store appearance settings. "+
						"Use when asked about store setup, accepted payment methods, or shipping configuration.",
				),
			),
			Handler: makeSettingsGetStorefront(bf),
		},
	}
}

func makeSettingsGetStorefront(bf BridgeFactory) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		bridge := bf(req)
		code, body, err := bridge.Call(ctx, "GET", "/v1/settings/storefront", nil, nil)
		return HandleBridgeResult(code, body, err)
	}
}
