package mcp

import (
	"context"
	"net/url"
	"testing"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/client"
)

type mockBridge struct {
	callCount int
	lastPath  string
}

func (m *mockBridge) Call(ctx context.Context, method, path string, query url.Values, body interface{}) (int, []byte, error) {
	m.callCount++
	m.lastPath = path
	return 200, []byte(`{"data":{}}`), nil
}

func (m *mockBridge) CallMultipart(_ context.Context, _, path string, _, _ string, _ []byte) (int, []byte, error) {
	m.callCount++
	m.lastPath = path
	return 200, []byte(`{"total":0,"created":0,"updated":0,"failed":0}`), nil
}

// allScopesList enumerates every scope referenced by toolScopeRequirement so
// the "all scopes" test paths see every tool. Keep this in sync with
// pkg/mcp/auth.go::toolScopeRequirement when adding new tools/scopes.
var allScopesList = []string{
	"listings:read", "listings:write",
	"orders:read", "orders:manage",
	"purchases:read",
	"disputes:read",
	"wallet:read",
	"profiles:read", "profiles:write",
	"chat:read", "chat:write",
	"notifications:read", "notifications:manage",
	"discounts:read", "discounts:write",
	"collections:read", "collections:write",
	"settings:read",
	"fiat:read",
	"fulfillment:read", "fulfillment:manage",
}

func TestNewMobazhaServer_AllScopes(t *testing.T) {
	bridge := &mockBridge{}
	bf := StaticBridgeFactory(bridge)
	allScopes := NewScopeSet(allScopesList)

	s := NewMobazhaServer(bf, allScopes, bridge, nil)
	if s == nil {
		t.Fatal("expected non-nil server")
	}
}

func TestNewMobazhaServer_LimitedScopes(t *testing.T) {
	bridge := &mockBridge{}
	bf := StaticBridgeFactory(bridge)
	limitedScopes := NewScopeSet([]string{"listings:read"})

	s := NewMobazhaServer(bf, limitedScopes, bridge, nil)
	if s == nil {
		t.Fatal("expected non-nil server")
	}
}

func TestNewAllToolsMobazhaServer(t *testing.T) {
	bridge := &mockBridge{}
	bf := StaticBridgeFactory(bridge)

	s := NewAllToolsMobazhaServer(bf, nil)
	if s == nil {
		t.Fatal("expected non-nil server")
	}
}

// TestScopeFilteredServerTools verifies that NewMobazhaServer with limited
// scopes only exposes permitted tools via the MCP protocol (issue C).
func TestScopeFilteredServerTools(t *testing.T) {
	bridge := &mockBridge{}
	bf := StaticBridgeFactory(bridge)
	limitedScopes := NewScopeSet([]string{"listings:read"})

	s := NewMobazhaServer(bf, limitedScopes, bridge, &ServerOptions{})

	c, err := client.NewInProcessClient(s)
	if err != nil {
		t.Fatalf("failed to create in-process client: %v", err)
	}
	defer c.Close()

	ctx := context.Background()
	if err := c.Start(ctx); err != nil {
		t.Fatalf("failed to start client: %v", err)
	}

	initReq := gomcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = gomcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = gomcp.Implementation{Name: "test-client", Version: "0.1.0"}
	if _, err := c.Initialize(ctx, initReq); err != nil {
		t.Fatalf("initialize failed: %v", err)
	}

	toolsResult, err := c.ListTools(ctx, gomcp.ListToolsRequest{})
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}

	toolNames := make(map[string]bool, len(toolsResult.Tools))
	for _, tool := range toolsResult.Tools {
		toolNames[tool.Name] = true
	}

	mustExist := []string{"listings_list_mine", "listings_get", "listings_get_template", "exchange_rates_get"}
	for _, name := range mustExist {
		if !toolNames[name] {
			t.Errorf("expected tool %q to be registered with listings:read scope", name)
		}
	}

	mustNotExist := []string{"listings_create", "listings_update", "listings_delete", "listings_import_json",
		"orders_get_sales", "orders_confirm", "orders_refund",
		"wallet_get_receiving_accounts", "profile_update", "chat_send_message"}
	for _, name := range mustNotExist {
		if toolNames[name] {
			t.Errorf("tool %q should NOT be registered without its required scope", name)
		}
	}
}

