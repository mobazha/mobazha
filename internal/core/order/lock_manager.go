package order

import (
	"context"
	"fmt"
	"sync"
	"time"
)

const (
	orderLockTimeout    = 30 * time.Second
	orderLockIdleExpiry = 1 * time.Hour
)

type lockEntry struct {
	mu       sync.Mutex
	lastUsed time.Time
}

// OrderLockManager provides per-order mutual exclusion to prevent
// concurrent processing of messages for the same order.
// While the DB-level mutex already serializes all writes per tenant,
// this lock adds defense-in-depth and prepares for future row-level locking.
type OrderLockManager struct {
	locks sync.Map
}

func NewOrderLockManager() *OrderLockManager {
	return &OrderLockManager{}
}

// Lock acquires the per-order lock with a timeout.
// Returns a context.Canceled error if the provided context is cancelled,
// or an error if the lock cannot be acquired within the timeout.
func (m *OrderLockManager) Lock(ctx context.Context, tenantID, orderID string) error {
	key := m.lockKey(tenantID, orderID)
	entry := m.getOrCreate(key)

	acquired := make(chan struct{})
	go func() {
		entry.mu.Lock()
		close(acquired)
	}()

	timer := time.NewTimer(orderLockTimeout)
	defer timer.Stop()

	select {
	case <-acquired:
		entry.lastUsed = time.Now()
		return nil
	case <-ctx.Done():
		go func() {
			<-acquired
			entry.mu.Unlock()
		}()
		return ctx.Err()
	case <-timer.C:
		go func() {
			<-acquired
			entry.mu.Unlock()
		}()
		return fmt.Errorf("order lock timeout after %s for %s", orderLockTimeout, key)
	}
}

// Unlock releases the per-order lock.
func (m *OrderLockManager) Unlock(tenantID, orderID string) {
	key := m.lockKey(tenantID, orderID)
	val, ok := m.locks.Load(key)
	if !ok {
		return
	}
	entry := val.(*lockEntry)
	entry.lastUsed = time.Now()
	entry.mu.Unlock()
}

// Stop is a no-op retained for interface compatibility.
// Cleanup is now driven by the shared scheduler.
func (m *OrderLockManager) Stop() {}

func (m *OrderLockManager) lockKey(tenantID, orderID string) string {
	return tenantID + ":" + orderID
}

func (m *OrderLockManager) getOrCreate(key string) *lockEntry {
	val, loaded := m.locks.LoadOrStore(key, &lockEntry{lastUsed: time.Now()})
	if loaded {
		return val.(*lockEntry)
	}
	return val.(*lockEntry)
}

// RunLockCleanupOnce executes a single pass of idle lock cleanup.
// Called by the shared scheduler (process-wide GlobalFn).
func (m *OrderLockManager) RunLockCleanupOnce() {
	m.cleanupIdle()
}

func (m *OrderLockManager) cleanupIdle() {
	cutoff := time.Now().Add(-orderLockIdleExpiry)
	removed := 0
	m.locks.Range(func(key, value any) bool {
		entry := value.(*lockEntry)
		if entry.lastUsed.Before(cutoff) {
			if entry.mu.TryLock() {
				m.locks.Delete(key)
				entry.mu.Unlock()
				removed++
			}
		}
		return true
	})
	if removed > 0 {
		log.Infof("OrderLockManager: cleaned up %d idle locks", removed)
	}
}

// ActiveLockCount returns the number of tracked locks (for testing).
func (m *OrderLockManager) ActiveLockCount() int {
	count := 0
	m.locks.Range(func(_, _ any) bool {
		count++
		return true
	})
	return count
}
