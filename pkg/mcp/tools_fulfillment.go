package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func fulfillmentToolRegistrars(bf BridgeFactory) []ToolRegistrar {
	return []ToolRegistrar{
		{
			Name: "sourcing_list_providers",
			Tool: gomcp.NewTool("sourcing_list_providers",
				gomcp.WithDescription(
					"List all fulfillment providers and their connection status (connected/disconnected). "+
						"Returns provider ID, name, status, and capabilities. "+
						"Use when asked about supply chain setup, connected suppliers, or fulfillment providers.",
				),
			),
			Handler: makeSourcingListProviders(bf),
		},
		{
			Name: "sourcing_browse_catalog",
			Tool: gomcp.NewTool("sourcing_browse_catalog",
				gomcp.WithDescription(
					"Browse the product catalog of a connected fulfillment provider. "+
						"Returns available products with pricing, variants, and images. "+
						"Use when asked to find products to sell, browse supplier catalog, or look for items to import.",
				),
				gomcp.WithString("provider_id",
					gomcp.Required(),
					gomcp.Description("The fulfillment provider ID (e.g. 'printful')"),
				),
				gomcp.WithString("offset",
					gomcp.Description("Pagination offset (default 0)"),
				),
				gomcp.WithString("limit",
					gomcp.Description("Number of results to return (default 20, max 100)"),
				),
			),
			Handler: makeSourcingBrowseCatalog(bf),
		},
		{
			Name: "sourcing_list_designs",
			Tool: gomcp.NewTool("sourcing_list_designs",
				gomcp.WithDescription(
					"List products designed in the supplier's dashboard (Sync Products). "+
						"These are POD products with custom designs ready to import. "+
						"Use when asked about designs, sync products, or products ready to import from the supplier.",
				),
				gomcp.WithString("provider_id",
					gomcp.Required(),
					gomcp.Description("The fulfillment provider ID (e.g. 'printful')"),
				),
			),
			Handler: makeSourcingListDesigns(bf),
		},
		{
			Name: "sourcing_import_product",
			Tool: gomcp.NewTool("sourcing_import_product",
				gomcp.WithDescription(
					"Import a product from a fulfillment provider into the store. "+
						"Requires provider ID, catalog product ID or sync product ID, selected variant IDs, and markup percentage. "+
						"Use when asked to import a product, add a supplier product to the store, or start selling a catalog item.",
				),
				gomcp.WithString("provider_id",
					gomcp.Required(),
					gomcp.Description("The fulfillment provider ID (e.g. 'printful')"),
				),
				gomcp.WithString("import_json",
					gomcp.Required(),
					gomcp.Description(
						"JSON object with import parameters: "+
							"{\"catalogProductId\":\"123\", \"variantIds\":[\"v1\",\"v2\"], \"retailMarkup\":50, "+
							"\"title\":\"My Product\", \"description\":\"...\", \"tags\":[\"tag1\"]}. "+
							"For design imports, use \"syncProductId\" instead of \"catalogProductId\".",
					),
				),
			),
			Handler: makeSourcingImportProduct(bf),
		},
		{
			Name: "sourcing_list_synced_products",
			Tool: gomcp.NewTool("sourcing_list_synced_products",
				gomcp.WithDescription(
					"List products already imported from fulfillment providers. "+
						"Returns synced products with their status (synced/pending/error), supplier cost, and retail price. "+
						"Use when asked about imported products, synced products, or supply chain product status.",
				),
				gomcp.WithString("provider_id",
					gomcp.Required(),
					gomcp.Description("The fulfillment provider ID (e.g. 'printful')"),
				),
			),
			Handler: makeSourcingListSyncedProducts(bf),
		},
		{
			Name: "sourcing_unlink_product",
			Tool: gomcp.NewTool("sourcing_unlink_product",
				gomcp.WithDescription(
					"Unlink (remove) a synced product mapping between a supplier product and a Mobazha listing. "+
						"This does NOT delete the listing itself, only removes the supplier association. "+
						"Use when asked to disconnect a product from its supplier, remove an import link, or clean up orphan mappings.",
				),
				gomcp.WithString("provider_id",
					gomcp.Required(),
					gomcp.Description("The fulfillment provider ID (e.g. 'printful')"),
				),
				gomcp.WithString("mapping_id",
					gomcp.Required(),
					gomcp.Description("The synced product mapping ID to remove"),
				),
			),
			Handler: makeSourcingUnlinkProduct(bf),
		},
		{
			Name: "sourcing_check_price_drift",
			Tool: gomcp.NewTool("sourcing_check_price_drift",
				gomcp.WithDescription(
					"Check for active supply chain alerts including price drift, stock changes, and rule actions. "+
						"Returns undismissed alerts with type, severity, and affected products. "+
						"Use when asked about price changes, cost drift, stock alerts, or supply chain issues.",
				),
			),
			Handler: makeSourcingCheckPriceDrift(bf),
		},
	}
}

