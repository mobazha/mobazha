package notifications

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/mobazha/mobazha3.0/internal/database/dbstore"
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
	evt := &events.NewOrder{Title: "Test"}

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

func TestNotificationSink_OrderNotificationReplayIsIdempotent(t *testing.T) {
	db, err := dbstore.NewMemoryDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := db.Update(func(tx database.Tx) error {
		return tx.Migrate(&models.NotificationRecord{})
	}); err != nil {
		t.Fatal(err)
	}

	cap := &notifyCapture{}
	sink := NewNotificationSink(db, cap.notify)
	meta := events.EventMeta{Category: "order", Name: "order.funded", Persistent: true}
	first := &events.OrderFunded{OrderID: "gst_replayed", BuyerID: "buyer", Title: "Test"}
	if err := sink.Handle(context.Background(), meta, first); err != nil {
		t.Fatalf("first Handle error: %v", err)
	}
	if first.ID == "" {
		t.Fatal("expected stable notification ID to be set")
	}

	if err := db.Update(func(tx database.Tx) error {
		return tx.Update("read", true, map[string]interface{}{"id = ?": first.ID}, &models.NotificationRecord{})
	}); err != nil {
		t.Fatal(err)
	}

	second := &events.OrderFunded{OrderID: "gst_replayed", BuyerID: "buyer", Title: "Test"}
	if err := sink.Handle(context.Background(), meta, second); err != nil {
		t.Fatalf("second Handle error: %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("expected replay to use same notification ID, got %q want %q", second.ID, first.ID)
	}

	var records []models.NotificationRecord
	if err := db.View(func(tx database.Tx) error {
		return tx.Read().Find(&records).Error
	}); err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("expected one persisted notification after replay, got %d", len(records))
	}
	if !records[0].Read {
		t.Fatal("expected replay to preserve the existing read state")
	}

	cap.mu.Lock()
	pushes := len(cap.captured)
	cap.mu.Unlock()
	if pushes != 1 {
		t.Fatalf("expected one websocket push after replay, got %d", pushes)
	}
}

func TestDeterministicNotificationIDUsesEntityFieldByCategory(t *testing.T) {
	type syntheticOrderNotification struct {
		events.Notification
		OrderID string
	}

	meta := events.EventMeta{Category: "order", Name: "order.synthetic", Persistent: true}
	id, ok := deterministicNotificationID(meta, syntheticOrderNotification{OrderID: "order-1"})
	if !ok {
		t.Fatal("expected order notification with OrderID to be deduplicated")
	}

	replayID, ok := deterministicNotificationID(meta, &syntheticOrderNotification{OrderID: "order-1"})
	if !ok {
		t.Fatal("expected pointer event with OrderID to be deduplicated")
	}
	if replayID != id {
		t.Fatalf("expected same deterministic ID for value and pointer, got %q and %q", id, replayID)
	}
}

func TestDeterministicNotificationIDIgnoresUnknownCategory(t *testing.T) {
	type syntheticNotification struct {
		events.Notification
		OrderID string
	}

	meta := events.EventMeta{Category: "catalog", Name: "catalog.synthetic", Persistent: true}
	if id, ok := deterministicNotificationID(meta, syntheticNotification{OrderID: "order-1"}); ok {
		t.Fatalf("expected unknown category to skip deterministic ID, got %q", id)
	}
}

func TestNotificationSink_DifferentOrderNotificationTypesRemainDistinct(t *testing.T) {
	db, err := dbstore.NewMemoryDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := db.Update(func(tx database.Tx) error {
		return tx.Migrate(&models.NotificationRecord{})
	}); err != nil {
		t.Fatal(err)
	}

	cap := &notifyCapture{}
	sink := NewNotificationSink(db, cap.notify)
	orderID := "gst_distinct_lifecycle"
	eventsToPersist := []struct {
		meta  events.EventMeta
		event any
	}{
		{
			meta:  events.EventMeta{Category: "order", Name: "order.funded", Persistent: true},
			event: &events.OrderFunded{OrderID: orderID, BuyerID: "buyer", Title: "Test"},
		},
		{
			meta:  events.EventMeta{Category: "order", Name: "order.confirmed", Persistent: true},
			event: &events.OrderConfirmation{OrderID: orderID, VendorID: "seller"},
		},
	}

	for _, item := range eventsToPersist {
		if err := sink.Handle(context.Background(), item.meta, item.event); err != nil {
			t.Fatalf("Handle error for %s: %v", item.meta.Name, err)
		}
	}

	var records []models.NotificationRecord
	if err := db.View(func(tx database.Tx) error {
		return tx.Read().Find(&records).Error
	}); err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 {
		t.Fatalf("expected different lifecycle notification types to persist separately, got %d", len(records))
	}
}

func TestNotificationSink_OrderConfirmationConcurrentReplayIsIdempotent(t *testing.T) {
	db, err := dbstore.NewMemoryDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := db.Update(func(tx database.Tx) error {
		return tx.Migrate(&models.NotificationRecord{})
	}); err != nil {
		t.Fatal(err)
	}

	cap := &notifyCapture{}
	sink := NewNotificationSink(db, cap.notify)
	meta := events.EventMeta{Category: "order", Name: "order.confirmed", Persistent: true}

	const workers = 8
	errCh := make(chan error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errCh <- sink.Handle(
				context.Background(),
				meta,
				&events.OrderConfirmation{OrderID: "gst_concurrent_replay", VendorID: "seller"},
			)
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("Handle error: %v", err)
		}
	}

	var records []models.NotificationRecord
	if err := db.View(func(tx database.Tx) error {
		return tx.Read().Find(&records).Error
	}); err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("expected one persisted notification after concurrent replay, got %d", len(records))
	}

	cap.mu.Lock()
	pushes := len(cap.captured)
	cap.mu.Unlock()
	if pushes != 1 {
		t.Fatalf("expected one websocket push after concurrent replay, got %d", pushes)
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
	evt := &events.OrderFunded{TenantID: "seller", Title: "Test"}

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

func TestNotificationSink_PersistentNotificationIgnoresTenantOnNonRoutableDB(t *testing.T) {
	db := &mockDB{}
	localCap := &notifyCapture{}
	routedCap := &notifyCapture{}
	sink := NewTenantAwareNotificationSink(db, localCap.notify, func(string) func(any) error {
		return routedCap.notify
	})

	meta := events.EventMeta{Category: "order", Name: "order.funded", Persistent: true}
	evt := &events.OrderFunded{TenantID: database.StandaloneTenantID, Title: "Test"}

	err := sink.Handle(context.Background(), meta, evt)
	if err != nil {
		t.Fatalf("Handle error: %v", err)
	}

	db.mu.Lock()
	saved := len(db.saved)
	db.mu.Unlock()
	if saved != 1 {
		t.Fatalf("expected local DB to receive 1 record, got %d", saved)
	}

	localCap.mu.Lock()
	localPushes := len(localCap.captured)
	localCap.mu.Unlock()
	if localPushes != 1 {
		t.Fatalf("expected local websocket to receive 1 push, got %d", localPushes)
	}

	routedCap.mu.Lock()
	routedPushes := len(routedCap.captured)
	routedCap.mu.Unlock()
	if routedPushes != 0 {
		t.Fatalf("expected tenant-routed websocket to receive 0 pushes, got %d", routedPushes)
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
