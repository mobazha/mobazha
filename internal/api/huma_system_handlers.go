//go:build !private_distribution

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/danielgtaylor/huma/v2"
)

// registerNodeHumaSystemAdminOperations registers admin system operations
// (config, health, publish, logs, MCP, etc.) that require authentication.
func (g *Gateway) registerFullNodeHumaSystemAdminOperations(api huma.API) {
	type jsonBody struct {
		Body json.RawMessage `json:",omitempty"`
	}

	// ── Full-build-only endpoints ────────────────────────────────────

	huma.Register(api, huma.Operation{
		OperationID: "system-publish-post",
		Method:      http.MethodPost,
		Path:        "/v1/system/publish",
		Summary:     "Publish store data",
		Tags:        []string{"system"},
		Security:    adminOnlyAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/system/publish", http.NoBody)
		rr := httptest.NewRecorder()
		g.handlePOSTPublish(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "system-cache-delete",
		Method:      http.MethodDelete,
		Path:        "/v1/system/cache",
		Summary:     "Purge publishing cache",
		Tags:        []string{"system"},
		Security:    adminOnlyAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodDelete, "/v1/system/cache", http.NoBody)
		rr := httptest.NewRecorder()
		g.handlePOSTPurgeCache(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "system-network-get",
		Method:      http.MethodGet,
		Path:        "/v1/system/network",
		Summary:     "Inspect network overlays and listening multiaddrs",
		Tags:        []string{"system"},
		Security:    adminOnlyAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodGet, "/v1/system/network", nil)
		rr := httptest.NewRecorder()
		g.handleGETSystemNetwork(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "system-network-post",
		Method:      http.MethodPost,
		Path:        "/v1/system/network",
		Summary:     "Change network overlays (standalone)",
		Tags:        []string{"system"},
		Security:    adminOnlyAuthSecurity,
	}, func(ctx context.Context, in *jsonBody) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/system/network", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTSystemNetwork(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "system-domain-get",
		Method:      http.MethodGet,
		Path:        "/v1/system/domain",
		Summary:     "Inspect store domain bindings",
		Tags:        []string{"system"},
		Security:    adminOnlyAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodGet, "/v1/system/domain", nil)
		rr := httptest.NewRecorder()
		g.handleGETSystemDomain(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "system-domain-post",
		Method:      http.MethodPost,
		Path:        "/v1/system/domain",
		Summary:     "Update store domain configuration",
		Tags:        []string{"system"},
		Security:    adminOnlyAuthSecurity,
	}, func(ctx context.Context, in *jsonBody) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/system/domain", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTSystemDomain(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "system-connect-platform-post",
		Method:      http.MethodPost,
		Path:        "/v1/system/connect-platform",
		Summary:     "Bind standalone store owner to SaaS identity",
		Tags:        []string{"system"},
		Security:    adminOnlyAuthSecurity,
	}, func(ctx context.Context, in *jsonBody) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/system/connect-platform", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTConnectPlatform(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	// claim-store POST uses custom auth logic (JWT + admin proof), so no
	// Security: nodeAuthSecurity.
	huma.Register(api, huma.Operation{
		OperationID: "system-claim-store-post",
		Method:      http.MethodPost,
		Path:        "/v1/system/claim-store",
		Summary:     "Claim standalone store via JWT + admin proof",
		Tags:        []string{"system"},
	}, func(ctx context.Context, in *jsonBody) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/system/claim-store", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTClaimStore(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

// registerNodeHumaSystemPublicOperations registers public system operations
// (GET-only) that do not require authentication.
func (g *Gateway) registerFullNodeHumaSystemPublicOperations(api huma.API) {
}
