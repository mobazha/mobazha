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
			Name:        "agent_artifacts_list",
			Description: "List tenant-scoped agent artifacts for the current workspace, optionally filtered by skill run, kind, or review status. Use this to resume or inspect intermediate work without reading raw chat history.",
			Parameters:  mustJSON(`{"type":"object","properties":{"skillRunId":{"type":"string","description":"Optional related skill run ID"},"kind":{"type":"string","enum":["source_material","candidate","proposal","validation_report"],"description":"Optional artifact category filter"},"status":{"type":"string","enum":["new","ready","needs_review","skipped"],"description":"Optional artifact review state filter"},"limit":{"type":"integer","description":"Max items (default 20)"},"offset":{"type":"integer","description":"Pagination offset"}}}`),
		},
		{
			Name:        "agent_artifacts_get",
			Description: "Get a single tenant-scoped agent artifact by ID, including its summary and structured payload.",
			Parameters:  mustJSON(`{"type":"object","properties":{"artifactId":{"type":"string","description":"Artifact ID"}},"required":["artifactId"]}`),
		},
		{
			Name:        "agent_artifacts_create",
			Description: "Create a tenant-scoped agent artifact for source material, extracted candidates, reviewable proposals, or validation notes. Use this to preserve intermediate work for user review without writing business state.",
			Parameters:  mustJSON(`{"type":"object","properties":{"threadId":{"type":"string","description":"Current agent thread/session ID when known"},"turnId":{"type":"string","description":"Current agent turn ID when known"},"skillRunId":{"type":"string","description":"Optional related skill run ID"},"skillId":{"type":"string","description":"Skill ID producing the artifact, e.g. product.import"},"kind":{"type":"string","enum":["source_material","candidate","proposal","validation_report"],"description":"Artifact category to create"},"status":{"type":"string","enum":["new","ready","needs_review","skipped"],"description":"Review state for the artifact; omit for the server default"},"name":{"type":"string","description":"Human-readable artifact name"},"contentType":{"type":"string","description":"MIME type or logical content type"},"sourceUri":{"type":"string","description":"Optional source URI or file reference"},"sourceName":{"type":"string","description":"Optional source filename or label"},"summary":{"type":"string","description":"Short human-readable summary"},"text":{"type":"string","description":"Plain text material to store as artifact data"},"metadata":{"type":"object","description":"Small structured metadata for source/candidate provenance"},"data":{"type":"object","description":"Structured artifact payload such as extracted candidates or review proposals"}}}`),
		},
		{
			Name:        "agent_artifacts_update",
			Description: "Update a tenant-scoped agent artifact's review status, name, summary, or structured payload. Use this to revise intermediate work; it does not write listing/business state.",
			Parameters:  mustJSON(`{"type":"object","properties":{"artifactId":{"type":"string","description":"Artifact ID returned by agent_artifacts_create or listed in context"},"status":{"type":"string","enum":["new","ready","needs_review","skipped"],"description":"Review state for the artifact"},"name":{"type":"string","description":"Updated human-readable artifact name"},"summary":{"type":"string","description":"Updated short human-readable summary"},"data":{"type":"object","description":"Updated structured artifact payload such as extracted candidates, review proposals, field sources, or validation notes"}},"required":["artifactId"]}`),
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
			Name:        "orders_decline",
			Description: "Decline a pending order.",
			Parameters:  mustJSON(`{"type":"object","properties":{"orderId":{"type":"string","description":"Order ID to decline"},"reason":{"type":"string","description":"Decline reason"}},"required":["orderId"]}`),
		},
		{
			Name:        "orders_ship",
			Description: "Mark an order as shipped.",
			Parameters:  mustJSON(`{"type":"object","properties":{"orderId":{"type":"string","description":"Order ID"},"shipper":{"type":"string","description":"Shipping carrier name"},"trackingNumber":{"type":"string","description":"Tracking number"},"note":{"type":"string","description":"Shipment note"}},"required":["orderId"]}`),
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
			Description: "List recent chat rooms.",
			Parameters:  mustJSON(`{"type":"object","properties":{}}`),
		},
		{
			Name:        "chat_get_messages",
			Description: "Get messages from a specific chat room.",
			Parameters:  mustJSON(`{"type":"object","properties":{"roomID":{"type":"string","description":"Chat room ID"},"limit":{"type":"integer","description":"Max messages (default 20)"},"before":{"type":"string","description":"Pagination token for older messages"},"after":{"type":"string","description":"Pagination token for newer messages"},"since":{"type":"string","description":"Backward-compat alias for before"}},"required":["roomID"]}`),
		},
		{
			Name:        "chat_send_message",
			Description: "Send a message to a chat room.",
			Parameters:  mustJSON(`{"type":"object","properties":{"roomID":{"type":"string","description":"Chat room ID"},"body":{"type":"string","description":"Message text"},"message":{"type":"string","description":"Backward-compat alias of body"}},"required":["roomID"]}`),
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
