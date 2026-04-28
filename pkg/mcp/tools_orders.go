package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func ordersToolRegistrars(bf BridgeFactory) []ToolRegistrar {
	return []ToolRegistrar{
		{
			Name: "orders_get_sales",
			Tool: gomcp.NewTool("orders_get_sales",
				gomcp.WithDescription(
					"List seller's sales orders with buyer info, payment status, and shipping state. "+
						"Returns order ID, buyer name, items, total amount, order state, and timestamps. "+
						"Use when asked about recent sales, order status, revenue, or shipping progress.",
				),
				gomcp.WithString("limit",
					gomcp.Description("Maximum number of orders to return (default: 20)"),
				),
				gomcp.WithString("offset",
					gomcp.Description("Number of orders to skip for pagination"),
				),
			),
			Handler: makeSalesHandler(bf),
		},
		{
			Name: "orders_get_purchases",
			Tool: gomcp.NewTool("orders_get_purchases",
				gomcp.WithDescription(
					"List buyer's purchase orders with seller info, payment status, and delivery tracking. "+
						"Returns order ID, seller name, items, total amount, order state, and timestamps. "+
						"Use when asked about purchases, things bought, or order tracking as a buyer.",
				),
				gomcp.WithString("limit",
					gomcp.Description("Maximum number of orders to return (default: 20)"),
				),
				gomcp.WithString("offset",
					gomcp.Description("Number of orders to skip for pagination"),
				),
			),
			Handler: makePurchasesHandler(bf),
		},
		{
			Name: "orders_get_detail",
			Tool: gomcp.NewTool("orders_get_detail",
				gomcp.WithDescription(
					"Get full details of a specific order by order ID, including items, payment info, "+
						"shipping address, dispute status, and complete transaction history. "+
						"Use when asked about a specific order's details or to investigate an order issue.",
				),
				gomcp.WithString("order_id",
					gomcp.Required(),
					gomcp.Description("The order ID (e.g., QmOrderHash...)"),
				),
			),
			Handler: makeOrderDetailHandler(bf),
		},
		{
			Name: "orders_confirm",
			Tool: gomcp.NewTool("orders_confirm",
				gomcp.WithDescription(
					"Confirm a seller's acceptance of a buyer's order. "+
						"Requires: order must be in PENDING state (newly placed, awaiting seller confirmation). "+
						"After: order moves to AWAITING_SHIPMENT, and on-chain payment begins processing. "+
						"Use when a new order comes in and the seller wants to accept it.",
				),
				gomcp.WithString("order_id",
					gomcp.Required(),
					gomcp.Description("The order ID to confirm"),
				),
			),
			Handler: makeOrderConfirm(bf),
		},
		{
			Name: "orders_decline",
			Tool: gomcp.NewTool("orders_decline",
				gomcp.WithDescription(
					"Decline a buyer's order as a seller. "+
						"Requires: order must be in PENDING state. "+
						"After: order is cancelled and any held funds are released back to the buyer. "+
						"Use when the seller cannot or does not want to ship or complete an order.",
				),
				gomcp.WithString("order_id",
					gomcp.Required(),
					gomcp.Description("The order ID to decline"),
				),
			),
			Handler: makeOrderDecline(bf),
		},
		{
			Name: "orders_ship",
			Tool: gomcp.NewTool("orders_ship",
				gomcp.WithDescription(
					"Mark an order as shipped by the seller. "+
						"Requires: order must be in AWAITING_SHIPMENT state (confirmed and payment processed). "+
						"After: order moves to SHIPPED state, buyer is notified. "+
						"For physical goods, include shipping carrier and tracking number.",
				),
				gomcp.WithString("order_id",
					gomcp.Required(),
					gomcp.Description("The order ID to ship"),
				),
				gomcp.WithString("note",
					gomcp.Description("Optional shipment note to the buyer"),
				),
				gomcp.WithString("shipper",
					gomcp.Description("Shipping carrier name (e.g., 'UPS', 'FedEx', 'USPS')"),
				),
				gomcp.WithString("tracking_number",
					gomcp.Description("Shipment tracking number"),
				),
			),
			Handler: makeOrderShip(bf),
		},
		{
			Name: "orders_refund",
			Tool: gomcp.NewTool("orders_refund",
				gomcp.WithDescription(
					"Refund an order's payment back to the buyer. [FINANCIAL] "+
						"Requires: order must be in FUNDED or AWAITING_SHIPMENT state. "+
						"After: funds are returned to buyer on-chain. "+
						"The AI client MUST confirm with the user before calling this tool.",
				),
				gomcp.WithString("order_id",
					gomcp.Required(),
					gomcp.Description("The order ID to refund"),
				),
			),
			Handler: makeOrderRefund(bf),
		},
		{
			Name: "orders_cancel",
			Tool: gomcp.NewTool("orders_cancel",
				gomcp.WithDescription(
					"Cancel an order as a buyer. [FINANCIAL] "+
						"Requires: order must be in a cancellable state (e.g., AWAITING_SHIPMENT for direct payment). "+
						"After: order is cancelled and escrowed funds are returned to the buyer on-chain. "+
						"The AI client MUST confirm with the user before calling this tool.",
				),
				gomcp.WithString("order_id",
					gomcp.Required(),
					gomcp.Description("The order ID to cancel"),
				),
			),
			Handler: makeOrderCancel(bf),
		},
		{
			Name: "orders_get_cases",
			Tool: gomcp.NewTool("orders_get_cases",
				gomcp.WithDescription(
					"List dispute cases where the current user is involved as buyer, seller, or moderator. "+
						"Returns case ID, order ID, participants, dispute reason, and resolution status. "+
						"Use when asked about open disputes, arbitration cases, or unresolved order issues.",
				),
				gomcp.WithString("limit", gomcp.Description("Max results (default 20)")),
				gomcp.WithString("offset", gomcp.Description("Pagination offset")),
			),
			Handler: makeCasesHandler(bf),
		},
		{
			Name: "orders_complete",
			Tool: gomcp.NewTool("orders_complete",
				gomcp.WithDescription(
					"Complete an order, releasing escrow funds to the seller. "+
						"Requires: order must be in SHIPPED state (buyer action). "+
						"After: payment is finalized and released from escrow to the seller. "+
						"This is typically a buyer action after receiving their goods.",
				),
				gomcp.WithString("order_id",
					gomcp.Required(),
					gomcp.Description("The order ID to complete"),
				),
			),
			Handler: makeOrderComplete(bf),
		},
	}
}

