package notifications

import (
	"context"
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/logging"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

type tenantRoutableDatabase interface {
	database.Database
	TenantID() string
	ForTenant(tenantID string) (database.Database, error)
}

// NotificationSink is an EventSink that replaces the old Notifier for-select loop.
// It handles two paths:
//   - Persistent: events with Persistent=true → DB persist + WebSocket push
//   - WebSocket-only: everything else → type-specific JSON wrapper + WebSocket push
type NotificationSink struct {
	db              database.Database
	notifyFunc      func(any) error
	notifyForTenant func(string) func(any) error
}

// NewNotificationSink creates a new NotificationSink.
func NewNotificationSink(db database.Database, notifyFunc func(any) error) *NotificationSink {
	if notifyFunc == nil {
		notifyFunc = func(any) error { return nil }
	}
	return &NotificationSink{db: db, notifyFunc: notifyFunc}
}

// NewTenantAwareNotificationSink creates a sink that can route persistent
// notifications to an explicit target tenant when the event carries TenantID.
func NewTenantAwareNotificationSink(
	db database.Database,
	notifyFunc func(any) error,
	notifyForTenant func(string) func(any) error,
) *NotificationSink {
	s := NewNotificationSink(db, notifyFunc)
	s.notifyForTenant = notifyForTenant
	return s
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
	targetTenantID := s.routableTargetTenantID(event)
	db := s.dbForTenant(targetTenantID)
	notifyFunc := s.notifyFuncForTenant(targetTenantID)

	id, deterministicID := deterministicNotificationID(meta, event)
	if id == "" {
		r := make([]byte, 20)
		if _, err := rand.Read(r); err != nil {
			log.Errorf("Error generating notification ID: %s", err)
			return err
		}
		id = hex.EncodeToString(r)
	}

	setNotificationFields(event, id, meta.Name)

	out, err := json.MarshalIndent(event, "", "    ")
	if err != nil {
		log.Errorf("Error marshaling notification: %s", err)
		return err
	}

	duplicate := false
	err = db.Update(func(tx database.Tx) error {
		if deterministicID {
			var existing models.NotificationRecord
			if err := tx.Read().Where("id = ?", id).First(&existing).Error; err == nil {
				duplicate = true
				return nil
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			inserted, err := insertNotificationIfAbsent(tx, db, &models.NotificationRecord{
				ID:           id,
				Timestamp:    time.Now(),
				Read:         false,
				Type:         meta.Name,
				Notification: out,
			})
			if err != nil {
				return err
			}
			duplicate = !inserted
			return nil
		}
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
	if duplicate {
		return nil
	}

	unread := s.getUnreadCount(db)

	if err := notifyFunc(notificationPushMessage{
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

type tenantIDProvider interface {
	TenantID() string
}

func insertNotificationIfAbsent(
	tx database.Tx,
	db database.Database,
	record *models.NotificationRecord,
) (bool, error) {
	if scoped, ok := db.(tenantIDProvider); ok {
		record.TenantID = scoped.TenantID()
	}

	// Tx has no insert-ignore primitive. Use GORM's atomic conflict handling,
	// while setting TenantID explicitly to preserve tenant isolation on Create.
	res := tx.Read().Clauses(clause.OnConflict{DoNothing: true}).Create(record)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

func deterministicNotificationID(meta events.EventMeta, event interface{}) (string, bool) {
	if meta.Name != "order.confirmed" {
		return "", false
	}
	orderID := ""
	switch e := event.(type) {
	case *events.OrderConfirmation:
		orderID = e.OrderID
	case events.OrderConfirmation:
		orderID = e.OrderID
	default:
		return "", false
	}
	if orderID == "" {
		return "", false
	}
	sum := sha1.Sum([]byte(meta.Name + ":" + orderID))
	return "stable:" + hex.EncodeToString(sum[:]), true
}

// getUnreadCount queries the unread notification count from DB.
func (s *NotificationSink) getUnreadCount(db database.Database) int {
	var count int64
	err := db.View(func(tx database.Tx) error {
		return tx.Read().Model(&models.NotificationRecord{}).Where("read = ?", false).Count(&count).Error
	})
	if err != nil {
		log.Warningf("Error querying unread count for WS push: %s", err)
		return 0
	}
	return int(count)
}

func (s *NotificationSink) routableTargetTenantID(event interface{}) string {
	tenantID := extractTargetTenantID(event)
	if tenantID == "" {
		return ""
	}
	rdb, ok := s.db.(tenantRoutableDatabase)
	if !ok || rdb.TenantID() == tenantID {
		return ""
	}
	return tenantID
}

func (s *NotificationSink) dbForTenant(tenantID string) database.Database {
	if tenantID == "" {
		return s.db
	}
	rdb, ok := s.db.(tenantRoutableDatabase)
	if !ok {
		return s.db
	}
	if rdb.TenantID() == tenantID {
		return s.db
	}
	db, err := rdb.ForTenant(tenantID)
	if err != nil {
		log.Warningf("Error resolving notification DB for tenant %s: %s", tenantID, err)
		return s.db
	}
	return db
}

func (s *NotificationSink) notifyFuncForTenant(tenantID string) func(any) error {
	if tenantID == "" || s.notifyForTenant == nil {
		return s.notifyFunc
	}
	notifyFunc := s.notifyForTenant(tenantID)
	if notifyFunc == nil {
		return s.notifyFunc
	}
	return notifyFunc
}

func extractTargetTenantID(event interface{}) string {
	v := reflect.ValueOf(event)
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return ""
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return ""
	}
	field := v.FieldByName("TenantID")
	if !field.IsValid() || field.Kind() != reflect.String {
		return ""
	}
	return field.String()
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
