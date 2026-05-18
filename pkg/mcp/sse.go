package mcp

import (
	stdlog "log"
	"fmt"
	"net/http"
	"time"

	"github.com/mark3labs/mcp-go/server"
)

// NewStreamableHTTPMobazhaServer creates a Streamable-HTTP MCP server wrapped
// with the ai:use scope guard.
//
// The return type is http.Handler (not *server.StreamableHTTPServer) on
// purpose: every caller mounts it via mux.Handle("/v1/mcp", h), and exposing
// only http.Handler makes it impossible to bypass the front-door scope check
// by reaching for the raw streamable server.
//
// Streamable HTTP handles both GET (SSE stream) and POST (JSON-RPC messages)
// on a single endpoint path, so clients only need one URL without sub-paths.
//
// Authentication / authorization model:
//   - Any caller (admin JWT/Basic OR mbz_* API token) must already pass the
//     gateway AuthenticationMiddleware before reaching this handler.
//   - The wrapped guard then resolves identity via opts.IdentityPath and
//     requires the contracts.ScopeAIUse permission. Admin identities receive
//     AllScopes() from /v1/auth/identity (which includes ai:use), so they
//     pass automatically; API tokens must be minted with "ai:use".
//
// When opts.AuditLogger is set, opts.IdentityPath MUST also be set so the
// IdentityFn can resolve the caller. An empty IdentityPath with audit enabled
// would cause every entry to log with empty user/peer fields. An empty
// IdentityPath also DISABLES the ai:use guard with a WARNING — this is only
// reachable through misconfiguration, both real call sites set it.
func NewStreamableHTTPMobazhaServer(gatewayURL string, httpClient *http.Client, opts *ServerOptions) http.Handler {
	bf := SSEBridgeFactory(gatewayURL, httpClient)

	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	localOpts := &ServerOptions{}
	if opts != nil {
		*localOpts = *opts
	}
	if localOpts.Transport == "" {
		localOpts.Transport = "streamable-http"
	}
	// Allocate the identity cache up-front so the audit IdentityFn and the
	// ai:use guard share entries — one /v1/auth/identity round-trip per token
	// per TTL window instead of two.
	if localOpts.IdentityCache == nil {
		localOpts.IdentityCache = NewIdentityCache(5 * time.Minute)
	}
	if localOpts.IdentityFn == nil && localOpts.AuditLogger != nil {
		if localOpts.IdentityPath == "" {
			stdlog.Printf("[mcp] WARNING: AuditLogger set without IdentityPath; audit entries will have empty identity")
		} else {
			localOpts.IdentityFn = SSEIdentityFunc(gatewayURL, localOpts.IdentityPath, httpClient, localOpts.IdentityCache)
		}
	}
	mcpServer := NewAllToolsMobazhaServer(bf, localOpts)

	streamable := server.NewStreamableHTTPServer(mcpServer)

	// Front-door ai:use enforcement. Without IdentityPath we cannot resolve
	// the caller's scopes, so we fall back to "unguarded" with a loud warning
	// rather than silently 503-ing. Both production call sites set the path.
	if localOpts.IdentityPath == "" {
		stdlog.Printf("[mcp] WARNING: IdentityPath not set; ai:use scope guard DISABLED for /v1/mcp")
		return streamable
	}
	return RequireAIUseScope(streamable, gatewayURL, localOpts.IdentityPath, httpClient, localOpts.IdentityCache)
}

