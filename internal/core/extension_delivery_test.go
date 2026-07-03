package core

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mobazha/mobazha/internal/repo"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/extensions"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/stretchr/testify/require"
)

type extensionControllerFunc func(context.Context, extensions.Event) error

func (f extensionControllerFunc) HandleExtensionEvent(ctx context.Context, event extensions.Event) error {
	return f(ctx, event)
}

type extensionControllerModule struct{ controller extensions.Controller }

func (extensionControllerModule) Descriptor() extensions.ModuleDescriptor {
	return extensions.ModuleDescriptor{ID: "io.mobazha.test", Version: "1.0.0", Contracts: []string{extensions.ContractOrderExtensionDeliveryV1}}
}
func (m extensionControllerModule) Controller() extensions.Controller { return m.controller }

func TestExtensionDelivery_RetriesModuleControllerAndAcknowledges(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Update(func(tx database.Tx) error { return tx.Migrate(&models.ExtensionDelivery{}) }))
	require.NoError(t, db.Update(func(tx database.Tx) error {
		return tx.Save(&models.ExtensionDelivery{TenantID: database.StandaloneTenantID, EventID: "evt-1", SourceID: "peer-1", OrderRole: "vendor", ProviderID: "io.mobazha.test", EventType: "order.paid", EventVersion: "v1", OrderID: "order-1", OrderVersion: 1, ExtensionID: "ext-1", IdempotencyKey: "evt-1", CreatedAt: time.Now().UTC()})
	}))

	calls := 0
	module := extensionControllerModule{controller: extensionControllerFunc(func(_ context.Context, event extensions.Event) error {
		calls++
		require.Equal(t, "evt-1", event.EventID)
		if calls == 1 {
			return errors.New("temporary failure")
		}
		return nil
	})}
	node := &MobazhaNode{storageFields: storageFields{db: db}, orderExtensionFields: orderExtensionFields{orderExtensionModules: mustRegisterOrderExtensionModules(t, module)}}
	node.runExtensionDeliveries(context.Background())
	require.NoError(t, db.Update(func(tx database.Tx) error {
		_, err := tx.UpdateColumns(map[string]interface{}{"next_attempt_at": nil}, map[string]interface{}{"event_id = ?": "evt-1"}, &models.ExtensionDelivery{})
		return err
	}))
	node.runExtensionDeliveries(context.Background())

	var delivery models.ExtensionDelivery
	require.NoError(t, db.View(func(tx database.Tx) error { return tx.Read().Where("event_id = ?", "evt-1").First(&delivery).Error }))
	require.Equal(t, 2, calls)
	require.Equal(t, 2, delivery.Attempts)
	require.NotNil(t, delivery.DeliveredAt)
	require.Nil(t, delivery.DeadLetteredAt)
	require.Empty(t, delivery.LeaseOwner)
	require.Nil(t, delivery.LeaseExpiresAt)
}

func TestExtensionDelivery_ControllerCanReenterDeliveryLoop(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	require.NoError(t, db.Update(func(tx database.Tx) error { return tx.Migrate(&models.ExtensionDelivery{}) }))
	require.NoError(t, db.Update(func(tx database.Tx) error {
		return tx.Save(&models.ExtensionDelivery{TenantID: database.StandaloneTenantID, EventID: "evt-reenter", SourceID: "peer-1", OrderRole: "vendor", ProviderID: "io.mobazha.test", EventType: "order.paid", EventVersion: "v1", OrderID: "order-1", OrderVersion: 1, ExtensionID: "ext-1", IdempotencyKey: "evt-reenter", CreatedAt: time.Now().UTC()})
	}))

	var node *MobazhaNode
	calls := 0
	module := extensionControllerModule{controller: extensionControllerFunc(func(ctx context.Context, _ extensions.Event) error {
		calls++
		node.runExtensionDeliveries(ctx)
		return nil
	})}
	node = &MobazhaNode{storageFields: storageFields{db: db}, orderExtensionFields: orderExtensionFields{orderExtensionModules: mustRegisterOrderExtensionModules(t, module)}}
	node.runExtensionDeliveries(context.Background())
	require.Equal(t, 1, calls)
}

func TestExtensionDelivery_ActiveLeasePreventsConcurrentClaim(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	require.NoError(t, db.Update(func(tx database.Tx) error { return tx.Migrate(&models.ExtensionDelivery{}) }))
	require.NoError(t, db.Update(func(tx database.Tx) error {
		return tx.Save(&models.ExtensionDelivery{TenantID: database.StandaloneTenantID, EventID: "evt-lease", SourceID: "peer-1", OrderRole: "vendor", ProviderID: "io.mobazha.test", EventType: "order.paid", EventVersion: "v1", OrderID: "order-1", OrderVersion: 1, ExtensionID: "ext-1", IdempotencyKey: "evt-lease", CreatedAt: time.Now().UTC()})
	}))

	started := make(chan struct{})
	release := make(chan struct{})
	calls := 0
	module := extensionControllerModule{controller: extensionControllerFunc(func(context.Context, extensions.Event) error {
		calls++
		close(started)
		<-release
		return nil
	})}
	node := &MobazhaNode{storageFields: storageFields{db: db}, orderExtensionFields: orderExtensionFields{orderExtensionModules: mustRegisterOrderExtensionModules(t, module)}}
	done := make(chan struct{})
	go func() {
		defer close(done)
		node.runExtensionDeliveries(context.Background())
	}()
	<-started
	node.runExtensionDeliveries(context.Background())
	require.Equal(t, 1, calls)
	close(release)
	<-done
}

func mustRegisterOrderExtensionModules(t *testing.T, modules ...extensions.Module) []registeredOrderExtensionModule {
	t.Helper()
	registered, err := snapshotOrderExtensionModules(modules)
	require.NoError(t, err)
	return registered
}
