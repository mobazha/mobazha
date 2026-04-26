package mcp

import (
	"context"
	"fmt"
	"net/url"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func notificationsToolRegistrars(bf BridgeFactory) []ToolRegistrar {
	return []ToolRegistrar{
		{
			Name: "notifications_list",
			Tool: gomcp.NewTool("notifications_list",
				gomcp.WithDescription(
					"List store notifications including order events, payment confirmations, and system alerts. "+
						"Returns notification type, message, read status, and timestamp. "+
						"Use when asked about recent activity, what happened, or checking for new events.",
				),
				gomcp.WithString("limit",
					gomcp.Description("Maximum number of notifications to return (default: 20)"),
				),
				gomcp.WithString("offset",
					gomcp.Description("Number of notifications to skip for pagination"),
				),
			),
			Handler: makeNotificationsList(bf),
		},
		{
			Name: "notifications_unread_count",
			Tool: gomcp.NewTool("notifications_unread_count",
				gomcp.WithDescription(
					"Get the count of unread notifications. "+
						"Returns a single number representing how many unread notifications exist. "+
						"Use when asked how many new notifications there are or if anything needs attention.",
				),
			),
			Handler: makeNotificationsUnreadCount(bf),
		},
		{
			Name: "notifications_mark_read",
			Tool: gomcp.NewTool("notifications_mark_read",
				gomcp.WithDescription(
					"Mark a notification as read, or mark all notifications as read. "+
						"If notification_id is provided, marks that single notification. "+
						"If omitted, marks all notifications as read.",
				),
				gomcp.WithString("notification_id",
					gomcp.Description("The notification ID to mark as read. Omit to mark all as read."),
				),
			),
			Handler: makeNotificationsMarkRead(bf),
		},
	}
}

func makeNotificationsList(bf BridgeFactory) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		query := url.Values{}
		if v := req.GetString("limit", ""); v != "" {
			query.Set("limit", v)
		}
		if v := req.GetString("offset", ""); v != "" {
			query.Set("offset", v)
		}
		bridge := bf(req)
		code, body, err := bridge.Call(ctx, "GET", "/v1/notifications", query, nil)
		return HandleBridgeResult(code, body, err)
	}
}

func makeNotificationsUnreadCount(bf BridgeFactory) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		bridge := bf(req)
		code, body, err := bridge.Call(ctx, "GET", "/v1/notifications/count", nil, nil)
		return HandleBridgeResult(code, body, err)
	}
}

func makeNotificationsMarkRead(bf BridgeFactory) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		bridge := bf(req)
		notifID := req.GetString("notification_id", "")
		if notifID != "" {
			path := fmt.Sprintf("/v1/notifications/%s/read", url.PathEscape(notifID))
			code, body, err := bridge.Call(ctx, "POST", path, nil, nil)
			return HandleBridgeResult(code, body, err)
		}
		code, body, err := bridge.Call(ctx, "POST", "/v1/notifications/read", nil, nil)
		return HandleBridgeResult(code, body, err)
	}
}
