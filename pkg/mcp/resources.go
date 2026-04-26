package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerResources(s *server.MCPServer, bridge Bridge) {
	s.AddResource(
		gomcp.NewResource(
			"mobazha://store/me/summary",
			"Store Summary",
			gomcp.WithResourceDescription("Overview of your store including profile, listing count, and key stats"),
			gomcp.WithMIMEType("application/json"),
		),
		makeStoreSummaryResource(bridge),
	)

	s.AddResource(
		gomcp.NewResource(
			"mobazha://store/me/listings",
			"My Listings",
			gomcp.WithResourceDescription("Complete list of your store's product listings"),
			gomcp.WithMIMEType("application/json"),
		),
		makeListingsResource(bridge),
	)

	s.AddResource(
		gomcp.NewResource(
			"mobazha://store/me/orders/recent",
			"Recent Orders",
			gomcp.WithResourceDescription("Your most recent sales and purchase orders"),
			gomcp.WithMIMEType("application/json"),
		),
		makeRecentOrdersResource(bridge),
	)

	s.AddResource(
		gomcp.NewResource(
			"mobazha://notifications/unread",
			"Unread Notifications",
			gomcp.WithResourceDescription("Count and list of unread store notifications"),
			gomcp.WithMIMEType("application/json"),
		),
		makeUnreadNotificationsResource(bridge),
	)
}

// bridgeGet calls Bridge.Call and checks both transport errors and HTTP status codes.
func bridgeGet(ctx context.Context, bridge Bridge, path, label string) ([]byte, error) {
	code, body, err := bridge.Call(ctx, "GET", path, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", label, err)
	}
	if code < 200 || code >= 300 {
		return nil, fmt.Errorf("%s: HTTP %d", label, code)
	}
	return body, nil
}

func makeStoreSummaryResource(bridge Bridge) server.ResourceHandlerFunc {
	return func(ctx context.Context, req gomcp.ReadResourceRequest) ([]gomcp.ResourceContents, error) {
		profileBody, err := bridgeGet(ctx, bridge, "/v1/profiles", "fetch profile")
		if err != nil {
			return nil, err
		}

		listingsBody, err := bridgeGet(ctx, bridge, "/v1/listings/index", "fetch listings")
		if err != nil {
			return nil, err
		}

		summary := map[string]json.RawMessage{
			"profile":  profileBody,
			"listings": listingsBody,
		}
		data, _ := json.Marshal(summary)

		return []gomcp.ResourceContents{
			gomcp.TextResourceContents{
				URI:      req.Params.URI,
				MIMEType: "application/json",
				Text:     string(data),
			},
		}, nil
	}
}

func makeListingsResource(bridge Bridge) server.ResourceHandlerFunc {
	return func(ctx context.Context, req gomcp.ReadResourceRequest) ([]gomcp.ResourceContents, error) {
		body, err := bridgeGet(ctx, bridge, "/v1/listings/index", "fetch listings")
		if err != nil {
			return nil, err
		}
		return []gomcp.ResourceContents{
			gomcp.TextResourceContents{
				URI:      req.Params.URI,
				MIMEType: "application/json",
				Text:     string(body),
			},
		}, nil
	}
}

func makeRecentOrdersResource(bridge Bridge) server.ResourceHandlerFunc {
	return func(ctx context.Context, req gomcp.ReadResourceRequest) ([]gomcp.ResourceContents, error) {
		salesBody, err := bridgeGet(ctx, bridge, "/v1/sales", "fetch sales")
		if err != nil {
			return nil, err
		}

		purchasesBody, err := bridgeGet(ctx, bridge, "/v1/purchases", "fetch purchases")
		if err != nil {
			return nil, err
		}

		orders := map[string]json.RawMessage{
			"sales":     salesBody,
			"purchases": purchasesBody,
		}
		data, _ := json.Marshal(orders)

		return []gomcp.ResourceContents{
			gomcp.TextResourceContents{
				URI:      req.Params.URI,
				MIMEType: "application/json",
				Text:     string(data),
			},
		}, nil
	}
}

func makeUnreadNotificationsResource(bridge Bridge) server.ResourceHandlerFunc {
	return func(ctx context.Context, req gomcp.ReadResourceRequest) ([]gomcp.ResourceContents, error) {
		body, err := bridgeGet(ctx, bridge, "/v1/notifications/count", "fetch notification count")
		if err != nil {
			return nil, err
		}
		return []gomcp.ResourceContents{
			gomcp.TextResourceContents{
				URI:      req.Params.URI,
				MIMEType: "application/json",
				Text:     string(body),
			},
		}, nil
	}
}
