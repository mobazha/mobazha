package mcp

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func listingsToolRegistrars(bf BridgeFactory) []ToolRegistrar {
	return []ToolRegistrar{
		{
			Name: "listings_import_json",
			Tool: gomcp.NewTool("listings_import_json",
				gomcp.WithDescription(
					"Batch import multiple product listings from a JSON payload. "+
						"Accepts a JSON object with listings array (and optional profile, shippingProfiles, collections). "+
						"Each listing needs: title, contractType (PHYSICAL_GOOD/DIGITAL_GOOD/SERVICE/CRYPTOCURRENCY), "+
						"price, pricingCurrency, images (filenames referencing entries in images_base64 map). "+
						"Returns import result: total, created, updated, failed counts and error details. "+
						"Use this for bulk product migration from Shopify/Amazon/CSV exports instead of calling listings_create repeatedly.",
				),
				gomcp.WithString("import_json",
					gomcp.Required(),
					gomcp.Description(
						"JSON object following the import schema: "+
							"{\"listings\":[{\"title\":\"...\",\"contractType\":\"DIGITAL_GOOD\",\"price\":\"9.99\","+
							"\"pricingCurrency\":\"USD\",\"description\":\"...\",\"tags\":[\"...\"],\"images\":[\"photo1.jpg\"]}], "+
							"\"shippingProfiles\":[...], \"collections\":[...], \"profile\":{...}}",
					),
				),
				gomcp.WithString("images_base64",
					gomcp.Description(
						"Optional JSON object mapping image filenames to base64-encoded image data. "+
							"Keys must match filenames in listings[].images arrays. "+
							"Example: {\"photo1.jpg\":\"<base64>\",\"photo2.png\":\"<base64>\"}. "+
							"Omit if listings don't require images (e.g., digital goods with external links).",
					),
				),
			),
			Handler: makeListingsImportJSON(bf),
		},
		{
			Name: "listings_list_mine",
			Tool: gomcp.NewTool("listings_list_mine",
				gomcp.WithDescription(
					"List the seller's own product listings with title, price, quantity, and status. "+
						"Returns an array of listing summaries including slug, title, price, currency, quantity, and creation date. "+
						"Use when asked about inventory, what products are in the store, how many items are listed, or stock levels.",
				),
			),
			Handler: makeListingsListMine(bf),
		},
		{
			Name: "listings_get",
			Tool: gomcp.NewTool("listings_get",
				gomcp.WithDescription(
					"Get detailed information about a specific product listing by slug or CID. "+
						"Returns full listing data including title, description, price, images, variants, shipping options, and metadata. "+
						"Use when asked about a specific product's details, pricing, description, or configuration.",
				),
				gomcp.WithString("slug_or_cid",
					gomcp.Required(),
					gomcp.Description("The listing slug (URL-friendly name) or IPFS CID"),
				),
			),
			Handler: makeListingsGet(bf),
		},
		{
			Name: "listings_get_template",
			Tool: gomcp.NewTool("listings_get_template",
				gomcp.WithDescription(
					"Get the JSON template/schema for creating a new product listing. "+
						"Returns all available fields with their default values. "+
						"Always call this before listings_create to understand the required structure.",
				),
			),
			Handler: makeListingsGetTemplate(bf),
		},
		{
			Name: "listings_create",
			Tool: gomcp.NewTool("listings_create",
				gomcp.WithDescription(
					"Create a new product listing in the store. "+
						"First call listings_get_template to see the required JSON structure, then fill in the fields. "+
						"The listing_json parameter should be a complete listing JSON object.",
				),
				gomcp.WithString("listing_json",
					gomcp.Required(),
					gomcp.Description("Complete listing JSON object (use listings_get_template to see the structure)"),
				),
			),
			Handler: makeListingsCreate(bf),
		},
		{
			Name: "listings_update",
			Tool: gomcp.NewTool("listings_update",
				gomcp.WithDescription(
					"Update an existing product listing. "+
						"Pass the complete listing JSON with modifications. Use listings_get first to retrieve the current data.",
				),
				gomcp.WithString("listing_json",
					gomcp.Required(),
					gomcp.Description("Complete listing JSON object with updates"),
				),
			),
			Handler: makeListingsUpdate(bf),
		},
		{
			Name: "listings_delete",
			Tool: gomcp.NewTool("listings_delete",
				gomcp.WithDescription(
					"Delete a product listing permanently. [DESTRUCTIVE] This action cannot be undone. "+
						"The AI client MUST confirm with the user before calling this tool.",
				),
				gomcp.WithString("slug",
					gomcp.Required(),
					gomcp.Description("The listing slug to delete"),
				),
			),
			Handler: makeListingsDelete(bf),
		},
	}
}

func makeListingsListMine(bf BridgeFactory) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		bridge := bf(req)
		code, body, err := bridge.Call(ctx, "GET", "/v1/listings/index", nil, nil)
		return HandleBridgeResult(code, body, err)
	}
}

