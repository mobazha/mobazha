package webhook

import (
	"sync/atomic"
	"testing"
	"time"
)

// fakeStore is a minimal EndpointStore stub for engine option tests.
// Only methods exercised by deliveryWorker / cleanupWorker need real
// behaviour; the rest panic so an accidental call is loud.
type fakeStore struct {
	getPendingCalls atomic.Int64
	cleanupCalls    atomic.Int64
}

func (s *fakeStore) GetPending(limit int) ([]Delivery, error) {
	s.getPendingCalls.Add(1)
	return nil, nil
}

func (s *fakeStore) CleanupOld(d time.Duration) (int64, error) {
	s.cleanupCalls.Add(1)
	return 0, nil
}

// Unused interface methods.
func (s *fakeStore) ListActive() ([]Endpoint, error) { panic("unused") }
func (s *fakeStore) GetEndpoint(id string) (*Endpoint, error) {
	panic("unused")
}
func (s *fakeStore) CreateEndpoint(ep *Endpoint) error { panic("unused") }
func (s *fakeStore) UpdateEndpoint(id string, updates map[string]interface{}) error {
	panic("unused")
}
func (s *fakeStore) DeleteEndpoint(id string) error    { panic("unused") }
func (s *fakeStore) ListEndpoints() ([]Endpoint, error) { panic("unused") }
func (s *fakeStore) CountEndpoints() (int64, error)     { panic("unused") }
func (s *fakeStore) CreateDeliveries(d []Delivery) error {
	panic("unused")
}
func (s *fakeStore) UpdateResult(id string, r DeliveryResult) error {
	panic("unused")
}
func (s *fakeStore) ListDeliveries(endpointID, status string, limit, offset int) ([]Delivery, int64, error) {
	panic("unused")
}

// shortPollConfig is a Config tuned for fast option-suppression tests.
// PollInterval=20ms so deliveryWorker (if started) will tick within 100ms.
func shortPollConfig() Config {
	c := DefaultConfig()
	c.PollInterval = 20 * time.Millisecond
	return c
}

// TestNewEngine_DefaultStartsDeliveryWorker confirms baseline behaviour:
// without options, NewEngine starts the internal deliveryWorker (which
// polls store.GetPending every PollInterval).
func TestNewEngine_DefaultStartsDeliveryWorker(t *testing.T) {
	store := &fakeStore{}
	e := NewEngine(store, shortPollConfig())
	defer e.Stop()

	// Wait for at least one poll tick.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if store.getPendingCalls.Load() > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got := store.getPendingCalls.Load(); got == 0 {
		t.Fatalf("expected deliveryWorker to call GetPending; got 0 calls")
	}
	if e.skipDeliveryWorker {
		t.Fatalf("skipDeliveryWorker should be false by default")
	}
}

// TestWithoutDeliveryWorker confirms TD-090 fix: when scheduler will
// drive RunDeliveryOnce, the in-engine worker MUST NOT spawn — otherwise
// the same pending row is delivered twice (no atomic claim in
// processPendingDeliveries).
func TestWithoutDeliveryWorker(t *testing.T) {
	store := &fakeStore{}
	e := NewEngine(store, shortPollConfig(), WithoutDeliveryWorker())
	defer e.Stop()

	if !e.skipDeliveryWorker {
		t.Fatalf("expected skipDeliveryWorker=true with WithoutDeliveryWorker option")
	}

	// Wait a few poll intervals — store should never see GetPending.
	time.Sleep(150 * time.Millisecond)
	if got := store.getPendingCalls.Load(); got != 0 {
		t.Fatalf("expected zero GetPending calls when deliveryWorker suppressed; got %d", got)
	}

	// External callers (the scheduler) can still drive delivery.
	e.RunDeliveryOnce()
	if got := store.getPendingCalls.Load(); got != 1 {
		t.Fatalf("RunDeliveryOnce should call GetPending exactly once; got %d", got)
	}
}

// TestWithoutCleanupWorker confirms the cleanup goroutine is suppressed
// when the option is supplied. Cleanup is harmless to double-run, but the
// option exists for symmetry / tests that don't want background loops.
func TestWithoutCleanupWorker(t *testing.T) {
	store := &fakeStore{}
	e := NewEngine(store, shortPollConfig(), WithoutCleanupWorker())
	defer e.Stop()

	if !e.skipCleanupWorker {
		t.Fatalf("expected skipCleanupWorker=true with WithoutCleanupWorker option")
	}

	// External callers can still drive cleanup.
	e.RunCleanupOnce()
	if got := store.cleanupCalls.Load(); got != 1 {
		t.Fatalf("RunCleanupOnce should call CleanupOld exactly once; got %d", got)
	}
}

// TestStop_NoWorkers ensures Stop() is safe even when both workers are
// suppressed (the shutdown channel still exists; close() must be idempotent).
func TestStop_NoWorkers(t *testing.T) {
	store := &fakeStore{}
	e := NewEngine(store, shortPollConfig(),
		WithoutDeliveryWorker(), WithoutCleanupWorker())
	// Stop twice to check sync.Once protects against panic.
	e.Stop()
	e.Stop()
}
