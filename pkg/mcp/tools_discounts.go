package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func discountsToolRegistrars(bf BridgeFactory) []ToolRegistrar {
	return []ToolRegistrar{
		{
			Name: "discounts_list",
			Tool: gomcp.NewTool("discounts_list",
				gomcp.WithDescription(
					"List all discount codes/coupons configured for the store. "+
						"Returns discount details including code, type (percentage/fixed), value, and status. "+
						"Use when asked about active promotions, coupon codes, or discount management.",
				),
			),
			Handler: makeDiscountsList(bf),
		},
		{
			Name: "discounts_create",
			Tool: gomcp.NewTool("discounts_create",
				gomcp.WithDescription(
					"Create a new discount code/coupon for the store. "+
						"Pass a JSON object with discount details including code, type, value, and optional expiry.",
				),
				gomcp.WithString("discount_json",
					gomcp.Required(),
					gomcp.Description("JSON object with discount details (code, discountType, value, etc.)"),
				),
			),
			Handler: makeDiscountsCreate(bf),
		},
		{
			Name: "discounts_update",
			Tool: gomcp.NewTool("discounts_update",
				gomcp.WithDescription(
					"Update an existing discount code/coupon. "+
						"Pass the discount ID and a JSON object with the fields to update.",
				),
				gomcp.WithString("discount_id",
					gomcp.Required(),
					gomcp.Description("The discount ID to update"),
				),
				gomcp.WithString("discount_json",
					gomcp.Required(),
					gomcp.Description("JSON object with discount fields to update"),
				),
			),
			Handler: makeDiscountsUpdate(bf),
		},
		{
			Name: "discounts_delete",
			Tool: gomcp.NewTool("discounts_delete",
				gomcp.WithDescription(
					"Delete a discount code/coupon permanently. [DESTRUCTIVE] This action cannot be undone. "+
						"The AI client MUST confirm with the user before calling this tool.",
				),
				gomcp.WithString("discount_id",
					gomcp.Required(),
					gomcp.Description("The discount ID to delete"),
				),
			),
			Handler: makeDiscountsDelete(bf),
		},
	}
}

func makeDiscountsList(bf BridgeFactory) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		bridge := bf(req)
		code, body, err := bridge.Call(ctx, "GET", "/v1/discounts", nil, nil)
		return HandleBridgeResult(code, body, err)
	}
}

func makeDiscountsCreate(bf BridgeFactory) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		discountJSON := req.GetString("discount_json", "")
		if discountJSON == "" {
			return gomcp.NewToolResultError("discount_json is required"), nil
		}
		var payload json.RawMessage
		if err := json.Unmarshal([]byte(discountJSON), &payload); err != nil {
			return gomcp.NewToolResultError(fmt.Sprintf("invalid JSON: %v", err)), nil
		}
		bridge := bf(req)
		code, body, err := bridge.Call(ctx, "POST", "/v1/discounts", nil, payload)
		return HandleBridgeResult(code, body, err)
	}
}

func makeDiscountsUpdate(bf BridgeFactory) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		discountID := req.GetString("discount_id", "")
		if discountID == "" {
			return gomcp.NewToolResultError("discount_id is required"), nil
		}
		discountJSON := req.GetString("discount_json", "")
		if discountJSON == "" {
			return gomcp.NewToolResultError("discount_json is required"), nil
		}
		var payload json.RawMessage
		if err := json.Unmarshal([]byte(discountJSON), &payload); err != nil {
			return gomcp.NewToolResultError(fmt.Sprintf("invalid JSON: %v", err)), nil
		}
		bridge := bf(req)
		path := fmt.Sprintf("/v1/discounts/%s", url.PathEscape(discountID))
		code, body, err := bridge.Call(ctx, "PUT", path, nil, payload)
		return HandleBridgeResult(code, body, err)
	}
}

func makeDiscountsDelete(bf BridgeFactory) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		discountID := req.GetString("discount_id", "")
		if discountID == "" {
			return gomcp.NewToolResultError("discount_id is required"), nil
		}
		bridge := bf(req)
		path := fmt.Sprintf("/v1/discounts/%s", url.PathEscape(discountID))
		code, body, err := bridge.Call(ctx, "DELETE", path, nil, nil)
		return HandleBridgeResult(code, body, err)
	}
}