// NewPublicShoppingMCPServer creates a public, unauthenticated Streamable-HTTP
// MCP server that exposes only Phase 0 AI shopping tools.
//
// No ai:use scope guard is applied — shopping tools are intentionally public:
// they only touch public listing APIs, payment-methods, and anonymous guest
// checkout. Separating this from /v1/mcp (which requires ai:use) keeps the
// security boundary clear.
//
// Mount this handler at /v1/mcp/shopping so external AI platforms (Coze, Grok,
// Gemini CLI, etc.) can call it without any Mobazha account or API token.
//
// Returns an error when the Shopping config is absent or the DemoStorePeerID is
// empty — callers should skip mounting if err != nil.
func NewPublicShoppingMCPServer(gatewayURL string, httpClient *http.Client, opts *ServerOptions) (http.Handler, error) {
	if opts == nil || opts.Shopping == nil || opts.Shopping.DemoStorePeerID == "" {
		return nil, fmt.Errorf("mcp.NewPublicShoppingMCPServer: Shopping.DemoStorePeerID is required")
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	storeURL := opts.StoreGatewayURL
	if storeURL == "" {
		storeURL = gatewayURL
	}

	var searchBridge Bridge
	if opts.SearchURL != "" {
		searchBridge = NewHTTPBridge(opts.SearchURL, "", "", httpClient)
	}

	// storeBridge calls the SaaS gateway with X-Store-PeerID for routing.
	// No Bearer token is needed — guest checkout is public.
	storeBridge := NewHTTPBridge(storeURL, "", opts.Shopping.DemoStorePeerID, httpClient)
	signer := NewQuoteTokenSigner(opts.QuoteTokenSecret)
	registrars := shoppingToolRegistrars(searchBridge, storeBridge, *opts.Shopping, signer)

	s := server.NewMCPServer(
		"Mobazha AI Shopping",
		serverVersion,
		server.WithToolCapabilities(true),
	)
	for _, reg := range registrars {
		s.AddTool(reg.Tool, reg.Handler)
	}

	if opts.AuditLogger != nil {
		transport := "streamable-http-public"
		var identityFn IdentityFunc
		if opts.IdentityPath != "" {
			cache := opts.IdentityCache
			if cache == nil {
				cache = NewIdentityCache(5 * time.Minute)
			}
			identityFn = SSEIdentityFunc(gatewayURL, opts.IdentityPath, httpClient, cache)
		}
		s.Use(AuditMiddleware(opts.AuditLogger, transport, identityFn))
	}

	return server.NewStreamableHTTPServer(s), nil
}

// NewSSEMobazhaServer creates an SSE-based MCP server wrapped with the ai:use
// scope guard. See NewStreamableHTTPMobazhaServer for the full authorization
// model — the same applies here.
//
// Deprecated: prefer NewStreamableHTTPMobazhaServer. SSE requires clients to
// know about the /sse sub-path; streamable HTTP uses a single endpoint.
func NewSSEMobazhaServer(gatewayURL string, httpClient *http.Client, opts *ServerOptions) http.Handler {
	bf := SSEBridgeFactory(gatewayURL, httpClient)

	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	localOpts := &ServerOptions{}
	if opts != nil {
		*localOpts = *opts
	}
	if localOpts.Transport == "" {
		localOpts.Transport = "sse"
	}
	if localOpts.IdentityCache == nil {
		localOpts.IdentityCache = NewIdentityCache(5 * time.Minute)
	}
	if localOpts.IdentityFn == nil && localOpts.AuditLogger != nil {
		if localOpts.IdentityPath == "" {
			stdlog.Printf("[mcp] WARNING: AuditLogger set without IdentityPath; audit entries will have empty identity")
		} else {
			localOpts.IdentityFn = SSEIdentityFunc(gatewayURL, localOpts.IdentityPath, httpClient, localOpts.IdentityCache)
		}
	}
	mcpServer := NewAllToolsMobazhaServer(bf, localOpts)

	sseServer := server.NewSSEServer(mcpServer,
		server.WithStaticBasePath("/v1/mcp"),
		server.WithKeepAlive(true),
		server.WithKeepAliveInterval(30*time.Second),
	)

	if localOpts.IdentityPath == "" {
		stdlog.Printf("[mcp] WARNING: IdentityPath not set; ai:use scope guard DISABLED for /v1/mcp (SSE)")
		return sseServer
	}
	return RequireAIUseScope(sseServer, gatewayURL, localOpts.IdentityPath, httpClient, localOpts.IdentityCache)
}
