package mcp

import (
	gomcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	serverName    = "Mobazha Store Manager"
	serverVersion = "0.3.0"
)

// ToolRegistrar registers a single MCP tool with its handler.
type ToolRegistrar struct {
	Name    string
	Tool    gomcp.Tool
	Handler server.ToolHandlerFunc
}

// ServerOptions holds optional dependencies for MCP server construction.
type ServerOptions struct {
	AuditLogger AuditLogger
	Transport   string
	IdentityFn  IdentityFunc
	SearchURL   string // Base URL for the public Search API (mobazha.info). If empty, search tools are not registered.
	// IdentityPath is the deployment-specific identity API path. REQUIRED when
	// AuditLogger is set so the IdentityFn can resolve the caller's user/peer,
	// and REQUIRED for ai:use scope enforcement on the HTTP transport.
	// Standalone: "/v1/auth/identity". Hosting: "/platform/v1/auth/identity".
	IdentityPath string
	// IdentityCache is shared between the ai:use scope guard (front-door HTTP
	// middleware) and SSEIdentityFunc (per-tool-call audit attribution). When
	// nil, NewStreamableHTTPMobazhaServer / NewSSEMobazhaServer allocates one
	// internally. Pass an explicit cache only if multiple servers should share
	// it.
	IdentityCache *IdentityCache

	// Shopping enables Phase 0 AI shopping tools. If nil, shopping tools are
	// not registered. StoreGatewayURL is the base URL of the store node that
	// handles guest checkout (e.g., "https://app.mobazha.org").
	Shopping         *ShoppingConfig
	StoreGatewayURL  string
	QuoteTokenSecret []byte // HMAC secret for quote tokens; nil = random per-process
}

// NewMobazhaServer creates the MCP server with tools filtered by the given scopes.
// Used for stdio mode (single user) with resources registered.
func NewMobazhaServer(bf BridgeFactory, scopes ScopeSet, staticBridge Bridge, opts *ServerOptions) *server.MCPServer {
	s := server.NewMCPServer(
		serverName,
		serverVersion,
		server.WithToolCapabilities(true),
		server.WithResourceCapabilities(true, false),
	)

	allRegistrars := getAllToolRegistrars(bf, opts)

	for _, reg := range allRegistrars {
		required, exists := toolScopeRequirement[reg.Name]
		if scopes != nil && exists && required != "" && !scopes.Has(required) {
			continue
		}
		s.AddTool(reg.Tool, reg.Handler)
	}

	if staticBridge != nil {
		registerResources(s, staticBridge)
	}

	if opts != nil && opts.AuditLogger != nil {
		s.Use(AuditMiddleware(opts.AuditLogger, opts.Transport, opts.IdentityFn))
	}

	return s
}

// NewAllToolsMobazhaServer creates the MCP server with all tools registered
// (no scope filtering). Used for SSE mode where scope checks happen per-request.
// Resources are not registered in SSE mode.
func NewAllToolsMobazhaServer(bf BridgeFactory, opts *ServerOptions) *server.MCPServer {
	s := server.NewMCPServer(
		serverName,
		serverVersion,
		server.WithToolCapabilities(true),
	)

	for _, reg := range getAllToolRegistrars(bf, opts) {
		s.AddTool(reg.Tool, reg.Handler)
	}

	if opts != nil && opts.AuditLogger != nil {
		s.Use(AuditMiddleware(opts.AuditLogger, opts.Transport, opts.IdentityFn))
	}

	return s
}