func makeSalesHandler(bf BridgeFactory) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		query := url.Values{}
		if v := req.GetString("limit", ""); v != "" {
			query.Set("limit", v)
		}
		if v := req.GetString("offset", ""); v != "" {
			query.Set("offset", v)
		}
		bridge := bf(req)
		code, body, err := bridge.Call(ctx, "GET", "/v1/sales", query, nil)
		return HandleBridgeResult(code, body, err)
	}
}

func makePurchasesHandler(bf BridgeFactory) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		query := url.Values{}
		if v := req.GetString("limit", ""); v != "" {
			query.Set("limit", v)
		}
		if v := req.GetString("offset", ""); v != "" {
			query.Set("offset", v)
		}
		bridge := bf(req)
		code, body, err := bridge.Call(ctx, "GET", "/v1/purchases", query, nil)
		return HandleBridgeResult(code, body, err)
	}
}

func makeOrderDetailHandler(bf BridgeFactory) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		orderID := req.GetString("order_id", "")
		if orderID == "" {
			return gomcp.NewToolResultError("order_id is required"), nil
		}
		bridge := bf(req)
		path := fmt.Sprintf("/v1/orders/%s", url.PathEscape(orderID))
		code, body, err := bridge.Call(ctx, "GET", path, nil, nil)
		return HandleBridgeResult(code, body, err)
	}
}

