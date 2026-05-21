package notifications

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
)

type mockTx struct {
	database.Tx
	saved []interface{}
}

func (tx *mockTx) Save(v interface{}) error {
	tx.saved = append(tx.saved, v)
	return nil
}

type mockDB struct {
	database.Database
	mu    sync.Mutex
	saved []interface{}
}

func (db *mockDB) Update(fn func(database.Tx) error) error {
	tx := &mockTx{}
	if err := fn(tx); err != nil {
		return err
	}
	db.mu.Lock()
	db.saved = append(db.saved, tx.saved...)
	db.mu.Unlock()
	return nil
}

func (db *mockDB) View(_ func(database.Tx) error) error {
	return nil
}

type routableMockDB struct {
	*mockDB
	tenantID string
	tenants  map[string]*routableMockDB
}

func (db *routableMockDB) TenantID() string {
	return db.tenantID
}

func (db *routableMockDB) ForTenant(tenantID string) (database.Database, error) {
	if tenant, ok := db.tenants[tenantID]; ok {
		return tenant, nil
	}
	return db, nil
}

type notifyCapture struct {
	mu       sync.Mutex
	captured []interface{}
}

func (c *notifyCapture) notify(v any) error {
	c.mu.Lock()
	c.captured = append(c.captured, v)
	c.mu.Unlock()
	return nil
}

func TestNotificationSink_Name(t *testing.T) {
	sink := NewNotificationSink(nil, nil)
	if sink.Name() != "notification" {
		t.Errorf("expected 'notification', got %q", sink.Name())
	}
}

func TestNotificationSink_AcceptAll(t *testing.T) {
	sink := NewNotificationSink(nil, nil)
	meta := events.EventMeta{Category: "order", Name: "order.created", Persistent: true}
	if !sink.Accept(meta) {
		t.Error("expected Accept to return true")
	}
}

func TestNotificationSink_PersistentNotification(t *testing.T) {
	db := &mockDB{}
	cap := &notifyCapture{}
	sink := NewNotificationSink(db, cap.notify)

	meta := events.EventMeta{Category: "order", Name: "order.created", Persistent: true}
	evt := &events.NewOrder{OrderID: "ord-1", Title: "Test"}

	err := sink.Handle(context.Background(), meta, evt)
	if err != nil {
		t.Fatalf("Handle error: %v", err)
	}

	if evt.ID == "" {
		t.Error("expected notification ID to be set on event")
	}
	if evt.Typ != "order.created" {
		t.Errorf("expected Typ='order.created', got %q", evt.Typ)
	}

	db.mu.Lock()
	defer db.mu.Unlock()
	if len(db.saved) != 1 {
		t.Fatalf("expected 1 saved record, got %d", len(db.saved))
	}
	rec, ok := db.saved[0].(*models.NotificationRecord)
	if !ok {
		t.Fatalf("expected *models.NotificationRecord, got %T", db.saved[0])
	}
	if rec.Type != "order.created" {
		t.Errorf("expected record type 'order.created', got %q", rec.Type)
	}
	if rec.Read {
		t.Error("expected record to be unread")
	}

	cap.mu.Lock()
	defer cap.mu.Unlock()
	if len(cap.captured) != 1 {
		t.Fatalf("expected 1 WebSocket push, got %d", len(cap.captured))
	}

	raw, err := json.Marshal(cap.captured[0])
	if err != nil {
		t.Fatal(err)
	}
	var wrapped map[string]interface{}
	json.Unmarshal(raw, &wrapped)
	if wrapped["type"] != "notification" {
		t.Errorf("expected WebSocket message type 'notification', got %v", wrapped["type"])
	}
	data, ok := wrapped["data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'data' to be a map")
	}
	if _, ok := data["notification"]; !ok {
		t.Error("expected 'data' to contain 'notification' key")
	}
}

func TestNotificationSink_PersistentNotificationRoutesExplicitTenant(t *testing.T) {
	buyer := &routableMockDB{mockDB: &mockDB{}, tenantID: "buyer"}
	seller := &routableMockDB{mockDB: &mockDB{}, tenantID: "seller"}
	tenants := map[string]*routableMockDB{"buyer": buyer, "seller": seller}
	buyer.tenants = tenants
	seller.tenants = tenants

	buyerCap := &notifyCapture{}
	sellerCap := &notifyCapture{}
	sink := NewTenantAwareNotificationSink(buyer, buyerCap.notify, func(tenantID string) func(any) error {
		if tenantID == "seller" {
			return sellerCap.notify
		}
		return buyerCap.notify
	})

	meta := events.EventMeta{Category: "order", Name: "order.funded", Persistent: true}
	evt := &events.OrderFunded{TenantID: "seller", OrderID: "ord-1", Title: "Test"}

	err := sink.Handle(context.Background(), meta, evt)
	if err != nil {
		t.Fatalf("Handle error: %v", err)
	}

	buyer.mu.Lock()
	buyerSaved := len(buyer.saved)
	buyer.mu.Unlock()
	if buyerSaved != 0 {
		t.Fatalf("expected buyer DB to receive 0 records, got %d", buyerSaved)
	}

	seller.mu.Lock()
	sellerSaved := len(seller.saved)
	seller.mu.Unlock()
	if sellerSaved != 1 {
		t.Fatalf("expected seller DB to receive 1 record, got %d", sellerSaved)
	}

	buyerCap.mu.Lock()
	buyerPushes := len(buyerCap.captured)
	buyerCap.mu.Unlock()
	if buyerPushes != 0 {
		t.Fatalf("expected buyer to receive 0 pushes, got %d", buyerPushes)
	}

	sellerCap.mu.Lock()
	sellerPushes := len(sellerCap.captured)
	sellerCap.mu.Unlock()
	if sellerPushes != 1 {
		t.Fatalf("expected seller to receive 1 push, got %d", sellerPushes)
	}
}

