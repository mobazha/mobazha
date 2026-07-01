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

// registerNodeHumaWebhookOperations registers bridged webhook OpenAPI ops (AH-1.4 Batch 4).
func (g *Gateway) registerNodeHumaWebhookOperations(api huma.API) {
	type jsonBody struct {
		Body json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "webhooks-post",
		Method:      http.MethodPost,
		Path:        "/v1/webhooks",
		Summary:     "Create webhook endpoint",
		Tags:        []string{"webhooks"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *jsonBody) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/webhooks", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handleCreateWebhook(rr, req)
		data, err := nodeBridgeFlexJSON(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "webhooks-get",
		Method:      http.MethodGet,
		Path:        "/v1/webhooks",
		Summary:     "List webhook endpoints",
		Tags:        []string{"webhooks"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodGet, "/v1/webhooks", nil)
		rr := httptest.NewRecorder()
		g.handleListWebhooks(rr, req)
		data, err := nodeBridgeFlexJSON(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	type idPath struct {
		ID string `path:"id" doc:"Webhook endpoint ID."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "webhooks-id-get",
		Method:      http.MethodGet,
		Path:        "/v1/webhooks/{id}",
		Summary:     "Get webhook endpoint",
		Tags:        []string{"webhooks"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *idPath) (*nodeDataOutput, error) {
		rawURL := "/v1/webhooks/" + url.PathEscape(in.ID)
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{"id": in.ID})
		rr := httptest.NewRecorder()
		g.handleGetWebhook(rr, req)
		data, err := nodeBridgeFlexJSON(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	type idPatch struct {
		ID   string          `path:"id"`
		Body json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "webhooks-id-patch",
		Method:      http.MethodPatch,
		Path:        "/v1/webhooks/{id}",
		Summary:     "Update webhook endpoint",
		Tags:        []string{"webhooks"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *idPatch) (*nodeDataOutput, error) {
		rawURL := "/v1/webhooks/" + url.PathEscape(in.ID)
		req := nodeBridgeRequestWithVars(ctx, http.MethodPatch, rawURL, bytes.NewReader(in.Body), map[string]string{"id": in.ID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handleUpdateWebhook(rr, req)
		data, err := nodeBridgeFlexJSON(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "webhooks-id-delete",
		Method:      http.MethodDelete,
		Path:        "/v1/webhooks/{id}",
		Summary:     "Delete webhook endpoint",
		Tags:        []string{"webhooks"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *idPath) (*nodeNoContentOutput, error) {
		rawURL := "/v1/webhooks/" + url.PathEscape(in.ID)
		req := nodeBridgeRequestWithVars(ctx, http.MethodDelete, rawURL, nil, map[string]string{"id": in.ID})
		rr := httptest.NewRecorder()
		g.handleDeleteWebhook(rr, req)
		if err := nodeBridgeNoContent(rr); err != nil {
			return nil, err
		}
		return &nodeNoContentOutput{}, nil
	})

	type deliveriesQ struct {
		ID     string `path:"id"`
		Limit  string `query:"limit"`
		Offset string `query:"offset"`
		Status string `query:"status"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "webhooks-id-deliveries-get",
		Method:      http.MethodGet,
		Path:        "/v1/webhooks/{id}/deliveries",
		Summary:     "List webhook delivery attempts",
		Tags:        []string{"webhooks"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, q *deliveriesQ) (*nodeDataOutput, error) {
		v := url.Values{}
		if q.Limit != "" {
			v.Set("limit", q.Limit)
		}
		if q.Offset != "" {
			v.Set("offset", q.Offset)
		}
		if q.Status != "" {
			v.Set("status", q.Status)
		}
		rawURL := "/v1/webhooks/" + url.PathEscape(q.ID) + "/deliveries"
		if enc := v.Encode(); enc != "" {
			rawURL += "?" + enc
		}
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{"id": q.ID})
		rr := httptest.NewRecorder()
		g.handleListWebhookDeliveries(rr, req)
		data, err := nodeBridgeFlexJSON(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "webhooks-id-test-post",
		Method:      http.MethodPost,
		Path:        "/v1/webhooks/{id}/test",
		Summary:     "Enqueue test webhook event",
		Tags:        []string{"webhooks"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *idPath) (*nodeDataOutput, error) {
		rawURL := "/v1/webhooks/" + url.PathEscape(in.ID) + "/test"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, bytes.NewReader([]byte("{}")), map[string]string{"id": in.ID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handleTestWebhook(rr, req)
		data, err := nodeBridgeFlexJSON(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}
