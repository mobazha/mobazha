package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mobazha/mobazha3.0/internal/core/coreiface"
	"github.com/gorilla/mux"
)

func (g *Gateway) handleGetNotifications(w http.ResponseWriter, r *http.Request) {
	limit := r.URL.Query().Get("limit")
	if limit == "" {
		limit = "-1"
	}
	l, err := strconv.Atoi(limit)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	offsetID := r.URL.Query().Get("offsetID")
	filter := r.URL.Query().Get("filter")

	types := strings.Split(filter, ",")
	var filters []string
	for _, t := range types {
		if t != "" {
			filters = append(filters, t)
		}
	}

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	notifications, total, err := node.GetNotifications(offsetID, l, filters)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	type notifData struct {
		Unread        int               `json:"unread"`
		Total         int64             `json:"total"`
		Notifications []json.RawMessage `json:"notifications"`
	}

	type NotificationRecord struct {
		Timestamp    time.Time              `json:"timestamp"`
		Read         bool                   `json:"read"`
		Type         string                 `json:"type"`
		Notification map[string]interface{} `json:"notification"`
	}

	payload := notifData{0, total, []json.RawMessage{}}
	for _, n := range notifications {
		var data map[string]interface{}
		if err := json.Unmarshal(n.Notification, &data); err != nil {
			continue
		}

		notificationUpdate := NotificationRecord{
			Timestamp:    n.Timestamp,
			Read:         n.Read,
			Type:         n.Type,
			Notification: data,
		}

		normalizedBytes, _ := json.Marshal(notificationUpdate)
		payload.Notifications = append(payload.Notifications, normalizedBytes)
	}
	sanitizedJSONResponse(w, payload)
}

func (g *Gateway) handlePOSTMarkNotificationMessageAsRead(w http.ResponseWriter, r *http.Request) {
	notifID := mux.Vars(r)["notifID"]

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	err := node.MarkNotificationAsRead(notifID)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	sanitizedJSONResponse(w, struct{}{})
}

func (g *Gateway) handlePOSTMarkNotificationsMessageAsRead(w http.ResponseWriter, r *http.Request) {
	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	err := node.MarkAllNotificationsAsRead()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	sanitizedJSONResponse(w, struct{}{})
}