func makeOrderConfirm(bf BridgeFactory) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		orderID := req.GetString("order_id", "")
		if orderID == "" {
			return gomcp.NewToolResultError("order_id is required"), nil
		}
		bridge := bf(req)
		path := fmt.Sprintf("/v1/orders/%s/confirm", url.PathEscape(orderID))
		payload := map[string]interface{}{"decline": false}
		code, body, err := bridge.Call(ctx, "POST", path, nil, payload)
		return HandleBridgeResult(code, body, err)
	}
}

func makeOrderDecline(bf BridgeFactory) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		orderID := req.GetString("order_id", "")
		if orderID == "" {
			return gomcp.NewToolResultError("order_id is required"), nil
		}
		bridge := bf(req)
		path := fmt.Sprintf("/v1/orders/%s/confirm", url.PathEscape(orderID))
		payload := map[string]interface{}{"decline": true}
		code, body, err := bridge.Call(ctx, "POST", path, nil, payload)
		return HandleBridgeResult(code, body, err)
	}
}

func makeOrderShip(bf BridgeFactory) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		orderID := req.GetString("order_id", "")
		if orderID == "" {
			return gomcp.NewToolResultError("order_id is required"), nil
		}
		bridge := bf(req)

		payload := map[string]interface{}{}
		if note := req.GetString("note", ""); note != "" {
			payload["note"] = note
		}
		shipper := req.GetString("shipper", "")
		tracking := req.GetString("tracking_number", "")
		if shipper != "" || tracking != "" {
			payload["physicalDelivery"] = map[string]string{
				"shipper":        shipper,
				"trackingNumber": tracking,
			}
		}

		path := fmt.Sprintf("/v1/orders/%s/ship", url.PathEscape(orderID))
		code, body, err := bridge.Call(ctx, "POST", path, nil, payload)
		return HandleBridgeResult(code, body, err)
	}
}

func makeOrderRefund(bf BridgeFactory) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		orderID := req.GetString("order_id", "")
		if orderID == "" {
			return gomcp.NewToolResultError("order_id is required"), nil
		}
		bridge := bf(req)
		path := fmt.Sprintf("/v1/orders/%s/refund", url.PathEscape(orderID))
		code, body, err := bridge.Call(ctx, "POST", path, nil, json.RawMessage("{}"))
		return HandleBridgeResult(code, body, err)
	}
}

func makeCasesHandler(bf BridgeFactory) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		query := url.Values{}
		if v := req.GetString("limit", ""); v != "" {
			query.Set("limit", v)
		}
		if v := req.GetString("offset", ""); v != "" {
			query.Set("offset", v)
		}
		bridge := bf(req)
		code, body, err := bridge.Call(ctx, "GET", "/v1/cases", query, nil)
		return HandleBridgeResult(code, body, err)
	}
}

func makeOrderCancel(bf BridgeFactory) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		orderID := req.GetString("order_id", "")
		if orderID == "" {
			return gomcp.NewToolResultError("order_id is required"), nil
		}
		bridge := bf(req)
		path := fmt.Sprintf("/v1/orders/%s/cancel", url.PathEscape(orderID))
		code, body, err := bridge.Call(ctx, "POST", path, nil, json.RawMessage("{}"))
		return HandleBridgeResult(code, body, err)
	}
}

func makeOrderComplete(bf BridgeFactory) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		orderID := req.GetString("order_id", "")
		if orderID == "" {
			return gomcp.NewToolResultError("order_id is required"), nil
		}
		bridge := bf(req)
		path := fmt.Sprintf("/v1/orders/%s/complete", url.PathEscape(orderID))
		code, body, err := bridge.Call(ctx, "POST", path, nil, json.RawMessage("{}"))
		return HandleBridgeResult(code, body, err)
	}
}
