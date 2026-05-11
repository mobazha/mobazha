//go:build !private_distribution

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

// registerNodeHumaNotificationOperations registers bridged notification + channel OpenAPI ops (AH-1.4 Batch 4).
//
// Core notification list/count/read/batch ops are registered in huma_notification_core.go
// (shared across private_distribution / full builds). Channel ops (Telegram/Discord) require external
// outbound delivery and remain gated to non-private_distribution builds.
func (g *Gateway) registerNodeHumaNotificationOperations(api huma.API) {
	g.registerNodeHumaNotificationCoreOperations(api)

	huma.Register(api, huma.Operation{
		OperationID: "notifications-channels-get",
		Method:      http.MethodGet,
		Path:        "/v1/notifications/channels",
		Summary:     "List notification channels",
		Tags:        []string{"notifications"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodGet, "/v1/notifications/channels", nil)
		rr := httptest.NewRecorder()
		g.handleGETNotificationChannels(rr, req)
		data, err := nodeBridgeFlexJSON(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	type channelPOSTBody struct {
		Body json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "notifications-channels-post",
		Method:      http.MethodPost,
		Path:        "/v1/notifications/channels",
		Summary:     "Create notification channel",
		Tags:        []string{"notifications"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *channelPOSTBody) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/notifications/channels", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTNotificationChannel(rr, req)
		data, err := nodeBridgeFlexJSON(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "notifications-channels-detect-chat-post",
		Method:      http.MethodPost,
		Path:        "/v1/notifications/channels/detect-chat",
		Summary:     "Detect chat binding for notification channel",
		Tags:        []string{"notifications"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *channelPOSTBody) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/notifications/channels/detect-chat", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTDetectTelegramChat(rr, req)
		data, err := nodeBridgeFlexJSON(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "notifications-channel-types-get",
		Method:      http.MethodGet,
		Path:        "/v1/notifications/channel-types",
		Summary:     "List notification channel types",
		Tags:        []string{"notifications"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodGet, "/v1/notifications/channel-types", nil)
		rr := httptest.NewRecorder()
		g.handleGETNotificationChannelTypes(rr, req)
		data, err := nodeBridgeFlexJSON(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	type notificationChannelPath struct {
		ID string `path:"id" doc:"Channel ID."`
	}
	type notificationChannelPutIn struct {
		ID   string          `path:"id" doc:"Channel ID."`
		Body json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "notifications-channels-id-put",
		Method:      http.MethodPut,
		Path:        "/v1/notifications/channels/{id}",
		Summary:     "Update notification channel",
		Tags:        []string{"notifications"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *notificationChannelPutIn) (*nodeDataOutput, error) {
		rawURL := "/v1/notifications/channels/" + url.PathEscape(in.ID)
		req := nodeBridgeRequestWithVars(ctx, http.MethodPut, rawURL, bytes.NewReader(in.Body), map[string]string{"id": in.ID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePUTNotificationChannel(rr, req)
		data, err := nodeBridgeFlexJSON(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "notifications-channels-id-delete",
		Method:      http.MethodDelete,
		Path:        "/v1/notifications/channels/{id}",
		Summary:     "Delete notification channel",
		Tags:        []string{"notifications"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *notificationChannelPath) (*nodeNoContentOutput, error) {
		rawURL := "/v1/notifications/channels/" + url.PathEscape(in.ID)
		req := nodeBridgeRequestWithVars(ctx, http.MethodDelete, rawURL, nil, map[string]string{"id": in.ID})
		rr := httptest.NewRecorder()
		g.handleDELETENotificationChannel(rr, req)
		if err := nodeBridgeNoContent(rr); err != nil {
			return nil, err
		}
		return &nodeNoContentOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "notifications-channels-id-test-post",
		Method:      http.MethodPost,
		Path:        "/v1/notifications/channels/{id}/test",
		Summary:     "Send test notification on channel",
		Tags:        []string{"notifications"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *struct {
		ID   string          `path:"id" doc:"Channel ID."`
		Body json.RawMessage `json:",omitempty"`
	}) (*nodeDataOutput, error) {
		rawURL := "/v1/notifications/channels/" + url.PathEscape(in.ID) + "/test"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, bytes.NewReader(in.Body), map[string]string{"id": in.ID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTNotificationChannelTest(rr, req)
		data, err := nodeBridgeFlexJSON(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}
