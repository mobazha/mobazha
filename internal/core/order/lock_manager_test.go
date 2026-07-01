package order

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrderLockManager_BasicLockUnlock(t *testing.T) {
	m := NewOrderLockManager()
	defer m.Stop()

	err := m.Lock(context.Background(), "tenant1", "order1")
	require.NoError(t, err)

	assert.Equal(t, 1, m.ActiveLockCount())

	m.Unlock("tenant1", "order1")
}

func TestOrderLockManager_DifferentOrdersDoNotBlock(t *testing.T) {
	m := NewOrderLockManager()
	defer m.Stop()

	err := m.Lock(context.Background(), "tenant1", "order1")
	require.NoError(t, err)

	err = m.Lock(context.Background(), "tenant1", "order2")
	require.NoError(t, err)

	m.Unlock("tenant1", "order1")
	m.Unlock("tenant1", "order2")
}

func TestOrderLockManager_DifferentTenantsDoNotBlock(t *testing.T) {
	m := NewOrderLockManager()
	defer m.Stop()

	err := m.Lock(context.Background(), "tenant1", "order1")
	require.NoError(t, err)

	err = m.Lock(context.Background(), "tenant2", "order1")
	require.NoError(t, err)

	m.Unlock("tenant1", "order1")
	m.Unlock("tenant2", "order1")
}

func TestOrderLockManager_SameOrderBlocks(t *testing.T) {
	m := NewOrderLockManager()
	defer m.Stop()

	err := m.Lock(context.Background(), "tenant1", "order1")
	require.NoError(t, err)

	var acquired int32
	go func() {
		_ = m.Lock(context.Background(), "tenant1", "order1")
		atomic.StoreInt32(&acquired, 1)
		m.Unlock("tenant1", "order1")
	}()

	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, int32(0), atomic.LoadInt32(&acquired))

	m.Unlock("tenant1", "order1")

	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, int32(1), atomic.LoadInt32(&acquired))
}

func TestOrderLockManager_ContextCancellation(t *testing.T) {
	m := NewOrderLockManager()
	defer m.Stop()

	err := m.Lock(context.Background(), "tenant1", "order1")
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err = m.Lock(ctx, "tenant1", "order1")
	assert.ErrorIs(t, err, context.Canceled)

	m.Unlock("tenant1", "order1")
}

func TestOrderLockManager_ConcurrentAccess(t *testing.T) {
	m := NewOrderLockManager()
	defer m.Stop()

	var counter int64
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := m.Lock(context.Background(), "tenant1", "order1")
			if err != nil {
				return
			}
			atomic.AddInt64(&counter, 1)
			time.Sleep(time.Microsecond)
			m.Unlock("tenant1", "order1")
		}()
	}

	wg.Wait()
	assert.Equal(t, int64(100), counter)
}

func TestOrderLockManager_UnlockNonExistentKey(t *testing.T) {
	m := NewOrderLockManager()
	defer m.Stop()

	m.Unlock("tenant1", "nonexistent")
}

func TestOrderLockManager_Stop(t *testing.T) {
	m := NewOrderLockManager()

	err := m.Lock(context.Background(), "tenant1", "order1")
	require.NoError(t, err)
	m.Unlock("tenant1", "order1")

	m.Stop()
}
