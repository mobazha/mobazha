package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func profileToolRegistrars(bf BridgeFactory) []ToolRegistrar {
	return []ToolRegistrar{
		{
			Name: "profile_get",
			Tool: gomcp.NewTool("profile_get",
				gomcp.WithDescription(
					"Get the store's profile information including name, description, avatar, contact info, "+
						"social media links, and store statistics. "+
						"Use when asked about the store identity, who owns the store, or store configuration.",
				),
			),
			Handler: makeProfileGet(bf),
		},
		{
			Name: "profile_update",
			Tool: gomcp.NewTool("profile_update",
				gomcp.WithDescription(
					"Update the store's profile information. "+
						"Pass a JSON object with the fields to update. Use profile_get first to see current values. "+
						"Supported fields: name, about, shortDescription, location, contactInfo, colors, nsfw.",
				),
				gomcp.WithString("profile_json",
					gomcp.Required(),
					gomcp.Description("JSON object with profile fields to update"),
				),
			),
			Handler: makeProfileUpdate(bf),
		},
	}
}

func makeProfileGet(bf BridgeFactory) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		bridge := bf(req)
		code, body, err := bridge.Call(ctx, "GET", "/v1/profiles", nil, nil)
		return HandleBridgeResult(code, body, err)
	}
}

func makeProfileUpdate(bf BridgeFactory) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		profileJSON := req.GetString("profile_json", "")
		if profileJSON == "" {
			return gomcp.NewToolResultError("profile_json is required"), nil
		}
		var payload json.RawMessage
		if err := json.Unmarshal([]byte(profileJSON), &payload); err != nil {
			return gomcp.NewToolResultError(fmt.Sprintf("invalid JSON: %v", err)), nil
		}
		bridge := bf(req)
		code, body, err := bridge.Call(ctx, "PUT", "/v1/profiles", nil, payload)
		return HandleBridgeResult(code, body, err)
	}
}
