package mcp

import (
	"context"
	"net/url"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// searchToolRegistrars returns tools that call the public Search API (mobazha.info).
// Unlike other tool registrars that take a BridgeFactory for per-request auth,
// search tools use a single pre-configured Bridge since the Search API is public.
func searchToolRegistrars(searchBridge Bridge) []ToolRegistrar {
	return []ToolRegistrar{
		{
			Name: "search_listings",
			Tool: gomcp.NewTool("search_listings",
				gomcp.WithDescription(
					"Search for products/listings across the Mobazha marketplace. "+
						"Returns matching listings with title, price, images, rating, and seller info. "+
						"Use when asked to find products, browse categories, or discover items for sale.",
				),
				gomcp.WithString("query",
					gomcp.Description("Search keywords (e.g., 'vintage watch', 'handmade soap'). Leave empty to browse all."),
				),
				gomcp.WithString("page",
					gomcp.Description("Page number for pagination (default: 1)."),
				),
				gomcp.WithString("pageSize",
					gomcp.Description("Results per page, max 100 (default: 20)."),
				),
				gomcp.WithString("sortBy",
					gomcp.Description("Sort order: 'newest', 'price-asc', 'price-desc', 'rating', 'relevance' (default: relevance when query provided, newest otherwise)."),
				),
			),
			Handler: makeSearchListings(searchBridge),
		},
		{
			Name: "search_profiles",
			Tool: gomcp.NewTool("search_profiles",
				gomcp.WithDescription(
					"Search for store profiles/sellers on the Mobazha marketplace. "+
						"Returns matching profiles with store name, description, rating, and listing count. "+
						"Use when asked to find sellers, stores, or vendors.",
				),
				gomcp.WithString("query",
					gomcp.Description("Search keywords for store name or description."),
				),
				gomcp.WithString("page",
					gomcp.Description("Page number for pagination (default: 1)."),
				),
				gomcp.WithString("pageSize",
					gomcp.Description("Results per page, max 100 (default: 20)."),
				),
			),
			Handler: makeSearchProfiles(searchBridge),
		},
	}
}

func makeSearchListings(searchBridge Bridge) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		q := url.Values{}
		if v := req.GetString("query", ""); v != "" {
			q.Set("q", v)
		}
		if v := req.GetString("page", ""); v != "" {
			q.Set("p", v)
		}
		if v := req.GetString("pageSize", ""); v != "" {
			q.Set("pageSize", v)
		}
		if v := req.GetString("sortBy", ""); v != "" {
			q.Set("sortBy", v)
		}

		code, body, err := searchBridge.Call(ctx, "GET", "/search/v1/listings", q, nil)
		return HandleBridgeResult(code, body, err)
	}
}

func makeSearchProfiles(searchBridge Bridge) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		q := url.Values{}
		if v := req.GetString("query", ""); v != "" {
			q.Set("q", v)
		}
		if v := req.GetString("page", ""); v != "" {
			q.Set("p", v)
		}
		if v := req.GetString("pageSize", ""); v != "" {
			q.Set("pageSize", v)
		}

		code, body, err := searchBridge.Call(ctx, "GET", "/search/v1/profiles", q, nil)
		return HandleBridgeResult(code, body, err)
	}
}
