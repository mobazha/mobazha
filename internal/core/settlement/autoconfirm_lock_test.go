//go:build !private_distribution

package settlement

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestTryLockAutoConfirm_SingleOrder(t *testing.T) {
	svc := &SettlementService{nodeID: "test-lock"}

	unlock := svc.TryLockAutoConfirm("order-1")
	if unlock == nil {
		t.Fatal("first TryLockAutoConfirm should succeed")
	}

	unlock2 := svc.TryLockAutoConfirm("order-1")
	if unlock2 != nil {
		t.Fatal("second TryLockAutoConfirm for same order should return nil")
	}

	unlock()
	unlock3 := svc.TryLockAutoConfirm("order-1")
	if unlock3 == nil {
		t.Fatal("TryLockAutoConfirm should succeed after unlock")
	}
	unlock3()
}

func TestTryLockAutoConfirm_DifferentOrders(t *testing.T) {
	svc := &SettlementService{nodeID: "test-lock"}

	unlock1 := svc.TryLockAutoConfirm("order-1")
	if unlock1 == nil {
		t.Fatal("lock for order-1 should succeed")
	}
	defer unlock1()

	unlock2 := svc.TryLockAutoConfirm("order-2")
	if unlock2 == nil {
		t.Fatal("lock for order-2 should succeed while order-1 is locked")
	}
	defer unlock2()
}

func TestTryLockAutoConfirm_ConcurrentSafety(t *testing.T) {
	svc := &SettlementService{nodeID: "test-lock"}
	const orderID = "concurrent-order"

	var lockCount int32
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			unlock := svc.TryLockAutoConfirm(orderID)
			if unlock != nil {
				atomic.AddInt32(&lockCount, 1)
				unlock()
			}
		}()
	}
	wg.Wait()

	if lockCount == 0 {
		t.Fatal("no goroutine was able to acquire the lock")
	}
	t.Logf("concurrent test: %d out of 100 goroutines acquired the lock", lockCount)
}