// TestResourcesRegisteredInStdioMode verifies that NewMobazhaServer (stdio)
// registers 4 resources (issue A/B).
func TestResourcesRegisteredInStdioMode(t *testing.T) {
	bridge := &mockBridge{}
	bf := StaticBridgeFactory(bridge)
	allScopes := NewScopeSet(allScopesList)

	s := NewMobazhaServer(bf, allScopes, bridge, &ServerOptions{})

	c, err := client.NewInProcessClient(s)
	if err != nil {
		t.Fatalf("failed to create in-process client: %v", err)
	}
	defer c.Close()

	ctx := context.Background()
	if err := c.Start(ctx); err != nil {
		t.Fatalf("failed to start client: %v", err)
	}

	initReq := gomcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = gomcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = gomcp.Implementation{Name: "test-client", Version: "0.1.0"}
	if _, err := c.Initialize(ctx, initReq); err != nil {
		t.Fatalf("initialize failed: %v", err)
	}

	resourcesResult, err := c.ListResources(ctx, gomcp.ListResourcesRequest{})
	if err != nil {
		t.Fatalf("ListResources failed: %v", err)
	}

	expectedURIs := map[string]bool{
		"mobazha://store/me/summary":      false,
		"mobazha://store/me/listings":     false,
		"mobazha://store/me/orders/recent": false,
		"mobazha://notifications/unread":  false,
	}

	for _, res := range resourcesResult.Resources {
		if _, ok := expectedURIs[res.URI]; ok {
			expectedURIs[res.URI] = true
		}
	}

	for uri, found := range expectedURIs {
		if !found {
			t.Errorf("expected resource %q to be registered in stdio mode", uri)
		}
	}

	if len(resourcesResult.Resources) != 4 {
		t.Errorf("expected exactly 4 resources, got %d", len(resourcesResult.Resources))
	}
}

// TestSSEServerHasNoResources verifies that NewAllToolsMobazhaServer (SSE)
// does not register any resources (issue B).
func TestSSEServerHasNoResources(t *testing.T) {
	bridge := &mockBridge{}
	bf := StaticBridgeFactory(bridge)

	s := NewAllToolsMobazhaServer(bf, &ServerOptions{})

	c, err := client.NewInProcessClient(s)
	if err != nil {
		t.Fatalf("failed to create in-process client: %v", err)
	}
	defer c.Close()

	ctx := context.Background()
	if err := c.Start(ctx); err != nil {
		t.Fatalf("failed to start client: %v", err)
	}

	initReq := gomcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = gomcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = gomcp.Implementation{Name: "test-client", Version: "0.1.0"}
	if _, err := c.Initialize(ctx, initReq); err != nil {
		t.Fatalf("initialize failed: %v", err)
	}

	resourcesResult, err := c.ListResources(ctx, gomcp.ListResourcesRequest{})
	if err != nil {
		// SSE server may not have resource capability — that's acceptable
		return
	}

	if len(resourcesResult.Resources) != 0 {
		t.Errorf("SSE server should not have resources, got %d", len(resourcesResult.Resources))
	}
}

// TestSSEServerRegistersAllTools verifies that NewAllToolsMobazhaServer
// registers all tools without scope filtering (issue B).
func TestSSEServerRegistersAllTools(t *testing.T) {
	bridge := &mockBridge{}
	bf := StaticBridgeFactory(bridge)
	opts := &ServerOptions{SearchURL: "http://test-search:8080"}

	s := NewAllToolsMobazhaServer(bf, opts)

	c, err := client.NewInProcessClient(s)
	if err != nil {
		t.Fatalf("failed to create in-process client: %v", err)
	}
	defer c.Close()

	ctx := context.Background()
	if err := c.Start(ctx); err != nil {
		t.Fatalf("failed to start client: %v", err)
	}

	initReq := gomcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = gomcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = gomcp.Implementation{Name: "test-client", Version: "0.1.0"}
	if _, err := c.Initialize(ctx, initReq); err != nil {
		t.Fatalf("initialize failed: %v", err)
	}

	toolsResult, err := c.ListTools(ctx, gomcp.ListToolsRequest{})
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}

	allRegistrars := getAllToolRegistrars(bf, opts)
	if len(toolsResult.Tools) != len(allRegistrars) {
		t.Errorf("SSE server should have all %d tools, got %d", len(allRegistrars), len(toolsResult.Tools))
	}

	toolNames := make(map[string]bool, len(toolsResult.Tools))
	for _, tool := range toolsResult.Tools {
		toolNames[tool.Name] = true
	}

	for _, reg := range allRegistrars {
		if !toolNames[reg.Name] {
			t.Errorf("SSE server missing tool %q", reg.Name)
		}
	}

	for _, tool := range toolsResult.Tools {
		if tool.Description == "" {
			t.Errorf("tool %q has empty description (bad for AI discoverability)", tool.Name)
		}
	}
}

