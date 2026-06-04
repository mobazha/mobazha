//go:build !private_distribution

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/models"
)

func TestHandleGetNotificationsIncludesUnreadCount(t *testing.T) {
	node := &mockNode{
		getNotificationsFunc: func(offsetID string, limit int, typeFilters []string) ([]models.NotificationRecord, int64, error) {
			return []models.NotificationRecord{
				{
					ID:           "n1",
					Timestamp:    time.Unix(1700000000, 0).UTC(),
					Read:         false,
					Type:         "order.funded",
					Notification: []byte(`{"notificationID":"n1","orderID":"ord-1","title":"Test"}`),
				},
			}, 3, nil
		},
		getNotificationsUnreadCountFunc: func() (int, error) {
			return 2, nil
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/notifications?limit=20", nil)
	ctx := context.WithValue(req.Context(), nodeContextKey, contracts.NodeService(node))
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	(&Gateway{}).handleGetNotifications(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	var body struct {
		Data struct {
			Unread        int               `json:"unread"`
			Total         int64             `json:"total"`
			Notifications []json.RawMessage `json:"notifications"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, rr.Body.String())
	}
	if body.Data.Unread != 2 {
		t.Fatalf("expected unread count 2, got %d", body.Data.Unread)
	}
	if body.Data.Total != 3 {
		t.Fatalf("expected total 3, got %d", body.Data.Total)
	}
	if len(body.Data.Notifications) != 1 {
		t.Fatalf("expected one notification, got %d", len(body.Data.Notifications))
	}
}
