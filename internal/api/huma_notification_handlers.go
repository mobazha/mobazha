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
func (g *Gateway) registerNodeHumaNotificationOperations(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "notifications-get-count",
		Method:      http.MethodGet,
		Path:        "/v1/notifications/count",
		Summary:     "Notification unread and total counts",
		Tags:        []string{"notifications"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodGet, "/v1/notifications/count", nil)
		rr := httptest.NewRecorder()
		g.handleGetNotificationCount(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	type batchBody struct {
		Body json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "notifications-post-batch",
		Method:      http.MethodPost,
		Path:        "/v1/notifications/batch",
		Summary:     "Batch mark read or delete notifications",
		Tags:        []string{"notifications"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *batchBody) (*nodeDataOutput, error) {
		rd := bytes.NewReader(in.Body)
		if len(in.Body) == 0 {
			rd = bytes.NewReader([]byte("{}"))
		}
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/notifications/batch", rd)
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handleBatchNotifications(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "notifications-post-read-all",
		Method:      http.MethodPost,
		Path:        "/v1/notifications/read",
		Summary:     "Mark all notifications as read",
		Tags:        []string{"notifications"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/notifications/read", bytes.NewReader([]byte("{}")))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTMarkNotificationsMessageAsRead(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	type notifListQuery struct {
		Limit    string `query:"limit"`
		OffsetID string `query:"offsetID"`
		Filter   string `query:"filter"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "notifications-get-list",
		Method:      http.MethodGet,
		Path:        "/v1/notifications",
		Summary:     "List notifications",
		Tags:        []string{"notifications"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, q *notifListQuery) (*nodeDataOutput, error) {
		v := url.Values{}
		if q.Limit != "" {
			v.Set("limit", q.Limit)
		}
		if q.OffsetID != "" {
			v.Set("offsetID", q.OffsetID)
		}
		if q.Filter != "" {
			v.Set("filter", q.Filter)
		}
		rawURL := "/v1/notifications"
		if enc := v.Encode(); enc != "" {
			rawURL += "?" + enc
		}
		req := nodeBridgeRequest(ctx, http.MethodGet, rawURL, nil)
		rr := httptest.NewRecorder()
		g.handleGetNotifications(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	type notifReadOne struct {
		NotifID string `path:"notifID" doc:"Notification ID."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "notifications-post-notif-read",
		Method:      http.MethodPost,
		Path:        "/v1/notifications/{notifID}/read",
		Summary:     "Mark one notification as read",
		Tags:        []string{"notifications"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *notifReadOne) (*nodeDataOutput, error) {
		rawURL := "/v1/notifications/" + url.PathEscape(in.NotifID) + "/read"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, bytes.NewReader([]byte("{}")), map[string]string{"notifID": in.NotifID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTMarkNotificationMessageAsRead(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

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
