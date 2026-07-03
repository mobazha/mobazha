package mcp

import (
	"context"
	stdlog "log"
	"net/http"
	"time"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/mobazha/mobazha/pkg/redact"
)

// MCPAuditEntry contains data for a single audit log entry.
type MCPAuditEntry struct {
	UserID    string
	PeerID    string
	ToolName  string
	Arguments string
	Success   bool
	ErrorMsg  string
	Duration  time.Duration
	Transport string
}

// AuditLogger persists audit entries. Implementations should be non-blocking.
type AuditLogger interface {
	Log(entry MCPAuditEntry)
}

// StdoutAuditLogger writes audit entries to the standard library logger.
// Suitable for standalone nodes and development environments where a database
// audit trail is unnecessary. Output is single-line JSON-ish and parseable.
type StdoutAuditLogger struct{}

// NewStdoutAuditLogger returns an AuditLogger backed by the standard logger.
func NewStdoutAuditLogger() *StdoutAuditLogger {
	return &StdoutAuditLogger{}
}

// Log emits one structured line per tool call. Errors and identity values are
// included verbatim; redaction of sensitive arguments has already happened
// upstream in AuditMiddleware.
func (l *StdoutAuditLogger) Log(entry MCPAuditEntry) {
	status := "ok"
	if !entry.Success {
		status = "err"
	}
	stdlog.Printf("[mcp-audit] tool=%s status=%s transport=%s duration_ms=%d user=%s peer=%s args=%s err=%q",
		entry.ToolName, status, entry.Transport, entry.Duration.Milliseconds(),
		entry.UserID, entry.PeerID, entry.Arguments, entry.ErrorMsg)
}

// IdentityFunc resolves the caller's identity from a tool request.
// In stdio mode, returns a fixed identity. In SSE mode, reads from headers.
type IdentityFunc func(req gomcp.CallToolRequest) (userID, peerID string)

// StaticIdentityFunc returns a fixed identity for stdio mode.
func StaticIdentityFunc(userID, peerID string) IdentityFunc {
	return func(_ gomcp.CallToolRequest) (string, string) {
		return userID, peerID
	}
}

// SSEIdentityFunc resolves caller identity from CallToolRequest headers
// using the identity API with caching. identityPath is deployment-specific
// and required. Failures are silently ignored (audit still logs with empty
// identity rather than blocking tool calls).
//
// cache is shared with the ai:use scope guard so each token's identity is
// fetched once per TTL window across both call sites. If nil, a private
// 5-minute cache is allocated — kept for tests and stand-alone use.
func SSEIdentityFunc(gatewayURL, identityPath string, httpClient *http.Client, cache *IdentityCache) IdentityFunc {
	if cache == nil {
		cache = NewIdentityCache(5 * time.Minute)
	}
	return func(req gomcp.CallToolRequest) (string, string) {
		if req.Header == nil {
			return "", ""
		}
		identity, err := ResolveIdentityFromHeaders(req.Header, gatewayURL, identityPath, httpClient, cache)
		if err != nil || identity == nil {
			return "", ""
		}
		return identity.UserID, identity.PeerID
	}
}

// AuditMiddleware returns a ToolHandlerMiddleware compatible with MCPServer.Use().
// It wraps every tool call with audit logging.
func AuditMiddleware(logger AuditLogger, transport string, identityFn IdentityFunc) server.ToolHandlerMiddleware {
	return func(next server.ToolHandlerFunc) server.ToolHandlerFunc {
		return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
			start := time.Now()
			result, err := next(ctx, req)
			duration := time.Since(start)

			userID, peerID := "", ""
			if identityFn != nil {
				userID, peerID = identityFn(req)
			}

			entry := MCPAuditEntry{
				UserID:    userID,
				PeerID:    peerID,
				ToolName:  req.Params.Name,
				Arguments: redact.RedactMapJSON(req.GetArguments()),
				Success:   err == nil && (result == nil || !result.IsError),
				Duration:  duration,
				Transport: transport,
			}
			if err != nil {
				entry.ErrorMsg = err.Error()
			} else if result != nil && result.IsError {
				entry.ErrorMsg = extractErrorText(result)
			}

			logger.Log(entry)

			return result, err
		}
	}
}

// WithAudit wraps a tool handler with audit logging.
// Deprecated: Use AuditMiddleware with MCPServer.Use() instead.
func WithAudit(logger AuditLogger, transport string, identityFn IdentityFunc,
	handler server.ToolHandlerFunc, toolName string) server.ToolHandlerFunc {

	if logger == nil {
		return handler
	}

	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		start := time.Now()
		result, err := handler(ctx, req)
		duration := time.Since(start)

		userID, peerID := "", ""
		if identityFn != nil {
			userID, peerID = identityFn(req)
		}

		entry := MCPAuditEntry{
			UserID:    userID,
			PeerID:    peerID,
			ToolName:  toolName,
			Arguments: redact.RedactMapJSON(req.GetArguments()),
			Success:   err == nil && (result == nil || !result.IsError),
			Duration:  duration,
			Transport: transport,
		}
		if err != nil {
			entry.ErrorMsg = err.Error()
		} else if result != nil && result.IsError {
			entry.ErrorMsg = extractErrorText(result)
		}

		logger.Log(entry)

		return result, err
	}
}


func extractErrorText(result *gomcp.CallToolResult) string {
	if result == nil || len(result.Content) == 0 {
		return ""
	}
	for _, c := range result.Content {
		if tc, ok := c.(gomcp.TextContent); ok {
			return truncate([]byte(tc.Text), 500)
		}
	}
	return ""
}