func makeListingsGet(bf BridgeFactory) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		slugOrCID := req.GetString("slug_or_cid", "")
		if slugOrCID == "" {
			return gomcp.NewToolResultError("slug_or_cid is required"), nil
		}
		bridge := bf(req)
		path := fmt.Sprintf("/v1/listings/mine/%s", url.PathEscape(slugOrCID))
		code, body, err := bridge.Call(ctx, "GET", path, nil, nil)
		return HandleBridgeResult(code, body, err)
	}
}

func makeListingsGetTemplate(bf BridgeFactory) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		bridge := bf(req)
		code, body, err := bridge.Call(ctx, "GET", "/v1/listings/template", nil, nil)
		return HandleBridgeResult(code, body, err)
	}
}

func makeListingsCreate(bf BridgeFactory) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		listingJSON := req.GetString("listing_json", "")
		if listingJSON == "" {
			return gomcp.NewToolResultError("listing_json is required"), nil
		}
		var payload json.RawMessage
		if err := json.Unmarshal([]byte(listingJSON), &payload); err != nil {
			return gomcp.NewToolResultError(fmt.Sprintf("invalid JSON: %v", err)), nil
		}
		bridge := bf(req)
		code, body, err := bridge.Call(ctx, "POST", "/v1/listings", nil, payload)
		return HandleBridgeResult(code, body, err)
	}
}

func makeListingsUpdate(bf BridgeFactory) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		listingJSON := req.GetString("listing_json", "")
		if listingJSON == "" {
			return gomcp.NewToolResultError("listing_json is required"), nil
		}
		var payload json.RawMessage
		if err := json.Unmarshal([]byte(listingJSON), &payload); err != nil {
			return gomcp.NewToolResultError(fmt.Sprintf("invalid JSON: %v", err)), nil
		}
		bridge := bf(req)
		code, body, err := bridge.Call(ctx, "PUT", "/v1/listings", nil, payload)
		return HandleBridgeResult(code, body, err)
	}
}

func makeListingsDelete(bf BridgeFactory) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		slug := req.GetString("slug", "")
		if slug == "" {
			return gomcp.NewToolResultError("slug is required"), nil
		}
		bridge := bf(req)
		path := fmt.Sprintf("/v1/listings/%s", url.PathEscape(slug))
		code, body, err := bridge.Call(ctx, "DELETE", path, nil, nil)
		return HandleBridgeResult(code, body, err)
	}
}

func makeListingsImportJSON(bf BridgeFactory) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		importJSON := req.GetString("import_json", "")
		if importJSON == "" {
			return gomcp.NewToolResultError("import_json is required"), nil
		}

		var payload json.RawMessage
		if err := json.Unmarshal([]byte(importJSON), &payload); err != nil {
			return gomcp.NewToolResultError(fmt.Sprintf("invalid JSON: %v", err)), nil
		}

		imageFiles := make(map[string][]byte)
		if imagesB64 := req.GetString("images_base64", ""); imagesB64 != "" {
			var imagesMap map[string]string
			if err := json.Unmarshal([]byte(imagesB64), &imagesMap); err != nil {
				return gomcp.NewToolResultError(fmt.Sprintf("invalid images_base64 JSON: %v", err)), nil
			}
			for name, b64 := range imagesMap {
				data, err := base64.StdEncoding.DecodeString(b64)
				if err != nil {
					return gomcp.NewToolResultError(fmt.Sprintf("invalid base64 for image %q: %v", name, err)), nil
				}
				imageFiles[name] = data
			}
		}

		zipData, err := buildImportZIP(payload, imageFiles)
		if err != nil {
			return gomcp.NewToolResultError(fmt.Sprintf("failed to build import ZIP: %v", err)), nil
		}

		bridge := bf(req)
		code, body, err := bridge.CallMultipart(ctx, "POST", "/v1/listings/import/json", "file", "import.zip", zipData)
		return HandleBridgeResult(code, body, err)
	}
}

// buildImportZIP creates an in-memory ZIP archive containing listings.json
// and optional image files under the images/ directory.
func buildImportZIP(jsonPayload json.RawMessage, imageFiles map[string][]byte) ([]byte, error) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	f, err := w.Create("listings.json")
	if err != nil {
		return nil, fmt.Errorf("create zip entry: %w", err)
	}
	if _, err := f.Write(jsonPayload); err != nil {
		return nil, fmt.Errorf("write json to zip: %w", err)
	}

	for name, data := range imageFiles {
		imgEntry, err := w.Create("images/" + name)
		if err != nil {
			return nil, fmt.Errorf("create image entry %q: %w", name, err)
		}
		if _, err := imgEntry.Write(data); err != nil {
			return nil, fmt.Errorf("write image %q: %w", name, err)
		}
	}

	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("close zip: %w", err)
	}
	return buf.Bytes(), nil
}
