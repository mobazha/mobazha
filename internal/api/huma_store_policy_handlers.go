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

func (g *Gateway) registerNodeHumaStorePolicyPublicOperations(api huma.API) {
	type peerPolicyPath struct {
		PeerID string `path:"peerID" doc:"Store peer ID."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "store-policy-peer-published-get",
		Method:      http.MethodGet,
		Path:        "/v1/store-policy/{peerID}/published",
		Summary:     "Get published store policy",
		Tags:        []string{"store-policy"},
	}, func(ctx context.Context, in *peerPolicyPath) (*nodeDataOutput, error) {
		rawURL := "/v1/store-policy/" + url.PathEscape(in.PeerID) + "/published"
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{"peerID": in.PeerID})
		rr := httptest.NewRecorder()
		g.handleGetPublishedStorePolicy(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerNodeHumaStorePolicyAdminOperations(api huma.API) {
	type jsonBody struct {
		Body json.RawMessage `json:",omitempty"`
	}
	type moderatorPath struct {
		PeerID string `path:"peerID" doc:"Moderator peer ID."`
	}

	huma.Register(api, huma.Operation{
		OperationID: "store-policy-get",
		Method:      http.MethodGet,
		Path:        "/v1/store-policy",
		Summary:     "Get store policy",
		Tags:        []string{"store-policy"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodGet, "/v1/store-policy", nil)
		rr := httptest.NewRecorder()
		g.handleGetStorePolicy(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "store-policy-moderators-get",
		Method:      http.MethodGet,
		Path:        "/v1/store-policy/moderators",
		Summary:     "List store moderators",
		Tags:        []string{"store-policy"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodGet, "/v1/store-policy/moderators", nil)
		rr := httptest.NewRecorder()
		g.handleGetStorePolicyModerators(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "store-policy-moderators-put",
		Method:      http.MethodPut,
		Path:        "/v1/store-policy/moderators",
		Summary:     "Replace store moderators",
		Tags:        []string{"store-policy"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *jsonBody) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPut, "/v1/store-policy/moderators", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePutStorePolicyModerators(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "store-policy-moderators-post",
		Method:      http.MethodPost,
		Path:        "/v1/store-policy/moderators",
		Summary:     "Add or update store moderator",
		Tags:        []string{"store-policy"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *jsonBody) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/store-policy/moderators", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePostStorePolicyModerator(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "store-policy-moderators-peer-delete",
		Method:      http.MethodDelete,
		Path:        "/v1/store-policy/moderators/{peerID}",
		Summary:     "Remove store moderator",
		Tags:        []string{"store-policy"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *moderatorPath) (*nodeDataOutput, error) {
		rawURL := "/v1/store-policy/moderators/" + url.PathEscape(in.PeerID)
		req := nodeBridgeRequestWithVars(ctx, http.MethodDelete, rawURL, nil, map[string]string{"peerID": in.PeerID})
		rr := httptest.NewRecorder()
		g.handleDeleteStorePolicyModerator(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}