func TestNotificationSink_WebSocketOnly_Cart(t *testing.T) {
	cap := &notifyCapture{}
	sink := NewNotificationSink(nil, cap.notify)

	meta := events.EventMeta{Category: "cart", Name: "cart.updated"}
	evt := &events.ShoppingCartUpdate{ItemsCount: 3}

	err := sink.Handle(context.Background(), meta, evt)
	if err != nil {
		t.Fatalf("Handle error: %v", err)
	}

	cap.mu.Lock()
	defer cap.mu.Unlock()
	if len(cap.captured) != 1 {
		t.Fatalf("expected 1 WebSocket push, got %d", len(cap.captured))
	}

	raw, err := json.Marshal(cap.captured[0])
	if err != nil {
		t.Fatal(err)
	}
	var wrapped map[string]interface{}
	json.Unmarshal(raw, &wrapped)
	if _, ok := wrapped["shoppingCart"]; !ok {
		t.Errorf("expected 'shoppingCart' wrapper, got keys: %v", wrapped)
	}
}

func TestNotificationSink_WebSocketOnly_Wallet(t *testing.T) {
	cap := &notifyCapture{}
	sink := NewNotificationSink(nil, cap.notify)

	meta := events.EventMeta{Category: "wallet", Name: "wallet.tx_received"}
	evt := &events.TransactionReceived{}

	err := sink.Handle(context.Background(), meta, evt)
	if err != nil {
		t.Fatalf("Handle error: %v", err)
	}

	cap.mu.Lock()
	defer cap.mu.Unlock()
	if len(cap.captured) != 1 {
		t.Fatalf("expected 1 push, got %d", len(cap.captured))
	}

	raw, _ := json.Marshal(cap.captured[0])
	var wrapped map[string]interface{}
	json.Unmarshal(raw, &wrapped)
	if _, ok := wrapped["wallet"]; !ok {
		t.Errorf("expected 'wallet' wrapper, got keys: %v", wrapped)
	}
}

func TestNotificationSink_WebSocketOnly_Publish(t *testing.T) {
	cap := &notifyCapture{}
	sink := NewNotificationSink(nil, cap.notify)

	meta := events.EventMeta{Category: "publish", Name: "publish.started"}
	evt := &events.PublishStarted{}

	err := sink.Handle(context.Background(), meta, evt)
	if err != nil {
		t.Fatalf("Handle error: %v", err)
	}

	cap.mu.Lock()
	defer cap.mu.Unlock()
	if len(cap.captured) != 1 {
		t.Fatalf("expected 1 push, got %d", len(cap.captured))
	}

	raw, _ := json.Marshal(cap.captured[0])
	var wrapped map[string]interface{}
	json.Unmarshal(raw, &wrapped)
	if wrapped["status"] != "publishing" {
		t.Errorf("expected status='publishing', got %v", wrapped["status"])
	}
}

func TestNotificationSink_UnknownCategory_NoPush(t *testing.T) {
	cap := &notifyCapture{}
	sink := NewNotificationSink(nil, cap.notify)

	meta := events.EventMeta{Category: "unknown", Name: "unknown.event"}
	err := sink.Handle(context.Background(), meta, struct{}{})
	if err != nil {
		t.Fatalf("Handle error: %v", err)
	}

	cap.mu.Lock()
	defer cap.mu.Unlock()
	if len(cap.captured) != 0 {
		t.Errorf("expected 0 pushes for unknown category, got %d", len(cap.captured))
	}
}

func TestSetNotificationFields_Reflection(t *testing.T) {
	evt := &events.NewOrder{OrderID: "ord-1"}
	setNotificationFields(evt, "id-123", "order.created")
	if evt.ID != "id-123" || evt.Typ != "order.created" {
		t.Errorf("expected ID=id-123 Typ=order.created, got ID=%s Typ=%s", evt.ID, evt.Typ)
	}
}

func TestSetNotificationFields_NonNotificationEvent(t *testing.T) {
	evt := &events.ShoppingCartUpdate{ItemsCount: 1}
	setNotificationFields(evt, "id-123", "cart")
	// ShoppingCartUpdate doesn't embed Notification — should be a no-op
}

func TestSetNotificationFields_NilEvent(t *testing.T) {
	setNotificationFields(nil, "id", "typ")
	// should not panic
}

func TestNotificationSink_Concurrency(t *testing.T) {
	sink := NewNotificationSink(nil, nil)
	if sink.Concurrency() != 1 {
		t.Errorf("expected concurrency 1, got %d", sink.Concurrency())
	}
}

func TestNotificationSink_NilNotifyFunc(t *testing.T) {
	sink := NewNotificationSink(nil, nil)
	meta := events.EventMeta{Category: "cart", Name: "cart.updated"}
	evt := &events.ShoppingCartUpdate{}
	if err := sink.Handle(context.Background(), meta, evt); err != nil {
		t.Fatalf("Handle with nil notify should not error: %v", err)
	}
}
