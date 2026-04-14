package core

import (
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/repo"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── mock netDBWriter ─────────────────────────────────────────────

type mockNetDBWriter struct {
	mu   sync.Mutex
	calls map[string]int
	err   error // if set, all methods return this error
}

func newMockNetDBWriter() *mockNetDBWriter {
	return &mockNetDBWriter{calls: make(map[string]int)}
}

func (m *mockNetDBWriter) record(method string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls[method]++
	return m.err
}

func (m *mockNetDBWriter) count(method string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls[method]
}

func (m *mockNetDBWriter) SetOwnProfile(_ *models.Profile) error          { return m.record("SetOwnProfile") }
func (m *mockNetDBWriter) SetOwnListing(_ *pb.SignedListing) error        { return m.record("SetOwnListing") }
func (m *mockNetDBWriter) SetOwnListingIndex(_ models.ListingIndex) error { return m.record("SetOwnListingIndex") }
func (m *mockNetDBWriter) DeleteOwnListing(_ string) error                { return m.record("DeleteOwnListing") }
func (m *mockNetDBWriter) SetOwnFollowing(_ models.Following) error       { return m.record("SetOwnFollowing") }
func (m *mockNetDBWriter) SetOwnFollowers(_ models.Followers) error       { return m.record("SetOwnFollowers") }
func (m *mockNetDBWriter) SetOwnStoreMetadata(_ string, _ json.RawMessage) error {
	return m.record("SetOwnStoreMetadata")
}
func (m *mockNetDBWriter) SetOwnRatingIndex(_ models.RatingIndex) error { return m.record("SetOwnRatingIndex") }
func (m *mockNetDBWriter) SetOwnRating(_ string, _ json.RawMessage) error {
	return m.record("SetOwnRating")
}

// ── helpers ──────────────────────────────────────────────────────

func newTestDB(t *testing.T) database.Database {
	t.Helper()
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	require.NoError(t, MigrateNodeSettings(db))
	return db
}

func seedSyncProfile(t *testing.T, db database.Database) {
	t.Helper()
	err := db.Update(func(tx database.Tx) error {
		return tx.SetProfile(&models.Profile{Name: "Test Store"})
	})
	require.NoError(t, err)
}

func seedSyncFollowing(t *testing.T, db database.Database) {
	t.Helper()
	err := db.Update(func(tx database.Tx) error {
		return tx.SetFollowing(models.Following{"peer1", "peer2"})
	})
	require.NoError(t, err)
}

func seedSyncFollowers(t *testing.T, db database.Database) {
	t.Helper()
	err := db.Update(func(tx database.Tx) error {
		return tx.SetFollowers(models.Followers{"follower1"})
	})
	require.NoError(t, err)
}

func newSyncService(db database.Database, writer *mockNetDBWriter, bus events.Bus) *NetDBSyncService {
	return NewNetDBSyncService(NetDBSyncServiceConfig{
		NetDB:    writer,
		DB:       db,
		EventBus: bus,
		NodeID:   "test-sync",
	})
}

// ── Tests ────────────────────────────────────────────────────────

func TestNetDBSync_Dispatch_ProfileChanged(t *testing.T) {
	db := newTestDB(t)
	seedSyncProfile(t, db)
	writer := newMockNetDBWriter()

	svc := newSyncService(db, writer, nil)
	svc.dispatch(&events.ProfileChanged{})

	assert.Equal(t, 1, writer.count("SetOwnProfile"))
}

func TestNetDBSync_Dispatch_FollowingChanged(t *testing.T) {
	db := newTestDB(t)
	seedSyncProfile(t, db)
	seedSyncFollowing(t, db)
	writer := newMockNetDBWriter()

	svc := newSyncService(db, writer, nil)
	svc.dispatch(&events.FollowingChanged{})

	assert.Equal(t, 1, writer.count("SetOwnFollowing"))
	assert.Equal(t, 1, writer.count("SetOwnProfile"), "FollowingChanged should also push profile")
}

func TestNetDBSync_Dispatch_FollowersChanged(t *testing.T) {
	db := newTestDB(t)
	seedSyncProfile(t, db)
	seedSyncFollowers(t, db)
	writer := newMockNetDBWriter()

	svc := newSyncService(db, writer, nil)
	svc.dispatch(&events.FollowersChanged{})

	assert.Equal(t, 1, writer.count("SetOwnFollowers"))
	assert.Equal(t, 1, writer.count("SetOwnProfile"), "FollowersChanged should also push profile")
}

func TestNetDBSync_Dispatch_ListingDeleted(t *testing.T) {
	writer := newMockNetDBWriter()
	db := newTestDB(t)
	svc := newSyncService(db, writer, nil)

	svc.dispatch(&events.ListingDeleted{Cid: "QmTest123"})
	assert.Equal(t, 1, writer.count("DeleteOwnListing"))

	svc.dispatch(&events.ListingDeleted{Cid: ""})
	assert.Equal(t, 1, writer.count("DeleteOwnListing"), "empty CID should skip delete")
}

func TestNetDBSync_Dispatch_StorefrontChanged(t *testing.T) {
	writer := newMockNetDBWriter()
	db := newTestDB(t)
	svc := newSyncService(db, writer, nil)

	cfg := json.RawMessage(`{"theme":"dark"}`)
	svc.dispatch(&events.StorefrontChanged{Config: cfg})

	assert.Equal(t, 1, writer.count("SetOwnStoreMetadata"))
}

func TestNetDBSync_DirtyFlag_Cycle(t *testing.T) {
	db := newTestDB(t)
	writer := newMockNetDBWriter()
	svc := newSyncService(db, writer, nil)

	svc.markDirty("profile")
	svc.markDirty("following")

	dirty := svc.allDirtyKeys()
	assert.Contains(t, dirty, "profile")
	assert.Contains(t, dirty, "following")
	assert.NotContains(t, dirty, "followers")

	svc.clearDirty("profile")
	dirty = svc.allDirtyKeys()
	assert.NotContains(t, dirty, "profile")
	assert.Contains(t, dirty, "following")

	svc.clearDirty("following")
	dirty = svc.allDirtyKeys()
	assert.Empty(t, dirty)
}

func TestNetDBSync_MarkDirty_OnPushFailure(t *testing.T) {
	db := newTestDB(t)
	seedSyncProfile(t, db)
	writer := newMockNetDBWriter()
	writer.err = errors.New("network error")

	svc := newSyncService(db, writer, nil)
	svc.dispatch(&events.ProfileChanged{})

	assert.Equal(t, 1, writer.count("SetOwnProfile"))
	dirty := svc.allDirtyKeys()
	assert.Contains(t, dirty, "profile", "profile should be marked dirty on push failure")
}

func TestNetDBSync_Reconcile_PushesOnlyDirty(t *testing.T) {
	db := newTestDB(t)
	seedSyncProfile(t, db)
	seedSyncFollowing(t, db)
	seedSyncFollowers(t, db)
	writer := newMockNetDBWriter()
	svc := newSyncService(db, writer, nil)

	svc.markDirty("profile")
	svc.markDirty("following")

	svc.Reconcile()

	assert.Equal(t, 1, writer.count("SetOwnProfile"))
	assert.Equal(t, 1, writer.count("SetOwnFollowing"))
	assert.Equal(t, 0, writer.count("SetOwnFollowers"), "followers not dirty, should not push")

	dirty := svc.allDirtyKeys()
	assert.Empty(t, dirty, "successful reconcile should clear all dirty flags")
}

func TestNetDBSync_StartStop(t *testing.T) {
	db := newTestDB(t)
	seedSyncProfile(t, db)
	writer := newMockNetDBWriter()
	bus := events.NewBus()

	svc := newSyncService(db, writer, bus)
	svc.Start()

	bus.Emit(&events.ProfileChanged{})
	time.Sleep(50 * time.Millisecond)

	svc.Stop()
	assert.GreaterOrEqual(t, writer.count("SetOwnProfile"), 1)
}
