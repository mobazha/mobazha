package notifications

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
)

// NotificationSink is an EventSink that replaces the old Notifier for-select loop.
// It handles two paths:
//   - Persistent: events with Legacy != "" → DB persist + WebSocket push
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

// Accept implements events.EventSink. Accepts all registered events.
func (s *NotificationSink) Accept(_ events.EventMeta) bool { return true }

// Handle implements events.EventSink.
func (s *NotificationSink) Handle(_ context.Context, meta events.EventMeta, event interface{}) error {
	if meta.Legacy != "" {
		return s.handlePersistentNotification(meta, event)
	}
	return s.handleWebSocketOnly(meta, event)
}

// handlePersistentNotification replicates the old Notifier's notification path:
// assign ID + type on the embedded Notification struct, persist to DB, push via WebSocket.
func (s *NotificationSink) handlePersistentNotification(meta events.EventMeta, event interface{}) error {
	r := make([]byte, 20)
	rand.Read(r)
	id := hex.EncodeToString(r)

	setNotificationFields(event, id, meta.Legacy)

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
			Type:         meta.Legacy,
			Notification: out,
		})
	})
	if err != nil {
		log.Errorf("Error saving notification to the database: %s", err)
		return err
	}

	if err := s.notifyFunc(notificationWrapper{event}); err != nil {
		log.Errorf("Error sending notification: %s", err)
		return err
	}
	return nil
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
func setNotificationFields(event interface{}, id, typ string) {
	switch e := event.(type) {
	case *events.NewOrder:
		e.ID, e.Typ = id, typ
	case *events.OrderFunded:
		e.ID, e.Typ = id, typ
	case *events.OrderPaymentReceived:
		e.ID, e.Typ = id, typ
	case *events.OrderConfirmation:
		e.ID, e.Typ = id, typ
	case *events.OrderDeclined:
		e.ID, e.Typ = id, typ
	case *events.OrderCancel:
		e.ID, e.Typ = id, typ
	case *events.Refund:
		e.ID, e.Typ = id, typ
	case *events.OrderFulfillment:
		e.ID, e.Typ = id, typ
	case *events.OrderCompletion:
		e.ID, e.Typ = id, typ
	case *events.DisputeOpen:
		e.ID, e.Typ = id, typ
	case *events.CaseOpen:
		e.ID, e.Typ = id, typ
	case *events.CaseUpdate:
		e.ID, e.Typ = id, typ
	case *events.DisputeClose:
		e.ID, e.Typ = id, typ
	case *events.DisputeAccepted:
		e.ID, e.Typ = id, typ
	case *events.VendorFinalizedPayment:
		e.ID, e.Typ = id, typ
	case *events.Follow:
		e.ID, e.Typ = id, typ
	case *events.Unfollow:
		e.ID, e.Typ = id, typ
	case *events.PaymentLockedReceived:
		e.ID, e.Typ = id, typ
	case *events.PaymentExpiredNotification:
		e.ID, e.Typ = id, typ
	case *events.PaymentCancelledByBuyer:
		e.ID, e.Typ = id, typ
	}
}

// wrapForWebSocket applies the type-specific JSON wrappers that the frontend expects.
// Returns nil for events that have no known WebSocket wrapper.
func wrapForWebSocket(meta events.EventMeta, event interface{}) interface{} {
	switch meta.Category {
	case "chat":
		return wrapChatEvent(event)
	case "chatgroup":
		return wrapChatGroupEvent(event)
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

func wrapChatEvent(event interface{}) interface{} {
	switch event.(type) {
	case *events.ChatMessage:
		return chatMessageWrapper{event}
	case *events.ChatRead:
		return messageReadWrapper{event}
	case *events.ChatTyping:
		return messageTypingWrapper{event}
	case *events.ChannelMessage:
		return channelMessageWrapper{event}
	default:
		return nil
	}
}

func wrapChatGroupEvent(event interface{}) interface{} {
	switch event.(type) {
	case *events.ChatGroupCreate:
		return chatGroupCreateWrapper{event}
	case *events.ChatGroupUpdate:
		return chatGroupUpdateWrapper{event}
	case *events.ChatGroupDelete:
		return chatGroupDeleteWrapper{event}
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
