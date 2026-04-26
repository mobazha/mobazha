package mcp

import (
	"encoding/json"
	"fmt"

	gomcp "github.com/mark3labs/mcp-go/mcp"
)

// apiErrorEnvelope matches the Mobazha API error response format:
// {"error": {"code": "...", "message": "..."}}
type apiErrorEnvelope struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// HandleBridgeResult converts an HTTP response into an MCP tool result.
// For 2xx: returns the response body as JSON text content.
// For non-2xx: returns an MCP error result with a descriptive message.
func HandleBridgeResult(statusCode int, body []byte, err error) (*gomcp.CallToolResult, error) {
	if err != nil {
		return nil, fmt.Errorf("bridge call failed: %w", err)
	}

	if statusCode >= 200 && statusCode < 300 {
		if len(body) == 0 {
			return gomcp.NewToolResultText("Operation completed successfully."), nil
		}
		return gomcp.NewToolResultText(string(body)), nil
	}

	return mapHTTPError(statusCode, body), nil
}

func mapHTTPError(statusCode int, body []byte) *gomcp.CallToolResult {
	var envelope apiErrorEnvelope
	if err := json.Unmarshal(body, &envelope); err == nil && envelope.Error.Message != "" {
		return gomcp.NewToolResultError(
			fmt.Sprintf("%s (HTTP %d, code: %s)", envelope.Error.Message, statusCode, envelope.Error.Code))
	}

	switch statusCode {
	case 400:
		return gomcp.NewToolResultError(fmt.Sprintf("Bad request (HTTP 400): %s", truncate(body, 200)))
	case 401:
		return gomcp.NewToolResultError("Authentication failed. Check your API token.")
	case 403:
		return gomcp.NewToolResultError("Permission denied. Your API token lacks the required scope for this operation.")
	case 404:
		return gomcp.NewToolResultError("Resource not found.")
	case 409:
		return gomcp.NewToolResultError(fmt.Sprintf("Conflict: %s", truncate(body, 200)))
	case 429:
		return gomcp.NewToolResultError("Rate limit exceeded. Please wait before retrying.")
	default:
		if statusCode >= 500 {
			return gomcp.NewToolResultError(fmt.Sprintf("Server error (HTTP %d). Please try again later.", statusCode))
		}
		return gomcp.NewToolResultError(fmt.Sprintf("Unexpected error (HTTP %d): %s", statusCode, truncate(body, 200)))
	}
}

func truncate(b []byte, maxLen int) string {
	s := string(b)
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}
