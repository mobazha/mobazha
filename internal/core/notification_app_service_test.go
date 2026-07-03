package core

import (
	"fmt"
	"testing"
	"time"

	"github.com/mobazha/mobazha/internal/repo"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestNotificationAppService(t *testing.T) *NotificationAppService {
	t.Helper()
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return NewNotificationAppService(NotificationAppServiceConfig{DB: db})
}

func seedNotification(t *testing.T, svc *NotificationAppService, id, typ string, read bool, ts time.Time) {
	t.Helper()
	rec := &models.NotificationRecord{
		ID:           id,
		Timestamp:    ts,
		Read:         read,
		Type:         typ,
		Notification: []byte(`{"summary":"test"}`),
	}
	err := svc.db.Update(func(tx database.Tx) error { return tx.Save(rec) })
	require.NoError(t, err)
}

// ── GetNotifications ────────────────────────────────────────────

func TestNotificationAppService_GetNotifications_EmptyDB(t *testing.T) {
	svc := newTestNotificationAppService(t)
	notifs, total, err := svc.GetNotifications("", 10, nil)
	require.NoError(t, err)
	assert.Empty(t, notifs)
	assert.Equal(t, int64(0), total)
}

func TestNotificationAppService_GetNotifications_ReturnsAllDescending(t *testing.T) {
	svc := newTestNotificationAppService(t)
	now := time.Now().Truncate(time.Second)
	for i := 0; i < 5; i++ {
		seedNotification(t, svc, fmt.Sprintf("n%d", i), "order", false, now.Add(time.Duration(i)*time.Minute))
	}
	notifs, total, err := svc.GetNotifications("", 10, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(5), total)
	assert.Len(t, notifs, 5)
	assert.Equal(t, "n4", notifs[0].ID)
	assert.Equal(t, "n0", notifs[4].ID)
}

func TestNotificationAppService_GetNotifications_WithLimit(t *testing.T) {
	svc := newTestNotificationAppService(t)
	now := time.Now().Truncate(time.Second)
	for i := 0; i < 5; i++ {
		seedNotification(t, svc, fmt.Sprintf("n%d", i), "order", false, now.Add(time.Duration(i)*time.Minute))
	}
	notifs, total, err := svc.GetNotifications("", 3, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(5), total)
	assert.Len(t, notifs, 3)
}

func TestNotificationAppService_GetNotifications_WithOffset(t *testing.T) {
	svc := newTestNotificationAppService(t)
	now := time.Now().Truncate(time.Second)
	for i := 0; i < 5; i++ {
		seedNotification(t, svc, fmt.Sprintf("n%d", i), "order", false, now.Add(time.Duration(i)*time.Minute))
	}
	notifs, _, err := svc.GetNotifications("n3", 10, nil)
	require.NoError(t, err)
	assert.Len(t, notifs, 3)
	assert.Equal(t, "n2", notifs[0].ID)
}

func TestNotificationAppService_GetNotifications_WithTypeFilter(t *testing.T) {
	svc := newTestNotificationAppService(t)
	now := time.Now().Truncate(time.Second)
	seedNotification(t, svc, "n1", "order", false, now)
	seedNotification(t, svc, "n2", "payment", false, now.Add(time.Minute))
	seedNotification(t, svc, "n3", "order", false, now.Add(2*time.Minute))
	seedNotification(t, svc, "n4", "chat", false, now.Add(3*time.Minute))

	notifs, total, err := svc.GetNotifications("", 10, []string{"order"})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, notifs, 2)
	for _, n := range notifs {
		assert.Equal(t, "order", n.Type)
	}
}

// ── MarkNotificationAsRead ──────────────────────────────────────

func TestNotificationAppService_MarkNotificationAsRead(t *testing.T) {
	svc := newTestNotificationAppService(t)
	now := time.Now().Truncate(time.Second)
	seedNotification(t, svc, "n1", "order", false, now)

	require.NoError(t, svc.MarkNotificationAsRead("n1"))

	notifs, _, err := svc.GetNotifications("", 10, nil)
	require.NoError(t, err)
	assert.True(t, notifs[0].Read)
}

