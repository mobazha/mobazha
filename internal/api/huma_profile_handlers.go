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

// registerNodeHumaProfilePublicOperations registers public profile retrieval
// operations that do not require authentication.
func (g *Gateway) registerNodeHumaProfilePublicOperations(api huma.API) {
	g.registerProfilesGetScoped(api)
	g.registerProfilesGetSelf(api)
	g.registerProfilesBatchFetchGet(api)
	g.registerProfilesBatchFetchPost(api)
}

// registerNodeHumaProfileAdminOperations registers admin profile ops that
// require authentication (create, update).
func (g *Gateway) registerNodeHumaProfileAdminOperations(api huma.API) {
	g.registerProfilesCreate(api)
	g.registerProfilesCreateScoped(api)
	g.registerProfilesUpdate(api)
	g.registerProfilesUpdateScoped(api)
}

// --- Auth (batch route registered once with security; gorilla mux also exposes public parity) ---

func (g *Gateway) registerProfilesBatchFetchGet(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "profiles-batch-fetch-get",
		Method:      http.MethodGet,
		Path:        "/v1/profiles/batch",
		Summary:     "Batch fetch profiles (GET; body often empty—legacy accepts JSON array of peer IDs via POST)",
		Tags:        []string{"profiles"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodGet, "/v1/profiles/batch", bytes.NewReader([]byte("[]")))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTFetchProfiles(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerProfilesBatchFetchPost(api huma.API) {
	type profileBatchBody struct {
		Body json.RawMessage
	}
	huma.Register(api, huma.Operation{
		OperationID: "profiles-batch-fetch-post",
		Method:      http.MethodPost,
		Path:        "/v1/profiles/batch",
		Summary:     "Batch fetch profiles by peer ID list",
		Tags:        []string{"profiles"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *profileBatchBody) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/profiles/batch", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTFetchProfiles(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerProfilesCreate(api huma.API) {
	type profilePatchBodyInput struct {
		Body json.RawMessage
	}
	huma.Register(api, huma.Operation{
		OperationID: "profiles-create",
		Method:      http.MethodPost,
		Path:        "/v1/profiles",
		Summary:     "Create profile (authenticated peer)",
		Tags:        []string{"profiles"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *profilePatchBodyInput) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/profiles", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTProfile(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerProfilesCreateScoped(api huma.API) {
	type profileScopedBodyInput struct {
		PeerID string `path:"peerID" doc:"Target peer ID (must match caller)."`
		Body   json.RawMessage
	}
	huma.Register(api, huma.Operation{
		OperationID: "profiles-create-scoped",
		Method:      http.MethodPost,
		Path:        "/v1/profiles/{peerID}",
		Summary:     "Create profile with explicit peer path",
		Tags:        []string{"profiles"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *profileScopedBodyInput) (*nodeDataOutput, error) {
		rawURL := "/v1/profiles/" + url.PathEscape(in.PeerID)
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, bytes.NewReader(in.Body), map[string]string{"peerID": in.PeerID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTProfile(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerProfilesUpdate(api huma.API) {
	type profilePatchBodyInput struct {
		Body json.RawMessage
	}
	huma.Register(api, huma.Operation{
		OperationID: "profiles-update",
		Method:      http.MethodPut,
		Path:        "/v1/profiles",
		Summary:     "Patch profile merge (authenticated peer)",
		Tags:        []string{"profiles"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *profilePatchBodyInput) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPut, "/v1/profiles", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePUTProfile(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerProfilesUpdateScoped(api huma.API) {
	type profileScopedBodyInput struct {
		PeerID string `path:"peerID" doc:"Profile peer segment (must match caller)."`
		Body   json.RawMessage
	}
	huma.Register(api, huma.Operation{
		OperationID: "profiles-update-scoped",
		Method:      http.MethodPut,
		Path:        "/v1/profiles/{peerID}",
		Summary:     "Patch profile with explicit peer path",
		Tags:        []string{"profiles"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *profileScopedBodyInput) (*nodeDataOutput, error) {
		rawURL := "/v1/profiles/" + url.PathEscape(in.PeerID)
		req := nodeBridgeRequestWithVars(ctx, http.MethodPut, rawURL, bytes.NewReader(in.Body), map[string]string{"peerID": in.PeerID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePUTProfile(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

// --- Public ---

func (g *Gateway) registerProfilesGetScoped(api huma.API) {
	type profileGetInput struct {
		PeerID   string `path:"peerID" doc:"Public profile peer ID."`
		UseCache bool   `query:"usecache" required:"false" doc:"Return cached profile when true."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "profiles-get-by-peer-id",
		Method:      http.MethodGet,
		Path:        "/v1/profiles/{peerID}",
		Summary:     "Get public profile by peer ID",
		Tags:        []string{"profiles"},
	}, func(ctx context.Context, in *profileGetInput) (*nodeDataOutput, error) {
		rawURL := "/v1/profiles/" + url.PathEscape(in.PeerID)
		if in.UseCache {
			rawURL += "?usecache=true"
		}
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{"peerID": in.PeerID})
		rr := httptest.NewRecorder()
		g.handleGETProfile(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerProfilesGetSelf(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "profiles-get-self",
		Method:      http.MethodGet,
		Path:        "/v1/profiles",
		Summary:     "Get profile for implicit context (self or anonymous)",
		Tags:        []string{"profiles"},
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodGet, "/v1/profiles", nil)
		rr := httptest.NewRecorder()
		g.handleGETProfile(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}
