package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func chatToolRegistrars(bf BridgeFactory) []ToolRegistrar {
	return []ToolRegistrar{
		{
			Name: "chat_get_conversations",
			Tool: gomcp.NewTool("chat_get_conversations",
				gomcp.WithDescription(
					"List Matrix chat rooms with buyers and sellers. "+
						"Returns room previews including members, last message, unread count, and timestamp. "+
						"Use when asked about messages, buyer inquiries, customer communications, or unread chats.",
				),
			),
			Handler: makeChatConversations(bf),
		},
		{
			Name: "chat_get_messages",
			Tool: gomcp.NewTool("chat_get_messages",
				gomcp.WithDescription(
					"Get Matrix chat messages for a specific room or peer. "+
						"When peer_id is provided, this tool resolves/creates the DM room first, then fetches room messages. "+
						"Use when asked to read a conversation, see what a buyer/seller said, or review chat history.",
				),
				gomcp.WithString("room_id",
					gomcp.Description("Matrix room ID (preferred). If omitted, peer_id is used to resolve a DM room."),
				),
				gomcp.WithString("peer_id",
					gomcp.Description("Peer ID of the conversation partner (legacy input; resolves to a Matrix DM room)"),
				),
				gomcp.WithString("limit",
					gomcp.Description("Maximum number of messages to return"),
				),
				gomcp.WithString("before",
					gomcp.Description("Pagination cursor: fetch messages before this event"),
				),
				gomcp.WithString("after",
					gomcp.Description("Pagination cursor: fetch messages after this event"),
				),
				gomcp.WithString("offset",
					gomcp.Description("Legacy pagination field; forwarded for compatibility"),
				),
			),
			Handler: makeChatGetMessages(bf),
		},
		{
			Name: "chat_send_message",
			Tool: gomcp.NewTool("chat_send_message",
				gomcp.WithDescription(
					"Send a Matrix chat message to a room or peer. "+
						"When peer_id is provided, this tool resolves/creates the DM room first.",
				),
				gomcp.WithString("room_id",
					gomcp.Description("Matrix room ID (preferred). If omitted, peer_id is used to resolve a DM room."),
				),
				gomcp.WithString("peer_id",
					gomcp.Description("Peer ID to send the message to (legacy input; resolves to a Matrix DM room)"),
				),
				gomcp.WithString("message",
					gomcp.Required(),
					gomcp.Description("The message text to send"),
				),
				gomcp.WithString("order_id",
					gomcp.Description("Legacy compatibility field (currently not used by Matrix message endpoint)"),
				),
			),
			Handler: makeChatSendMessage(bf),
		},
	}
}

func makeChatConversations(bf BridgeFactory) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		bridge := bf(req)
		code, body, err := bridge.Call(ctx, "GET", "/v1/chat/rooms", nil, nil)
		return HandleBridgeResult(code, body, err)
	}
}

func makeChatGetMessages(bf BridgeFactory) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		roomID := req.GetString("room_id", "")
		peerID := req.GetString("peer_id", "")
		if roomID == "" {
			if peerID == "" {
				return gomcp.NewToolResultError("room_id or peer_id is required"), nil
			}
			bridge := bf(req)
			resolvedRoomID, toolErr, err := resolveDMRoomID(ctx, bridge, peerID)
			if err != nil {
				return nil, err
			}
			if toolErr != nil {
				return toolErr, nil
			}
			roomID = resolvedRoomID
		}

		query := url.Values{}
		if v := req.GetString("limit", ""); v != "" {
			query.Set("limit", v)
		}
		if v := req.GetString("before", ""); v != "" {
			query.Set("before", v)
		}
		if v := req.GetString("after", ""); v != "" {
			query.Set("after", v)
		}
		if v := req.GetString("offset", ""); v != "" {
			query.Set("offset", v)
		}
		bridge := bf(req)
		path := fmt.Sprintf("/v1/chat/rooms/%s/messages", url.PathEscape(roomID))
		code, body, err := bridge.Call(ctx, "GET", path, query, nil)
		return HandleBridgeResult(code, body, err)
	}
}

func makeChatSendMessage(bf BridgeFactory) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		roomID := req.GetString("room_id", "")
		peerID := req.GetString("peer_id", "")
		if roomID == "" {
			if peerID == "" {
				return gomcp.NewToolResultError("room_id or peer_id is required"), nil
			}
			bridge := bf(req)
			resolvedRoomID, toolErr, err := resolveDMRoomID(ctx, bridge, peerID)
			if err != nil {
				return nil, err
			}
			if toolErr != nil {
				return toolErr, nil
			}
			roomID = resolvedRoomID
		}

		message := req.GetString("message", "")
		if message == "" {
			return gomcp.NewToolResultError("message is required"), nil
		}

		// Legacy arg retained for compatibility; Matrix message endpoint only requires body.
		_ = req.GetString("order_id", "")

		payload := map[string]interface{}{
			"body": message,
		}
		bridge := bf(req)
		path := fmt.Sprintf("/v1/chat/rooms/%s/messages", url.PathEscape(roomID))
		code, body, err := bridge.Call(ctx, "POST", path, nil, payload)
		return HandleBridgeResult(code, body, err)
	}
}

func resolveDMRoomID(
	ctx context.Context,
	bridge Bridge,
	peerID string,
) (string, *gomcp.CallToolResult, error) {
	payloads := []map[string]interface{}{
		{"targetPeerID": peerID, "isDM": true},
		{"peerID": peerID, "isDM": true}, // fallback for older handlers
	}

	lastStatus := 0
	var lastBody []byte
	for _, payload := range payloads {
		code, body, err := bridge.Call(ctx, "POST", "/v1/chat/rooms", nil, payload)
		if err != nil {
			return "", nil, fmt.Errorf("resolve chat room failed: %w", err)
		}
		if code >= 200 && code < 300 {
			roomID := extractRoomID(body)
			if roomID == "" {
				return "", gomcp.NewToolResultError("resolved room response missing roomId"), nil
			}
			return roomID, nil, nil
		}

		lastStatus = code
		lastBody = body
		// Fallback payload is only meaningful for 4xx shape mismatch.
		if code < 400 || code >= 500 {
			break
		}
	}

	return "", mapHTTPError(lastStatus, lastBody), nil
}

func extractRoomID(body []byte) string {
	if len(body) == 0 {
		return ""
	}

	var decoded interface{}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return ""
	}

	return findRoomID(decoded)
}

func findRoomID(v interface{}) string {
	switch typed := v.(type) {
	case map[string]interface{}:
		for _, key := range []string{"roomId", "roomID"} {
			if raw, ok := typed[key]; ok {
				if roomID, ok := raw.(string); ok && roomID != "" {
					return roomID
				}
			}
		}

		if raw, ok := typed["data"]; ok {
			if roomID := findRoomID(raw); roomID != "" {
				return roomID
			}
		}

		for _, key := range []string{"room", "result"} {
			if raw, ok := typed[key]; ok {
				if roomID := findRoomID(raw); roomID != "" {
					return roomID
				}
			}
		}

	case []interface{}:
		for _, item := range typed {
			if roomID := findRoomID(item); roomID != "" {
				return roomID
			}
		}

	case string:
		roomID := strings.TrimSpace(typed)
		if strings.HasPrefix(roomID, "!") {
			return roomID
		}
	}

	return ""
}