func TestNotificationAppService_MarkNotificationAsRead_AlreadyRead(t *testing.T) {
	svc := newTestNotificationAppService(t)
	now := time.Now().Truncate(time.Second)
	seedNotification(t, svc, "n1", "order", true, now)
	require.NoError(t, svc.MarkNotificationAsRead("n1"))
}

func TestNotificationAppService_MarkNotificationAsRead_NotFound(t *testing.T) {
	svc := newTestNotificationAppService(t)
	err := svc.MarkNotificationAsRead("nonexistent")
	assert.Error(t, err)
}

// ── MarkAllNotificationsAsRead ──────────────────────────────────

func TestNotificationAppService_MarkAllNotificationsAsRead(t *testing.T) {
	svc := newTestNotificationAppService(t)
	now := time.Now().Truncate(time.Second)
	seedNotification(t, svc, "n1", "order", false, now)
	seedNotification(t, svc, "n2", "chat", false, now.Add(time.Minute))

	require.NoError(t, svc.MarkAllNotificationsAsRead())

	unread, err := svc.GetNotificationsUnreadCount()
	require.NoError(t, err)
	assert.Equal(t, 0, unread)
}

// ── GetNotificationsUnreadCount ─────────────────────────────────

func TestNotificationAppService_GetNotificationsUnreadCount(t *testing.T) {
	svc := newTestNotificationAppService(t)
	now := time.Now().Truncate(time.Second)
	seedNotification(t, svc, "n1", "order", false, now)
	seedNotification(t, svc, "n2", "order", true, now.Add(time.Minute))
	seedNotification(t, svc, "n3", "chat", false, now.Add(2*time.Minute))

	count, err := svc.GetNotificationsUnreadCount()
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestNotificationAppService_GetNotificationsUnreadCount_AllRead(t *testing.T) {
	svc := newTestNotificationAppService(t)
	now := time.Now().Truncate(time.Second)
	seedNotification(t, svc, "n1", "order", true, now)

	count, err := svc.GetNotificationsUnreadCount()
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

// ── GetNotificationsTotalCount ──────────────────────────────────

func TestNotificationAppService_GetNotificationsTotalCount(t *testing.T) {
	svc := newTestNotificationAppService(t)
	now := time.Now().Truncate(time.Second)
	seedNotification(t, svc, "n1", "order", false, now)
	seedNotification(t, svc, "n2", "chat", true, now.Add(time.Minute))

	total, err := svc.GetNotificationsTotalCount()
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
}

// ── BatchMarkNotificationsAsRead ────────────────────────────────

func TestNotificationAppService_BatchMarkNotificationsAsRead(t *testing.T) {
	svc := newTestNotificationAppService(t)
	now := time.Now().Truncate(time.Second)
	seedNotification(t, svc, "n1", "order", false, now)
	seedNotification(t, svc, "n2", "chat", false, now.Add(time.Minute))
	seedNotification(t, svc, "n3", "payment", false, now.Add(2*time.Minute))

	require.NoError(t, svc.BatchMarkNotificationsAsRead([]string{"n1", "n3"}))

	unread, err := svc.GetNotificationsUnreadCount()
	require.NoError(t, err)
	assert.Equal(t, 1, unread)
}

func TestNotificationAppService_BatchMarkNotificationsAsRead_Empty(t *testing.T) {
	svc := newTestNotificationAppService(t)
	require.NoError(t, svc.BatchMarkNotificationsAsRead(nil))
}

// ── BatchDeleteNotifications ────────────────────────────────────

func TestNotificationAppService_BatchDeleteNotifications(t *testing.T) {
	svc := newTestNotificationAppService(t)
	now := time.Now().Truncate(time.Second)
	seedNotification(t, svc, "n1", "order", false, now)
	seedNotification(t, svc, "n2", "chat", false, now.Add(time.Minute))
	seedNotification(t, svc, "n3", "payment", false, now.Add(2*time.Minute))

	require.NoError(t, svc.BatchDeleteNotifications([]string{"n1", "n2"}))

	total, err := svc.GetNotificationsTotalCount()
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
}

func TestNotificationAppService_BatchDeleteNotifications_Empty(t *testing.T) {
	svc := newTestNotificationAppService(t)
	require.NoError(t, svc.BatchDeleteNotifications(nil))
}
