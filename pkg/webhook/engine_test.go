package webhook

import (
	"sync/atomic"
	"testing"
	"time"
)

// fakeStore is a minimal EndpointStore stub for engine tests.
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

func (s *fakeStore) ListActive() ([]Endpoint, error)            { panic("unused") }
func (s *fakeStore) GetEndpoint(id string) (*Endpoint, error)   { panic("unused") }
func (s *fakeStore) CreateEndpoint(ep *Endpoint) error          { panic("unused") }
func (s *fakeStore) UpdateEndpoint(id string, updates map[string]interface{}) error {
	panic("unused")
}
func (s *fakeStore) DeleteEndpoint(id string) error              { panic("unused") }
func (s *fakeStore) ListEndpoints() ([]Endpoint, error)          { panic("unused") }
func (s *fakeStore) CountEndpoints() (int64, error)              { panic("unused") }
func (s *fakeStore) CreateDeliveries(d []Delivery) error         { panic("unused") }
func (s *fakeStore) UpdateResult(id string, r DeliveryResult) error { panic("unused") }
func (s *fakeStore) ListDeliveries(endpointID, status string, limit, offset int) ([]Delivery, int64, error) {
	panic("unused")
}

func TestNewEngine_NoInternalWorkers(t *testing.T) {
	store := &fakeStore{}
	_ = NewEngine(store, DefaultConfig())

	// No internal workers should be running. Wait and verify no calls.
	time.Sleep(100 * time.Millisecond)
	if got := store.getPendingCalls.Load(); got != 0 {
		t.Fatalf("expected no GetPending calls (no internal worker); got %d", got)
	}
	if got := store.cleanupCalls.Load(); got != 0 {
		t.Fatalf("expected no CleanupOld calls (no internal worker); got %d", got)
	}
}

func TestRunDeliveryOnce(t *testing.T) {
	store := &fakeStore{}
	e := NewEngine(store, DefaultConfig())

	e.RunDeliveryOnce()
	if got := store.getPendingCalls.Load(); got != 1 {
		t.Fatalf("RunDeliveryOnce should call GetPending exactly once; got %d", got)
	}
}

func TestRunCleanupOnce(t *testing.T) {
	store := &fakeStore{}
	e := NewEngine(store, DefaultConfig())

	e.RunCleanupOnce()
	if got := store.cleanupCalls.Load(); got != 1 {
		t.Fatalf("RunCleanupOnce should call CleanupOld exactly once; got %d", got)
	}
}

