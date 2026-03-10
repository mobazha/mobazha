package ai

import "encoding/json"

// SellerTools returns tool definitions available to the seller AI assistant.
// These mirror the MCP Server tools but in LLM function-calling format.
func SellerTools() []ToolDefinition {
	return []ToolDefinition{
		{
			Name:        "listings_list_mine",
			Description: "List the seller's own products with pagination. Returns product titles, slugs, prices, and status.",
			Parameters:  mustJSON(`{"type":"object","properties":{"limit":{"type":"integer","description":"Max items (default 20)"},"offset":{"type":"integer","description":"Pagination offset"}}}`),
		},
		{
			Name:        "listings_get",
			Description: "Get detailed information about a specific product by its slug.",
			Parameters:  mustJSON(`{"type":"object","properties":{"slug":{"type":"string","description":"Product slug identifier"}},"required":["slug"]}`),
		},
		{
			Name:        "listings_get_template",
			Description: "Get an empty listing template showing all available fields and their structure. Useful before creating a new product.",
			Parameters:  mustJSON(`{"type":"object","properties":{}}`),
		},
		{
			Name:        "listings_create",
			Description: "Create a new product listing. The listing JSON should follow the template structure.",
			Parameters:  mustJSON(`{"type":"object","properties":{"listing":{"type":"object","description":"Complete listing JSON following the template structure"}},"required":["listing"]}`),
		},
		{
			Name:        "listings_update",
			Description: "Update an existing product listing by slug.",
			Parameters:  mustJSON(`{"type":"object","properties":{"slug":{"type":"string","description":"Product slug"},"listing":{"type":"object","description":"Updated listing JSON"}},"required":["slug","listing"]}`),
		},
		{
			Name:        "listings_delete",
			Description: "[DESTRUCTIVE] Delete a product listing permanently. Cannot be undone.",
			Parameters:  mustJSON(`{"type":"object","properties":{"slug":{"type":"string","description":"Product slug to delete"}},"required":["slug"]}`),
		},
		{
			Name:        "orders_get_sales",
			Description: "Get the seller's incoming orders (sales). Returns order IDs, statuses, buyers, and amounts.",
			Parameters:  mustJSON(`{"type":"object","properties":{"limit":{"type":"integer","description":"Max items (default 20)"},"offset":{"type":"integer","description":"Pagination offset"}}}`),
		},
		{
			Name:        "orders_get_detail",
			Description: "Get full details of a specific order by order ID.",
			Parameters:  mustJSON(`{"type":"object","properties":{"orderId":{"type":"string","description":"Order ID"}},"required":["orderId"]}`),
		},
		{
			Name:        "orders_confirm",
			Description: "Confirm (accept) a pending order.",
			Parameters:  mustJSON(`{"type":"object","properties":{"orderId":{"type":"string","description":"Order ID to confirm"}},"required":["orderId"]}`),
		},
		{
			Name:        "orders_reject",
			Description: "Reject (decline) a pending order.",
			Parameters:  mustJSON(`{"type":"object","properties":{"orderId":{"type":"string","description":"Order ID to reject"},"reason":{"type":"string","description":"Rejection reason"}},"required":["orderId"]}`),
		},
		{
			Name:        "orders_fulfill",
			Description: "Mark an order as shipped/fulfilled.",
			Parameters:  mustJSON(`{"type":"object","properties":{"orderId":{"type":"string","description":"Order ID"},"shipper":{"type":"string","description":"Shipping carrier name"},"trackingNumber":{"type":"string","description":"Tracking number"},"note":{"type":"string","description":"Fulfillment note"}},"required":["orderId"]}`),
		},
		{
			Name:        "orders_refund",
			Description: "[FINANCIAL] Issue a refund for an order. This operation involves fund transfer.",
			Parameters:  mustJSON(`{"type":"object","properties":{"orderId":{"type":"string","description":"Order ID to refund"}},"required":["orderId"]}`),
		},
		{
			Name:        "orders_complete",
			Description: "Mark an order as completed (after delivery confirmation).",
			Parameters:  mustJSON(`{"type":"object","properties":{"orderId":{"type":"string","description":"Order ID to complete"}},"required":["orderId"]}`),
		},
		{
			Name:        "profile_get",
			Description: "Get the seller's store profile (name, description, avatar, etc).",
			Parameters:  mustJSON(`{"type":"object","properties":{}}`),
		},
		{
			Name:        "profile_update",
			Description: "Update the seller's store profile.",
			Parameters:  mustJSON(`{"type":"object","properties":{"profile":{"type":"object","description":"Profile fields to update"}},"required":["profile"]}`),
		},
		{
			Name:        "chat_get_conversations",
			Description: "List recent chat conversations with buyers.",
			Parameters:  mustJSON(`{"type":"object","properties":{}}`),
		},
		{
			Name:        "chat_get_messages",
			Description: "Get messages from a specific chat conversation.",
			Parameters:  mustJSON(`{"type":"object","properties":{"peerID":{"type":"string","description":"Buyer's peer ID"},"limit":{"type":"integer","description":"Max messages (default 20)"},"offsetId":{"type":"string","description":"Message ID for pagination"}},"required":["peerID"]}`),
		},
		{
			Name:        "chat_send_message",
			Description: "Send a message to a buyer.",
			Parameters:  mustJSON(`{"type":"object","properties":{"peerID":{"type":"string","description":"Buyer's peer ID"},"message":{"type":"string","description":"Message text"},"subject":{"type":"string","description":"Message subject"}},"required":["peerID","message"]}`),
		},
		{
			Name:        "notifications_list",
			Description: "List recent notifications.",
			Parameters:  mustJSON(`{"type":"object","properties":{"limit":{"type":"integer","description":"Max items (default 20)"},"offset":{"type":"integer","description":"Pagination offset"}}}`),
		},
		{
			Name:        "exchange_rates_get",
			Description: "Get current exchange rates for all supported currencies.",
			Parameters:  mustJSON(`{"type":"object","properties":{}}`),
		},
		{
			Name:        "discounts_list",
			Description: "List all discount codes for the store.",
			Parameters:  mustJSON(`{"type":"object","properties":{}}`),
		},
		{
			Name:        "discounts_create",
			Description: "Create a new discount code.",
			Parameters:  mustJSON(`{"type":"object","properties":{"discount":{"type":"object","description":"Discount definition (code, type, value, etc)"}},"required":["discount"]}`),
		},
		{
			Name:        "discounts_update",
			Description: "Update an existing discount code.",
			Parameters:  mustJSON(`{"type":"object","properties":{"discountId":{"type":"string","description":"Discount ID"},"discount":{"type":"object","description":"Updated discount fields"}},"required":["discountId","discount"]}`),
		},
		{
			Name:        "discounts_delete",
			Description: "Delete a discount code.",
			Parameters:  mustJSON(`{"type":"object","properties":{"discountId":{"type":"string","description":"Discount ID to delete"}},"required":["discountId"]}`),
		},
		{
			Name:        "collections_list",
			Description: "List all product collections.",
			Parameters:  mustJSON(`{"type":"object","properties":{}}`),
		},
		{
			Name:        "collections_create",
			Description: "Create a new product collection.",
			Parameters:  mustJSON(`{"type":"object","properties":{"collection":{"type":"object","description":"Collection definition (name, description)"}},"required":["collection"]}`),
		},
	}
}

func mustJSON(s string) json.RawMessage {
	if !json.Valid([]byte(s)) {
		panic("invalid JSON in tool definition: " + s)
	}
	return json.RawMessage(s)
}
