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

// registerNodeHumaAuthPublicOperations registers public auth ops
// (node version fingerprint — unauthenticated health check).
func (g *Gateway) registerNodeHumaAuthPublicOperations(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "admin-version-get",
		Method:      http.MethodGet,
		Path:        "/v1/admin/version",
		Summary:     "Node binary version fingerprint (public)",
		Tags:        []string{"system"},
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodGet, "/v1/admin/version", nil)
		rr := httptest.NewRecorder()
		g.handleAdminVersion(rr, req)
		data, err := nodeBridgeFlexJSON(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

// registerNodeHumaAuthAdminOperations registers authenticated admin auth ops:
// tokens, identity, scopes.
func (g *Gateway) registerNodeHumaAuthAdminOperations(api huma.API) {
	type jsonBody struct {
		Body json.RawMessage `json:",omitempty"`
	}

	type tokenIDPath struct {
		TokenID string `path:"tokenID"`
	}

	huma.Register(api, huma.Operation{
		OperationID: "admin-password-post",
		Method:      http.MethodPost,
		Path:        "/v1/admin/password",
		Summary:     "Rotate standalone admin password",
		Tags:        []string{"auth"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *jsonBody) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/admin/password", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handleChangePassword(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "auth-tokens-post",
		Method:      http.MethodPost,
		Path:        "/v1/auth/tokens",
		Summary:     "Mint local API token (standalone)",
		Tags:        []string{"auth"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *jsonBody) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/auth/tokens", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTAuthToken(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "auth-tokens-get",
		Method:      http.MethodGet,
		Path:        "/v1/auth/tokens",
		Summary:     "List local API tokens (standalone)",
		Tags:        []string{"auth"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodGet, "/v1/auth/tokens", nil)
		rr := httptest.NewRecorder()
		g.handleGETAuthTokens(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "auth-tokens-token-id-delete",
		Method:      http.MethodDelete,
		Path:        "/v1/auth/tokens/{tokenID}",
		Summary:     "Revoke local API token by ID",
		Tags:        []string{"auth"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *tokenIDPath) (*nodeNoContentOutput, error) {
		raw := "/v1/auth/tokens/" + url.PathEscape(in.TokenID)
		req := nodeBridgeRequestWithVars(ctx, http.MethodDelete, raw, nil, map[string]string{"tokenID": in.TokenID})
		rr := httptest.NewRecorder()
		g.handleDELETEAuthToken(rr, req)
		if err := nodeBridgeNoContent(rr); err != nil {
			return nil, err
		}
		return &nodeNoContentOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "auth-identity-get",
		Method:      http.MethodGet,
		Path:        "/v1/auth/identity",
		Summary:     "Inspect resolved principal and scopes",
		Tags:        []string{"auth"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodGet, "/v1/auth/identity", nil)
		rr := httptest.NewRecorder()
		g.handleGETAuthIdentity(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "auth-scopes-get",
		Method:      http.MethodGet,
		Path:        "/v1/auth/scopes",
		Summary:     "Enumerate token scope catalog",
		Tags:        []string{"auth"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodGet, "/v1/auth/scopes", nil)
		rr := httptest.NewRecorder()
		g.handleGETAuthScopes(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}
