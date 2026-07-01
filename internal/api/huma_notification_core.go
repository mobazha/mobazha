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

// registerNodeHumaNotificationCoreOperations registers the local notification
// query/mutation operations valid in standard and restricted profiles.
//
// All ops here read/write a node-local SQLite table (NotificationRecord) and
// require no P2P, federation, or outbound delivery. Local-first sellers
// rely on these for local event surfacing (new guest order, payment detected,
// payment confirmed, auto-sweep status, system health alerts).
//
// Outbound channels (Telegram/Discord delivery) are registered separately in
// huma_notification_handlers.go and are omitted by the restricted profile.
func (g *Gateway) registerNodeHumaNotificationCoreOperations(api huma.API) {
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
}