func makeSourcingListProviders(bf BridgeFactory) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		bridge := bf(req)
		code, body, err := bridge.Call(ctx, "GET", "/v1/fulfillment/providers", nil, nil)
		return HandleBridgeResult(code, body, err)
	}
}

func makeSourcingBrowseCatalog(bf BridgeFactory) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		providerID := req.GetString("provider_id", "")
		if providerID == "" {
			return gomcp.NewToolResultError("provider_id is required"), nil
		}
		query := url.Values{}
		if v := req.GetString("offset", ""); v != "" {
			query.Set("offset", v)
		}
		if v := req.GetString("limit", ""); v != "" {
			query.Set("limit", v)
		}
		bridge := bf(req)
		path := fmt.Sprintf("/v1/fulfillment/%s/catalog", url.PathEscape(providerID))
		code, body, err := bridge.Call(ctx, "GET", path, query, nil)
		return HandleBridgeResult(code, body, err)
	}
}

func makeSourcingListDesigns(bf BridgeFactory) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		providerID := req.GetString("provider_id", "")
		if providerID == "" {
			return gomcp.NewToolResultError("provider_id is required"), nil
		}
		bridge := bf(req)
		path := fmt.Sprintf("/v1/fulfillment/%s/store-products", url.PathEscape(providerID))
		code, body, err := bridge.Call(ctx, "GET", path, nil, nil)
		return HandleBridgeResult(code, body, err)
	}
}

func makeSourcingImportProduct(bf BridgeFactory) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		providerID := req.GetString("provider_id", "")
		if providerID == "" {
			return gomcp.NewToolResultError("provider_id is required"), nil
		}
		importJSON := req.GetString("import_json", "")
		if importJSON == "" {
			return gomcp.NewToolResultError("import_json is required"), nil
		}
		var payload json.RawMessage
		if err := json.Unmarshal([]byte(importJSON), &payload); err != nil {
			return gomcp.NewToolResultError(fmt.Sprintf("invalid JSON: %v", err)), nil
		}
		bridge := bf(req)
		path := fmt.Sprintf("/v1/fulfillment/%s/import", url.PathEscape(providerID))
		code, body, err := bridge.Call(ctx, "POST", path, nil, payload)
		return HandleBridgeResult(code, body, err)
	}
}

func makeSourcingListSyncedProducts(bf BridgeFactory) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		providerID := req.GetString("provider_id", "")
		if providerID == "" {
			return gomcp.NewToolResultError("provider_id is required"), nil
		}
		bridge := bf(req)
		path := fmt.Sprintf("/v1/fulfillment/%s/synced-products", url.PathEscape(providerID))
		code, body, err := bridge.Call(ctx, "GET", path, nil, nil)
		return HandleBridgeResult(code, body, err)
	}
}

func makeSourcingUnlinkProduct(bf BridgeFactory) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		providerID := req.GetString("provider_id", "")
		if providerID == "" {
			return gomcp.NewToolResultError("provider_id is required"), nil
		}
		mappingID := req.GetString("mapping_id", "")
		if mappingID == "" {
			return gomcp.NewToolResultError("mapping_id is required"), nil
		}
		bridge := bf(req)
		path := fmt.Sprintf("/v1/fulfillment/%s/synced-products/%s",
			url.PathEscape(providerID), url.PathEscape(mappingID))
		code, body, err := bridge.Call(ctx, "DELETE", path, nil, nil)
		return HandleBridgeResult(code, body, err)
	}
}

func makeSourcingCheckPriceDrift(bf BridgeFactory) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		bridge := bf(req)
		query := url.Values{"dismissed": {"false"}}
		code, body, err := bridge.Call(ctx, "GET", "/v1/fulfillment/alerts", query, nil)
		return HandleBridgeResult(code, body, err)
	}
}
