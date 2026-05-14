package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"

	"github.com/danielgtaylor/huma/v2"
)

// registerCommonSystemAdminOps registers system admin endpoints shared between
// the full build and the private_distribution build. Callers still register build-specific
// endpoints (publish, network, domain, etc.) separately.
func (g *Gateway) registerCommonSystemAdminOps(api huma.API) {
	type jsonBody struct {
		Body json.RawMessage `json:",omitempty"`
	}

	huma.Register(api, huma.Operation{
		OperationID: "config-get",
		Method:      http.MethodGet,
		Path:        "/v1/config",
		Summary:     "Get gateway configuration snapshot",
		Tags:        []string{"system"},
		// config-get is the gateway snapshot used by tooling; an entry in
		// routeScopeMap (GET /v1/config -> ScopeSettingsRead) lets tokens
		// read it, so nodeAuthSecurity is the correct declaration.
		Security: nodeAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodGet, "/v1/config", nil)
		rr := httptest.NewRecorder()
		g.handleGETConfig(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "system-info-get",
		Method:      http.MethodGet,
		Path:        "/v1/system/info",
		Summary:     "Get system/network info snapshot",
		Tags:        []string{"system"},
		Security:    adminOnlyAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodGet, "/v1/system/info", nil)
		rr := httptest.NewRecorder()
		g.handleGETSystemInfo(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "system-shutdown-post",
		Method:      http.MethodPost,
		Path:        "/v1/system/shutdown",
		Summary:     "Shutdown standalone node process",
		Tags:        []string{"system"},
		Security:    adminOnlyAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/system/shutdown", http.NoBody)
		rr := httptest.NewRecorder()
		g.handlePOSTShutdown(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "system-logs-get",
		Method:      http.MethodGet,
		Path:        "/v1/system/logs",
		Summary:     "Tail recent node log lines",
		Tags:        []string{"system"},
		Security:    adminOnlyAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodGet, "/v1/system/logs", nil)
		rr := httptest.NewRecorder()
		g.handleGETSystemLogs(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "system-update-trigger-post",
		Method:      http.MethodPost,
		Path:        "/v1/system/update-trigger",
		Summary:     "Ask launcher/native updater to action",
		Tags:        []string{"system"},
		Security:    adminOnlyAuthSecurity,
	}, func(ctx context.Context, in *jsonBody) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/system/update-trigger", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTUpdateTrigger(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "system-update-config-get",
		Method:      http.MethodGet,
		Path:        "/v1/system/update-config",
		Summary:     "Read auto-update configuration sidecar",
		Tags:        []string{"system"},
		Security:    adminOnlyAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodGet, "/v1/system/update-config", nil)
		rr := httptest.NewRecorder()
		g.handleGETUpdateConfig(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "system-update-config-put",
		Method:      http.MethodPut,
		Path:        "/v1/system/update-config",
		Summary:     "Write auto-update configuration sidecar",
		Tags:        []string{"system"},
		Security:    adminOnlyAuthSecurity,
	}, func(ctx context.Context, in *jsonBody) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPut, "/v1/system/update-config", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePUTUpdateConfig(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "system-diagnostics-get",
		Method:      http.MethodGet,
		Path:        "/v1/system/diagnostics",
		Summary:     "Export structured diagnostics blob",
		Tags:        []string{"system"},
		Security:    adminOnlyAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodGet, "/v1/system/diagnostics", nil)
		rr := httptest.NewRecorder()
		g.handleGETSystemDiagnostics(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "system-health-get",
		Method:      http.MethodGet,
		Path:        "/v1/system/health",
		Summary:     "Node health and telemetry",
		Tags:        []string{"system"},
		Security:    adminOnlyAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodGet, "/v1/system/health", nil)
		rr := httptest.NewRecorder()
		g.handleGETSystemHealth(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "system-doctor-get",
		Method:      http.MethodGet,
		Path:        "/v1/system/doctor",
		Summary:     "Run self-check diagnostics snapshot",
		Tags:        []string{"system"},
		Security:    adminOnlyAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodGet, "/v1/system/doctor", nil)
		rr := httptest.NewRecorder()
		g.handleGETSystemDoctor(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	// ── MCP (AI) ────────────────────────────────────────────────────

	huma.Register(api, huma.Operation{
		OperationID: "system-mcp-capability-get",
		Method:      http.MethodGet,
		Path:        "/v1/system/mcp/capability",
		Summary:     "Detect MCP client capabilities",
		Tags:        []string{"system", "ai"},
		Security:    adminOnlyAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodGet, "/v1/system/mcp/capability", nil)
		rr := httptest.NewRecorder()
		g.handleGETMCPCapability(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "system-mcp-connect-post",
		Method:      http.MethodPost,
		Path:        "/v1/system/mcp/connect",
		Summary:     "Auto-configure MCP client",
		Tags:        []string{"system", "ai"},
		Security:    adminOnlyAuthSecurity,
	}, func(ctx context.Context, in *jsonBody) (*nodeDataOutput, error) {
		body := in.Body
		if len(body) == 0 {
			body = json.RawMessage(`{}`)
		}
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/system/mcp/connect", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTMCPConnect(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	type mcpClientJSON struct {
		Client string          `path:"client"`
		Body   json.RawMessage `json:",omitempty"`
	}
	type mcpClientOnlyPath struct {
		Client string `path:"client"`
	}

	huma.Register(api, huma.Operation{
		OperationID: "system-mcp-connect-client-post",
		Method:      http.MethodPost,
		Path:        "/v1/system/mcp/connect/{client}",
		Summary:     "Auto-configure a specific MCP client",
		Tags:        []string{"system", "ai"},
		Security:    adminOnlyAuthSecurity,
	}, func(ctx context.Context, in *mcpClientJSON) (*nodeDataOutput, error) {
		raw := "/v1/system/mcp/connect/" + url.PathEscape(in.Client)
		body := in.Body
		if len(body) == 0 {
			body = json.RawMessage(`{}`)
		}
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, raw, bytes.NewReader(body), map[string]string{
			"client": in.Client,
		})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTMCPConnectClient(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "system-mcp-clients-get",
		Method:      http.MethodGet,
		Path:        "/v1/system/mcp/clients",
		Summary:     "List configured MCP clients",
		Tags:        []string{"system", "ai"},
		Security:    adminOnlyAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodGet, "/v1/system/mcp/clients", nil)
		rr := httptest.NewRecorder()
		g.handleGETMCPClients(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "system-mcp-disconnect-post",
		Method:      http.MethodPost,
		Path:        "/v1/system/mcp/disconnect",
		Summary:     "Remove MCP integration",
		Tags:        []string{"system", "ai"},
		Security:    adminOnlyAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/system/mcp/disconnect", http.NoBody)
		rr := httptest.NewRecorder()
		g.handlePOSTMCPDisconnect(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "system-mcp-disconnect-client-post",
		Method:      http.MethodPost,
		Path:        "/v1/system/mcp/disconnect/{client}",
		Summary:     "Remove integration for one MCP client",
		Tags:        []string{"system", "ai"},
		Security:    adminOnlyAuthSecurity,
	}, func(ctx context.Context, in *mcpClientOnlyPath) (*nodeDataOutput, error) {
		raw := "/v1/system/mcp/disconnect/" + url.PathEscape(in.Client)
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, raw, http.NoBody, map[string]string{
			"client": in.Client,
		})
		rr := httptest.NewRecorder()
		g.handlePOSTMCPDisconnectClient(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	// setup POST uses custom auth logic inside its handler (optional-auth /
	// JWT+admin-password), so must NOT carry Security: nodeAuthSecurity.
	huma.Register(api, huma.Operation{
		OperationID: "system-setup-post",
		Method:      http.MethodPost,
		Path:        "/v1/system/setup",
		Summary:     "Standalone one-time password setup",
		Tags:        []string{"system"},
	}, func(ctx context.Context, in *jsonBody) (*nodeDataOutput, error) {
		req := g.nodeBridgeRequestWithOptionalAuth(ctx, http.MethodPost, "/v1/system/setup", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handleSetup(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

// registerCommonSystemPublicOps registers public system endpoints shared
// between the full build and the private_distribution build.
func (g *Gateway) registerCommonSystemPublicOps(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "system-setup-get",
		Method:      http.MethodGet,
		Path:        "/v1/system/setup",
		Summary:     "Standalone onboarding status (public)",
		Tags:        []string{"system"},
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodGet, "/v1/system/setup", nil)
		rr := httptest.NewRecorder()
		g.handleSetup(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}
