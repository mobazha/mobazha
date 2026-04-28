package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func collectionsToolRegistrars(bf BridgeFactory) []ToolRegistrar {
	return []ToolRegistrar{
		{
			Name: "collections_list",
			Tool: gomcp.NewTool("collections_list",
				gomcp.WithDescription(
					"List all product collections in the store. "+
						"Returns collection names, descriptions, and product counts. "+
						"Use when asked about store categories, product groups, or how products are organized.",
				),
			),
			Handler: makeCollectionsList(bf),
		},
		{
			Name: "collections_create",
			Tool: gomcp.NewTool("collections_create",
				gomcp.WithDescription(
					"Create a new product collection to organize store products. "+
						"Pass a JSON object with collection details including title and description.",
				),
				gomcp.WithString("collection_json",
					gomcp.Required(),
					gomcp.Description(`JSON object with collection details, e.g. {"title":"Best Sellers","description":"Our top products"}`),
				),
			),
			Handler: makeCollectionsCreate(bf),
		},
		{
			Name: "collections_add_products",
			Tool: gomcp.NewTool("collections_add_products",
				gomcp.WithDescription(
					"Add products to an existing collection. "+
						"Pass the collection ID and a JSON object with a slugs array.",
				),
				gomcp.WithString("collection_id",
					gomcp.Required(),
					gomcp.Description("The collection ID to add products to"),
				),
				gomcp.WithString("products_json",
					gomcp.Required(),
					gomcp.Description(`JSON object with a "slugs" array, e.g. {"slugs":["slug-1","slug-2"]}`),
				),
			),
			Handler: makeCollectionsAddProducts(bf),
		},
	}
}

func makeCollectionsList(bf BridgeFactory) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		bridge := bf(req)
		code, body, err := bridge.Call(ctx, "GET", "/v1/collections", nil, nil)
		return HandleBridgeResult(code, body, err)
	}
}

func makeCollectionsCreate(bf BridgeFactory) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		collectionJSON := req.GetString("collection_json", "")
		if collectionJSON == "" {
			return gomcp.NewToolResultError("collection_json is required"), nil
		}
		var payload json.RawMessage
		if err := json.Unmarshal([]byte(collectionJSON), &payload); err != nil {
			return gomcp.NewToolResultError(fmt.Sprintf("invalid JSON: %v", err)), nil
		}
		bridge := bf(req)
		code, body, err := bridge.Call(ctx, "POST", "/v1/collections", nil, payload)
		return HandleBridgeResult(code, body, err)
	}
}

func makeCollectionsAddProducts(bf BridgeFactory) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		collectionID := req.GetString("collection_id", "")
		if collectionID == "" {
			return gomcp.NewToolResultError("collection_id is required"), nil
		}
		productsJSON := req.GetString("products_json", "")
		if productsJSON == "" {
			return gomcp.NewToolResultError("products_json is required"), nil
		}
		var payload json.RawMessage
		if err := json.Unmarshal([]byte(productsJSON), &payload); err != nil {
			return gomcp.NewToolResultError(fmt.Sprintf("invalid JSON: %v", err)), nil
		}
		bridge := bf(req)
		path := fmt.Sprintf("/v1/collections/%s/products", url.PathEscape(collectionID))
		code, body, err := bridge.Call(ctx, "POST", path, nil, payload)
		return HandleBridgeResult(code, body, err)
	}
}