func TestAllToolsHaveScopeMapping(t *testing.T) {
	bf := StaticBridgeFactory(&mockBridge{})
	opts := &ServerOptions{SearchURL: "http://test-search:8080"}
	registrars := getAllToolRegistrars(bf, opts)
	if len(registrars) == 0 {
		t.Fatal("no tool registrars found")
	}

	for _, reg := range registrars {
		if _, exists := toolScopeRequirement[reg.Name]; !exists {
			t.Errorf("tool %q is registered but missing from toolScopeRequirement map", reg.Name)
		}
	}

	registrarNames := make(map[string]bool, len(registrars))
	for _, reg := range registrars {
		registrarNames[reg.Name] = true
	}
	for name := range toolScopeRequirement {
		if !registrarNames[name] {
			t.Errorf("toolScopeRequirement has %q but no matching registrar exists", name)
		}
	}
}

func TestFilterToolsByScopes_AllScopes(t *testing.T) {
	// Use the canonical scope vocabulary from pkg/contracts (profiles:read,
	// notifications:manage, etc.). Out-of-date strings here would silently
	// drop tools and mask the very mismatch this test is meant to catch.
	scopes := NewScopeSet([]string{
		"listings:read", "listings:write", "orders:read", "orders:manage",
		"purchases:read", "disputes:read",
		"wallet:read", "profiles:read", "profiles:write", "chat:read", "chat:write",
		"notifications:read", "notifications:manage",
		"discounts:read", "discounts:write", "collections:read", "collections:write",
		"settings:read", "fiat:read",
		"fulfillment:read", "fulfillment:manage",
	})

	allowed := FilterToolsByScopes(scopes)
	if len(allowed) != len(toolScopeRequirement) {
		t.Errorf("with all scopes, expected %d tools, got %d", len(toolScopeRequirement), len(allowed))
	}
}

func TestFilterToolsByScopes_LimitedScopes(t *testing.T) {
	scopes := NewScopeSet([]string{"listings:read"})

	allowed := FilterToolsByScopes(scopes)

	allowedSet := make(map[string]bool)
	for _, name := range allowed {
		allowedSet[name] = true
	}

	if !allowedSet["listings_list_mine"] {
		t.Error("listings_list_mine should be allowed with listings:read")
	}
	if !allowedSet["listings_get"] {
		t.Error("listings_get should be allowed with listings:read")
	}
	if !allowedSet["listings_get_template"] {
		t.Error("listings_get_template should be allowed with listings:read")
	}
	if !allowedSet["exchange_rates_get"] {
		t.Error("exchange_rates_get should always be allowed (no scope required)")
	}
	if !allowedSet["search_listings"] {
		t.Error("search_listings should always be allowed (no scope required)")
	}
	if !allowedSet["search_profiles"] {
		t.Error("search_profiles should always be allowed (no scope required)")
	}
	if allowedSet["orders_get_sales"] {
		t.Error("orders_get_sales should NOT be allowed without orders:read")
	}
	if allowedSet["wallet_get_receiving_accounts"] {
		t.Error("wallet_get_receiving_accounts should NOT be allowed without wallet:read")
	}
	if allowedSet["listings_create"] {
		t.Error("listings_create should NOT be allowed without listings:write")
	}
}

func TestFilterToolsByScopes_NoScopes(t *testing.T) {
	scopes := NewScopeSet([]string{})

	allowed := FilterToolsByScopes(scopes)

	publicTools := map[string]bool{
		"exchange_rates_get": true,
		"search_listings":    true,
		"search_profiles":    true,
	}
	for _, name := range allowed {
		if !publicTools[name] {
			t.Errorf("only public tools should be allowed with no scopes, got %s", name)
		}
	}
	if len(allowed) != len(publicTools) {
		t.Errorf("expected %d public tools with no scopes, got %d", len(publicTools), len(allowed))
	}
}

func TestFetchIdentityFromPath_Success(t *testing.T) {
	bridge := &mockBridge{}

	identity, err := FetchIdentityFromPath(context.Background(), bridge, "/platform/v1/auth/identity")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if identity == nil {
		t.Fatal("expected non-nil identity")
	}
}

func TestFetchIdentityFromPath_RequiresPath(t *testing.T) {
	bridge := &mockBridge{}

	if _, err := FetchIdentityFromPath(context.Background(), bridge, ""); err == nil {
		t.Fatal("expected error for empty identity path")
	}
}

func TestScopeSet_Has(t *testing.T) {
	ss := NewScopeSet([]string{"listings:read", "orders:read"})
	if !ss.Has("listings:read") {
		t.Error("expected has listings:read")
	}
	if ss.Has("wallet:read") {
		t.Error("expected not has wallet:read")
	}
}
