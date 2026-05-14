package notifications

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/logging"
	"github.com/mobazha/mobazha3.0/pkg/models"
)

var log = logging.MustGetLogger("NOTF")

type notificationPushMessage struct {
	Type string               `json:"type"`
	Data notificationPushData `json:"data"`
}

type notificationPushData struct {
	Notification any `json:"notification"`
	Unread       int `json:"unread"`
}

type shoppingCartWrapper struct {
	ShoppingCart any `json:"shoppingCart"`
}

type walletWrapper struct {
	Wallet any `json:"wallet"`
}

type partialPaymentWrapper struct {
	PartialPayment any `json:"partialPaymentReceived"`
}

type statusWrapper struct {
	Status any `json:"status"`
}

// NotificationSink is an EventSink that replaces the old Notifier for-select loop.
// It handles two paths:
//   - Persistent: events with Persistent=true → DB persist + WebSocket push
//   - WebSocket-only: everything else → type-specific JSON wrapper + WebSocket push
type NotificationSink struct {
	db         database.Database
	notifyFunc func(any) error
}

// NewNotificationSink creates a new NotificationSink.
func NewNotificationSink(db database.Database, notifyFunc func(any) error) *NotificationSink {
	if notifyFunc == nil {
		notifyFunc = func(any) error { return nil }
	}
	return &NotificationSink{db: db, notifyFunc: notifyFunc}
}

// Name implements events.EventSink.
func (s *NotificationSink) Name() string { return "notification" }

// Concurrency implements events.ConcurrentSink.
// Notification requires sequential DB writes to preserve ordering.
func (s *NotificationSink) Concurrency() int { return 1 }

// Accept implements events.EventSink. Accepts all registered events.
func (s *NotificationSink) Accept(_ events.EventMeta) bool { return true }

// Handle implements events.EventSink.
func (s *NotificationSink) Handle(_ context.Context, meta events.EventMeta, event interface{}) error {
	if meta.Persistent {
		return s.handlePersistentNotification(meta, event)
	}
	return s.handleWebSocketOnly(meta, event)
}

// handlePersistentNotification replicates the old Notifier's notification path:
// assign ID + type on the embedded Notification struct, persist to DB, push via WebSocket.
func (s *NotificationSink) handlePersistentNotification(meta events.EventMeta, event interface{}) error {
	r := make([]byte, 20)
	if _, err := rand.Read(r); err != nil {
		log.Errorf("Error generating notification ID: %s", err)
		return err
	}
	id := hex.EncodeToString(r)

	setNotificationFields(event, id, meta.Name)

	out, err := json.MarshalIndent(event, "", "    ")
	if err != nil {
		log.Errorf("Error marshaling notification: %s", err)
		return err
	}

	err = s.db.Update(func(tx database.Tx) error {
		return tx.Save(&models.NotificationRecord{
			ID:           id,
			Timestamp:    time.Now(),
			Read:         false,
			Type:         meta.Name,
			Notification: out,
		})
	})
	if err != nil {
		log.Errorf("Error saving notification to the database: %s", err)
		return err
	}

	unread := s.getUnreadCount()

	if err := s.notifyFunc(notificationPushMessage{
		Type: "notification",
		Data: notificationPushData{
			Notification: event,
			Unread:       unread,
		},
	}); err != nil {
		log.Errorf("Error sending notification: %s", err)
		return err
	}
	return nil
}

// getUnreadCount queries the unread notification count from DB.
func (s *NotificationSink) getUnreadCount() int {
	var count int64
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Model(&models.NotificationRecord{}).Where("read = ?", false).Count(&count).Error
	})
	if err != nil {
		log.Warningf("Error querying unread count for WS push: %s", err)
		return 0
	}
	return int(count)
}

// handleWebSocketOnly replicates the old Notifier's non-persistent paths
// (chat, wallet, publish, cart, chatgroup, payment.partial).
func (s *NotificationSink) handleWebSocketOnly(meta events.EventMeta, event interface{}) error {
	wrapped := wrapForWebSocket(meta, event)
	if wrapped == nil {
		return nil
	}
	if err := s.notifyFunc(wrapped); err != nil {
		log.Errorf("Error sending WebSocket event: %s", err)
		return err
	}
	return nil
}

// setNotificationFields sets the ID and Type on events that embed events.Notification.
// Uses reflection to find the embedded Notification struct, so new event types
// that embed Notification are automatically supported without code changes.
func setNotificationFields(event interface{}, id, typ string) {
	v := reflect.ValueOf(event)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return
	}
	nf := v.FieldByName("Notification")
	if !nf.IsValid() || nf.Type() != reflect.TypeOf(events.Notification{}) {
		return
	}
	nf.FieldByName("ID").SetString(id)
	nf.FieldByName("Typ").SetString(typ)
}

// wrapForWebSocket applies the type-specific JSON wrappers that the frontend expects.
// Returns nil for events that have no known WebSocket wrapper.
func wrapForWebSocket(meta events.EventMeta, event interface{}) interface{} {
	switch meta.Category {
	case "wallet":
		return wrapWalletEvent(event)
	case "publish":
		return wrapPublishEvent(event)
	case "cart":
		return shoppingCartWrapper{event}
	case "payment":
		if meta.Name == "payment.partial" {
			return partialPaymentWrapper{event}
		}
		return nil
	default:
		return nil
	}
}

func wrapWalletEvent(event interface{}) interface{} {
	switch event.(type) {
	case *events.BlockReceived:
		return walletWrapper{struct {
			Block any `json:"block"`
		}{Block: event}}
	case *events.TransactionReceived:
		return walletWrapper{struct {
			Transaction any `json:"transaction"`
		}{Transaction: event}}
	case *events.SpendFromPaymentAddress:
		return walletWrapper{struct {
			Transaction any `json:"transaction"`
		}{Transaction: event}}
	case *events.WalletUpdate:
		return struct {
			WalletUpdate any `json:"walletUpdate"`
		}{WalletUpdate: event}
	default:
		return nil
	}
}

func wrapPublishEvent(event interface{}) interface{} {
	switch event.(type) {
	case *events.PublishStarted:
		return statusWrapper{"publishing"}
	case *events.PublishFinished:
		return statusWrapper{"publish complete"}
	case *events.PublishingError:
		return statusWrapper{"error publishing"}
	default:
		return nil
	}
}
