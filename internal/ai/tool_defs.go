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
			Name:        "agent_skill_runs_create",
			Description: "Create a tenant-scoped agent skill run to track durable work for a business skill such as product.import. Use this before creating artifacts when no run exists yet.",
			Parameters:  mustJSON(`{"type":"object","properties":{"skillId":{"type":"string","description":"Skill ID, e.g. product.import"},"threadId":{"type":"string","description":"Current agent thread/session ID when known"},"turnId":{"type":"string","description":"Current agent turn ID when known"},"storeId":{"type":"string","description":"Current store/profile ID when known"},"status":{"type":"string","enum":["created","running","waiting_for_review","waiting_for_approval","completed","failed"],"description":"Initial skill run status; omit for created"},"input":{"type":"object","description":"Small structured task input or source summary"}},"required":["skillId"]}`),
		},
		{
			Name:        "agent_skill_runs_list",
			Description: "List tenant-scoped agent skill runs for the current workspace, optionally filtered by skill or status. Use this to resume import tasks and inspect durable agent work state.",
			Parameters:  mustJSON(`{"type":"object","properties":{"skillId":{"type":"string","description":"Optional skill ID filter, e.g. product.import"},"status":{"type":"string","enum":["created","running","waiting_for_review","waiting_for_approval","completed","failed"],"description":"Optional skill run status filter"},"limit":{"type":"integer","description":"Max items (default 20)"},"offset":{"type":"integer","description":"Pagination offset"}}}`),
		},
		{
			Name:        "agent_skill_runs_get",
			Description: "Get one tenant-scoped agent skill run by ID, including its status, input summary, output summary, and related task metadata.",
			Parameters:  mustJSON(`{"type":"object","properties":{"runId":{"type":"string","description":"Agent skill run ID"}},"required":["runId"]}`),
		},
		{
			Name:        "agent_skill_runs_update",
			Description: "Update a tenant-scoped agent skill run status, output summary, or error after producing artifacts. This only updates agent run metadata and does not write listing/business state.",
			Parameters:  mustJSON(`{"type":"object","properties":{"runId":{"type":"string","description":"Agent skill run ID"},"status":{"type":"string","enum":["created","running","waiting_for_review","waiting_for_approval","completed","failed"],"description":"Updated skill run status"},"output":{"type":"object","description":"Small structured output summary such as produced artifact IDs or counts"},"error":{"type":"string","description":"Short failure summary when status is failed"}},"required":["runId"]}`),
		},
		{
			Name:        "agent_product_import_ingest",
			Description: "Start a product.import run from explicit product import source materials and create reviewable source, candidate, proposal, and validation artifacts. Use only after the user asks to import or organize materials as product listings; this does not directly publish listings. For files attached to the current chat turn, pass the attachmentId/sourceName metadata only; the server injects the attachment payload. A successful ingest is a delivery point; do not call agent_product_import_advance unless resuming an existing run that explicitly requires another transition.",
			Parameters:  mustJSON(`{"type":"object","properties":{"threadId":{"type":"string","description":"Current agent thread/session ID when known"},"storeId":{"type":"string","description":"Current store/profile ID when known"},"language":{"type":"string","description":"Preferred extraction language; the agent chat runtime supplies the conversation language."},"sources":{"type":"array","description":"Product import source materials to stage for review. For current-turn attachments, include attachmentId or sourceName and omit large base64 payloads; the server will attach the file content.","items":{"type":"object","properties":{"attachmentId":{"type":"string","description":"Current-turn attachment ID from chat context, when importing an attached file"},"sourceName":{"type":"string","description":"Filename or human label for the source"},"contentType":{"type":"string","description":"MIME type or logical content type such as text/csv, image/jpeg, or application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"},"text":{"type":"string","description":"Plain text source content for pasted CSV, notes, or supplier data. Do not invent this for binary attachments."},"contentBase64":{"type":"string","description":"Base64-encoded binary source content only when explicitly available in tool arguments; do not reproduce chat attachment base64 manually."}},"required":["sourceName"]}}},"required":["sources"]}`),
		},
		{
			Name:        "agent_product_import_advance",
			Description: "Advance a product.import run: promote extracted candidate artifacts into reviewable proposals, add next actions for sources that still need AI extraction, optionally create approval requests, and refresh workbench state. After this tool returns, stop and summarize counts, proposal IDs, skipped IDs, or pending next actions to the user unless the user explicitly asks you to continue.",
			Parameters:  mustJSON(`{"type":"object","properties":{"runId":{"type":"string","description":"Product import skill run ID"},"sourceArtifactIds":{"type":"array","items":{"type":"string"},"description":"Optional source artifact IDs (art_...) to inspect for next actions"},"candidateArtifactIds":{"type":"array","items":{"type":"string"},"description":"Optional candidate artifact IDs (art_...) returned by agent_artifacts_create/list to promote into proposals. Do not pass candidate item IDs such as candidate-001."},"createApprovals":{"type":"boolean","description":"Create approval requests for newly created proposals"}},"required":["runId"]}`),
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
			Name:        "agent_attachments_analyze",
			Description: "Analyze a file attached to the current chat turn when visual understanding or fuller text is needed. For images, runs vision on demand with your question. For text-like attachments, returns the available excerpt from context. Use when the user asks about attachment contents, wants a description, or needs copy/listing suggestions from an image. For product.import ingest or review workflows, call agent_product_import_ingest instead.",
			Parameters:  mustJSON(`{"type":"object","properties":{"attachmentId":{"type":"string","description":"Attachment ID from the current turn context, when multiple files are attached"},"sourceName":{"type":"string","description":"Attachment filename when ID is unknown"},"question":{"type":"string","description":"Focused question about the attachment, in the user's language"},"language":{"type":"string","description":"Optional response language code such as en or zh"}},"required":["question"]}`),
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
